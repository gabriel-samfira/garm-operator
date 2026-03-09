// SPDX-License-Identifier: MIT
package client

import (
	"github.com/cloudbase/garm/client/credentials"

	"github.com/mercedes-benz/garm-operator/pkg/metrics"
)

type GiteaCredentialsClient interface {
	GetGiteaCredentials(params *credentials.GetGiteaCredentialsParams) (*credentials.GetGiteaCredentialsOK, error)
	ListGiteaCredentials(params *credentials.ListGiteaCredentialsParams) (*credentials.ListGiteaCredentialsOK, error)
	CreateGiteaCredentials(params *credentials.CreateGiteaCredentialsParams) (*credentials.CreateGiteaCredentialsOK, error)
	UpdateGiteaCredentials(params *credentials.UpdateGiteaCredentialsParams) (*credentials.UpdateGiteaCredentialsOK, error)
	DeleteGiteaCredentials(params *credentials.DeleteGiteaCredentialsParams) error
}

type giteaCredentialClient struct {
	GarmClient
}

func (e *giteaCredentialClient) GetGiteaCredentials(params *credentials.GetGiteaCredentialsParams) (*credentials.GetGiteaCredentialsOK, error) {
	return EnsureAuth(func() (*credentials.GetGiteaCredentialsOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_credentials.Get").Inc()
		creds, err := e.GarmAPI().Credentials.GetGiteaCredentials(params, e.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_credentials.Get").Inc()
			return nil, err
		}
		return creds, nil
	})
}

func (e *giteaCredentialClient) ListGiteaCredentials(params *credentials.ListGiteaCredentialsParams) (*credentials.ListGiteaCredentialsOK, error) {
	return EnsureAuth(func() (*credentials.ListGiteaCredentialsOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_credentials.List").Inc()
		creds, err := e.GarmAPI().Credentials.ListGiteaCredentials(params, e.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_credentials.List").Inc()
			return nil, err
		}
		return creds, nil
	})
}

func (e *giteaCredentialClient) CreateGiteaCredentials(params *credentials.CreateGiteaCredentialsParams) (*credentials.CreateGiteaCredentialsOK, error) {
	return EnsureAuth(func() (*credentials.CreateGiteaCredentialsOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_credentials.Create").Inc()
		creds, err := e.GarmAPI().Credentials.CreateGiteaCredentials(params, e.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_credentials.Create").Inc()
			return nil, err
		}
		return creds, nil
	})
}

func (e *giteaCredentialClient) UpdateGiteaCredentials(params *credentials.UpdateGiteaCredentialsParams) (*credentials.UpdateGiteaCredentialsOK, error) {
	return EnsureAuth(func() (*credentials.UpdateGiteaCredentialsOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_credentials.Update").Inc()
		creds, err := e.GarmAPI().Credentials.UpdateGiteaCredentials(params, e.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_credentials.Update").Inc()
			return nil, err
		}
		return creds, nil
	})
}

func (e *giteaCredentialClient) DeleteGiteaCredentials(params *credentials.DeleteGiteaCredentialsParams) error {
	_, err := EnsureAuth(func() (interface{}, error) {
		metrics.TotalGarmCalls.WithLabelValues("gitea_credentials.Delete").Inc()
		if err := e.GarmAPI().Credentials.DeleteGiteaCredentials(params, e.Token()); err != nil {
			metrics.GarmCallErrors.WithLabelValues("gitea_credentials.Delete").Inc()
			return nil, err
		}
		return nil, nil
	})
	return err
}

func NewGiteaCredentialsClient() GiteaCredentialsClient {
	return &giteaCredentialClient{
		Client,
	}
}
