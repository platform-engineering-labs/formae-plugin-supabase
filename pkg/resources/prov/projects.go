// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package prov

import (
	"context"

	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
)

// ProjectIDs returns the project refs the plugin should iterate over for
// discovery-style List calls. If `scoped` is non-empty, only that one ref
// is returned (matches the `ProjectRef` field in the target config) —
// avoids walking every project the PAT can see. Otherwise the client
// fetches `/v1/projects` and returns every ref.
func ProjectIDs(ctx context.Context, c *supatransport.Client, scoped string) []string {
	if scoped != "" {
		return []string{scoped}
	}
	var projects []struct {
		ID string `json:"id"`
	}
	if err := c.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects"}, &projects); err != nil {
		return nil
	}
	ids := make([]string, 0, len(projects))
	for _, p := range projects {
		if p.ID != "" {
			ids = append(ids, p.ID)
		}
	}
	return ids
}
