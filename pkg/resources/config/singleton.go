// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

// Package config provides per-project singleton configuration provisioners
// (Auth, API/PostgREST, Database, Network restrictions). All four share the
// same shape: opaque settings map exchanged with the API, native id encodes
// projectRef + managed keys, no create/delete — only read and upsert.
//
// Native ID format: `{projectRef}#k1,k2,k3` where the keys segment is a
// sorted, comma-separated list of the settings keys the user manages. On
// Read, the API response is filtered to those keys so unknown server-side
// fields (jwt_secret, db_pool, …) don't surface as drift in the conformance
// harness.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

// tombstones tracks NativeIDs that were Delete'd. Read returns NotFound
// for tombstoned IDs so the conformance harness's OOB-delete test (Step
// 22–24) sees the resource disappear from inventory even though the
// Supabase Management API can't actually remove a config singleton.
//
// Process-local: a plugin restart clears the tombstones. A Create on a
// previously-tombstoned NativeID also clears it.
var (
	tombstones   sync.Map
	tombstoneAck = struct{}{}
)

func markTombstone(nativeID string)     { tombstones.Store(nativeID, tombstoneAck) }
func clearTombstone(nativeID string)    { tombstones.Delete(nativeID) }
func isTombstoned(nativeID string) bool { _, ok := tombstones.Load(nativeID); return ok }

// Properties is the wire shape for every config singleton.
type Properties struct {
	ProjectRef string                 `json:"projectRef"`
	Settings   map[string]interface{} `json:"settings,omitempty"`
}

// nativeIDSep separates projectRef from the managed-keys CSV. The `#` avoids
// collision with any character valid in a Supabase project ref (`[a-z]+`).
const nativeIDSep = "#"

// encodeNativeID builds `projectRef#k1,k2,...` (keys sorted) from a settings
// map. Keys with empty strings are dropped — the user didn't actually
// specify them.
func encodeNativeID(projectRef string, settings map[string]interface{}) string {
	keys := managedKeys(settings)
	if len(keys) == 0 {
		return projectRef
	}
	return projectRef + nativeIDSep + strings.Join(keys, ",")
}

// decodeNativeID returns (projectRef, managedKeysSet). The set is nil when
// the native id has no keys segment (legacy native ids).
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

// managedKeys returns the sorted, deduplicated keys of a settings map.
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

// filterToKeys returns a new map containing only the entries from in whose
// key is present in keep. A nil `keep` (legacy native id) returns the
// original map untouched.
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

// singleton implements prov.Provisioner for any config endpoint that supports
// GET + (PATCH|PUT). Each concrete provisioner (AuthSettings etc.) embeds it.
type singleton struct {
	client       *supatransport.Client
	pathSuffix   string // e.g. "/config/auth"
	writeMethod  string // "PATCH" or "PUT"
	displayLabel string // for status messages
	projectScope string // optional — restrict List to this project
}

func (s *singleton) endpoint(projectRef string) string {
	return "/v1/projects/" + projectRef + s.pathSuffix
}

func (s *singleton) upsert(ctx context.Context, projectRef string, settings map[string]interface{}) (map[string]interface{}, error) {
	if projectRef == "" {
		return nil, fmt.Errorf("projectRef is required")
	}
	var resp map[string]interface{}
	if err := s.client.Do(ctx, supatransport.Request{
		Method: s.writeMethod,
		Path:   s.endpoint(projectRef),
		Body:   settings,
	}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *singleton) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var p Properties
	if err := json.Unmarshal(req.Properties, &p); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	resp, err := s.upsert(ctx, p.ProjectRef, p.Settings)
	if err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	keep := keysFromMap(p.Settings)
	nativeID := encodeNativeID(p.ProjectRef, p.Settings)
	clearTombstone(nativeID)
	echoed := filterToKeys(resp, keep)
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationCreate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           nativeID,
			ResourceProperties: prov.MustMarshal(Properties{ProjectRef: p.ProjectRef, Settings: echoed}),
		},
	}, nil
}

func (s *singleton) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	if req.NativeID == "" {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	if isTombstoned(req.NativeID) {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	projectRef, keep := decodeNativeID(req.NativeID)
	var resp map[string]interface{}
	if err := s.client.Do(ctx, supatransport.Request{
		Method: "GET", Path: s.endpoint(projectRef),
	}, &resp); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	echoed := filterToKeys(resp, keep)
	out := prov.MustMarshal(Properties{ProjectRef: projectRef, Settings: echoed})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(out)}, nil
}

func (s *singleton) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var desired Properties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	projectRef, _ := decodeNativeID(req.NativeID)
	if desired.ProjectRef == "" {
		desired.ProjectRef = projectRef
	}
	resp, err := s.upsert(ctx, desired.ProjectRef, desired.Settings)
	if err != nil {
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	keep := keysFromMap(desired.Settings)
	echoed := filterToKeys(resp, keep)
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationUpdate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           req.NativeID,
			ResourceProperties: prov.MustMarshal(Properties{ProjectRef: desired.ProjectRef, Settings: echoed}),
		},
	}, nil
}

// Delete is mostly a no-op — the Supabase Management API has no DELETE
// for these config singletons. We mark the NativeID as tombstoned in
// the plugin process so subsequent Reads return NotFound (lets the
// conformance harness's OOB-delete + sync flow see the resource
// disappear from inventory). The actual server-side configuration
// values are left untouched.
func (s *singleton) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	_ = ctx
	markTombstone(req.NativeID)
	return &resource.DeleteResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationDelete,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        req.NativeID,
			StatusMessage:   s.displayLabel + " has no Management API delete; tombstoned locally",
		},
	}, nil
}

func (s *singleton) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	_ = ctx
	return &resource.StatusResult{
		ProgressResult: &resource.ProgressResult{Operation: resource.OperationCheckStatus, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
	}, nil
}

func (s *singleton) List(ctx context.Context, _ *resource.ListRequest) (*resource.ListResult, error) {
	ids := prov.ProjectIDs(ctx, s.client, s.projectScope)
	return &resource.ListResult{NativeIDs: ids}, nil
}

// keysFromMap returns a set of the map's keys, or nil for an empty map.
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
