// SPDX-License-Identifier: MIT

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/cloudbase/garm/client/endpoints"
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

// GiteaEndpointReconciler reconciles a GiteaEndpoint object
type GiteaEndpointReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=giteaendpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=giteaendpoints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=giteaendpoints/finalizers,verbs=update
// +kubebuilder:rbac:groups="",namespace=xxxxx,resources=secrets,verbs=get;list;watch;

func (r *GiteaEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, retErr error) {
	log := log.FromContext(ctx)

	endpoint := &garmoperatorv1beta1.GiteaEndpoint{}
	if err := r.Get(ctx, req.NamespacedName, endpoint); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("GiteaEndpoint resource not found.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	initialEndpoint := endpoint.DeepCopy()

	// Ignore objects that are paused
	if annotations.IsPaused(endpoint) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// ensure the finalizer
	if finalizerAdded, err := finalizers.EnsureFinalizer(ctx, r.Client, endpoint, key.GiteaEndpointFinalizerName); err != nil || finalizerAdded {
		return ctrl.Result{}, err
	}

	endpointClient := garmClient.NewGiteaEndpointClient()

	// Initialize conditions to unknown if not set already
	endpoint.InitializeConditions()

	// always update the status
	defer func() {
		if !reflect.DeepEqual(endpoint.Status, initialEndpoint.Status) {
			if err := r.Status().Update(ctx, endpoint); err != nil {
				log.Error(err, "failed to update status")
				res = ctrl.Result{}
				retErr = err
			}
		}
	}()

	// Handle deleted endpoints
	if !endpoint.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, endpointClient, endpoint)
	}

	return r.reconcileNormal(ctx, endpointClient, endpoint)
}

func (r *GiteaEndpointReconciler) reconcileNormal(ctx context.Context, client garmClient.GiteaEndpointClient, endpoint *garmoperatorv1beta1.GiteaEndpoint) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.WithValues("endpoint", endpoint.Name)

	// fetch CACertbundle from secret
	caCertBundleSecret, err := r.handleCaCertBundleSecret(ctx, endpoint)
	if err != nil {
		return ctrl.Result{}, err
	}

	// get endpoint in garm db with resource name
	garmEndpoint, err := r.getExistingEndpoint(client, endpoint.Name)
	if err != nil {
		event.Error(r.Recorder, endpoint, err.Error())
		conditions.MarkFalse(endpoint, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
		return ctrl.Result{}, err
	}

	// if not found, create endpoint in garm db
	if reflect.ValueOf(garmEndpoint).IsZero() {
		garmEndpoint, err = r.createEndpoint(ctx, client, endpoint, caCertBundleSecret)
		if err != nil {
			event.Error(r.Recorder, endpoint, err.Error())
			conditions.MarkFalse(endpoint, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
			return ctrl.Result{}, err
		}
		// persist GARM's effective default values as annotations
		r.saveLastKnownGarmDefaults(ctx, endpoint, garmEndpoint)
	}

	// update endpoint only if spec differs from garm state
	if r.endpointNeedsUpdate(endpoint, garmEndpoint, caCertBundleSecret) {
		garmEndpoint, err = r.updateEndpoint(ctx, client, endpoint, caCertBundleSecret)
		if err != nil {
			// If it's a 400 error (validation error like "cannot update endpoint URLs with existing credentials"),
			// use explicit backoff to avoid spamming GARM
			if garmClient.IsBadRequestError(err) {
				event.Error(r.Recorder, endpoint, "Cannot update endpoint - likely credentials still attached")
				conditions.MarkFalse(endpoint, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			event.Error(r.Recorder, endpoint, err.Error())
			conditions.MarkFalse(endpoint, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
			return ctrl.Result{}, err
		}
		// persist GARM's effective values as annotations
		r.saveLastKnownGarmDefaults(ctx, endpoint, garmEndpoint)
	}

	// set and update endpoint status
	conditions.MarkTrue(endpoint, conditions.ReadyCondition, conditions.SuccessfulReconcileReason, "")
	log.Info("reconciling endpoint successfully done", "endpoint", garmEndpoint.Name)

	return ctrl.Result{}, nil
}

func (r *GiteaEndpointReconciler) getExistingEndpoint(client garmClient.GiteaEndpointClient, name string) (params.ForgeEndpoint, error) {
	endpoint, err := client.GetGiteaEndpoint(endpoints.NewGetGiteaEndpointParams().WithName(name))
	if err != nil && garmClient.IsNotFoundError(err) {
		return params.ForgeEndpoint{}, nil
	}

	if err != nil {
		return params.ForgeEndpoint{}, err
	}

	return endpoint.Payload, nil
}

func (r *GiteaEndpointReconciler) createEndpoint(ctx context.Context, client garmClient.GiteaEndpointClient, endpoint *garmoperatorv1beta1.GiteaEndpoint, caCertBundleSecret string) (params.ForgeEndpoint, error) {
	log := log.FromContext(ctx)
	log.WithValues("endpoint", endpoint.Name)

	log.Info("GiteaEndpoint doesn't exist on garm side. Creating new endpoint in garm.")
	event.Creating(r.Recorder, endpoint, "endpoint doesn't exist on garm side")

	retValue, err := client.CreateGiteaEndpoint(endpoints.NewCreateGiteaEndpointParams().WithBody(params.CreateGiteaEndpointParams{
		Name:                     endpoint.Name,
		Description:              endpoint.Spec.Description,
		APIBaseURL:               endpoint.Spec.APIBaseURL,
		BaseURL:                  endpoint.Spec.BaseURL,
		CACertBundle:             []byte(caCertBundleSecret),
		ToolsMetadataURL:         endpoint.Spec.ToolsMetadataURL,
		UseInternalToolsMetadata: endpoint.Spec.UseInternalToolsMetadata,
	}))
	if err != nil {
		log.V(1).Info(fmt.Sprintf("client.CreateGiteaEndpoint error: %s", err))
		return params.ForgeEndpoint{}, err
	}

	log.V(1).Info(fmt.Sprintf("endpoint %s created - return Value %v", endpoint.Name, retValue))

	log.Info("creating endpoint in garm succeeded")
	event.Info(r.Recorder, endpoint, "creating endpoint in garm succeeded")

	return retValue.Payload, nil
}

// saveLastKnownGarmDefaults persists GARM's effective values for fields that have
// server-side defaults (ToolsMetadataURL, UseInternalToolsMetadata) as annotations.
// This prevents reconciliation loops when the spec has zero values but GARM returns defaults.
func (r *GiteaEndpointReconciler) saveLastKnownGarmDefaults(ctx context.Context, endpoint *garmoperatorv1beta1.GiteaEndpoint, garmEndpoint params.ForgeEndpoint) {
	log := log.FromContext(ctx)

	anns := endpoint.GetAnnotations()
	if anns == nil {
		anns = make(map[string]string)
	}

	anns[key.LastToolsMetadataURL] = garmEndpoint.ToolsMetadataURL
	if garmEndpoint.UseInternalToolsMetadata != nil {
		anns[key.LastUseInternalToolsMetadata] = strconv.FormatBool(*garmEndpoint.UseInternalToolsMetadata)
	}
	endpoint.SetAnnotations(anns)

	if err := r.Update(ctx, endpoint); err != nil {
		log.Error(err, "failed to save GARM default annotations")
	}
}

func (r *GiteaEndpointReconciler) endpointNeedsUpdate(endpoint *garmoperatorv1beta1.GiteaEndpoint, garmEndpoint params.ForgeEndpoint, caCertBundleSecret string) bool {
	descDiff := endpoint.Spec.Description != garmEndpoint.Description
	apiDiff := endpoint.Spec.APIBaseURL != garmEndpoint.APIBaseURL
	baseDiff := endpoint.Spec.BaseURL != garmEndpoint.BaseURL
	caDiff := caCertBundleSecret != string(garmEndpoint.CACertBundle)

	// For fields with GARM defaults: if the spec has a zero value, compare GARM's
	// current value against the last-known annotation instead. This avoids a
	// reconciliation loop when GARM returns defaults for unset fields.
	anns := endpoint.GetAnnotations()

	var toolsDiff bool
	if endpoint.Spec.ToolsMetadataURL == "" {
		lastTools := anns[key.LastToolsMetadataURL]
		toolsDiff = lastTools != "" && lastTools != garmEndpoint.ToolsMetadataURL
	} else {
		toolsDiff = endpoint.Spec.ToolsMetadataURL != garmEndpoint.ToolsMetadataURL
	}

	var internalDiff bool
	if endpoint.Spec.UseInternalToolsMetadata == nil {
		lastInternal := anns[key.LastUseInternalToolsMetadata]
		if lastInternal != "" && garmEndpoint.UseInternalToolsMetadata != nil {
			internalDiff = lastInternal != strconv.FormatBool(*garmEndpoint.UseInternalToolsMetadata)
		}
	} else if garmEndpoint.UseInternalToolsMetadata != nil {
		internalDiff = *endpoint.Spec.UseInternalToolsMetadata != *garmEndpoint.UseInternalToolsMetadata
	} else {
		internalDiff = true
	}

	needsUpdate := descDiff || apiDiff || baseDiff || caDiff || toolsDiff || internalDiff

	return needsUpdate
}

func (r *GiteaEndpointReconciler) updateEndpoint(ctx context.Context, client garmClient.GiteaEndpointClient, endpoint *garmoperatorv1beta1.GiteaEndpoint, caCertBundleSecret string) (params.ForgeEndpoint, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("update endpoint")

	retValue, err := client.UpdateGiteaEndpoint(
		endpoints.NewUpdateGiteaEndpointParams().
			WithName(endpoint.Name).
			WithBody(params.UpdateGiteaEndpointParams{
				Description:              util.StringPtr(endpoint.Spec.Description),
				APIBaseURL:               util.StringPtr(endpoint.Spec.APIBaseURL),
				BaseURL:                  util.StringPtr(endpoint.Spec.BaseURL),
				CACertBundle:             []byte(caCertBundleSecret),
				ToolsMetadataURL:         endpoint.Spec.ToolsMetadataURL,
				UseInternalToolsMetadata: endpoint.Spec.UseInternalToolsMetadata,
			}))
	if err != nil {
		log.V(1).Info(fmt.Sprintf("client.UpdateGiteaEndpoint error: %s", err))
		return params.ForgeEndpoint{}, err
	}

	return retValue.Payload, nil
}

func (r *GiteaEndpointReconciler) reconcileDelete(ctx context.Context, client garmClient.GiteaEndpointClient, endpoint *garmoperatorv1beta1.GiteaEndpoint) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.WithValues("endpoint", endpoint.Name)

	log.Info("starting endpoint deletion")
	event.Deleting(r.Recorder, endpoint, "starting endpoint deletion")
	conditions.MarkFalse(endpoint, conditions.ReadyCondition, conditions.DeletingReason, conditions.DeletingGiteaEndpointMsg)

	err := client.DeleteGiteaEndpoint(
		endpoints.NewDeleteGiteaEndpointParams().
			WithName(endpoint.Name),
	)
	if err != nil {
		log.V(1).Info(fmt.Sprintf("client.DeleteGiteaEndpoint error: %s", err))
		event.Error(r.Recorder, endpoint, err.Error())
		conditions.MarkFalse(endpoint, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
		return ctrl.Result{}, err
	}

	if controllerutil.ContainsFinalizer(endpoint, key.GiteaEndpointFinalizerName) {
		controllerutil.RemoveFinalizer(endpoint, key.GiteaEndpointFinalizerName)
		if err := r.Update(ctx, endpoint); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("endpoint deletion done")

	return ctrl.Result{}, nil
}

func (r *GiteaEndpointReconciler) handleCaCertBundleSecret(ctx context.Context, endpoint *garmoperatorv1beta1.GiteaEndpoint) (string, error) {
	// as caCertBundle is optional we exit early if it is not set and do not set any conditions
	if reflect.ValueOf(endpoint.Spec.CACertBundleSecretRef).IsZero() {
		return "", nil
	}

	caCertBundleSecret, err := secret.FetchRef(ctx, r.Client, &endpoint.Spec.CACertBundleSecretRef, endpoint.Namespace)
	if err != nil {
		conditions.MarkFalse(endpoint, conditions.ReadyCondition, conditions.FetchingWebhookSecretRefFailedReason, err.Error())
		conditions.MarkFalse(endpoint, conditions.WebhookSecretReference, conditions.FetchingWebhookSecretRefFailedReason, err.Error())
		return "", err
	}
	conditions.MarkTrue(endpoint, conditions.WebhookSecretReference, conditions.FetchingWebhookSecretRefSuccessReason, "")
	return caCertBundleSecret, nil
}

func (r *GiteaEndpointReconciler) findEndpointsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secretObj, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	var endpointList garmoperatorv1beta1.GiteaEndpointList
	if err := r.List(ctx, &endpointList); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, c := range endpointList.Items {
		if c.Spec.CACertBundleSecretRef.Name == secretObj.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: c.Namespace,
					Name:      c.Name,
				},
			})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *GiteaEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmoperatorv1beta1.GiteaEndpoint{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findEndpointsForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}
