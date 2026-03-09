// SPDX-License-Identifier: MIT

package scalesets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudbase/garm/client/enterprises"
	"github.com/cloudbase/garm/client/organizations"
	"github.com/cloudbase/garm/client/repositories"
	"github.com/cloudbase/garm/client/scalesets"
	"github.com/cloudbase/garm/params"
	"sigs.k8s.io/controller-runtime/pkg/log"

	garmoperatorv1beta1 "github.com/mercedes-benz/garm-operator/api/v1beta1"
	garmClient "github.com/mercedes-benz/garm-operator/pkg/client"
)

func CreateScaleSet(ctx context.Context, client garmClient.ScaleSetClient, scaleSet *garmoperatorv1beta1.ScaleSet, image *garmoperatorv1beta1.Image, gitHubScopeRef garmoperatorv1beta1.GitHubScope) (params.ScaleSet, error) {
	log := log.FromContext(ctx).
		WithName("CreateScaleSet")
	log.Info("creating scale set", "scaleSet", scaleSet.Name)

	result := params.ScaleSet{}

	id := gitHubScopeRef.GetID()
	scope, err := garmoperatorv1beta1.ToGitHubScopeKind(gitHubScopeRef.GetKind())
	if err != nil {
		return result, err
	}

	extraSpecs := json.RawMessage([]byte{})
	if scaleSet.Spec.ExtraSpecs != "" {
		err := json.Unmarshal([]byte(scaleSet.Spec.ExtraSpecs), &extraSpecs)
		if err != nil {
			return result, err
		}
	}

	scaleSetParams := params.CreateScaleSetParams{
		RunnerPrefix: params.RunnerPrefix{
			Prefix: scaleSet.Spec.RunnerPrefix,
		},
		Name:                   scaleSet.Spec.Name,
		ScaleSetID:             scaleSet.Spec.ScaleSetID,
		DisableUpdate:          scaleSet.Spec.DisableUpdate,
		ProviderName:           scaleSet.Spec.ProviderName,
		MaxRunners:             scaleSet.Spec.MaxRunners,
		MinIdleRunners:         scaleSet.Spec.MinIdleRunners,
		Image:                  image.Spec.Tag,
		Flavor:                 scaleSet.Spec.Flavor,
		OSType:                 scaleSet.Spec.OSType,
		OSArch:                 scaleSet.Spec.OSArch,
		Tags:                   scaleSet.Spec.Tags,
		Enabled:                scaleSet.Spec.Enabled,
		RunnerBootstrapTimeout: scaleSet.Spec.RunnerBootstrapTimeout,
		ExtraSpecs:             extraSpecs,
		EnableShell:            scaleSet.Spec.EnableShell,
		GitHubRunnerGroup:      scaleSet.Spec.GitHubRunnerGroup,
		TemplateID:             scaleSet.Spec.TemplateID,
	}

	switch scope {
	case garmoperatorv1beta1.EnterpriseScope:
		res, err := client.CreateEnterpriseScaleSet(
			enterprises.NewCreateEnterpriseScaleSetParams().
				WithEnterpriseID(id).
				WithBody(scaleSetParams))
		if err != nil {
			return params.ScaleSet{}, err
		}
		result = res.Payload
	case garmoperatorv1beta1.OrganizationScope:
		res, err := client.CreateOrgScaleSet(
			organizations.NewCreateOrgScaleSetParams().
				WithOrgID(id).
				WithBody(scaleSetParams))
		if err != nil {
			return params.ScaleSet{}, err
		}
		result = res.Payload
	case garmoperatorv1beta1.RepositoryScope:
		res, err := client.CreateRepoScaleSet(
			repositories.NewCreateRepoScaleSetParams().
				WithRepoID(id).
				WithBody(scaleSetParams))
		if err != nil {
			return params.ScaleSet{}, err
		}
		result = res.Payload
	default:
		err := fmt.Errorf("no valid scope specified: %s", scope)
		return params.ScaleSet{}, err
	}

	return result, nil
}

func UpdateScaleSet(ctx context.Context, client garmClient.ScaleSetClient, scaleSet *garmoperatorv1beta1.ScaleSet, image *garmoperatorv1beta1.Image) error {
	log := log.FromContext(ctx).
		WithName("UpdateScaleSet")

	log.Info("updating scale set", "scaleSet", scaleSet.Name, "id", scaleSet.Status.ID)

	updateParams := params.UpdateScaleSetParams{
		RunnerPrefix: params.RunnerPrefix{
			Prefix: scaleSet.Spec.RunnerPrefix,
		},
		MaxRunners:             &scaleSet.Spec.MaxRunners,
		MinIdleRunners:         &scaleSet.Spec.MinIdleRunners,
		Flavor:                 scaleSet.Spec.Flavor,
		OSType:                 scaleSet.Spec.OSType,
		OSArch:                 scaleSet.Spec.OSArch,
		Enabled:                &scaleSet.Spec.Enabled,
		RunnerBootstrapTimeout: &scaleSet.Spec.RunnerBootstrapTimeout,
		ExtraSpecs:             json.RawMessage([]byte(scaleSet.Spec.ExtraSpecs)),
		EnableShell:            &scaleSet.Spec.EnableShell,
		GitHubRunnerGroup:      &scaleSet.Spec.GitHubRunnerGroup,
		TemplateID:             scaleSet.Spec.TemplateID,
	}
	if image != nil {
		updateParams.Image = image.Spec.Tag
	}

	_, err := client.UpdateScaleSet(scalesets.NewUpdateScaleSetParams().WithScalesetID(scaleSet.Status.ID).WithBody(updateParams))
	if err != nil {
		return err
	}

	return nil
}

func GarmScaleSetExists(client garmClient.ScaleSetClient, scaleSet *garmoperatorv1beta1.ScaleSet) bool {
	result, err := client.GetScaleSet(scalesets.NewGetScaleSetParams().WithScalesetID(scaleSet.Status.ID))
	if err != nil {
		return false
	}
	return result.Payload.ID != 0
}
