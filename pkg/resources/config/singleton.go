// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

// Package config provides per-project singleton configuration provisioners
// (Auth, API/PostgREST, Database, Network restrictions). All four share the
// same shape: opaque settings map exchanged with the API, native id = project
// ref, no create/delete — only read and upsert.
package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

// Properties is the wire shape for every config singleton.
type Properties struct {
	ProjectRef string                 `json:"projectRef"`
	Settings   map[string]interface{} `json:"settings,omitempty"`
}

// singleton implements prov.Provisioner for any config endpoint that supports
// GET + (PATCH|PUT). Each concrete provisioner (AuthSettings etc.) embeds it.
type singleton struct {
	client       *supatransport.Client
	pathSuffix   string // e.g. "/config/auth"
	writeMethod  string // "PATCH" or "PUT"
	displayLabel string // for status messages
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
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationCreate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           p.ProjectRef,
			ResourceProperties: prov.MustMarshal(Properties{ProjectRef: p.ProjectRef, Settings: resp}),
		},
	}, nil
}

func (s *singleton) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	if req.NativeID == "" {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	var resp map[string]interface{}
	if err := s.client.Do(ctx, supatransport.Request{
		Method: "GET", Path: s.endpoint(req.NativeID),
	}, &resp); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	out := prov.MustMarshal(Properties{ProjectRef: req.NativeID, Settings: resp})
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(out)}, nil
}

func (s *singleton) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	var desired Properties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if desired.ProjectRef == "" {
		desired.ProjectRef = req.NativeID
	}
	resp, err := s.upsert(ctx, desired.ProjectRef, desired.Settings)
	if err != nil {
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationUpdate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           req.NativeID,
			ResourceProperties: prov.MustMarshal(Properties{ProjectRef: desired.ProjectRef, Settings: resp}),
		},
	}, nil
}

// Delete is a no-op: singletons cannot be removed.
func (s *singleton) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	_ = ctx
	return &resource.DeleteResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationDelete,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        req.NativeID,
			StatusMessage:   s.displayLabel + " cannot be deleted; reported success without API call",
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
	var projects []struct {
		ID string `json:"id"`
	}
	if err := s.client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects"}, &projects); err != nil {
		return &resource.ListResult{NativeIDs: []string{}}, nil
	}
	ids := make([]string, 0, len(projects))
	for _, pr := range projects {
		if pr.ID != "" {
			ids = append(ids, pr.ID)
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
