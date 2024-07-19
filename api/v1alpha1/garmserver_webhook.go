// SPDX-License-Identifier: MIT

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var garmserverlog = logf.Log.WithName("garmserver-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *GarmServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-garm-operator-mercedes-benz-com-v1alpha1-garmserver,mutating=true,failurePolicy=fail,sideEffects=None,groups=garm-operator.mercedes-benz.com,resources=garmservers,verbs=create;update,versions=v1alpha1,name=mgarmserver.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &GarmServer{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *GarmServer) Default() {
	garmserverlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-garm-operator-mercedes-benz-com-v1alpha1-garmserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=garm-operator.mercedes-benz.com,resources=garmservers,verbs=create;update,versions=v1alpha1,name=vgarmserver.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GarmServer{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GarmServer) ValidateCreate() (admission.Warnings, error) {
	garmserverlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GarmServer) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	garmserverlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GarmServer) ValidateDelete() (admission.Warnings, error) {
	garmserverlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
