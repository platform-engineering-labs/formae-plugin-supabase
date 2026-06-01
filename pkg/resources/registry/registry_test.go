// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

//go:build unit

package registry_test

import (
	"testing"

	// Side-effect imports register all resource types.
	_ "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/auth"
	_ "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/config"
	_ "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/functions"
	_ "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/platform"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
)

func TestAllResourceTypesRegistered(t *testing.T) {
	want := []string{
		"SUPABASE::Platform::Project",
		"SUPABASE::Platform::Branch",
		"SUPABASE::Auth::APIKey",
		"SUPABASE::Functions::EdgeFunction",
		"SUPABASE::Functions::Secret",
		"SUPABASE::Config::AuthSettings",
		"SUPABASE::Config::APISettings",
		"SUPABASE::Config::DatabaseSettings",
		"SUPABASE::Config::NetworkRestriction",
	}
	for _, rt := range want {
		if !registry.Has(rt) {
			t.Errorf("resource type %q not registered", rt)
		}
	}
	got := registry.ResourceTypes()
	if len(got) < len(want) {
		t.Errorf("registered %d types, want >= %d", len(got), len(want))
	}
}
