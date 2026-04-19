// SPDX-License-Identifier: MIT

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/cloudbase/garm/client/scalesets"
	"github.com/cloudbase/garm/params"
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
	scaleSetUtil "github.com/mercedes-benz/garm-operator/pkg/scalesets"
)

// ScaleSetReconciler reconciles a ScaleSet object
type ScaleSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=scalesets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=images,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=scalesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=garm-operator.mercedes-benz.com,namespace=xxxxx,resources=scalesets/finalizers,verbs=update

func (r *ScaleSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, retErr error) {
	log := log.FromContext(ctx)

	scaleSet := &garmoperatorv1beta1.ScaleSet{}
	if err := r.Get(ctx, req.NamespacedName, scaleSet); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "cannot fetch ScaleSet")
		event.Error(r.Recorder, scaleSet, err.Error())
		return ctrl.Result{}, err
	}

	initialScaleSet := scaleSet.DeepCopy()

	// Ignore objects that are paused
	if annotations.IsPaused(scaleSet) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// ensure the finalizer
	if finalizerAdded, err := finalizers.EnsureFinalizer(ctx, r.Client, scaleSet, key.ScaleSetFinalizerName); err != nil || finalizerAdded {
		return ctrl.Result{}, err
	}

	scaleSetClient := garmClient.NewScaleSetClient()

	// Initialize conditions to unknown if not set already
	scaleSet.InitializeConditions()

	// always update the status
	defer func() {
		if !reflect.DeepEqual(scaleSet.Status, initialScaleSet.Status) {
			if err := r.Status().Update(ctx, scaleSet); err != nil {
				log.Error(err, "failed to update status")
				res = ctrl.Result{}
				retErr = err
			}
		}
	}()

	// handle deletion
	if !scaleSet.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, scaleSetClient, scaleSet)
	}

	return r.reconcileNormal(ctx, scaleSetClient, scaleSet)
}

func (r *ScaleSetReconciler) reconcileNormal(ctx context.Context, scaleSetClient garmClient.ScaleSetClient, scaleSet *garmoperatorv1beta1.ScaleSet) (ctrl.Result, error) {
	gitHubScopeRef, err := r.fetchGitHubScopeCRD(ctx, scaleSet)
	if err != nil {
		r.errorLog(ctx, scaleSet, err)
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.ReconcileErrorReason, err.Error())
		conditions.MarkFalse(scaleSet, conditions.ScopeReference, conditions.FetchingScopeRefFailedReason, err.Error())
		return ctrl.Result{}, err
	}

	if gitHubScopeRef.GetID() == "" {
		err := fmt.Errorf("referenced GitHubScopeRef %s/%s not ready yet", scaleSet.Spec.GitHubScopeRef.Kind, scaleSet.Spec.GitHubScopeRef.Name)
		r.errorLog(ctx, scaleSet, err)
		conditions.MarkFalse(scaleSet, conditions.ScopeReference, conditions.ScopeRefNotReadyReason, err.Error())
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.ReconcileErrorReason, err.Error())
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(scaleSet, conditions.ScopeReference, conditions.FetchingScopeRefSuccessReason, fmt.Sprintf("Successfully fetched %s CR Ref", scaleSet.Spec.GitHubScopeRef.Kind))

	// scale set status id has been set and id matches existing garm scale set
	if scaleSet.Status.ID != "" && scaleSetUtil.GarmScaleSetExists(scaleSetClient, scaleSet) {
		return r.reconcileUpdate(ctx, scaleSetClient, scaleSet)
	}

	return r.reconcileCreate(ctx, scaleSetClient, scaleSet, gitHubScopeRef)
}

func (r *ScaleSetReconciler) reconcileCreate(ctx context.Context, scaleSetClient garmClient.ScaleSetClient, scaleSet *garmoperatorv1beta1.ScaleSet, gitHubScopeRef garmoperatorv1beta1.GitHubScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("ScaleSet doesn't exist on garm side. Creating new scale set in garm")

	// get image cr object by name
	image, err := r.getImageCR(ctx, scaleSet)
	if err != nil {
		conditions.MarkFalse(scaleSet, conditions.ImageReference, conditions.FetchingImageRefFailedReason, err.Error())
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.FetchingImageRefFailedReason, err.Error())
		r.errorLog(ctx, scaleSet, err)
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(scaleSet, conditions.ImageReference, conditions.FetchingImageRefSuccessReason, "Successfully fetched Image CR Ref")

	garmScaleSet, err := scaleSetUtil.CreateScaleSet(ctx, scaleSetClient, scaleSet, image, gitHubScopeRef)
	if err != nil {
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.ReconcileErrorReason, err.Error())
		r.errorLog(ctx, scaleSet, fmt.Errorf("failed creating scale set %s: %s", scaleSet.Name, err.Error()))
		return ctrl.Result{}, err
	}

	log.Info("creating scale set in garm succeeded")
	event.Info(r.Recorder, scaleSet, "creating scale set in garm succeeded")

	scaleSet.Status.ID = strconv.FormatUint(uint64(garmScaleSet.ID), 10)
	scaleSet.Status.ScaleSetID = garmScaleSet.ScaleSetID

	conditions.MarkTrue(scaleSet, conditions.ReadyCondition, conditions.SuccessfulReconcileReason, "")

	return ctrl.Result{}, nil
}

func (r *ScaleSetReconciler) reconcileUpdate(ctx context.Context, scaleSetClient garmClient.ScaleSetClient, scaleSet *garmoperatorv1beta1.ScaleSet) (ctrl.Result, error) {
	log := log.FromContext(ctx).
		WithName("reconcileUpdate")
	log.Info("scale set on garm side found", "id", scaleSet.Status.ID, "name", scaleSet.Name)

	image, err := r.getImageCR(ctx, scaleSet)
	if err != nil {
		conditions.MarkFalse(scaleSet, conditions.ImageReference, conditions.FetchingImageRefFailedReason, err.Error())
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.FetchingImageRefFailedReason, err.Error())
		r.errorLog(ctx, scaleSet, err)
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(scaleSet, conditions.ImageReference, conditions.FetchingImageRefSuccessReason, "Successfully fetched Image CR Ref")

	differs, err := r.compareScaleSetSpecs(ctx, scaleSet, image.Spec.Tag, scaleSetClient)
	if err != nil {
		err := fmt.Errorf("error comparing scale set specs: %s", err.Error())
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.ReconcileErrorReason, err.Error())
		r.errorLog(ctx, scaleSet, err)
		return ctrl.Result{}, err
	}

	if !differs {
		log.Info("scale set CR differs from scale set on garm side. Trigger a garm scale set update")

		if err = scaleSetUtil.UpdateScaleSet(ctx, scaleSetClient, scaleSet, image); err != nil {
			log.Error(err, "error updating scale set")
			conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.ReconcileErrorReason, err.Error())
			r.errorLog(ctx, scaleSet, err)
			return ctrl.Result{}, err
		}
	}

	conditions.MarkTrue(scaleSet, conditions.ReadyCondition, conditions.SuccessfulReconcileReason, "")
	return ctrl.Result{}, nil
}

func (r *ScaleSetReconciler) reconcileDelete(ctx context.Context, scaleSetClient garmClient.ScaleSetClient, scaleSet *garmoperatorv1beta1.ScaleSet) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Deleting ScaleSet", "scaleSet", scaleSet.Name)
	event.Deleting(r.Recorder, scaleSet, "")
	conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.DeletingReason, conditions.DeletingScaleSetMsg)
	if err := r.Status().Update(ctx, scaleSet); err != nil {
		return ctrl.Result{}, err
	}

	// scale set does not exist in garm database yet
	if scaleSet.Status.ID == "" && controllerutil.ContainsFinalizer(scaleSet, key.ScaleSetFinalizerName) {
		controllerutil.RemoveFinalizer(scaleSet, key.ScaleSetFinalizerName)
		if err := r.Update(ctx, scaleSet); err != nil {
			conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.ReconcileErrorReason, err.Error())
			r.errorLog(ctx, scaleSet, err)
			return ctrl.Result{}, err
		}

		log.Info("Successfully deleted scale set", "scaleSet", scaleSet.Name)
		return ctrl.Result{}, nil
	}

	// disable scale set before deleting
	disabled := false
	_, err := scaleSetClient.UpdateScaleSet(scalesets.NewUpdateScaleSetParams().
		WithScalesetID(scaleSet.Status.ID).
		WithBody(params.UpdateScaleSetParams{
			Enabled: &disabled,
		}))
	if err != nil {
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.DeletionFailedReason, err.Error())
		r.errorLog(ctx, scaleSet, err)
		return ctrl.Result{}, err
	}

	// delete scale set in garm
	if err := scaleSetClient.DeleteScaleSet(scalesets.NewDeleteScaleSetParams().WithScalesetID(scaleSet.Status.ID)); err != nil {
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.DeletionFailedReason, err.Error())
		r.errorLog(ctx, scaleSet, err)
		return ctrl.Result{}, err
	}

	// remove finalizer so k8s can delete resource
	controllerutil.RemoveFinalizer(scaleSet, key.ScaleSetFinalizerName)
	if err := r.Update(ctx, scaleSet); err != nil {
		conditions.MarkFalse(scaleSet, conditions.ReadyCondition, conditions.DeletionFailedReason, err.Error())
		r.errorLog(ctx, scaleSet, err)
		return ctrl.Result{}, err
	}

	log.Info("Successfully deleted scale set", "scaleSet", scaleSet.Name)
	return ctrl.Result{}, nil
}

func (r *ScaleSetReconciler) compareScaleSetSpecs(ctx context.Context, scaleSet *garmoperatorv1beta1.ScaleSet, imageTag string, scaleSetClient garmClient.ScaleSetClient) (bool, error) {
	log := log.FromContext(ctx).
		WithName("compareScaleSetSpecs")

	gitHubScopeRef, err := r.fetchGitHubScopeCRD(ctx, scaleSet)
	if err != nil {
		log.Error(err, "error fetching GitHubScopeRef")
		return false, err
	}

	garmScaleSet, err := scaleSetClient.GetScaleSet(scalesets.NewGetScaleSetParams().WithScalesetID(scaleSet.Status.ID))
	if err != nil {
		return false, err
	}

	tmpGarmScaleSet := params.ScaleSet{
		RunnerPrefix: params.RunnerPrefix{
			Prefix: scaleSet.Spec.RunnerPrefix,
		},
		Name:                   scaleSet.Spec.Name,
		ScaleSetID:             scaleSet.Status.ScaleSetID,
		DisableUpdate:          scaleSet.Spec.DisableUpdate,
		MaxRunners:             scaleSet.Spec.MaxRunners,
		MinIdleRunners:         scaleSet.Spec.MinIdleRunners,
		Image:                  imageTag,
		Flavor:                 scaleSet.Spec.Flavor,
		OSType:                 scaleSet.Spec.OSType,
		OSArch:                 scaleSet.Spec.OSArch,
		Enabled:                scaleSet.Spec.Enabled,
		RunnerBootstrapTimeout: scaleSet.Spec.RunnerBootstrapTimeout,
		ExtraSpecs:             json.RawMessage([]byte(scaleSet.Spec.ExtraSpecs)),
		EnableShell:            scaleSet.Spec.EnableShell,
		GitHubRunnerGroup:      scaleSet.Spec.GitHubRunnerGroup,
		ProviderName:           scaleSet.Spec.ProviderName,
	}

	if scaleSet.Spec.TemplateID != nil {
		tmpGarmScaleSet.TemplateID = *scaleSet.Spec.TemplateID
	}

	id, err := strconv.ParseUint(scaleSet.Status.ID, 10, 64)
	if err != nil {
		return false, err
	}
	tmpGarmScaleSet.ID = uint(id)

	switch gitHubScopeRef.GetKind() {
	case string(garmoperatorv1beta1.EnterpriseScope):
		tmpGarmScaleSet.EnterpriseID = gitHubScopeRef.GetID()
		tmpGarmScaleSet.EnterpriseName = gitHubScopeRef.GetName()
	case string(garmoperatorv1beta1.OrganizationScope):
		tmpGarmScaleSet.OrgID = gitHubScopeRef.GetID()
		tmpGarmScaleSet.OrgName = gitHubScopeRef.GetName()
	case string(garmoperatorv1beta1.RepositoryScope):
		tmpGarmScaleSet.RepoID = gitHubScopeRef.GetID()
		tmpGarmScaleSet.RepoName = gitHubScopeRef.GetName()
	}

	// Copy server-managed fields from garm response to avoid false DeepEqual positives
	tmpGarmScaleSet.Generation = garmScaleSet.Payload.Generation
	tmpGarmScaleSet.Endpoint = garmScaleSet.Payload.Endpoint
	tmpGarmScaleSet.CreatedAt = garmScaleSet.Payload.CreatedAt
	tmpGarmScaleSet.UpdatedAt = garmScaleSet.Payload.UpdatedAt
	tmpGarmScaleSet.State = garmScaleSet.Payload.State
	tmpGarmScaleSet.ExtendedState = garmScaleSet.Payload.ExtendedState
	tmpGarmScaleSet.DesiredRunnerCount = garmScaleSet.Payload.DesiredRunnerCount
	tmpGarmScaleSet.StatusMessages = garmScaleSet.Payload.StatusMessages
	tmpGarmScaleSet.TemplateName = garmScaleSet.Payload.TemplateName
	tmpGarmScaleSet.LastMessageID = garmScaleSet.Payload.LastMessageID

	// empty instances for comparison
	garmScaleSet.Payload.Instances = nil

	return reflect.DeepEqual(tmpGarmScaleSet, garmScaleSet.Payload), nil
}

func (r *ScaleSetReconciler) errorLog(ctx context.Context, obj client.Object, err error) {
	log := log.FromContext(ctx)

	log.Error(err, "error")
	event.Error(r.Recorder, obj, err.Error())
}

func (r *ScaleSetReconciler) fetchGitHubScopeCRD(ctx context.Context, scaleSet *garmoperatorv1beta1.ScaleSet) (garmoperatorv1beta1.GitHubScope, error) {
	gitHubScopeNamespacedName := types.NamespacedName{
		Namespace: scaleSet.Namespace,
		Name:      scaleSet.Spec.GitHubScopeRef.Name,
	}

	var gitHubScope client.Object

	switch scaleSet.Spec.GitHubScopeRef.Kind {
	case string(garmoperatorv1beta1.EnterpriseScope):
		gitHubScope = &garmoperatorv1beta1.Enterprise{}
		if err := r.Get(ctx, gitHubScopeNamespacedName, gitHubScope); err != nil {
			return nil, err
		}

	case string(garmoperatorv1beta1.OrganizationScope):
		gitHubScope = &garmoperatorv1beta1.Organization{}
		if err := r.Get(ctx, gitHubScopeNamespacedName, gitHubScope); err != nil {
			return nil, err
		}

	case string(garmoperatorv1beta1.RepositoryScope):
		gitHubScope = &garmoperatorv1beta1.Repository{}
		if err := r.Get(ctx, gitHubScopeNamespacedName, gitHubScope); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported GitHubScopeRef kind: %s", scaleSet.Spec.GitHubScopeRef.Kind)
	}

	return gitHubScope.(garmoperatorv1beta1.GitHubScope), nil
}

func (r *ScaleSetReconciler) getImageCR(ctx context.Context, scaleSet *garmoperatorv1beta1.ScaleSet) (*garmoperatorv1beta1.Image, error) {
	image := &garmoperatorv1beta1.Image{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: scaleSet.Namespace,
		Name:      scaleSet.Spec.ImageName,
	}, image); err != nil {
		return nil, err
	}
	return image, nil
}

func (r *ScaleSetReconciler) findScaleSetsForImage(ctx context.Context, obj client.Object) []reconcile.Request {
	image, ok := obj.(*garmoperatorv1beta1.Image)
	if !ok {
		return nil
	}

	var scaleSets garmoperatorv1beta1.ScaleSetList
	if err := r.List(ctx, &scaleSets); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, ss := range scaleSets.Items {
		if ss.Spec.ImageName == image.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: ss.Namespace,
					Name:      ss.Name,
				},
			})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScaleSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmoperatorv1beta1.ScaleSet{}).
		Watches(
			&garmoperatorv1beta1.Image{},
			handler.EnqueueRequestsFromMapFunc(r.findScaleSetsForImage),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}
