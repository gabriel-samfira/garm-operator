// SPDX-License-Identifier: MIT

package controller

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/client/enterprises"
	"github.com/cloudbase/garm/client/scalesets"
	"github.com/cloudbase/garm/params"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	garmoperatorv1beta1 "github.com/mercedes-benz/garm-operator/api/v1beta1"
	"github.com/mercedes-benz/garm-operator/pkg/client/key"
	"github.com/mercedes-benz/garm-operator/pkg/client/mock"
	"github.com/mercedes-benz/garm-operator/pkg/conditions"
)

func TestScaleSetReconciler_reconcileNormal(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	enterpriseID := "93068607-2d0d-4b76-a950-0e40d31955b8"
	enterpriseName := "test-enterprise"

	tests := []struct {
		name              string
		object            runtime.Object
		runtimeObjects    []runtime.Object
		expectGarmRequest func(m *mock.MockScaleSetClientMockRecorder)
		wantErr           bool
		expectedObject    *garmoperatorv1beta1.ScaleSet
	}{
		{
			name: "scaleset does not exist in garm - create",
			object: &garmoperatorv1beta1.ScaleSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ScaleSet",
					APIVersion: garmoperatorv1beta1.GroupVersion.Group + "/" + garmoperatorv1beta1.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-scaleset",
					Namespace: namespaceName,
					Finalizers: []string{
						key.ScaleSetFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.ScaleSetSpec{
					GitHubScopeRef: corev1.TypedLocalObjectReference{
						APIGroup: &garmoperatorv1beta1.GroupVersion.Group,
						Kind:     string(garmoperatorv1beta1.EnterpriseScope),
						Name:     enterpriseName,
					},
					Name:           "my-scaleset",
					ProviderName:   "kubernetes_external",
					MaxRunners:     5,
					MinIdleRunners: 2,
					ImageName:      "ubuntu-image",
					Flavor:         "medium",
					OSType:         commonParams.Linux,
					OSArch:         commonParams.Amd64,

					Enabled:                true,
					RunnerBootstrapTimeout: 20,
				},
			},
			expectedObject: &garmoperatorv1beta1.ScaleSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ScaleSet",
					APIVersion: garmoperatorv1beta1.GroupVersion.Group + "/" + garmoperatorv1beta1.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-scaleset",
					Namespace: namespaceName,
					Finalizers: []string{
						key.ScaleSetFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.ScaleSetSpec{
					GitHubScopeRef: corev1.TypedLocalObjectReference{
						APIGroup: &garmoperatorv1beta1.GroupVersion.Group,
						Kind:     string(garmoperatorv1beta1.EnterpriseScope),
						Name:     enterpriseName,
					},
					Name:           "my-scaleset",
					ProviderName:   "kubernetes_external",
					MaxRunners:     5,
					MinIdleRunners: 2,
					ImageName:      "ubuntu-image",
					Flavor:         "medium",
					OSType:         commonParams.Linux,
					OSArch:         commonParams.Amd64,

					Enabled:                true,
					RunnerBootstrapTimeout: 20,
				},
				Status: garmoperatorv1beta1.ScaleSetStatus{
					ID: "1",
					Conditions: []metav1.Condition{
						{
							Type:               string(conditions.ReadyCondition),
							Status:             metav1.ConditionTrue,
							Reason:             string(conditions.SuccessfulReconcileReason),
							Message:            "",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
						{
							Type:               string(conditions.ImageReference),
							Status:             metav1.ConditionTrue,
							Message:            "Successfully fetched Image CR Ref",
							Reason:             string(conditions.FetchingImageRefSuccessReason),
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
						{
							Type:               string(conditions.ScopeReference),
							Status:             metav1.ConditionTrue,
							Message:            "Successfully fetched Enterprise CR Ref",
							Reason:             string(conditions.FetchingScopeRefSuccessReason),
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					},
				},
			},
			runtimeObjects: []runtime.Object{
				&garmoperatorv1beta1.Image{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ubuntu-image",
						Namespace: namespaceName,
					},
					Spec: garmoperatorv1beta1.ImageSpec{
						Tag: "linux-ubuntu-22.04-amd64",
					},
				},
				&garmoperatorv1beta1.Enterprise{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Enterprise",
						APIVersion: garmoperatorv1beta1.GroupVersion.Group + "/" + garmoperatorv1beta1.GroupVersion.Version,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      enterpriseName,
						Namespace: namespaceName,
					},
					Spec: garmoperatorv1beta1.EnterpriseSpec{
						CredentialsRef: corev1.TypedLocalObjectReference{
							APIGroup: &garmoperatorv1beta1.GroupVersion.Group,
							Kind:     "GitHubCredential",
							Name:     "github-creds",
						},
						WebhookSecretRef: garmoperatorv1beta1.SecretRef{
							Name: "my-webhook-secret",
							Key:  "webhookSecret",
						},
					},
					Status: garmoperatorv1beta1.EnterpriseStatus{
						ID: enterpriseID,
						Conditions: []metav1.Condition{
							{
								Type:               string(conditions.ReadyCondition),
								Reason:             string(conditions.SuccessfulReconcileReason),
								Status:             metav1.ConditionTrue,
								Message:            "",
								LastTransitionTime: metav1.NewTime(time.Now()),
							},
						},
					},
				},
			},
			expectGarmRequest: func(m *mock.MockScaleSetClientMockRecorder) {
				extraSpecs := json.RawMessage([]byte{})
				m.CreateEnterpriseScaleSet(
					enterprises.NewCreateEnterpriseScaleSetParams().
						WithEnterpriseID(enterpriseID).
						WithBody(params.CreateScaleSetParams{
							RunnerPrefix: params.RunnerPrefix{
								Prefix: "",
							},
							Name:           "my-scaleset",
							ProviderName:   "kubernetes_external",
							MaxRunners:     5,
							MinIdleRunners: 2,
							Image:          "linux-ubuntu-22.04-amd64",
							Flavor:         "medium",
							OSType:         commonParams.Linux,
							OSArch:         commonParams.Amd64,

							Enabled:                true,
							RunnerBootstrapTimeout: 20,
							ExtraSpecs:             extraSpecs,
						}),
				).Return(&enterprises.CreateEnterpriseScaleSetOK{
					Payload: params.ScaleSet{
						ID:             1,
						Name:           "my-scaleset",
						ProviderName:   "kubernetes_external",
						MaxRunners:     5,
						MinIdleRunners: 2,
						Image:          "linux-ubuntu-22.04-amd64",
						Flavor:         "medium",
						OSType:         commonParams.Linux,
						OSArch:         commonParams.Amd64,
						Enabled:        true,
						EnterpriseID:   enterpriseID,
						EnterpriseName: enterpriseName,
					},
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "scaleset create - missing image CR",
			object: &garmoperatorv1beta1.ScaleSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ScaleSet",
					APIVersion: garmoperatorv1beta1.GroupVersion.Group + "/" + garmoperatorv1beta1.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-scaleset",
					Namespace: namespaceName,
					Finalizers: []string{
						key.ScaleSetFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.ScaleSetSpec{
					GitHubScopeRef: corev1.TypedLocalObjectReference{
						APIGroup: &garmoperatorv1beta1.GroupVersion.Group,
						Kind:     string(garmoperatorv1beta1.EnterpriseScope),
						Name:     enterpriseName,
					},
					Name:           "my-scaleset",
					ProviderName:   "kubernetes_external",
					MaxRunners:     5,
					MinIdleRunners: 2,
					ImageName:      "nonexistent-image",
					Flavor:         "medium",
					OSType:         commonParams.Linux,
					OSArch:         commonParams.Amd64,
					Enabled:        true,
				},
			},
			expectedObject: &garmoperatorv1beta1.ScaleSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ScaleSet",
					APIVersion: garmoperatorv1beta1.GroupVersion.Group + "/" + garmoperatorv1beta1.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-scaleset",
					Namespace: namespaceName,
					Finalizers: []string{
						key.ScaleSetFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.ScaleSetSpec{
					GitHubScopeRef: corev1.TypedLocalObjectReference{
						APIGroup: &garmoperatorv1beta1.GroupVersion.Group,
						Kind:     string(garmoperatorv1beta1.EnterpriseScope),
						Name:     enterpriseName,
					},
					Name:           "my-scaleset",
					ProviderName:   "kubernetes_external",
					MaxRunners:     5,
					MinIdleRunners: 2,
					ImageName:      "nonexistent-image",
					Flavor:         "medium",
					OSType:         commonParams.Linux,
					OSArch:         commonParams.Amd64,
					Enabled:        true,
				},
				Status: garmoperatorv1beta1.ScaleSetStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(conditions.ReadyCondition),
							Reason:             string(conditions.FetchingImageRefFailedReason),
							Status:             metav1.ConditionFalse,
							Message:            "images.garm-operator.mercedes-benz.com \"nonexistent-image\" not found",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
						{
							Type:               string(conditions.ImageReference),
							Reason:             string(conditions.FetchingImageRefFailedReason),
							Status:             metav1.ConditionFalse,
							Message:            "images.garm-operator.mercedes-benz.com \"nonexistent-image\" not found",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
						{
							Type:               string(conditions.ScopeReference),
							Status:             metav1.ConditionTrue,
							Message:            "Successfully fetched Enterprise CR Ref",
							Reason:             string(conditions.FetchingScopeRefSuccessReason),
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					},
				},
			},
			runtimeObjects: []runtime.Object{
				&garmoperatorv1beta1.Enterprise{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Enterprise",
						APIVersion: garmoperatorv1beta1.GroupVersion.Group + "/" + garmoperatorv1beta1.GroupVersion.Version,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      enterpriseName,
						Namespace: namespaceName,
					},
					Spec: garmoperatorv1beta1.EnterpriseSpec{
						CredentialsRef: corev1.TypedLocalObjectReference{
							APIGroup: &garmoperatorv1beta1.GroupVersion.Group,
							Kind:     "GitHubCredential",
							Name:     "github-creds",
						},
						WebhookSecretRef: garmoperatorv1beta1.SecretRef{
							Name: "my-webhook-secret",
							Key:  "webhookSecret",
						},
					},
					Status: garmoperatorv1beta1.EnterpriseStatus{
						ID: enterpriseID,
						Conditions: []metav1.Condition{
							{
								Type:               string(conditions.ReadyCondition),
								Reason:             string(conditions.SuccessfulReconcileReason),
								Status:             metav1.ConditionTrue,
								Message:            "",
								LastTransitionTime: metav1.NewTime(time.Now()),
							},
						},
					},
				},
			},
			expectGarmRequest: func(_ *mock.MockScaleSetClientMockRecorder) {},
			wantErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemeBuilder := runtime.SchemeBuilder{
				garmoperatorv1beta1.AddToScheme,
			}

			err := schemeBuilder.AddToScheme(scheme.Scheme)
			if err != nil {
				t.Fatal(err)
			}
			runtimeObjects := []runtime.Object{tt.object}
			runtimeObjects = append(runtimeObjects, tt.runtimeObjects...)
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(runtimeObjects...).WithStatusSubresource(&garmoperatorv1beta1.ScaleSet{}).Build()

			reconciler := &ScaleSetReconciler{
				Client:   client,
				Recorder: record.NewFakeRecorder(3),
			}

			scaleSet := tt.object.DeepCopyObject().(*garmoperatorv1beta1.ScaleSet)

			mockScaleSetClient := mock.NewMockScaleSetClient(mockCtrl)
			tt.expectGarmRequest(mockScaleSetClient.EXPECT())

			_, err = reconciler.reconcileNormal(context.Background(), mockScaleSetClient, scaleSet)
			if (err != nil) != tt.wantErr {
				t.Errorf("ScaleSetReconciler.reconcileNormal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			scaleSet.ObjectMeta.Annotations = nil
			scaleSet.ObjectMeta.ResourceVersion = ""

			conditions.NilLastTransitionTime(tt.expectedObject)
			conditions.NilLastTransitionTime(scaleSet)

			if !reflect.DeepEqual(scaleSet, tt.expectedObject) {
				t.Errorf("ScaleSetReconciler.reconcileNormal() \ngot = %#v\n want %#v", scaleSet, tt.expectedObject)
			}
		})
	}
}

func TestScaleSetReconciler_reconcileDelete(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name              string
		object            runtime.Object
		runtimeObjects    []runtime.Object
		expectGarmRequest func(m *mock.MockScaleSetClientMockRecorder)
		wantErr           bool
	}{
		{
			name: "delete scaleset with status ID",
			object: &garmoperatorv1beta1.ScaleSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ScaleSet",
					APIVersion: garmoperatorv1beta1.GroupVersion.Group + "/" + garmoperatorv1beta1.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-scaleset",
					Namespace: namespaceName,
					Finalizers: []string{
						key.ScaleSetFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.ScaleSetSpec{
					GitHubScopeRef: corev1.TypedLocalObjectReference{
						APIGroup: &garmoperatorv1beta1.GroupVersion.Group,
						Kind:     string(garmoperatorv1beta1.EnterpriseScope),
						Name:     "test-enterprise",
					},
					Name:         "my-scaleset",
					ProviderName: "kubernetes_external",
					MaxRunners:   5,
					Enabled:      true,
				},
				Status: garmoperatorv1beta1.ScaleSetStatus{
					ID: "42",
				},
			},
			runtimeObjects: []runtime.Object{},
			expectGarmRequest: func(m *mock.MockScaleSetClientMockRecorder) {
				disabled := false
				m.UpdateScaleSet(
					scalesets.NewUpdateScaleSetParams().
						WithScalesetID("42").
						WithBody(params.UpdateScaleSetParams{
							Enabled: &disabled,
						}),
				).Return(&scalesets.UpdateScaleSetOK{}, nil)
				m.DeleteScaleSet(
					scalesets.NewDeleteScaleSetParams().
						WithScalesetID("42"),
				).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "delete scaleset without status ID - skip garm delete",
			object: &garmoperatorv1beta1.ScaleSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ScaleSet",
					APIVersion: garmoperatorv1beta1.GroupVersion.Group + "/" + garmoperatorv1beta1.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-scaleset",
					Namespace: namespaceName,
					Finalizers: []string{
						key.ScaleSetFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.ScaleSetSpec{
					GitHubScopeRef: corev1.TypedLocalObjectReference{
						APIGroup: &garmoperatorv1beta1.GroupVersion.Group,
						Kind:     string(garmoperatorv1beta1.EnterpriseScope),
						Name:     "test-enterprise",
					},
					Name:         "my-scaleset",
					ProviderName: "kubernetes_external",
					MaxRunners:   5,
					Enabled:      true,
				},
				Status: garmoperatorv1beta1.ScaleSetStatus{
					ID: "",
				},
			},
			runtimeObjects:    []runtime.Object{},
			expectGarmRequest: func(_ *mock.MockScaleSetClientMockRecorder) {},
			wantErr:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemeBuilder := runtime.SchemeBuilder{
				garmoperatorv1beta1.AddToScheme,
			}

			err := schemeBuilder.AddToScheme(scheme.Scheme)
			if err != nil {
				t.Fatal(err)
			}

			runtimeObjects := []runtime.Object{tt.object}
			runtimeObjects = append(runtimeObjects, tt.runtimeObjects...)
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(runtimeObjects...).WithStatusSubresource(&garmoperatorv1beta1.ScaleSet{}).Build()

			reconciler := &ScaleSetReconciler{
				Client:   client,
				Recorder: record.NewFakeRecorder(3),
			}

			scaleSet := tt.object.DeepCopyObject().(*garmoperatorv1beta1.ScaleSet)

			mockScaleSetClient := mock.NewMockScaleSetClient(mockCtrl)
			tt.expectGarmRequest(mockScaleSetClient.EXPECT())

			_, err = reconciler.reconcileDelete(context.Background(), mockScaleSetClient, scaleSet)
			if (err != nil) != tt.wantErr {
				t.Errorf("ScaleSetReconciler.reconcileDelete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if controllerutil.ContainsFinalizer(scaleSet, key.ScaleSetFinalizerName) {
				t.Errorf("ScaleSetReconciler.reconcileDelete() finalizer still exists")
				return
			}
		})
	}
}
