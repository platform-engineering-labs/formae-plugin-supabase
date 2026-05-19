// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	// Side-effect imports register one resource type each via init().
	_ "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/auth"
	_ "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/config"
	_ "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/functions"
	_ "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/platform"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/model"
	"github.com/platform-engineering-labs/formae/pkg/plugin"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const envAccessToken = "SUPABASE_ACCESS_TOKEN"

// ErrNotImplemented is returned for resource types not handled by this plugin.
var ErrNotImplemented = errors.New("resource type not implemented")

// =============================================================================
// Plugin
// =============================================================================

// Plugin implements plugin.ResourcePlugin. CRUD/Status/List all delegate to
// the per-resource Provisioner registered by the side-effect imports above.
type Plugin struct {
	mu     sync.Mutex
	client *supatransport.Client
	target *registry.TargetConfig
}

var _ plugin.ResourcePlugin = &Plugin{}

func (p *Plugin) getDeps(targetCfg json.RawMessage) (*supatransport.Client, *registry.TargetConfig, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		return p.client, p.target, nil
	}
	cfg, err := parseTargetConfig(targetCfg)
	if err != nil {
		return nil, nil, err
	}
	token := os.Getenv(envAccessToken)
	if token == "" {
		return nil, nil, fmt.Errorf("%s must be set", envAccessToken)
	}
	c, err := supatransport.NewClient(supatransport.Config{
		BaseURL:     cfg.BaseURL,
		AccessToken: token,
	})
	if err != nil {
		return nil, nil, err
	}
	p.client = c
	p.target = cfg
	return p.client, p.target, nil
}

func parseTargetConfig(data json.RawMessage) (*registry.TargetConfig, error) {
	var cfg registry.TargetConfig
	if len(data) == 0 {
		return &cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid target config: %w", err)
	}
	return &cfg, nil
}

// dispatch resolves the Provisioner for a resource type. Returns nil + an
// error code suitable for the caller to embed in a failure result.
func (p *Plugin) dispatch(resourceType string, targetCfg json.RawMessage) (prov.Provisioner, resource.OperationErrorCode, error) {
	factory, ok := registry.GetFactory(resourceType)
	if !ok {
		return nil, resource.OperationErrorCodeInvalidRequest,
			fmt.Errorf("unsupported resource type %q", resourceType)
	}
	c, t, err := p.getDeps(targetCfg)
	if err != nil {
		return nil, resource.OperationErrorCodeInvalidCredentials, err
	}
	return factory(c, t), "", nil
}

// =============================================================================
// Configuration
// =============================================================================

func (p *Plugin) RateLimit() model.RateLimitConfig {
	return model.RateLimitConfig{
		Scope:                            model.RateLimitScopeNamespace,
		MaxRequestsPerSecondForNamespace: 2,
	}
}

func (p *Plugin) DiscoveryFilters() []model.MatchFilter { return nil }

func (p *Plugin) LabelConfig() model.LabelConfig {
	return model.LabelConfig{
		DefaultQuery: "$.name",
		ResourceOverrides: map[string]string{
			"SUPABASE::Functions::EdgeFunction":     "$.slug",
			"SUPABASE::Functions::Secret":           "$.name",
			"SUPABASE::Config::AuthSettings":        "$.projectRef",
			"SUPABASE::Config::APISettings":         "$.projectRef",
			"SUPABASE::Config::DatabaseSettings":    "$.projectRef",
			"SUPABASE::Config::NetworkRestriction":  "$.projectRef",
		},
	}
}

// =============================================================================
// CRUD dispatch
// =============================================================================

func (p *Plugin) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	pr, code, err := p.dispatch(req.ResourceType, req.TargetConfig)
	if err != nil {
		return prov.FailCreate(code, err.Error()), wrapErr(err)
	}
	return pr.Create(ctx, req)
}

func (p *Plugin) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	pr, code, err := p.dispatch(req.ResourceType, req.TargetConfig)
	if err != nil {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: code}, wrapErr(err)
	}
	return pr.Read(ctx, req)
}

func (p *Plugin) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	pr, code, err := p.dispatch(req.ResourceType, req.TargetConfig)
	if err != nil {
		return prov.FailUpdate(code, err.Error()), wrapErr(err)
	}
	return pr.Update(ctx, req)
}

func (p *Plugin) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	pr, code, err := p.dispatch(req.ResourceType, req.TargetConfig)
	if err != nil {
		return prov.FailDelete(code, err.Error()), wrapErr(err)
	}
	return pr.Delete(ctx, req)
}

func (p *Plugin) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	pr, code, err := p.dispatch(req.ResourceType, req.TargetConfig)
	if err != nil {
		return prov.FailStatus(code, err.Error()), wrapErr(err)
	}
	return pr.Status(ctx, req)
}

func (p *Plugin) List(ctx context.Context, req *resource.ListRequest) (*resource.ListResult, error) {
	pr, _, err := p.dispatch(req.ResourceType, req.TargetConfig)
	if err != nil {
		return &resource.ListResult{NativeIDs: []string{}}, nil
	}
	return pr.List(ctx, req)
}

// wrapErr surfaces only "unsupported resource type" as a hard error so that
// invalid resource types stop reconcile loops. Credentials errors are
// reported via the result and not as an error return.
func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotImplemented) {
		return err
	}
	// Other dispatch errors (credentials, target config) are signalled via
	// the result's ErrorCode; the SDK does not treat a nil error here as
	// success because the OperationStatus on the result is Failure.
	return nil
}
