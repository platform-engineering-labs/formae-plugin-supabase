// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package functions

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypeSecret = "SUPABASE::Functions::Secret"

func init() {
	registry.Register(
		ResourceTypeSecret,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationDelete,
			resource.OperationList,
		},
		func(c *supatransport.Client, cfg *registry.TargetConfig) prov.Provisioner {
			return &Secret{Client: c, ProjectScope: cfg.ProjectRef}
		},
	)
}

// Secret — SUPABASE::Functions::Secret.
//
// The Supabase API only exposes bulk endpoints. We expose one Forma resource
// per secret name and bridge to the bulk endpoints by POSTing/DELETEing a
// single-item array.
//
// Native id: `{project_ref}/{name}`.
//
// Caveat: secret values are write-only. The list endpoint returns names but
// not values, so drift on the value cannot be detected.
type Secret struct {
	Client       *supatransport.Client
	ProjectScope string
}

type SecretProperties struct {
	ProjectRef string `json:"projectRef,omitempty"`
	Name       string `json:"name,omitempty"`
	Value      string `json:"value,omitempty"`
}

func (s *Secret) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var p SecretProperties
	if err := json.Unmarshal(req.Properties, &p); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if p.ProjectRef == "" || p.Name == "" || p.Value == "" {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, "projectRef, name, value are required"), nil
	}
	if strings.HasPrefix(p.Name, "SUPABASE_") {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, "secret name must not start with SUPABASE_"), nil
	}
	body := []map[string]string{{"name": p.Name, "value": p.Value}}
	if err := s.Client.Do(ctx, supatransport.Request{
		Method: "POST",
		Path:   "/v1/projects/" + p.ProjectRef + "/secrets",
		Body:   body,
	}, nil); err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationCreate,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        prov.JoinTwoPart(p.ProjectRef, p.Name),
		},
	}, nil
}

func (s *Secret) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	project, name, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	var secrets []struct {
		Name string `json:"name"`
	}
	if err := s.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + project + "/secrets",
	}, &secrets); err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	for _, sec := range secrets {
		if sec.Name == name {
			out := prov.MustMarshal(SecretProperties{ProjectRef: project, Name: name})
			return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(out)}, nil
		}
	}
	return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
}

func (s *Secret) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	project, name, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	var desired SecretProperties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if desired.Value == "" {
		return &resource.UpdateResult{
			ProgressResult: &resource.ProgressResult{Operation: resource.OperationUpdate, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
		}, nil
	}
	body := []map[string]string{{"name": name, "value": desired.Value}}
	if err := s.Client.Do(ctx, supatransport.Request{
		Method: "POST",
		Path:   "/v1/projects/" + project + "/secrets",
		Body:   body,
	}, nil); err != nil {
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{Operation: resource.OperationUpdate, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
	}, nil
}

func (s *Secret) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	project, name, err := prov.ParseTwoPart(req.NativeID)
	if err != nil {
		return prov.FailDelete(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	body := []string{name}
	if err := s.Client.Do(ctx, supatransport.Request{
		Method: "DELETE",
		Path:   "/v1/projects/" + project + "/secrets",
		Body:   body,
	}, nil); err != nil {
		if supatransport.IsNotFound(err) {
			return prov.SuccessDelete(req.NativeID), nil
		}
		return prov.FailDelete(supatransport.ClassifyError(err), err.Error()), nil
	}
	return prov.SuccessDelete(req.NativeID), nil
}

func (s *Secret) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	_ = ctx
	return &resource.StatusResult{
		ProgressResult: &resource.ProgressResult{Operation: resource.OperationCheckStatus, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
	}, nil
}

func (s *Secret) List(ctx context.Context, _ *resource.ListRequest) (*resource.ListResult, error) {
	var ids []string
	for _, projectID := range prov.ProjectIDs(ctx, s.Client, s.ProjectScope) {
		var secrets []struct {
			Name string `json:"name"`
		}
		if err := s.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects/" + projectID + "/secrets"}, &secrets); err != nil {
			continue
		}
		for _, sec := range secrets {
			if sec.Name != "" {
				ids = append(ids, prov.JoinTwoPart(projectID, sec.Name))
			}
		}
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
