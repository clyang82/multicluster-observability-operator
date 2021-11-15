// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package templates

import (
	"path"

	"sigs.k8s.io/kustomize/v3/pkg/resource"

	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/rendering/templates"
)

var endpointObservabilityTemplates, prometheusTemplates []*resource.Resource

// GetEndpointObservabilityTemplates reads endpoint-observability manifest
func GetOrLoadEndpointObservabilityTemplates(r *templates.TemplateRenderer) ([]*resource.Resource, error) {
	if len(endpointObservabilityTemplates) > 0 {
		return endpointObservabilityTemplates, nil
	}

	basePath := path.Join(r.GetTemplatesPath(), "endpoint-observability")

	// add endpoint ovservability template
	if err := r.AddTemplateFromPath(basePath, &endpointObservabilityTemplates); err != nil {
		return endpointObservabilityTemplates, err
	}

	return endpointObservabilityTemplates, nil
}

// GetOrLoadPrometheusTemplates reads endpoint-observability manifest
func GetOrLoadPrometheusTemplates(r *templates.TemplateRenderer) ([]*resource.Resource, error) {
	if len(prometheusTemplates) > 0 {
		return prometheusTemplates, nil
	}

	basePath := path.Join(r.GetTemplatesPath(), "prometheus")

	// add endpoint ovservability template
	if err := r.AddTemplateFromPath(basePath, &prometheusTemplates); err != nil {
		return prometheusTemplates, err
	}

	return prometheusTemplates, nil
}
