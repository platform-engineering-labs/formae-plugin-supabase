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

const ResourceTypeDatabaseSettings = "SUPABASE::Config::DatabaseSettings"

func init() {
	registry.Register(
		ResourceTypeDatabaseSettings,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationList,
		},
		func(c *supatransport.Client, _ *registry.TargetConfig) prov.Provisioner {
			return &DatabaseSettings{singleton: singleton{
				client:       c,
				pathSuffix:   "/config/database/postgres",
				writeMethod:  "PUT",
				displayLabel: "database settings",
			}}
		},
	)
}

// DatabaseSettings — SUPABASE::Config::DatabaseSettings.
// Postgres tuning knobs (max_connections, shared_buffers, ...).
// Uses PUT, not PATCH.
type DatabaseSettings struct{ singleton }
