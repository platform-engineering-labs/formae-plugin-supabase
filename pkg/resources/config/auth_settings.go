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

const ResourceTypeAuthSettings = "SUPABASE::Config::AuthSettings"

func init() {
	registry.Register(
		ResourceTypeAuthSettings,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationList,
		},
		func(c *supatransport.Client, cfg *registry.TargetConfig) prov.Provisioner {
			return &AuthSettings{singleton: singleton{
				client:       c,
				pathSuffix:   "/config/auth",
				writeMethod:  "PATCH",
				displayLabel: "auth settings",
				projectScope: cfg.ProjectRef,
			}}
		},
	)
}

// AuthSettings — SUPABASE::Config::AuthSettings.
// Payload schema: UpdateAuthConfigBody.
type AuthSettings struct{ singleton }
