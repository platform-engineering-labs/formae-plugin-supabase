// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

// Package database holds per-project database-tier provisioners.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypePoolerConfig = "SUPABASE::Database::PoolerConfig"

func init() {
	registry.Register(
		ResourceTypePoolerConfig,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationList,
		},
		func(c *supatransport.Client, cfg *registry.TargetConfig) prov.Provisioner {
			return &PoolerConfig{client: c, projectScope: cfg.ProjectRef}
		},
	)
}

// PoolerConfig — SUPABASE::Database::PoolerConfig.
//
// Endpoints:
//
//	GET   /v1/projects/{ref}/config/database/pooler   → array<SupavisorConfigResponse>
//	PATCH /v1/projects/{ref}/config/database/pooler   → UpdateSupavisorConfigBody { default_pool_size, pool_mode }
//
// Singleton keyed by project ref. The GET response is an *array* (one entry
// per database — primary + read replicas); we surface the PRIMARY entry.
//
// Mutable fields (per UpdateSupavisorConfigBody):
//   - default_pool_size : int (0..3000), nullable
//   - pool_mode         : "transaction" | "session"
//
// Other GET fields (db_user, db_host, db_port, connection_string, …) are
// read-only and round-trip as additional keys in `settings`.
type PoolerConfig struct {
	client       *supatransport.Client
	projectScope string
}

// Properties is the wire shape — `projectRef` identifies the singleton;
// `settings` is the opaque pool-mode / pool-size map.
type Properties struct {
	ProjectRef string                 `json:"projectRef"`
	Settings   map[string]interface{} `json:"settings,omitempty"`
}

func (p *PoolerConfig) endpoint(projectRef string) string {
	return "/v1/projects/" + projectRef + "/config/database/pooler"
}

func (p *PoolerConfig) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var props Properties
	if err := json.Unmarshal(req.Properties, &props); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	updated, err := p.patch(ctx, props.ProjectRef, props.Settings)
	if err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	keep := keysFromMap(props.Settings)
	echoed := filterToKeys(updated, keep)
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationCreate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           encodeNativeID(props.ProjectRef, props.Settings),
			ResourceProperties: prov.MustMarshal(Properties{ProjectRef: props.ProjectRef, Settings: echoed}),
		},
	}, nil
}

func (p *PoolerConfig) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	if req.NativeID == "" {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	projectRef, keep := decodeNativeID(req.NativeID)
	primary, err := p.readPrimary(ctx, projectRef)
	if err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	echoed := filterToKeys(primary, keep)
	out := prov.MustMarshal(Properties{ProjectRef: projectRef, Settings: echoed})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(out)}, nil
}

func (p *PoolerConfig) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var desired Properties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	projectRef, _ := decodeNativeID(req.NativeID)
	if desired.ProjectRef == "" {
		desired.ProjectRef = projectRef
	}
	updated, err := p.patch(ctx, desired.ProjectRef, desired.Settings)
	if err != nil {
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	keep := keysFromMap(desired.Settings)
	echoed := filterToKeys(updated, keep)
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationUpdate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           req.NativeID,
			ResourceProperties: prov.MustMarshal(Properties{ProjectRef: desired.ProjectRef, Settings: echoed}),
		},
	}, nil
}

// Delete is a no-op — the pooler config singleton cannot be removed.
func (p *PoolerConfig) Delete(_ context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	return &resource.DeleteResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationDelete,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        req.NativeID,
			StatusMessage:   "pooler config cannot be deleted; reported success without API call",
		},
	}, nil
}

func (p *PoolerConfig) Status(_ context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	return &resource.StatusResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationCheckStatus,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        req.NativeID,
		},
	}, nil
}

func (p *PoolerConfig) List(ctx context.Context, _ *resource.ListRequest) (*resource.ListResult, error) {
	ids := prov.ProjectIDs(ctx, p.client, p.projectScope)
	return &resource.ListResult{NativeIDs: ids}, nil
}

// ------------------------------------------------------------------
// shared helpers: managed-key tracking via NativeID
// ------------------------------------------------------------------

const nativeIDSep = "#"

func encodeNativeID(projectRef string, settings map[string]interface{}) string {
	keys := managedKeys(settings)
	if len(keys) == 0 {
		return projectRef
	}
	return projectRef + nativeIDSep + strings.Join(keys, ",")
}

func decodeNativeID(nativeID string) (string, map[string]struct{}) {
	idx := strings.Index(nativeID, nativeIDSep)
	if idx < 0 {
		return nativeID, nil
	}
	projectRef := nativeID[:idx]
	keys := strings.Split(nativeID[idx+len(nativeIDSep):], ",")
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if k != "" {
			set[k] = struct{}{}
		}
	}
	return projectRef, set
}

func managedKeys(settings map[string]interface{}) []string {
	if len(settings) == 0 {
		return nil
	}
	out := make([]string, 0, len(settings))
	for k := range settings {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func filterToKeys(in map[string]interface{}, keep map[string]struct{}) map[string]interface{} {
	if keep == nil {
		return in
	}
	out := make(map[string]interface{}, len(keep))
	for k := range keep {
		if v, ok := in[k]; ok {
			out[k] = v
		}
	}
	return out
}

func keysFromMap(m map[string]interface{}) map[string]struct{} {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

// patch sends an UpdateSupavisorConfigBody. The API returns the updated
// primary entry; we hand back its JSON map.
func (p *PoolerConfig) patch(ctx context.Context, projectRef string, settings map[string]interface{}) (map[string]interface{}, error) {
	if projectRef == "" {
		return nil, fmt.Errorf("projectRef is required")
	}
	body := map[string]interface{}{}
	for _, k := range []string{"default_pool_size", "pool_mode"} {
		if v, ok := settings[k]; ok {
			body[k] = v
		}
	}
	var resp map[string]interface{}
	if err := p.client.Do(ctx, supatransport.Request{
		Method: "PATCH",
		Path:   p.endpoint(projectRef),
		Body:   body,
	}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// readPrimary fetches the pooler config array and returns the PRIMARY entry.
func (p *PoolerConfig) readPrimary(ctx context.Context, projectRef string) (map[string]interface{}, error) {
	var entries []map[string]interface{}
	if err := p.client.Do(ctx, supatransport.Request{
		Method: "GET",
		Path:   p.endpoint(projectRef),
	}, &entries); err != nil {
		return nil, err
	}
	for _, e := range entries {
		if t, _ := e["database_type"].(string); t == "PRIMARY" {
			return e, nil
		}
	}
	// Fall back to first entry if API doesn't tag a PRIMARY (single-DB projects).
	if len(entries) > 0 {
		return entries[0], nil
	}
	return nil, fmt.Errorf("pooler config response was empty")
}
