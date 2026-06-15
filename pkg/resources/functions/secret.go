// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package functions

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypeSecrets = "SUPABASE::Functions::Secrets"

func init() {
	registry.Register(
		ResourceTypeSecrets,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationDelete,
			resource.OperationList,
		},
		func(c *supatransport.Client, cfg *registry.TargetConfig) prov.Provisioner {
			return &Secrets{Client: c, ProjectScope: cfg.ProjectRef}
		},
	)
}

// Secrets — SUPABASE::Functions::Secrets.
//
// The Supabase secrets endpoint is a single bulk bag per project: there is no
// per-secret resource server-side. We model the whole bag as one Forma
// resource holding a name→value map, so every mutation is a single atomic bulk
// call. This avoids the lost-update race that per-secret resources hit when
// formae applies them concurrently (each concurrent single-item POST does a
// read-modify-write on the shared bag, and the last writer wins).
//
// Native id: the bare project ref — one Secrets bag per project.
//
// Caveat: secret values are write-only. The list endpoint returns names but
// not values, so drift on a value cannot be detected.
type Secrets struct {
	Client       *supatransport.Client
	ProjectScope string
}

type SecretsProperties struct {
	ProjectRef string            `json:"projectRef,omitempty"`
	Values     map[string]string `json:"values,omitempty"`
}

// reservedPrefix marks secret names the platform injects and manages itself.
const reservedPrefix = "SUPABASE_"

// upsertBody turns a name→value map into the sorted bulk-POST payload.
func upsertBody(values map[string]string) []map[string]string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	body := make([]map[string]string, 0, len(names))
	for _, name := range names {
		body = append(body, map[string]string{"name": name, "value": values[name]})
	}
	return body
}

func (s *Secrets) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var p SecretsProperties
	if err := json.Unmarshal(req.Properties, &p); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if p.ProjectRef == "" || len(p.Values) == 0 {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, "projectRef and at least one value are required"), nil
	}
	for name := range p.Values {
		if strings.HasPrefix(name, reservedPrefix) {
			return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, "secret name must not start with "+reservedPrefix), nil
		}
	}
	if err := s.Client.Do(ctx, supatransport.Request{
		Method: "POST",
		Path:   "/v1/projects/" + p.ProjectRef + "/secrets",
		Body:   upsertBody(p.Values),
	}, nil); err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationCreate,
			OperationStatus: resource.OperationStatusSuccess,
			NativeID:        p.ProjectRef,
		},
	}, nil
}

func (s *Secrets) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	project := req.NativeID
	var secrets []struct {
		Name string `json:"name"`
	}
	if err := s.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + project + "/secrets",
	}, &secrets); err != nil {
		if supatransport.IsNotFound(err) {
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	// The secrets endpoint stays 200 (with reserved SUPABASE_* entries) even
	// after every managed secret is deleted out-of-band. The bag exists only
	// while it holds at least one managed (non-reserved) name; otherwise report
	// NotFound so formae clears it from inventory.
	for _, sec := range secrets {
		if sec.Name != "" && !strings.HasPrefix(sec.Name, reservedPrefix) {
			// Values are write-only — the API never returns them, so we report
			// only the project ref. The framework does not diff write-only fields.
			out := prov.MustMarshal(SecretsProperties{ProjectRef: project})
			return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(out)}, nil
		}
	}
	return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
}

func (s *Secrets) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	project := req.NativeID
	var prior, desired SecretsProperties
	if err := json.Unmarshal(req.PriorProperties, &prior); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	for name := range desired.Values {
		if strings.HasPrefix(name, reservedPrefix) {
			return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, "secret name must not start with "+reservedPrefix), nil
		}
	}
	// Names present before but gone from the desired bag are removed server-side.
	var removed []string
	for name := range prior.Values {
		if _, keep := desired.Values[name]; !keep {
			removed = append(removed, name)
		}
	}
	if len(removed) > 0 {
		sort.Strings(removed)
		if err := s.Client.Do(ctx, supatransport.Request{
			Method: "DELETE",
			Path:   "/v1/projects/" + project + "/secrets",
			Body:   removed,
		}, nil); err != nil && !supatransport.IsNotFound(err) {
			return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
		}
	}
	if len(desired.Values) > 0 {
		if err := s.Client.Do(ctx, supatransport.Request{
			Method: "POST",
			Path:   "/v1/projects/" + project + "/secrets",
			Body:   upsertBody(desired.Values),
		}, nil); err != nil {
			return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
		}
	}
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{Operation: resource.OperationUpdate, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
	}, nil
}

func (s *Secrets) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	project := req.NativeID
	var secrets []struct {
		Name string `json:"name"`
	}
	if err := s.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + project + "/secrets",
	}, &secrets); err != nil {
		if supatransport.IsNotFound(err) {
			return prov.SuccessDelete(req.NativeID), nil
		}
		return prov.FailDelete(supatransport.ClassifyError(err), err.Error()), nil
	}
	var names []string
	for _, sec := range secrets {
		if sec.Name != "" && !strings.HasPrefix(sec.Name, reservedPrefix) {
			names = append(names, sec.Name)
		}
	}
	if len(names) > 0 {
		sort.Strings(names)
		if err := s.Client.Do(ctx, supatransport.Request{
			Method: "DELETE",
			Path:   "/v1/projects/" + project + "/secrets",
			Body:   names,
		}, nil); err != nil && !supatransport.IsNotFound(err) {
			return prov.FailDelete(supatransport.ClassifyError(err), err.Error()), nil
		}
	}
	return prov.SuccessDelete(req.NativeID), nil
}

func (s *Secrets) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	_ = ctx
	return &resource.StatusResult{
		ProgressResult: &resource.ProgressResult{Operation: resource.OperationCheckStatus, OperationStatus: resource.OperationStatusSuccess, NativeID: req.NativeID},
	}, nil
}

func (s *Secrets) List(ctx context.Context, _ *resource.ListRequest) (*resource.ListResult, error) {
	// One bag per project; the bag exists whenever the project does, so the
	// native id is just the project ref.
	ids := append([]string{}, prov.ProjectIDs(ctx, s.Client, s.ProjectScope)...)
	return &resource.ListResult{NativeIDs: ids}, nil
}
