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

const ResourceTypeOrganization = "SUPABASE::Platform::Organization"

func init() {
	registry.Register(
		ResourceTypeOrganization,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationList,
		},
		func(c *supatransport.Client, _ *registry.TargetConfig) prov.Provisioner {
			return &Organization{Client: c}
		},
	)
}

// Organization — SUPABASE::Platform::Organization.
//
// The Management API only exposes POST/GET. Update and Delete are reported
// as no-op successes with a hint in StatusMessage so reconcile does not loop.
type Organization struct {
	Client *supatransport.Client
}

type OrganizationProperties struct {
	ID   string `json:"id,omitempty"`
	Slug string `json:"slug,omitempty"`
	Name string `json:"name,omitempty"`
}

func (o *Organization) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var p OrganizationProperties
	if err := json.Unmarshal(req.Properties, &p); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if p.Name == "" {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, "name is required"), nil
	}
	var resp OrganizationProperties
	if err := o.Client.Do(ctx, supatransport.Request{
		Method: "POST", Path: "/v1/organizations", Body: map[string]any{"name": p.Name},
	}, &resp); err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	native := resp.Slug
	if native == "" {
		native = resp.ID
	}
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationCreate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           native,
			ResourceProperties: prov.MustMarshal(resp),
		},
	}, nil
}

func (o *Organization) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	if req.NativeID == "" {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	var p OrganizationProperties
	if err := o.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/organizations/" + req.NativeID,
	}, &p); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(prov.MustMarshal(p))}, nil
}

// Update is unsupported by the Supabase Management API.
func (o *Organization) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	_ = ctx
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationUpdate,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        req.NativeID,
			StatusMessage:   "organization update is not supported by the Supabase Management API",
		},
	}, nil
}

// Delete is unsupported by the Supabase Management API.
func (o *Organization) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	_ = ctx
	return &resource.DeleteResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationDelete,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        req.NativeID,
			StatusMessage:   "organization delete is not supported by the Supabase Management API",
		},
	}, nil
}

func (o *Organization) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	_ = ctx
	return &resource.StatusResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationCheckStatus,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        req.NativeID,
		},
	}, nil
}

func (o *Organization) List(ctx context.Context, _ *resource.ListRequest) (*resource.ListResult, error) {
	var orgs []OrganizationProperties
	if err := o.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/organizations"}, &orgs); err != nil {
		return &resource.ListResult{NativeIDs: []string{}}, nil
	}
	ids := make([]string, 0, len(orgs))
	for _, og := range orgs {
		if og.Slug != "" {
			ids = append(ids, og.Slug)
		} else if og.ID != "" {
			ids = append(ids, og.ID)
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
