// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypeAPISettings = "SUPABASE::Config::APISettings"

func init() {
	registry.Register(
		ResourceTypeAPISettings,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationList,
		},
		func(c *supatransport.Client, cfg *registry.TargetConfig) prov.Provisioner {
			return &APISettings{singleton: singleton{
				client:       c,
				pathSuffix:   "/postgrest",
				writeMethod:  "PATCH",
				displayLabel: "API settings",
				projectScope: cfg.ProjectRef,
			}}
		},
	)
}

// APISettings — SUPABASE::Config::APISettings.
// Maps to PostgREST configuration (schemas, max rows, ...).
type APISettings struct{ singleton }
