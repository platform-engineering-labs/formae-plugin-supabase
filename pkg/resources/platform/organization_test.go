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

func TestOrganization_Create(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"org-id","slug":"my-org","name":"my-org"}`)
	})
	defer srv.Close()
	o := &Organization{Client: c}
	props, _ := json.Marshal(OrganizationProperties{Name: "my-org"})
	res, _ := o.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.NativeID != "my-org" {
		t.Fatalf("NativeID = %q", res.ProgressResult.NativeID)
	}
}

func TestOrganization_UpdateNoOp(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not call API") })
	defer srv.Close()
	o := &Organization{Client: c}
	res, _ := o.Update(context.Background(), &resource.UpdateRequest{NativeID: "my-org"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestOrganization_List(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"id":"o1","slug":"a"},{"id":"o2","slug":"b"}]`)
	})
	defer srv.Close()
	o := &Organization{Client: c}
	res, _ := o.List(context.Background(), &resource.ListRequest{})
	if len(res.NativeIDs) != 2 || res.NativeIDs[0] != "a" {
		t.Fatalf("ids = %v", res.NativeIDs)
	}
}
