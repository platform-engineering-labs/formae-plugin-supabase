// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypeProject = "SUPABASE::Platform::Project"

func init() {
	registry.Register(
		ResourceTypeProject,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationDelete,
			resource.OperationCheckStatus,
			resource.OperationList,
		},
		func(c *supatransport.Client, _ *registry.TargetConfig) prov.Provisioner {
			return &Project{Client: c}
		},
	)
}

// Project — SUPABASE::Platform::Project.
//
// API mapping:
//   POST   /v1/projects            Create (async)
//   GET    /v1/projects/{ref}      Read / Status
//   PATCH  /v1/projects/{ref}      Update
//   DELETE /v1/projects/{ref}      Delete
//   GET    /v1/projects            List
type Project struct {
	Client *supatransport.Client
}

// ProjectProperties is the Forma-facing shape (PKL field names).
type ProjectProperties struct {
	ID                  string `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	OrganizationID      string `json:"organizationId,omitempty"`
	Region              string `json:"region,omitempty"`
	DBPass              string `json:"dbPass,omitempty"`
	Plan                string `json:"plan,omitempty"`
	DesiredInstanceSize string `json:"desiredInstanceSize,omitempty"`
	Status              string `json:"status,omitempty"`
	CreatedAt           string `json:"createdAt,omitempty"`
}

// projectAPI is the Supabase-API-facing shape (snake_case).
type projectAPI struct {
	ID                  string `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	OrganizationID      string `json:"organization_id,omitempty"`
	Region              string `json:"region,omitempty"`
	DBPass              string `json:"db_pass,omitempty"`
	Plan                string `json:"plan,omitempty"`
	DesiredInstanceSize string `json:"desired_instance_size,omitempty"`
	Status              string `json:"status,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
}

func (a projectAPI) toProps() ProjectProperties {
	return ProjectProperties{
		ID: a.ID, Name: a.Name, OrganizationID: a.OrganizationID, Region: a.Region,
		DBPass: a.DBPass, Plan: a.Plan, DesiredInstanceSize: a.DesiredInstanceSize,
		Status: a.Status, CreatedAt: a.CreatedAt,
	}
}

const (
	projectStatusActive       = "ACTIVE_HEALTHY"
	projectStatusInactive     = "INACTIVE"
	projectStatusInitFailed   = "INIT_FAILED"
	projectStatusRemoved      = "REMOVED"
)

func (p *Project) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var pp ProjectProperties
	if err := json.Unmarshal(req.Properties, &pp); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if pp.Name == "" || pp.OrganizationID == "" || pp.Region == "" || pp.DBPass == "" {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest,
			"name, organizationId, region, dbPass are required"), nil
	}
	body := map[string]any{
		"name":            pp.Name,
		"organization_id": pp.OrganizationID,
		"region":          pp.Region,
		"db_pass":         pp.DBPass,
	}
	if pp.Plan != "" {
		body["plan"] = pp.Plan
	}
	if pp.DesiredInstanceSize != "" {
		body["desired_instance_size"] = pp.DesiredInstanceSize
	}
	var apiResp projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "POST", Path: "/v1/projects", Body: body,
	}, &apiResp); err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	if apiResp.ID == "" {
		return prov.FailCreate(resource.OperationErrorCodeServiceInternalError,
			"create response missing project id"), nil
	}
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationCreate,
			OperationStatus: resource.OperationStatusInProgress,
			NativeID:        apiResp.ID,
			RequestID:       apiResp.ID,
		},
	}, nil
}

func (p *Project) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	if req.NativeID == "" {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	var apiResp projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + req.NativeID,
	}, &apiResp); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(prov.MustMarshal(apiResp.toProps()))}, nil
}

func (p *Project) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	if req.NativeID == "" {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, "native id required"), nil
	}
	var desired ProjectProperties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	body := map[string]any{}
	if desired.Name != "" {
		body["name"] = desired.Name
	}
	if len(body) == 0 {
		return &resource.UpdateResult{
			ProgressResult: &resource.ProgressResult{
				Operation:       resource.OperationUpdate,
				OperationStatus: resource.OperationStatusSuccess,
				NativeID:        req.NativeID,
			},
		}, nil
	}
	var apiResp projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "PATCH", Path: "/v1/projects/" + req.NativeID, Body: body,
	}, &apiResp); err != nil {
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationUpdate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           req.NativeID,
			ResourceProperties: prov.MustMarshal(apiResp.toProps()),
		},
	}, nil
}

func (p *Project) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	if req.NativeID == "" {
		return prov.FailDelete(resource.OperationErrorCodeInvalidRequest, "native id required"), nil
	}
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "DELETE", Path: "/v1/projects/" + req.NativeID,
	}, nil); err != nil {
		if supatransport.IsNotFound(err) {
			return prov.SuccessDelete(req.NativeID), nil
		}
		return prov.FailDelete(supatransport.ClassifyError(err), err.Error()), nil
	}
	return prov.SuccessDelete(req.NativeID), nil
}

func (p *Project) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	ref := req.RequestID
	if ref == "" {
		ref = req.NativeID
	}
	if ref == "" {
		return prov.FailStatus(resource.OperationErrorCodeInvalidRequest, "request id required"), nil
	}
	var apiResp projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + ref,
	}, &apiResp); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.StatusResult{
				ProgressResult: &resource.ProgressResult{
					Operation:       resource.OperationCheckStatus,
					OperationStatus: resource.OperationStatusSuccess,
					NativeID:        ref,
				},
			}, nil
		}
		return prov.FailStatus(supatransport.ClassifyError(err), err.Error()), nil
	}
	switch apiResp.Status {
	case projectStatusActive:
		return &resource.StatusResult{
			ProgressResult: &resource.ProgressResult{
				Operation:          resource.OperationCheckStatus,
				OperationStatus:    resource.OperationStatusSuccess,
				NativeID:           ref,
				ResourceProperties: prov.MustMarshal(apiResp.toProps()),
			},
		}, nil
	case projectStatusInactive, projectStatusInitFailed, projectStatusRemoved:
		return prov.FailStatus(resource.OperationErrorCodeServiceInternalError,
			fmt.Sprintf("project entered terminal status %q", apiResp.Status)), nil
	default:
		return &resource.StatusResult{
			ProgressResult: &resource.ProgressResult{
				Operation:       resource.OperationCheckStatus,
				OperationStatus: resource.OperationStatusInProgress,
				NativeID:        ref,
				RequestID:       ref,
				StatusMessage:   "project status: " + apiResp.Status,
			},
		}, nil
	}
}

func (p *Project) List(ctx context.Context, req *resource.ListRequest) (*resource.ListResult, error) {
	var projects []projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects"}, &projects); err != nil {
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
