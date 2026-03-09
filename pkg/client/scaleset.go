// SPDX-License-Identifier: MIT
package client

import (
	"github.com/cloudbase/garm/client/enterprises"
	"github.com/cloudbase/garm/client/organizations"
	"github.com/cloudbase/garm/client/repositories"
	"github.com/cloudbase/garm/client/scalesets"

	"github.com/mercedes-benz/garm-operator/pkg/metrics"
)

type ScaleSetClient interface {
	ListScaleSets(param *scalesets.ListScalesetsParams) (*scalesets.ListScalesetsOK, error)
	GetScaleSet(param *scalesets.GetScaleSetParams) (*scalesets.GetScaleSetOK, error)
	UpdateScaleSet(param *scalesets.UpdateScaleSetParams) (*scalesets.UpdateScaleSetOK, error)
	DeleteScaleSet(param *scalesets.DeleteScaleSetParams) error
	CreateEnterpriseScaleSet(param *enterprises.CreateEnterpriseScaleSetParams) (*enterprises.CreateEnterpriseScaleSetOK, error)
	CreateOrgScaleSet(param *organizations.CreateOrgScaleSetParams) (*organizations.CreateOrgScaleSetOK, error)
	CreateRepoScaleSet(param *repositories.CreateRepoScaleSetParams) (*repositories.CreateRepoScaleSetOK, error)
}

type scaleSetClient struct {
	GarmClient
}

func NewScaleSetClient() ScaleSetClient {
	return &scaleSetClient{
		Client,
	}
}

func (s *scaleSetClient) ListScaleSets(param *scalesets.ListScalesetsParams) (*scalesets.ListScalesetsOK, error) {
	return EnsureAuth(func() (*scalesets.ListScalesetsOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("scalesets.List").Inc()
		result, err := s.GarmAPI().Scalesets.ListScalesets(param, s.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("scalesets.List").Inc()
			return nil, err
		}
		return result, nil
	})
}

func (s *scaleSetClient) GetScaleSet(param *scalesets.GetScaleSetParams) (*scalesets.GetScaleSetOK, error) {
	return EnsureAuth(func() (*scalesets.GetScaleSetOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("scalesets.Get").Inc()
		result, err := s.GarmAPI().Scalesets.GetScaleSet(param, s.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("scalesets.Get").Inc()
			return nil, err
		}
		return result, nil
	})
}

func (s *scaleSetClient) UpdateScaleSet(param *scalesets.UpdateScaleSetParams) (*scalesets.UpdateScaleSetOK, error) {
	return EnsureAuth(func() (*scalesets.UpdateScaleSetOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("scalesets.Update").Inc()
		result, err := s.GarmAPI().Scalesets.UpdateScaleSet(param, s.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("scalesets.Update").Inc()
			return nil, err
		}
		return result, nil
	})
}

func (s *scaleSetClient) DeleteScaleSet(param *scalesets.DeleteScaleSetParams) error {
	_, err := EnsureAuth(func() (interface{}, error) {
		metrics.TotalGarmCalls.WithLabelValues("scalesets.Delete").Inc()
		err := s.GarmAPI().Scalesets.DeleteScaleSet(param, s.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("scalesets.Delete").Inc()
			return nil, err
		}
		return nil, nil
	})
	return err
}

func (s *scaleSetClient) CreateEnterpriseScaleSet(param *enterprises.CreateEnterpriseScaleSetParams) (*enterprises.CreateEnterpriseScaleSetOK, error) {
	return EnsureAuth(func() (*enterprises.CreateEnterpriseScaleSetOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("scalesets.CreateEnterprise").Inc()
		result, err := s.GarmAPI().Enterprises.CreateEnterpriseScaleSet(param, s.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("scalesets.CreateEnterprise").Inc()
			return nil, err
		}
		return result, nil
	})
}

func (s *scaleSetClient) CreateOrgScaleSet(param *organizations.CreateOrgScaleSetParams) (*organizations.CreateOrgScaleSetOK, error) {
	return EnsureAuth(func() (*organizations.CreateOrgScaleSetOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("scalesets.CreateOrg").Inc()
		result, err := s.GarmAPI().Organizations.CreateOrgScaleSet(param, s.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("scalesets.CreateOrg").Inc()
			return nil, err
		}
		return result, nil
	})
}

func (s *scaleSetClient) CreateRepoScaleSet(param *repositories.CreateRepoScaleSetParams) (*repositories.CreateRepoScaleSetOK, error) {
	return EnsureAuth(func() (*repositories.CreateRepoScaleSetOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("scalesets.CreateRepo").Inc()
		result, err := s.GarmAPI().Repositories.CreateRepoScaleSet(param, s.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("scalesets.CreateRepo").Inc()
			return nil, err
		}
		return result, nil
	})
}
