// SPDX-License-Identifier: MIT

package controller

import (
	"context"
	"reflect"
	"testing"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/client/templates"
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
	"github.com/mercedes-benz/garm-operator/pkg/util"
)

func TestTemplateReconciler_reconcileNormal(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name              string
		object            runtime.Object
		runtimeObjects    []runtime.Object
		expectGarmRequest func(m *mock.MockTemplateClientMockRecorder)
		wantErr           bool
		expectedObject    *garmoperatorv1beta1.Template
	}{
		{
			name: "template does not exist in garm - create",
			object: &garmoperatorv1beta1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-template",
					Namespace: "default",
					Finalizers: []string{
						key.TemplateFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.TemplateSpec{
					Description: "my template description",
					OSType:      commonParams.Linux,
					ForgeType:   params.GithubEndpointType,
					DataSecretRef: garmoperatorv1beta1.SecretRef{
						Name: "template-data-secret",
						Key:  "data",
					},
				},
			},
			runtimeObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "template-data-secret",
					},
					Data: map[string][]byte{
						"data": []byte("#!/bin/bash\necho hello"),
					},
				},
			},
			expectedObject: &garmoperatorv1beta1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-template",
					Namespace: "default",
					Finalizers: []string{
						key.TemplateFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.TemplateSpec{
					Description: "my template description",
					OSType:      commonParams.Linux,
					ForgeType:   params.GithubEndpointType,
					DataSecretRef: garmoperatorv1beta1.SecretRef{
						Name: "template-data-secret",
						Key:  "data",
					},
				},
				Status: garmoperatorv1beta1.TemplateStatus{
					ID: 1,
					Conditions: []metav1.Condition{
						{
							Type:               string(conditions.ReadyCondition),
							Reason:             string(conditions.SuccessfulReconcileReason),
							Status:             metav1.ConditionTrue,
							Message:            "",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
						{
							Type:               string(conditions.WebhookSecretReference),
							Reason:             string(conditions.FetchingWebhookSecretRefSuccessReason),
							Status:             metav1.ConditionTrue,
							Message:            "",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					},
				},
			},
			expectGarmRequest: func(m *mock.MockTemplateClientMockRecorder) {
				m.ListTemplates(templates.NewListTemplatesParams()).
					Return(&templates.ListTemplatesOK{
						Payload: []params.Template{},
					}, nil)
				m.CreateTemplate(templates.NewCreateTemplateParams().
					WithBody(params.CreateTemplateParams{
						Name:        "my-template",
						Description: "my template description",
						Data:        []byte("#!/bin/bash\necho hello"),
						OSType:      commonParams.Linux,
						ForgeType:   params.GithubEndpointType,
					})).Return(&templates.CreateTemplateOK{
					Payload: params.Template{
						ID:          1,
						Name:        "my-template",
						Description: "my template description",
						OSType:      commonParams.Linux,
						ForgeType:   params.GithubEndpointType,
					},
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "template exists in garm - update",
			object: &garmoperatorv1beta1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-template",
					Namespace: "default",
					Finalizers: []string{
						key.TemplateFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.TemplateSpec{
					Description: "updated description",
					OSType:      commonParams.Linux,
					ForgeType:   params.GithubEndpointType,
					DataSecretRef: garmoperatorv1beta1.SecretRef{
						Name: "template-data-secret",
						Key:  "data",
					},
				},
				Status: garmoperatorv1beta1.TemplateStatus{
					ID: 42,
				},
			},
			runtimeObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "template-data-secret",
					},
					Data: map[string][]byte{
						"data": []byte("#!/bin/bash\necho updated"),
					},
				},
			},
			expectedObject: &garmoperatorv1beta1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-template",
					Namespace: "default",
					Finalizers: []string{
						key.TemplateFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.TemplateSpec{
					Description: "updated description",
					OSType:      commonParams.Linux,
					ForgeType:   params.GithubEndpointType,
					DataSecretRef: garmoperatorv1beta1.SecretRef{
						Name: "template-data-secret",
						Key:  "data",
					},
				},
				Status: garmoperatorv1beta1.TemplateStatus{
					ID: 42,
					Conditions: []metav1.Condition{
						{
							Type:               string(conditions.ReadyCondition),
							Reason:             string(conditions.SuccessfulReconcileReason),
							Status:             metav1.ConditionTrue,
							Message:            "",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
						{
							Type:               string(conditions.WebhookSecretReference),
							Reason:             string(conditions.FetchingWebhookSecretRefSuccessReason),
							Status:             metav1.ConditionTrue,
							Message:            "",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					},
				},
			},
			expectGarmRequest: func(m *mock.MockTemplateClientMockRecorder) {
				m.ListTemplates(templates.NewListTemplatesParams()).
					Return(&templates.ListTemplatesOK{
						Payload: []params.Template{
							{
								ID:          42,
								Name:        "existing-template",
								Description: "old description",
								OSType:      commonParams.Linux,
								ForgeType:   params.GithubEndpointType,
							},
						},
					}, nil)
				m.UpdateTemplate(templates.NewUpdateTemplateParams().
					WithTemplateID(float64(42)).
					WithBody(params.UpdateTemplateParams{
						Name:        util.StringPtr("existing-template"),
						Description: util.StringPtr("updated description"),
						Data:        []byte("#!/bin/bash\necho updated"),
					})).Return(&templates.UpdateTemplateOK{
					Payload: params.Template{
						ID:          42,
						Name:        "existing-template",
						Description: "updated description",
						OSType:      commonParams.Linux,
						ForgeType:   params.GithubEndpointType,
					},
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "template data secret not found - error",
			object: &garmoperatorv1beta1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-template",
					Namespace: "default",
					Finalizers: []string{
						key.TemplateFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.TemplateSpec{
					Description: "my template",
					OSType:      commonParams.Linux,
					DataSecretRef: garmoperatorv1beta1.SecretRef{
						Name: "nonexistent-secret",
						Key:  "data",
					},
				},
			},
			runtimeObjects: []runtime.Object{},
			expectedObject: &garmoperatorv1beta1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-template",
					Namespace: "default",
					Finalizers: []string{
						key.TemplateFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.TemplateSpec{
					Description: "my template",
					OSType:      commonParams.Linux,
					DataSecretRef: garmoperatorv1beta1.SecretRef{
						Name: "nonexistent-secret",
						Key:  "data",
					},
				},
				Status: garmoperatorv1beta1.TemplateStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(conditions.ReadyCondition),
							Reason:             string(conditions.FetchingWebhookSecretRefFailedReason),
							Status:             metav1.ConditionFalse,
							Message:            "secrets \"nonexistent-secret\" not found",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
						{
							Type:               string(conditions.WebhookSecretReference),
							Reason:             string(conditions.FetchingWebhookSecretRefFailedReason),
							Status:             metav1.ConditionFalse,
							Message:            "secrets \"nonexistent-secret\" not found",
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					},
				},
			},
			expectGarmRequest: func(_ *mock.MockTemplateClientMockRecorder) {},
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
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(runtimeObjects...).WithStatusSubresource(&garmoperatorv1beta1.Template{}).Build()

			reconciler := &TemplateReconciler{
				Client:   client,
				Recorder: record.NewFakeRecorder(3),
			}

			tmpl := tt.object.DeepCopyObject().(*garmoperatorv1beta1.Template)

			mockTemplateClient := mock.NewMockTemplateClient(mockCtrl)
			tt.expectGarmRequest(mockTemplateClient.EXPECT())

			_, err = reconciler.reconcileNormal(context.Background(), mockTemplateClient, tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("TemplateReconciler.reconcileNormal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			tmpl.ObjectMeta.Annotations = nil
			tmpl.ObjectMeta.ResourceVersion = ""

			conditions.NilLastTransitionTime(tt.expectedObject)
			conditions.NilLastTransitionTime(tmpl)

			if !reflect.DeepEqual(tmpl, tt.expectedObject) {
				t.Errorf("TemplateReconciler.reconcileNormal() \ngot = %#v\n want %#v", tmpl, tt.expectedObject)
			}
		})
	}
}

func TestTemplateReconciler_reconcileDelete(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name              string
		object            runtime.Object
		runtimeObjects    []runtime.Object
		expectGarmRequest func(m *mock.MockTemplateClientMockRecorder)
		wantErr           bool
	}{
		{
			name: "delete template with status ID",
			object: &garmoperatorv1beta1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-template",
					Namespace: "default",
					Finalizers: []string{
						key.TemplateFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.TemplateSpec{
					Description: "my template",
					OSType:      commonParams.Linux,
					DataSecretRef: garmoperatorv1beta1.SecretRef{
						Name: "template-data-secret",
						Key:  "data",
					},
				},
				Status: garmoperatorv1beta1.TemplateStatus{
					ID: 42,
				},
			},
			runtimeObjects: []runtime.Object{},
			expectGarmRequest: func(m *mock.MockTemplateClientMockRecorder) {
				m.DeleteTemplate(
					templates.NewDeleteTemplateParams().
						WithTemplateID(float64(42)),
				).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "delete template without status ID - skip garm delete",
			object: &garmoperatorv1beta1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-template",
					Namespace: "default",
					Finalizers: []string{
						key.TemplateFinalizerName,
					},
				},
				Spec: garmoperatorv1beta1.TemplateSpec{
					Description: "my template",
					OSType:      commonParams.Linux,
					DataSecretRef: garmoperatorv1beta1.SecretRef{
						Name: "template-data-secret",
						Key:  "data",
					},
				},
				Status: garmoperatorv1beta1.TemplateStatus{
					ID: 0,
				},
			},
			runtimeObjects:    []runtime.Object{},
			expectGarmRequest: func(_ *mock.MockTemplateClientMockRecorder) {},
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
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(runtimeObjects...).WithStatusSubresource(&garmoperatorv1beta1.Template{}).Build()

			reconciler := &TemplateReconciler{
				Client:   client,
				Recorder: record.NewFakeRecorder(3),
			}

			tmpl := tt.object.DeepCopyObject().(*garmoperatorv1beta1.Template)

			mockTemplateClient := mock.NewMockTemplateClient(mockCtrl)
			tt.expectGarmRequest(mockTemplateClient.EXPECT())

			_, err = reconciler.reconcileDelete(context.Background(), mockTemplateClient, tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("TemplateReconciler.reconcileDelete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if controllerutil.ContainsFinalizer(tmpl, key.TemplateFinalizerName) {
				t.Errorf("TemplateReconciler.reconcileDelete() finalizer still exists")
				return
			}
		})
	}
}
