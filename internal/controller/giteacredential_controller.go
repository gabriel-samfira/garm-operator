// SPDX-License-Identifier: MIT

package controller

import (
	"context"
	"fmt"
	"reflect"

	garmcredentials "github.com/cloudbase/garm/client/credentials"
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

// GiteaCredentialReconciler reconciles a GiteaCredential object
type GiteaCredentialReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=giteacredentials,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=giteacredentials/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=giteacredentials/finalizers,verbs=update
//+kubebuilder:rbac:groups="",namespace=xxxxx,resources=secrets,verbs=get;list;watch;

func (r *GiteaCredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, retErr error) {
	log := log.FromContext(ctx)

	credentials := &garmoperatorv1beta1.GiteaCredential{}
	if err := r.Get(ctx, req.NamespacedName, credentials); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("GiteaCredential resource not found.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	initialCredentials := credentials.DeepCopy()

	// Ignore objects that are paused
	if annotations.IsPaused(credentials) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// ensure the finalizer
	if finalizerAdded, err := finalizers.EnsureFinalizer(ctx, r.Client, credentials, key.GiteaCredentialFinalizerName); err != nil || finalizerAdded {
		return ctrl.Result{}, err
	}

	credentialsClient := garmClient.NewGiteaCredentialsClient()

	// Initialize conditions to unknown if not set already
	credentials.InitializeConditions()

	// always update the status
	defer func() {
		if !reflect.DeepEqual(credentials.Status, initialCredentials.Status) {
			if err := r.Status().Update(ctx, credentials); err != nil {
				log.Error(err, "failed to update status")
				res = ctrl.Result{}
				retErr = err
			}
		}
	}()

	// Handle deleted credentials
	if !credentials.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, credentialsClient, credentials)
	}

	return r.reconcileNormal(ctx, credentialsClient, credentials)
}

func (r *GiteaCredentialReconciler) reconcileNormal(ctx context.Context, client garmClient.GiteaCredentialsClient, credentials *garmoperatorv1beta1.GiteaCredential) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.WithValues("credentials", credentials.Name)

	// get credentials in garm db with resource name
	garmGiteaCreds, err := r.getExistingCredentials(client, credentials.Name)
	if err != nil {
		event.Error(r.Recorder, credentials, err.Error())
		conditions.MarkFalse(credentials, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
		return ctrl.Result{}, err
	}

	// fetch endpoint resource
	endpoint, err := r.getEndpointRef(ctx, credentials)
	if err != nil {
		conditions.MarkFalse(credentials, conditions.ReadyCondition, conditions.FetchingGiteaEndpointRefFailedReason, err.Error())
		conditions.MarkFalse(credentials, conditions.GiteaEndpointReference, conditions.FetchingGiteaEndpointRefFailedReason, err.Error())
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(credentials, conditions.GiteaEndpointReference, conditions.FetchingGiteaEndpointRefSuccessReason, "Successfully fetched GiteaEndpoint CR Ref")

	// fetch secret
	giteaSecret, err := secret.FetchRef(ctx, r.Client, &credentials.Spec.SecretRef, credentials.Namespace)
	if err != nil {
		conditions.MarkFalse(credentials, conditions.ReadyCondition, conditions.FetchingWebhookSecretRefFailedReason, err.Error())
		conditions.MarkFalse(credentials, conditions.WebhookSecretReference, conditions.FetchingWebhookSecretRefFailedReason, err.Error())
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(credentials, conditions.WebhookSecretReference, conditions.FetchingWebhookSecretRefSuccessReason, "")

	// if not found, create credentials in garm db
	if reflect.ValueOf(garmGiteaCreds).IsZero() {
		garmGiteaCreds, err = r.createCredentials(ctx, client, credentials, endpoint.Name, giteaSecret)
		if err != nil {
			event.Error(r.Recorder, credentials, err.Error())
			conditions.MarkFalse(credentials, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
			return ctrl.Result{}, err
		}
	}

	// update credentials cr anytime the credentials in garm db changes
	garmGiteaCreds, err = r.updateCredentials(ctx, client, int64(garmGiteaCreds.ID), credentials, giteaSecret)
	if err != nil {
		event.Error(r.Recorder, credentials, err.Error())
		conditions.MarkFalse(credentials, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
		return ctrl.Result{}, err
	}

	// get detailed credentials, as update or list methods on garm client do not join repo, org and enterprise fields
	res, err := client.GetGiteaCredentials(garmcredentials.NewGetGiteaCredentialsParams().WithID(int64(garmGiteaCreds.ID)))
	if err != nil {
		event.Error(r.Recorder, credentials, err.Error())
		conditions.MarkFalse(credentials, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
		return ctrl.Result{}, err
	}
	garmGiteaCreds = res.Payload

	// set and update credentials status
	credentials.Status.ID = int64(garmGiteaCreds.ID)
	credentials.Status.BaseURL = garmGiteaCreds.BaseURL
	credentials.Status.APIBaseURL = garmGiteaCreds.APIBaseURL

	repos, orgs, enterprises := getGiteaRepoOrgEnterpriseNames(garmGiteaCreds)
	credentials.Status.Repositories = repos
	credentials.Status.Organizations = orgs
	credentials.Status.Enterprises = enterprises

	conditions.MarkTrue(credentials, conditions.ReadyCondition, conditions.SuccessfulReconcileReason, "")

	log.Info("reconciling credentials successfully done")
	return ctrl.Result{}, nil
}

func (r *GiteaCredentialReconciler) getExistingCredentials(client garmClient.GiteaCredentialsClient, name string) (params.ForgeCredentials, error) {
	credentials, err := client.ListGiteaCredentials(garmcredentials.NewListGiteaCredentialsParams())
	if err != nil {
		return params.ForgeCredentials{}, err
	}

	for _, creds := range credentials.Payload {
		if creds.Name == name {
			return creds, nil
		}
	}

	return params.ForgeCredentials{}, nil
}

func (r *GiteaCredentialReconciler) createCredentials(ctx context.Context, client garmClient.GiteaCredentialsClient, credentials *garmoperatorv1beta1.GiteaCredential, endpoint, giteaSecret string) (params.ForgeCredentials, error) {
	log := log.FromContext(ctx)
	log.WithValues("credentials", credentials.Name)

	log.Info("GiteaCredential doesn't exist on garm side. Creating new credentials in garm.")
	event.Creating(r.Recorder, credentials, "credentials doesn't exist on garm side")

	req := params.CreateGiteaCredentialsParams{
		Name:        credentials.Name,
		Description: credentials.Spec.Description,
		AuthType:    credentials.Spec.AuthType,
		Endpoint:    endpoint,
	}

	req.PAT.OAuth2Token = giteaSecret

	garmCredentials, err := client.CreateGiteaCredentials(garmcredentials.NewCreateGiteaCredentialsParams().WithBody(req))
	if err != nil {
		log.V(1).Info(fmt.Sprintf("client.CreateGiteaCredentials error: %s", err))
		return params.ForgeCredentials{}, err
	}

	log.V(1).Info(fmt.Sprintf("credentials %s created - return Value %v", credentials.Name, garmCredentials))

	log.Info("creating credentials in garm succeeded")
	event.Info(r.Recorder, credentials, "creating credentials in garm succeeded")

	return garmCredentials.Payload, nil
}

func (r *GiteaCredentialReconciler) updateCredentials(ctx context.Context, client garmClient.GiteaCredentialsClient, credentialsID int64, credentials *garmoperatorv1beta1.GiteaCredential, giteaSecret string) (params.ForgeCredentials, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("update credentials")

	req := params.UpdateGiteaCredentialsParams{
		Name:        util.StringPtr(credentials.Name),
		Description: util.StringPtr(credentials.Spec.Description),
		PAT:         &params.GithubPAT{OAuth2Token: giteaSecret},
	}

	retValue, err := client.UpdateGiteaCredentials(
		garmcredentials.NewUpdateGiteaCredentialsParams().
			WithID(credentialsID).
			WithBody(req))
	if err != nil {
		log.V(1).Info(fmt.Sprintf("client.UpdateGiteaCredentials error: %s", err))
		return params.ForgeCredentials{}, err
	}

	return retValue.Payload, nil
}

func (r *GiteaCredentialReconciler) reconcileDelete(ctx context.Context, client garmClient.GiteaCredentialsClient, credentials *garmoperatorv1beta1.GiteaCredential) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.WithValues("credentials", credentials.Name)

	log.Info("starting credentials deletion")
	event.Deleting(r.Recorder, credentials, "starting credentials deletion")
	conditions.MarkFalse(credentials, conditions.ReadyCondition, conditions.DeletingReason, conditions.DeletingGiteaCredentialsMsg)
	if err := r.Status().Update(ctx, credentials); err != nil {
		return ctrl.Result{}, err
	}

	if err := client.DeleteGiteaCredentials(
		garmcredentials.NewDeleteGiteaCredentialsParams().
			WithID(credentials.Status.ID),
	); err != nil {
		log.V(1).Info(fmt.Sprintf("client.DeleteGiteaCredentials error: %s", err))
		event.Error(r.Recorder, credentials, err.Error())
		conditions.MarkFalse(credentials, conditions.ReadyCondition, conditions.GarmAPIErrorReason, err.Error())
		if err := r.Status().Update(ctx, credentials); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	if controllerutil.ContainsFinalizer(credentials, key.GiteaCredentialFinalizerName) {
		controllerutil.RemoveFinalizer(credentials, key.GiteaCredentialFinalizerName)
		if err := r.Update(ctx, credentials); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("credentials deletion done")

	return ctrl.Result{}, nil
}

func (r *GiteaCredentialReconciler) getEndpointRef(ctx context.Context, credentials *garmoperatorv1beta1.GiteaCredential) (*garmoperatorv1beta1.GiteaEndpoint, error) {
	endpoint := &garmoperatorv1beta1.GiteaEndpoint{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: credentials.Namespace,
		Name:      credentials.Spec.EndpointRef.Name,
	}, endpoint)
	if err != nil {
		return endpoint, err
	}
	return endpoint, nil
}

func (r *GiteaCredentialReconciler) findCredentialsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secretObj, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	var creds garmoperatorv1beta1.GiteaCredentialList
	if err := r.List(ctx, &creds); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, c := range creds.Items {
		if c.Spec.SecretRef.Name == secretObj.Name {
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

func getGiteaRepoOrgEnterpriseNames(creds params.ForgeCredentials) ([]string, []string, []string) {
	var repos, orgs, enterprises []string
	for _, repo := range creds.Repositories {
		repos = append(repos, repo.Name)
	}
	for _, org := range creds.Organizations {
		orgs = append(orgs, org.Name)
	}
	for _, ent := range creds.Enterprises {
		enterprises = append(enterprises, ent.Name)
	}
	return repos, orgs, enterprises
}

// SetupWithManager sets up the controller with the Manager.
func (r *GiteaCredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmoperatorv1beta1.GiteaCredential{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findCredentialsForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}
