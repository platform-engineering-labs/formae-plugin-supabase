// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

// Package registry holds the per-resource-type Provisioner factory map.
//
// Each resource file in pkg/resources/{platform,auth,functions,config}
// registers itself via init() at package import time. The main plugin
// then looks up the factory by resource type and constructs a Provisioner
// bound to the live API client + target config.
package registry

import (
	"fmt"
	"sync"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

// TargetConfig is the deployment-level config carried in the forma target.
// Kept here (rather than in package main) so resource packages can depend on
// it without an import cycle.
type TargetConfig struct {
	BaseURL string `json:"BaseUrl"`
	// ProjectRef optionally scopes discovery + List() calls to a single
	// Supabase project. Without it, List() walks every project the token
	// can see (slow on large orgs; can time out the conformance harness's
	// 2-minute discovery window).
	ProjectRef string `json:"ProjectRef"`
}

// Factory builds a Provisioner bound to a live client and target.
type Factory func(client *supatransport.Client, cfg *TargetConfig) prov.Provisioner

type registration struct {
	factory    Factory
	operations []resource.Operation
}

var (
	mu            sync.RWMutex
	registrations = make(map[string]*registration)
)

// Register associates a Supabase resource type with its supported operations
// and a Factory. Called from init() at package load.
func Register(resourceType string, operations []resource.Operation, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registrations[resourceType]; exists {
		panic(fmt.Sprintf("duplicate registration for %q", resourceType))
	}
	registrations[resourceType] = &registration{factory: factory, operations: operations}
}

// GetFactory returns the factory for a resource type.
func GetFactory(resourceType string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	r, ok := registrations[resourceType]
	if !ok {
		return nil, false
	}
	return r.factory, true
}

// GetOperations returns the registered operations for a resource type.
func GetOperations(resourceType string) []resource.Operation {
	mu.RLock()
	defer mu.RUnlock()
	r, ok := registrations[resourceType]
	if !ok {
		return nil
	}
	return r.operations
}

// ResourceTypes returns every registered type. Useful for plugin metadata.
func ResourceTypes() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registrations))
	for t := range registrations {
		out = append(out, t)
	}
	return out
}

// Has reports whether a type is registered.
func Has(resourceType string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := registrations[resourceType]
	return ok
}
