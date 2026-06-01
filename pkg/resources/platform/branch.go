// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"context"
	"encoding/json"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypeBranch = "SUPABASE::Platform::Branch"

func init() {
	registry.Register(
		ResourceTypeBranch,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationDelete,
			resource.OperationCheckStatus,
			resource.OperationList,
		},
		func(c *supatransport.Client, cfg *registry.TargetConfig) prov.Provisioner {
			return &Branch{Client: c, ProjectScope: cfg.ProjectRef}
		},
	)
}

// Branch — SUPABASE::Platform::Branch.
//
// Async create: POST /v1/projects/{ref}/branches returns a branch id, then
// the branch transitions through statuses before being usable. Native id is
// `{parent_project_ref}/{branch_id}` so Update/Delete don't need extra
// lookups to recover the parent.
type Branch struct {
	Client       *supatransport.Client
	ProjectScope string
}

type BranchProperties struct {
	ID                  string `json:"id,omitempty"`
	ParentProjectRef    string `json:"parentProjectRef,omitempty"`
	ProjectRef          string `json:"project_ref,omitempty"`
	BranchName          string `json:"branch_name,omitempty"`
	GitBranch           string `json:"git_branch,omitempty"`
	IsDefault           bool   `json:"is_default,omitempty"`
	Persistent          bool   `json:"persistent,omitempty"`
	Region              string `json:"region,omitempty"`
	DesiredInstanceSize string `json:"desired_instance_size,omitempty"`
	Status              string `json:"status,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
}

const (
	// Terminal success states. Newer Supabase deployments report the
	// branch's underlying shadow-project status (ACTIVE_HEALTHY) once
	// migrations + functions complete; older surfaces still report the
	// per-phase MIGRATIONS_PASSED/FUNCTIONS_DEPLOYED markers. Accept all
	// three so the plugin doesn't spin in InProgress.
	branchStatusActiveHealthy     = "ACTIVE_HEALTHY"
	branchStatusFunctionsDeployed = "FUNCTIONS_DEPLOYED"
	branchStatusMigrationsPassed  = "MIGRATIONS_PASSED"
	branchStatusMigrationsFailed  = "MIGRATIONS_FAILED"
	branchStatusFunctionsFailed   = "FUNCTIONS_FAILED"
)

func (b *Branch) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	prov.Dbg("Branch.Create.start")
	var p BranchProperties
	if err := json.Unmarshal(req.Properties, &p); err != nil {
		prov.Dbg("Branch.Create.unmarshal.err %v", err)
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if p.ParentProjectRef == "" || p.BranchName == "" {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest,
			"parentProjectRef and branch_name are required"), nil
	}
	body := map[string]any{"branch_name": p.BranchName, "persistent": p.Persistent}
	if p.GitBranch != "" {
		body["git_branch"] = p.GitBranch
	}
	if p.Region != "" {
		body["region"] = p.Region
	}
	if p.DesiredInstanceSize != "" {
		body["desired_instance_size"] = p.DesiredInstanceSize
	}
	prov.Dbg("Branch.Create.post.start parent=%s body=%v", p.ParentProjectRef, body)
	var resp BranchProperties
	err := b.Client.Do(ctx, supatransport.Request{
		Method: "POST",
		Path:   "/v1/projects/" + p.ParentProjectRef + "/branches",
		Body:   body,
	}, &resp)
	prov.Dbg("Branch.Create.post.done err=%v respID=%q respStatus=%q", err, resp.ID, resp.Status)
	if err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	if resp.ID == "" {
		return prov.FailCreate(resource.OperationErrorCodeServiceInternalError, "create response missing id"), nil
	}
	native := prov.JoinTwoPart(p.ParentProjectRef, resp.ID)
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationCreate,
			OperationStatus: resource.OperationStatusInProgress,
			NativeID:        native,
			RequestID:       native,
		},
	}, nil
}

func (b *Branch) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	_, id, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	var p BranchProperties
	if err := b.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/branches/" + id}, &p); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(prov.MustMarshal(p))}, nil
}

func (b *Branch) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	_, id, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	var desired BranchProperties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	body := map[string]any{"persistent": desired.Persistent}
	if desired.BranchName != "" {
		body["branch_name"] = desired.BranchName
	}
	if desired.GitBranch != "" {
		body["git_branch"] = desired.GitBranch
	}
	var resp BranchProperties
	if err := b.Client.Do(ctx, supatransport.Request{Method: "PATCH", Path: "/v1/branches/" + id, Body: body}, &resp); err != nil {
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationUpdate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           req.NativeID,
			ResourceProperties: prov.MustMarshal(resp),
		},
	}, nil
}

func (b *Branch) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	prov.Dbg("Branch.Delete nativeID=%s", req.NativeID)
	_, id, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailDelete(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	derr := b.Client.Do(ctx, supatransport.Request{Method: "DELETE", Path: "/v1/branches/" + id}, nil)
	prov.Dbg("Branch.Delete.done id=%s err=%v", id, derr)
	if err := derr; err != nil {
		if supatransport.IsNotFound(err) {
			return prov.SuccessDelete(req.NativeID), nil
		}
		return prov.FailDelete(supatransport.ClassifyError(err), err.Error()), nil
	}
	return prov.SuccessDelete(req.NativeID), nil
}

func (b *Branch) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	native := req.RequestID
	if native == "" {
		native = req.NativeID
	}
	_, id, err := prov.ParseTwoPart(native)
	if err != nil {
		return prov.FailStatus(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	var p BranchProperties
	if err := b.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/branches/" + id}, &p); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.StatusResult{
				ProgressResult: &resource.ProgressResult{Operation: resource.OperationCheckStatus, OperationStatus: resource.OperationStatusSuccess, NativeID: native},
			}, nil
		}
		prov.Dbg("Branch.Status.err id=%s err=%v", id, err)
		return prov.FailStatus(supatransport.ClassifyError(err), err.Error()), nil
	}
	prov.Dbg("Branch.Status id=%s status=%q", id, p.Status)
	switch p.Status {
	case branchStatusActiveHealthy, branchStatusFunctionsDeployed, branchStatusMigrationsPassed:
		return &resource.StatusResult{
			ProgressResult: &resource.ProgressResult{
				Operation:          resource.OperationCheckStatus,
				OperationStatus:    resource.OperationStatusSuccess,
				NativeID:           native,
				ResourceProperties: prov.MustMarshal(p),
			},
		}, nil
	case branchStatusMigrationsFailed, branchStatusFunctionsFailed:
		return prov.FailStatus(resource.OperationErrorCodeServiceInternalError, "branch terminal status "+p.Status), nil
	default:
		return &resource.StatusResult{
			ProgressResult: &resource.ProgressResult{
				Operation:       resource.OperationCheckStatus,
				OperationStatus: resource.OperationStatusInProgress,
				NativeID:        native,
				RequestID:       native,
				StatusMessage:   "branch status: " + p.Status,
			},
		}, nil
	}
}

func (b *Branch) List(ctx context.Context, req *resource.ListRequest) (*resource.ListResult, error) {
	var ids []string
	for _, projectID := range prov.ProjectIDs(ctx, b.Client, b.ProjectScope) {
		var branches []BranchProperties
		if err := b.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects/" + projectID + "/branches"}, &branches); err != nil {
			continue
		}
		for _, br := range branches {
			if br.ID != "" {
				ids = append(ids, prov.JoinTwoPart(projectID, br.ID))
			}
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
