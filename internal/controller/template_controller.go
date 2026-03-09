// SPDX-License-Identifier: MIT

package controller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudbase/garm/client/templates"
	"github.com/cloudbase/garm/params"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmoperatorv1beta1 "github.com/mercedes-benz/garm-operator/api/v1beta1"
	"github.com/mercedes-benz/garm-operator/pkg/annotations"
	garmClient "github.com/mercedes-benz/garm-operator/pkg/client"
	"github.com/mercedes-benz/garm-operator/pkg/client/key"
	"github.com/mercedes-benz/garm-operator/pkg/conditions"
	"github.com/mercedes-benz/garm-operator/pkg/event"
	"github.com/mercedes-benz/garm-operator/pkg/finalizers"
	"github.com/mercedes-benz/garm-operator/pkg/secret"
	"github.com/mercedes-benz/garm-operator/pkg/util"
)

// TemplateReconciler reconciles a Template object
type TemplateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=templates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=templates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=templates/finalizers,verbs=update
// +kubebuilder:rbac:groups="",namespace=xxxxx,resources=secrets,verbs=get;list;watch;

func (r *TemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, retErr error) {
	log := log.FromContext(ctx)

	tmpl := &garmoperatorv1beta1.Template{}
	if err := r.Get(ctx, req.NamespacedName, tmpl); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Template resource not found.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	initialTemplate := tmpl.DeepCopy()

	// Ignore objects that are paused
	if annotations.IsPaused(tmpl) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// ensure the finalizer
	if finalizerAdded, err := finalizers.EnsureFinalizer(ctx, r.Client, tmpl, key.TemplateFinalizerName); err != nil || finalizerAdded {
		return ctrl.Result{}, err
	}

	templateClient := garmClient.NewTemplateClient()

	// Initialize conditions to unknown if not set already
	tmpl.InitializeConditions()

	// always update the status
	defer func() {
		if !reflect.DeepEqual(tmpl.Status, initialTemplate.Status) {
			if err := r.Status().Update(ctx, tmpl); err != nil {
				log.Error(err, "failed to update status")
				res = ctrl.Result{}
				retErr = err
			}
		}
	}()

	// Handle deleted templates
	if !tmpl.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, templateClient, tmpl)
	}

	return r.reconcileNormal(ctx, templateClient, tmpl)
}

func (r *TemplateReconciler) reconcileNormal(ctx context.Context, client garmClient.TemplateClient, tmpl *garmoperatorv1beta1.Template) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.WithValues("template", tmpl.Name)

	// fetch template data from secret
	templateData, err := secret.FetchRef(ctx, r.Client, &tmpl.Spec.DataSecretRef, tmpl.Namespace)
	if err != nil {
		conditions.MarkFalse(tmpl, conditions.ReadyCondition, conditions.FetchingWebhookSecretRefFailedReason, err.Error())
		conditions.MarkFalse(tmpl, conditions.WebhookSecretReference, conditions.FetchingWebhookSecretRefFailedReason, err.Error())
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(tmpl, conditions.WebhookSecretReference, conditions.FetchingWebhookSecretRefSuccessReason, "")

	// get existing template in garm db by name
	garmTemplate, err := r.getExistingTemplate(client, tmpl.Name)
	if err != nil {
		event.Error(r.Recorder, tmpl, err.Error())
		conditions.MarkFalse(tmpl, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
		return ctrl.Result{}, err
	}

	// if not found, create template in garm db
	if garmTemplate.ID == 0 {
		garmTemplate, err = r.createTemplate(ctx, client, tmpl, templateData)
		if err != nil {
			event.Error(r.Recorder, tmpl, err.Error())
			conditions.MarkFalse(tmpl, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
			return ctrl.Result{}, err
		}
	} else {
		// update template
		garmTemplate, err = r.updateTemplate(ctx, client, tmpl, templateData)
		if err != nil {
			event.Error(r.Recorder, tmpl, err.Error())
			conditions.MarkFalse(tmpl, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
			return ctrl.Result{}, err
		}
	}

	// set and update template status
	tmpl.Status.ID = garmTemplate.ID

	conditions.MarkTrue(tmpl, conditions.ReadyCondition, conditions.SuccessfulReconcileReason, "")
	log.Info("reconciling template successfully done", "template", garmTemplate.Name)

	return ctrl.Result{}, nil
}

func (r *TemplateReconciler) getExistingTemplate(client garmClient.TemplateClient, name string) (params.Template, error) {
	result, err := client.ListTemplates(templates.NewListTemplatesParams())
	if err != nil {
		return params.Template{}, err
	}

	for _, tmpl := range result.Payload {
		if tmpl.Name == name {
			return tmpl, nil
		}
	}

	return params.Template{}, nil
}

func (r *TemplateReconciler) createTemplate(ctx context.Context, client garmClient.TemplateClient, tmpl *garmoperatorv1beta1.Template, templateData string) (params.Template, error) {
	log := log.FromContext(ctx)
	log.WithValues("template", tmpl.Name)

	log.Info("Template doesn't exist on garm side. Creating new template in garm.")
	event.Creating(r.Recorder, tmpl, "template doesn't exist on garm side")

	retValue, err := client.CreateTemplate(templates.NewCreateTemplateParams().WithBody(params.CreateTemplateParams{
		Name:        tmpl.Name,
		Description: tmpl.Spec.Description,
		Data:        []byte(templateData),
		OSType:      tmpl.Spec.OSType,
		ForgeType:   tmpl.Spec.ForgeType,
	}))
	if err != nil {
		log.V(1).Info(fmt.Sprintf("client.CreateTemplate error: %s", err))
		return params.Template{}, err
	}

	log.V(1).Info(fmt.Sprintf("template %s created - return Value %v", tmpl.Name, retValue))

	log.Info("creating template in garm succeeded")
	event.Info(r.Recorder, tmpl, "creating template in garm succeeded")

	return retValue.Payload, nil
}

func (r *TemplateReconciler) updateTemplate(ctx context.Context, client garmClient.TemplateClient, tmpl *garmoperatorv1beta1.Template, templateData string) (params.Template, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("update template")

	retValue, err := client.UpdateTemplate(
		templates.NewUpdateTemplateParams().
			WithTemplateID(float64(tmpl.Status.ID)).
			WithBody(params.UpdateTemplateParams{
				Name:        util.StringPtr(tmpl.Name),
				Description: util.StringPtr(tmpl.Spec.Description),
				Data:        []byte(templateData),
			}))
	if err != nil {
		log.V(1).Info(fmt.Sprintf("client.UpdateTemplate error: %s", err))
		return params.Template{}, err
	}

	return retValue.Payload, nil
}

func (r *TemplateReconciler) reconcileDelete(ctx context.Context, client garmClient.TemplateClient, tmpl *garmoperatorv1beta1.Template) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.WithValues("template", tmpl.Name)

	log.Info("starting template deletion")
	event.Deleting(r.Recorder, tmpl, "starting template deletion")
	conditions.MarkFalse(tmpl, conditions.ReadyCondition, conditions.DeletingReason, conditions.DeletingTemplateMsg)

	if tmpl.Status.ID != 0 {
		err := client.DeleteTemplate(
			templates.NewDeleteTemplateParams().
				WithTemplateID(float64(tmpl.Status.ID)),
		)
		if err != nil {
			log.V(1).Info(fmt.Sprintf("client.DeleteTemplate error: %s", err))
			event.Error(r.Recorder, tmpl, err.Error())
			conditions.MarkFalse(tmpl, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
			return ctrl.Result{}, err
		}
	}

	if controllerutil.ContainsFinalizer(tmpl, key.TemplateFinalizerName) {
		controllerutil.RemoveFinalizer(tmpl, key.TemplateFinalizerName)
		if err := r.Update(ctx, tmpl); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("template deletion done")

	return ctrl.Result{}, nil
}

func (r *TemplateReconciler) findTemplatesForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secretObj, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	var templateList garmoperatorv1beta1.TemplateList
	if err := r.List(ctx, &templateList); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, t := range templateList.Items {
		if t.Spec.DataSecretRef.Name == secretObj.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: t.Namespace,
					Name:      t.Name,
				},
			})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *TemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmoperatorv1beta1.Template{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findTemplatesForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}
