// SPDX-License-Identifier: MIT
package client

import (
	"github.com/cloudbase/garm/client/templates"

	"github.com/mercedes-benz/garm-operator/pkg/metrics"
)

type TemplateClient interface {
	GetTemplate(params *templates.GetTemplateParams) (*templates.GetTemplateOK, error)
	ListTemplates(params *templates.ListTemplatesParams) (*templates.ListTemplatesOK, error)
	CreateTemplate(params *templates.CreateTemplateParams) (*templates.CreateTemplateOK, error)
	UpdateTemplate(params *templates.UpdateTemplateParams) (*templates.UpdateTemplateOK, error)
	DeleteTemplate(params *templates.DeleteTemplateParams) error
}

type templateClient struct {
	GarmClient
}

func (t *templateClient) GetTemplate(params *templates.GetTemplateParams) (*templates.GetTemplateOK, error) {
	return EnsureAuth(func() (*templates.GetTemplateOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("templates.Get").Inc()
		tmpl, err := t.GarmAPI().Templates.GetTemplate(params, t.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("templates.Get").Inc()
			return nil, err
		}
		return tmpl, nil
	})
}

func (t *templateClient) ListTemplates(params *templates.ListTemplatesParams) (*templates.ListTemplatesOK, error) {
	return EnsureAuth(func() (*templates.ListTemplatesOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("templates.List").Inc()
		tmpls, err := t.GarmAPI().Templates.ListTemplates(params, t.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("templates.List").Inc()
			return nil, err
		}
		return tmpls, nil
	})
}

func (t *templateClient) CreateTemplate(params *templates.CreateTemplateParams) (*templates.CreateTemplateOK, error) {
	return EnsureAuth(func() (*templates.CreateTemplateOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("templates.Create").Inc()
		tmpl, err := t.GarmAPI().Templates.CreateTemplate(params, t.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("templates.Create").Inc()
			return nil, err
		}
		return tmpl, nil
	})
}

func (t *templateClient) UpdateTemplate(params *templates.UpdateTemplateParams) (*templates.UpdateTemplateOK, error) {
	return EnsureAuth(func() (*templates.UpdateTemplateOK, error) {
		metrics.TotalGarmCalls.WithLabelValues("templates.Update").Inc()
		tmpl, err := t.GarmAPI().Templates.UpdateTemplate(params, t.Token())
		if err != nil {
			metrics.GarmCallErrors.WithLabelValues("templates.Update").Inc()
			return nil, err
		}
		return tmpl, nil
	})
}

func (t *templateClient) DeleteTemplate(params *templates.DeleteTemplateParams) error {
	_, err := EnsureAuth(func() (interface{}, error) {
		metrics.TotalGarmCalls.WithLabelValues("templates.Delete").Inc()
		if err := t.GarmAPI().Templates.DeleteTemplate(params, t.Token()); err != nil {
			metrics.GarmCallErrors.WithLabelValues("templates.Delete").Inc()
			return nil, err
		}
		return nil, nil
	})
	return err
}

func NewTemplateClient() TemplateClient {
	return &templateClient{
		Client,
	}
}
