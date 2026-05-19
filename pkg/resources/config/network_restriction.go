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

const ResourceTypeNetworkRestriction = "SUPABASE::Config::NetworkRestriction"

func init() {
	registry.Register(
		ResourceTypeNetworkRestriction,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationList,
		},
		func(c *supatransport.Client, _ *registry.TargetConfig) prov.Provisioner {
			return &NetworkRestriction{singleton: singleton{
				client:       c,
				pathSuffix:   "/network-restrictions",
				writeMethod:  "PATCH",
				displayLabel: "network restrictions",
			}}
		},
	)
}

// NetworkRestriction — SUPABASE::Config::NetworkRestriction.
// Payload shape: { "dbAllowedCidrs": [...], "dbAllowedCidrsV6": [...] }.
type NetworkRestriction struct{ singleton }
