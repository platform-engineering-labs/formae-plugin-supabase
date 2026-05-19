// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

//go:build unit

package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

func TestBranch_Create(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/parent1/branches" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"CREATING_PROJECT"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	props, _ := json.Marshal(BranchProperties{ParentProjectRef: "parent1", BranchName: "feat"})
	res, _ := b.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.NativeID != "parent1/branch-uuid" {
		t.Fatalf("NativeID = %q", res.ProgressResult.NativeID)
	}
}

func TestBranch_Status_FunctionsDeployed(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"FUNCTIONS_DEPLOYED"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Status(context.Background(), &resource.StatusRequest{RequestID: "parent1/branch-uuid"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestBranch_Status_Failure(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"MIGRATIONS_FAILED"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Status(context.Background(), &resource.StatusRequest{RequestID: "parent1/branch-uuid"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusFailure {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestBranch_Read_UsesGlobalEndpoint(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/branches/branch-uuid" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"FUNCTIONS_DEPLOYED"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Read(context.Background(), &resource.ReadRequest{NativeID: "parent1/branch-uuid"})
	if res.ErrorCode != "" {
		t.Fatalf("ErrorCode %v", res.ErrorCode)
	}
}
