// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: FSL-1.1-ALv2

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

// isCannotDeletePersistent reports whether err is the Supabase 422 that
// blocks DELETE on a branch with persistent=true.
func isCannotDeletePersistent(err error) bool {
	var apiErr *supatransport.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode != 422 {
		return false
	}
	return strings.Contains(strings.ToLower(apiErr.Message),
		"cannot delete persistent")
}

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

// branchAPI is the Supabase-API-facing shape (GET /v1/branches/{id} and
// /v1/projects/{ref}/branches). The API names the branch `name` and carries
// `parent_project_ref`; the Forma shape uses `branch_name` and
// `parentProjectRef`. Decoding the raw response straight into BranchProperties
// silently dropped both (mismatched JSON tags), which made discovery reject
// every branch for "missing required fields". Map explicitly instead.
type branchAPI struct {
	ID                  string `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	ProjectRef          string `json:"project_ref,omitempty"`
	ParentProjectRef    string `json:"parent_project_ref,omitempty"`
	GitBranch           string `json:"git_branch,omitempty"`
	IsDefault           bool   `json:"is_default,omitempty"`
	Persistent          bool   `json:"persistent,omitempty"`
	Region              string `json:"region,omitempty"`
	DesiredInstanceSize string `json:"desired_instance_size,omitempty"`
	Status              string `json:"status,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
}

// toProps converts the API shape to the Forma shape. parentFallback (the
// parent ref parsed from the native ID) is used when the API response omits
// parent_project_ref, so discovery and Read always yield a complete resource.
func (a branchAPI) toProps(parentFallback string) BranchProperties {
	parent := a.ParentProjectRef
	if parent == "" {
		parent = parentFallback
	}
	return BranchProperties{
		ID:                  a.ID,
		ParentProjectRef:    parent,
		ProjectRef:          a.ProjectRef,
		BranchName:          a.Name,
		GitBranch:           a.GitBranch,
		IsDefault:           a.IsDefault,
		Persistent:          a.Persistent,
		Region:              a.Region,
		DesiredInstanceSize: a.DesiredInstanceSize,
		Status:              a.Status,
		CreatedAt:           a.CreatedAt,
	}
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
	var p BranchProperties
	if err := json.Unmarshal(req.Properties, &p); err != nil {
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
	var resp BranchProperties
	if err := b.Client.Do(ctx, supatransport.Request{
		Method: "POST",
		Path:   "/v1/projects/" + p.ParentProjectRef + "/branches",
		Body:   body,
	}, &resp); err != nil {
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
	parent, id, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	// The single-branch GET (/v1/branches/{id}) returns only the branch's
	// Postgres connection info — not its name or parent_project_ref. The
	// branch metadata (name, parent) lives in the parent project's branch
	// list, so Read sources it there. Without name, discovery rejects the
	// branch for "missing required fields: [branch_name]".
	var list []branchAPI
	if err := b.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + parent + "/branches",
	}, &list); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	for _, api := range list {
		if api.ID == id {
			return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(prov.MustMarshal(api.toProps(parent)))}, nil
		}
	}
	// Branch id no longer present in the parent's list → gone.
	return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
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
	// Re-parse into a raw map so we can tell which fields the user
	// actually set vs which are Go zero-values. Patching a field the
	// user didn't touch is a silent side effect.
	var raw map[string]any
	if err := json.Unmarshal(req.DesiredProperties, &raw); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}

	body := map[string]any{}
	if _, ok := raw["persistent"]; ok {
		body["persistent"] = desired.Persistent
	}
	if desired.BranchName != "" {
		body["branch_name"] = desired.BranchName
	}
	if desired.GitBranch != "" {
		body["git_branch"] = desired.GitBranch
	}
	if len(body) == 0 {
		// No mutable fields touched — return the current cloud state
		// so formae sees a synchronous success rather than spinning.
		var cur BranchProperties
		if err := b.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/branches/" + id}, &cur); err != nil {
			return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
		}
		return &resource.UpdateResult{
			ProgressResult: &resource.ProgressResult{
				Operation:          resource.OperationUpdate,
				OperationStatus:    resource.OperationStatusSuccess,
				NativeID:           req.NativeID,
				ResourceProperties: prov.MustMarshal(cur),
			},
		}, nil
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
	_, id, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailDelete(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	// Bound the whole call below the harness's 40s "PluginOperatorMissing
	// InAction" watchdog. Worst case is DELETE + PATCH + DELETE = up to
	// three 30s HTTP timeouts (≈90s) — would otherwise look like a hang.
	callCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()

	derr := b.Client.Do(callCtx, supatransport.Request{Method: "DELETE", Path: "/v1/branches/" + id}, nil)
	// Supabase refuses DELETE on a persistent branch (HTTP 422). Flip
	// persistent=false via PATCH, then retry the delete.
	if derr != nil && isCannotDeletePersistent(derr) {
		if perr := b.Client.Do(callCtx, supatransport.Request{
			Method: "PATCH", Path: "/v1/branches/" + id,
			Body: map[string]any{"persistent": false},
		}, nil); perr != nil {
			return prov.FailDelete(supatransport.ClassifyError(perr), perr.Error()), nil
		}
		derr = b.Client.Do(callCtx, supatransport.Request{Method: "DELETE", Path: "/v1/branches/" + id}, nil)
	}
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
		return prov.FailStatus(supatransport.ClassifyError(err), err.Error()), nil
	}
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
