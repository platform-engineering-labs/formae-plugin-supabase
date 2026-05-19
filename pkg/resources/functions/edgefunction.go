// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package functions

import (
	"context"
	"encoding/json"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypeEdgeFunction = "SUPABASE::Functions::EdgeFunction"

func init() {
	registry.Register(
		ResourceTypeEdgeFunction,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationDelete,
			resource.OperationList,
		},
		func(c *supatransport.Client, _ *registry.TargetConfig) prov.Provisioner {
			return &EdgeFunction{Client: c}
		},
	)
}

// EdgeFunction — SUPABASE::Functions::EdgeFunction.
//
// Uses the simple JSON deploy path (single inline body string). The eszip
// multipart deploy endpoint at /v1/projects/{ref}/functions/deploy is out
// of scope here — drive that from CI with a dedicated workflow.
//
// Native id: `{project_ref}/{slug}`.
type EdgeFunction struct {
	Client *supatransport.Client
}

type EdgeFunctionProperties struct {
	ID            string `json:"id,omitempty"`
	ProjectRef    string `json:"projectRef,omitempty"`
	Slug          string `json:"slug,omitempty"`
	Name          string `json:"name,omitempty"`
	Body          string `json:"body,omitempty"`
	VerifyJWT     *bool  `json:"verify_jwt,omitempty"`
	Status        string `json:"status,omitempty"`
	Version       int    `json:"version,omitempty"`
	EntrypointURL string `json:"entrypoint_path,omitempty"`
	CreatedAt     int64  `json:"created_at,omitempty"`
	UpdatedAt     int64  `json:"updated_at,omitempty"`
}

func (e *EdgeFunction) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var p EdgeFunctionProperties
	if err := json.Unmarshal(req.Properties, &p); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if p.ProjectRef == "" || p.Slug == "" || p.Name == "" || p.Body == "" {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, "projectRef, slug, name, body are required"), nil
	}
	body := map[string]any{"slug": p.Slug, "name": p.Name, "body": p.Body}
	if p.VerifyJWT != nil {
		body["verify_jwt"] = *p.VerifyJWT
	}
	var resp EdgeFunctionProperties
	if err := e.Client.Do(ctx, supatransport.Request{
		Method: "POST",
		Path:   "/v1/projects/" + p.ProjectRef + "/functions",
		Body:   body,
	}, &resp); err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	if resp.Slug == "" {
		resp.Slug = p.Slug
	}
	resp.ProjectRef = p.ProjectRef
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationCreate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           prov.JoinTwoPart(p.ProjectRef, resp.Slug),
			ResourceProperties: prov.MustMarshal(resp),
		},
	}, nil
}

func (e *EdgeFunction) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	project, slug, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	var p EdgeFunctionProperties
	if err := e.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + project + "/functions/" + slug,
	}, &p); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	p.ProjectRef = project
	p.Slug = slug
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(prov.MustMarshal(p))}, nil
}

func (e *EdgeFunction) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	project, slug, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	var desired EdgeFunctionProperties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	body := map[string]any{}
	if desired.Name != "" {
		body["name"] = desired.Name
	}
	if desired.Body != "" {
		body["body"] = desired.Body
	}
	if desired.VerifyJWT != nil {
		body["verify_jwt"] = *desired.VerifyJWT
	}
	if len(body) == 0 {
		return &resource.UpdateResult{
			ProgressResult: &resource.ProgressResult{Operation: resource.OperationUpdate, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
		}, nil
	}
	var resp EdgeFunctionProperties
	if err := e.Client.Do(ctx, supatransport.Request{
		Method: "PATCH",
		Path:   "/v1/projects/" + project + "/functions/" + slug,
		Body:   body,
	}, &resp); err != nil {
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	resp.ProjectRef = project
	if resp.Slug == "" {
		resp.Slug = slug
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

func (e *EdgeFunction) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	project, slug, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailDelete(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if err := e.Client.Do(ctx, supatransport.Request{
		Method: "DELETE", Path: "/v1/projects/" + project + "/functions/" + slug,
	}, nil); err != nil {
		if supatransport.IsNotFound(err) {
			return prov.SuccessDelete(req.NativeID), nil
		}
		return prov.FailDelete(supatransport.ClassifyError(err), err.Error()), nil
	}
	return prov.SuccessDelete(req.NativeID), nil
}

func (e *EdgeFunction) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	_ = ctx
	return &resource.StatusResult{
		ProgressResult: &resource.ProgressResult{Operation: resource.OperationCheckStatus, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
	}, nil
}

func (e *EdgeFunction) List(ctx context.Context, _ *resource.ListRequest) (*resource.ListResult, error) {
	var projects []struct {
		ID string `json:"id"`
	}
	if err := e.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects"}, &projects); err != nil {
		return &resource.ListResult{NativeIDs: []string{}}, nil
	}
	var ids []string
	for _, pr := range projects {
		var fns []EdgeFunctionProperties
		if err := e.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects/" + pr.ID + "/functions"}, &fns); err != nil {
			continue
		}
		for _, f := range fns {
			if f.Slug != "" {
				ids = append(ids, prov.JoinTwoPart(pr.ID, f.Slug))
			}
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
