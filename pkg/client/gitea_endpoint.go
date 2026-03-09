// SPDX-License-Identifier: MIT
package client

import (
	"github.com/cloudbase/garm/client/endpoints"

	"github.com/mercedes-benz/garm-operator/pkg/metrics"
)

type GiteaEndpointClient interface {
	GetGiteaEndpoint(params *endpoints.GetGiteaEndpointParams) (*endpoints.GetGiteaEndpointOK, error)
	ListGiteaEndpoints(params *endpoints.ListGiteaEndpointsParams) (*endpoints.ListGiteaEndpointsOK, error)
	CreateGiteaEndpoint(params *endpoints.CreateGiteaEndpointParams) (*endpoints.CreateGiteaEndpointOK, error)
	UpdateGiteaEndpoint(params *endpoints.UpdateGiteaEndpointParams) (*endpoints.UpdateGiteaEndpointOK, error)
	DeleteGiteaEndpoint(params *endpoints.DeleteGiteaEndpointParams) error
}

type giteaEndpointClient struct {
	GarmClient
}

func (e *giteaEndpointClient) GetGiteaEndpoint(params *endpoints.GetGiteaEndpointParams) (*endpoints.GetGiteaEndpointOK, error) {
	return EnsureAuth(func() (*endpoints.GetGiteaEndpointOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_endpoints.Get").Inc()
		endpoint, err := e.GarmAPI().Endpoints.GetGiteaEndpoint(params, e.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_endpoints.Get").Inc()
			return nil, err
		}
		return endpoint, nil
	})
}

func (e *giteaEndpointClient) ListGiteaEndpoints(params *endpoints.ListGiteaEndpointsParams) (*endpoints.ListGiteaEndpointsOK, error) {
	return EnsureAuth(func() (*endpoints.ListGiteaEndpointsOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_endpoints.List").Inc()
		endpoints, err := e.GarmAPI().Endpoints.ListGiteaEndpoints(params, e.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_endpoints.List").Inc()
			return nil, err
		}
		return endpoints, nil
	})
}

func (e *giteaEndpointClient) CreateGiteaEndpoint(params *endpoints.CreateGiteaEndpointParams) (*endpoints.CreateGiteaEndpointOK, error) {
	return EnsureAuth(func() (*endpoints.CreateGiteaEndpointOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_endpoints.Create").Inc()
		endpoint, err := e.GarmAPI().Endpoints.CreateGiteaEndpoint(params, e.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_endpoints.Create").Inc()
			return nil, err
		}
		return endpoint, nil
	})
}

func (e *giteaEndpointClient) UpdateGiteaEndpoint(params *endpoints.UpdateGiteaEndpointParams) (*endpoints.UpdateGiteaEndpointOK, error) {
	return EnsureAuth(func() (*endpoints.UpdateGiteaEndpointOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_endpoints.Update").Inc()
		endpoint, err := e.GarmAPI().Endpoints.UpdateGiteaEndpoint(params, e.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_endpoints.Update").Inc()
			return nil, err
		}
		return endpoint, nil
	})
}

func (e *giteaEndpointClient) DeleteGiteaEndpoint(params *endpoints.DeleteGiteaEndpointParams) error {
	_, err := EnsureAuth(func() (interface{}, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_endpoints.Delete").Inc()
		if err := e.GarmAPI().Endpoints.DeleteGiteaEndpoint(params, e.Token()); err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_endpoints.Delete").Inc()
			return nil, err
		}
		return nil, nil
	})
	return err
}

func NewGiteaEndpointClient() GiteaEndpointClient {
	return &giteaEndpointClient{
		Client,
	}
}
