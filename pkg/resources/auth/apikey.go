// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"encoding/json"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypeAPIKey = "SUPABASE::Auth::APIKey"

func init() {
	registry.Register(
		ResourceTypeAPIKey,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationDelete,
			resource.OperationList,
		},
		func(c *supatransport.Client, cfg *registry.TargetConfig) prov.Provisioner {
			return &APIKey{Client: c, ProjectScope: cfg.ProjectRef}
		},
	)
}

// APIKey — SUPABASE::Auth::APIKey.
//
// Native id: `{project_ref}/{key_id}`.
// `?reveal=true` is added to GET/POST so the raw key value is returned.
type APIKey struct {
	Client       *supatransport.Client
	ProjectScope string // optional — if set, List only walks this project
}

// APIKeyProperties is the Forma-facing shape (matches PKL field names).
type APIKeyProperties struct {
	ID                string                 `json:"id,omitempty"`
	ProjectRef        string                 `json:"projectRef,omitempty"`
	Type              string                 `json:"apikey_type,omitempty"`
	Name              string                 `json:"name,omitempty"`
	Description       string                 `json:"description,omitempty"`
	APIKey            string                 `json:"apiKey,omitempty"`
	Prefix            string                 `json:"prefix,omitempty"`
	Hash              string                 `json:"hash,omitempty"`
	SecretJWTTemplate map[string]interface{} `json:"secretJwtTemplate,omitempty"`
	InsertedAt        string                 `json:"insertedAt,omitempty"`
	UpdatedAt         string                 `json:"updatedAt,omitempty"`
}

// apiKeyAPI is the Supabase-API-facing shape.
type apiKeyAPI struct {
	ID                string                 `json:"id,omitempty"`
	Type              string                 `json:"type,omitempty"`
	Name              string                 `json:"name,omitempty"`
	Description       string                 `json:"description,omitempty"`
	APIKey            string                 `json:"api_key,omitempty"`
	Prefix            string                 `json:"prefix,omitempty"`
	Hash              string                 `json:"hash,omitempty"`
	SecretJWTTemplate map[string]interface{} `json:"secret_jwt_template,omitempty"`
	InsertedAt        string                 `json:"inserted_at,omitempty"`
	UpdatedAt         string                 `json:"updated_at,omitempty"`
}

func (a apiKeyAPI) toProps(projectRef string) APIKeyProperties {
	return APIKeyProperties{
		ID: a.ID, ProjectRef: projectRef, Type: a.Type, Name: a.Name,
		Description: a.Description, APIKey: a.APIKey, Prefix: a.Prefix, Hash: a.Hash,
		SecretJWTTemplate: a.SecretJWTTemplate,
		InsertedAt:        a.InsertedAt, UpdatedAt: a.UpdatedAt,
	}
}

func (a *APIKey) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var p APIKeyProperties
	if err := json.Unmarshal(req.Properties, &p); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if p.ProjectRef == "" || p.Name == "" || p.Type == "" {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, "projectRef, name, type are required"), nil
	}
	body := map[string]any{"name": p.Name, "type": p.Type}
	if p.Description != "" {
		body["description"] = p.Description
	}
	if p.SecretJWTTemplate != nil {
		body["secret_jwt_template"] = p.SecretJWTTemplate
	}
	var apiResp apiKeyAPI
	if err := a.Client.Do(ctx, supatransport.Request{
		Method: "POST",
		Path:   "/v1/projects/" + p.ProjectRef + "/api-keys",
		Query:  map[string]string{"reveal": "true"},
		Body:   body,
	}, &apiResp); err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	if apiResp.ID == "" {
		return prov.FailCreate(resource.OperationErrorCodeServiceInternalError, "create response missing id"), nil
	}
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationCreate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           prov.JoinTwoPart(p.ProjectRef, apiResp.ID),
			ResourceProperties: prov.MustMarshal(apiResp.toProps(p.ProjectRef)),
		},
	}, nil
}

func (a *APIKey) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	project, id, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	var apiResp apiKeyAPI
	if err := a.Client.Do(ctx, supatransport.Request{
		Method: "GET",
		Path:   "/v1/projects/" + project + "/api-keys/" + id,
		Query:  map[string]string{"reveal": "true"},
	}, &apiResp); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(prov.MustMarshal(apiResp.toProps(project)))}, nil
}

func (a *APIKey) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	project, id, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	var desired APIKeyProperties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	body := map[string]any{}
	if desired.Name != "" {
		body["name"] = desired.Name
	}
	if desired.Description != "" {
		body["description"] = desired.Description
	}
	if desired.SecretJWTTemplate != nil {
		body["secret_jwt_template"] = desired.SecretJWTTemplate
	}
	if len(body) == 0 {
		return &resource.UpdateResult{
			ProgressResult: &resource.ProgressResult{Operation: resource.OperationUpdate, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
		}, nil
	}
	var apiResp apiKeyAPI
	if err := a.Client.Do(ctx, supatransport.Request{
		Method: "PATCH",
		Path:   "/v1/projects/" + project + "/api-keys/" + id,
		Body:   body,
	}, &apiResp); err != nil {
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationUpdate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           req.NativeID,
			ResourceProperties: prov.MustMarshal(apiResp.toProps(project)),
		},
	}, nil
}

func (a *APIKey) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	project, id, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailDelete(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if err := a.Client.Do(ctx, supatransport.Request{
		Method: "DELETE",
		Path:   "/v1/projects/" + project + "/api-keys/" + id,
	}, nil); err != nil {
		if supatransport.IsNotFound(err) {
			return prov.SuccessDelete(req.NativeID), nil
		}
		return prov.FailDelete(supatransport.ClassifyError(err), err.Error()), nil
	}
	return prov.SuccessDelete(req.NativeID), nil
}

func (a *APIKey) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	_ = ctx
	return &resource.StatusResult{
		ProgressResult: &resource.ProgressResult{Operation: resource.OperationCheckStatus, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
	}, nil
}

func (a *APIKey) List(ctx context.Context, _ *resource.ListRequest) (*resource.ListResult, error) {
	var ids []string
	for _, projectID := range prov.ProjectIDs(ctx, a.Client, a.ProjectScope) {
		var keys []apiKeyAPI
		if err := a.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects/" + projectID + "/api-keys"}, &keys); err != nil {
			continue
		}
		for _, k := range keys {
			if k.ID != "" {
				ids = append(ids, prov.JoinTwoPart(projectID, k.ID))
			}
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
