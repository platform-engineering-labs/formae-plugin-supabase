// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: FSL-1.1-ALv2

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/prov"
	"github.com/platform-engineering-labs/formae-plugin-supabase/pkg/resources/registry"
	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

const ResourceTypeProject = "SUPABASE::Platform::Project"

func init() {
	registry.Register(
		ResourceTypeProject,
		[]resource.Operation{
			resource.OperationCreate,
			resource.OperationRead,
			resource.OperationUpdate,
			resource.OperationDelete,
			resource.OperationCheckStatus,
			resource.OperationList,
		},
		func(c *supatransport.Client, _ *registry.TargetConfig) prov.Provisioner {
			return &Project{Client: c}
		},
	)
}

// Project — SUPABASE::Platform::Project.
//
// API mapping:
//
//	POST   /v1/projects                                Create (async)
//	GET    /v1/projects/{ref}                          Read / Status
//	PATCH  /v1/projects/{ref}                          Update metadata
//	DELETE /v1/projects/{ref}                          Delete (cascades config)
//	GET    /v1/projects                                List
//	PATCH  /v1/projects/{ref}/config/auth              Auth config block
//	PATCH  /v1/projects/{ref}/postgrest                API config block
//	PUT    /v1/projects/{ref}/config/database/postgres Database config block
//	PATCH  /v1/projects/{ref}/network-restrictions     Network restriction
//
// Project-scoped configuration (auth, api, database, networkRestriction) is
// nested in the Project resource. The lifecycle is owned by Project: Delete
// cascades because deleting the project removes all config server-side.
// This avoids the prior tombstone hack required to model these as standalone
// CRUD resources against an API that exposes no DELETE for them.
type Project struct {
	Client *supatransport.Client
}

// ConfigBlock is the wire shape for any nested project-scoped config.
type ConfigBlock struct {
	Settings map[string]any `json:"settings,omitempty"`
}

// ProjectProperties is the Forma-facing shape (matches PKL field names).
type ProjectProperties struct {
	ID                  string `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	OrganizationID      string `json:"organizationId,omitempty"`
	Region              string `json:"region,omitempty"`
	DBPass              string `json:"dbPass,omitempty"`
	Plan                string `json:"plan,omitempty"`
	DesiredInstanceSize string `json:"desiredInstanceSize,omitempty"`
	Status              string `json:"status,omitempty"`
	CreatedAt           string `json:"createdAt,omitempty"`

	Auth               *ConfigBlock `json:"auth,omitempty"`
	API                *ConfigBlock `json:"api,omitempty"`
	Database           *ConfigBlock `json:"database,omitempty"`
	NetworkRestriction *ConfigBlock `json:"networkRestriction,omitempty"`
}

// projectAPI is the Supabase-API-facing shape (snake_case).
type projectAPI struct {
	ID                  string `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	OrganizationID      string `json:"organization_id,omitempty"`
	Region              string `json:"region,omitempty"`
	DBPass              string `json:"db_pass,omitempty"`
	Plan                string `json:"plan,omitempty"`
	DesiredInstanceSize string `json:"desired_instance_size,omitempty"`
	Status              string `json:"status,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
}

func (a projectAPI) toProps() ProjectProperties {
	return ProjectProperties{
		ID:                  a.ID,
		Name:                a.Name,
		OrganizationID:      a.OrganizationID,
		Region:              a.Region,
		DBPass:              a.DBPass,
		Plan:                a.Plan,
		DesiredInstanceSize: a.DesiredInstanceSize,
		Status:              a.Status,
		CreatedAt:           a.CreatedAt,
	}
}

const (
	projectStatusActive     = "ACTIVE_HEALTHY"
	projectStatusInactive   = "INACTIVE"
	projectStatusInitFailed = "INIT_FAILED"
	projectStatusRemoved    = "REMOVED"
)

// isProjectGone reports whether an API error from a /v1/projects/{ref}
// call means the project no longer exists. Scoped to this file (not
// pkg/transport/supabase.IsNotFound) because the 400/403 message patterns
// overlap with real "bad request" / "permission revoked" responses on
// other endpoints, where treating them as NotFound would silently nuke
// inventory on the next sync.
//
// Supabase's project lifecycle returns three different non-404 shapes
// for a deleted project, depending on how far the reaper has progressed:
//
//   - HTTP 400 'Resource has been removed'   (immediately after DELETE)
//   - HTTP 403 'necessary privileges'        (post-reap auth-check fires)
//   - HTTP 404                               (terminal state, classified
//                                             by supatransport.IsNotFound)
func isProjectGone(err error) bool {
	var apiErr *supatransport.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case 400:
		return containsFold(apiErr.Message, "Resource has been removed") ||
			containsFold(apiErr.Message, "is being removed")
	case 403:
		return containsFold(apiErr.Message, "necessary privileges")
	}
	return false
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// configBinding ties a nested block to its API endpoint.
type configBinding struct {
	name        string // identifier used in cache + error messages
	pathSuffix  string
	writeMethod string
	get         func(*ProjectProperties) *ConfigBlock
	set         func(*ProjectProperties, *ConfigBlock)
	// keysSlot returns the address of the per-binding keys field on a
	// projectManagedKeys so applyConfigBlocks can record incrementally.
	keysSlot func(*projectManagedKeys) *map[string]struct{}
}

var configBindings = []configBinding{
	{
		name: "auth", pathSuffix: "/config/auth", writeMethod: "PATCH",
		get:      func(p *ProjectProperties) *ConfigBlock { return p.Auth },
		set:      func(p *ProjectProperties, b *ConfigBlock) { p.Auth = b },
		keysSlot: func(mk *projectManagedKeys) *map[string]struct{} { return &mk.auth },
	},
	{
		name: "api", pathSuffix: "/postgrest", writeMethod: "PATCH",
		get:      func(p *ProjectProperties) *ConfigBlock { return p.API },
		set:      func(p *ProjectProperties, b *ConfigBlock) { p.API = b },
		keysSlot: func(mk *projectManagedKeys) *map[string]struct{} { return &mk.api },
	},
	{
		name: "database", pathSuffix: "/config/database/postgres", writeMethod: "PUT",
		get:      func(p *ProjectProperties) *ConfigBlock { return p.Database },
		set:      func(p *ProjectProperties, b *ConfigBlock) { p.Database = b },
		keysSlot: func(mk *projectManagedKeys) *map[string]struct{} { return &mk.database },
	},
	{
		name: "networkRestriction", pathSuffix: "/network-restrictions", writeMethod: "PATCH",
		get:      func(p *ProjectProperties) *ConfigBlock { return p.NetworkRestriction },
		set:      func(p *ProjectProperties, b *ConfigBlock) { p.NetworkRestriction = b },
		keysSlot: func(mk *projectManagedKeys) *map[string]struct{} { return &mk.networkRestriction },
	},
}

// managedKeysCache tracks which keys of each config block the forma manages
// for a given project. Read uses it to filter GET responses so unmanaged
// cloud-side fields (jwt_secret, db_pool, ...) don't surface as drift.
//
// Process-local: a plugin restart loses the cache. Read then returns the
// full cloud config until the next Update repopulates it. Self-healing,
// at the cost of one bogus drift reconcile after a restart.
//
// Caveats worth knowing before relying on this for production rollouts:
//
//   - Restart amnesia: agents that respawn the plugin between Update and
//     the next Read will see drift until the user re-applies. Acceptable
//     today; if it becomes a problem, persist the cache to disk under the
//     agent state dir.
//   - Mirror over GET-back: Update writes desired blocks straight into the
//     response (see mirrorDesiredBlocks) instead of GET-ing each block
//     after PATCH. This trades latency for accuracy — server-side clamping
//     (e.g. Postgres rounding `max_connections`) is invisible until the
//     next reconcile when readConfigBlocks fetches fresh values.
type projectManagedKeys struct {
	auth, api, database, networkRestriction map[string]struct{}
}

var managedKeysCache sync.Map // projectID → *projectManagedKeys

// pendingCreateConfig holds the desired config blocks captured during Create
// so Status can apply them once the project reaches ACTIVE_HEALTHY. PATCH
// against a still-transitioning project gets rejected by the API.
//
// Lifecycle hole: this map is also process-local. If the plugin process
// restarts between Create returning InProgress and Status reaching the
// ACTIVE branch for the first time, the pending config silently no-ops.
// The user's next reconcile will detect drift (forma has blocks, Read
// returns none) and trigger an Update — recoverable, just slower.
var pendingCreateConfig sync.Map // projectID → ProjectProperties

func keysOf(m map[string]any) map[string]struct{} {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

// recordManagedKeys replaces the cache entry for projectID with the keys
// from every non-nil block in p. Use at Create time when no blocks have
// been PATCHed yet; for Update, prefer recordManagedKeysForBlock so a
// partial-success Update doesn't leave a stale cache.
func recordManagedKeys(projectID string, p *ProjectProperties) {
	mk := &projectManagedKeys{}
	for _, b := range configBindings {
		blk := b.get(p)
		if blk == nil {
			continue
		}
		*b.keysSlot(mk) = keysOf(blk.Settings)
	}
	managedKeysCache.Store(projectID, mk)
}

// recordManagedKeysForBlock merges the keys from a single block into the
// existing cache entry. Called per successful PATCH inside
// applyConfigBlocks so a deadline-exceeded retry doesn't wipe state for
// blocks already committed in this pass.
func recordManagedKeysForBlock(projectID string, b configBinding, blk *ConfigBlock) {
	if blk == nil {
		return
	}
	prev, _ := managedKeysCache.Load(projectID)
	mk, _ := prev.(*projectManagedKeys)
	if mk == nil {
		mk = &projectManagedKeys{}
	}
	*b.keysSlot(mk) = keysOf(blk.Settings)
	managedKeysCache.Store(projectID, mk)
}

func filterToKeys(in map[string]any, keep map[string]struct{}) map[string]any {
	if len(keep) == 0 {
		return nil
	}
	out := make(map[string]any, len(keep))
	// Sort for deterministic output.
	ks := make([]string, 0, len(keep))
	for k := range keep {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		if v, ok := in[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// readConfigBlocks issues one GET per binding and returns the populated
// blocks filtered by the managed-keys cache.
func (p *Project) readConfigBlocks(ctx context.Context, projectID string, out *ProjectProperties) {
	mk, _ := managedKeysCache.Load(projectID)
	keys, _ := mk.(*projectManagedKeys)
	for _, b := range configBindings {
		var keep map[string]struct{}
		if keys != nil {
			keep = *b.keysSlot(keys)
		}
		if len(keep) == 0 {
			continue
		}
		var resp map[string]any
		if err := p.Client.Do(ctx, supatransport.Request{
			Method: "GET",
			Path:   "/v1/projects/" + projectID + b.pathSuffix,
		}, &resp); err != nil {
			continue
		}
		if filtered := filterToKeys(resp, keep); filtered != nil {
			b.set(out, &ConfigBlock{Settings: filtered})
		}
	}
}

// applyConfigBlocks writes every non-nil block to its endpoint, recording
// managed keys incrementally so a deadline-exceeded retry doesn't clobber
// state for blocks already committed in this pass.
func (p *Project) applyConfigBlocks(ctx context.Context, projectID string, props *ProjectProperties) error {
	for _, b := range configBindings {
		block := b.get(props)
		if block == nil || len(block.Settings) == 0 {
			continue
		}
		if err := p.Client.Do(ctx, supatransport.Request{
			Method: b.writeMethod,
			Path:   "/v1/projects/" + projectID + b.pathSuffix,
			Body:   block.Settings,
		}, nil); err != nil {
			return fmt.Errorf("%s config patch: %w", b.name, err)
		}
		recordManagedKeysForBlock(projectID, b, block)
	}
	return nil
}

func (p *Project) Create(ctx context.Context, req *resource.CreateRequest) (*resource.CreateResult, error) {
	var pp ProjectProperties
	if err := json.Unmarshal(req.Properties, &pp); err != nil {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}
	if pp.Name == "" || pp.OrganizationID == "" || pp.Region == "" || pp.DBPass == "" {
		return prov.FailCreate(resource.OperationErrorCodeInvalidRequest,
			"name, organizationId, region, dbPass are required"), nil
	}
	body := map[string]any{
		"name":            pp.Name,
		"organization_id": pp.OrganizationID,
		"region":          pp.Region,
		"db_pass":         pp.DBPass,
	}
	if pp.Plan != "" {
		body["plan"] = pp.Plan
	}
	if pp.DesiredInstanceSize != "" {
		body["desired_instance_size"] = pp.DesiredInstanceSize
	}
	var apiResp projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "POST", Path: "/v1/projects", Body: body,
	}, &apiResp); err != nil {
		return prov.FailCreate(supatransport.ClassifyError(err), err.Error()), nil
	}
	if apiResp.ID == "" {
		return prov.FailCreate(resource.OperationErrorCodeServiceInternalError,
			"create response missing project id"), nil
	}
	// Stash desired config — Status drains and applies once project is ACTIVE.
	pendingCreateConfig.Store(apiResp.ID, pp)
	recordManagedKeys(apiResp.ID, &pp)
	return &resource.CreateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationCreate,
			OperationStatus: resource.OperationStatusInProgress,
			NativeID:        apiResp.ID,
			RequestID:       apiResp.ID,
		},
	}, nil
}

func (p *Project) Read(ctx context.Context, req *resource.ReadRequest) (*resource.ReadResult, error) {
	if req.NativeID == "" {
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeInvalidRequest}, nil
	}
	var apiResp projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + req.NativeID,
	}, &apiResp); err != nil {
		if supatransport.IsNotFound(err) || isProjectGone(err) {
			managedKeysCache.Delete(req.NativeID)
			pendingCreateConfig.Delete(req.NativeID)
			return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
		}
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: supatransport.ClassifyError(err)}, nil
	}
	// Supabase keeps a project visible via GET for a brief window after
	// DELETE, returning Status=REMOVED. Treat that as gone so formae's
	// sync prunes the inventory after our (or an OOB) delete.
	if apiResp.Status == projectStatusRemoved {
		managedKeysCache.Delete(req.NativeID)
		pendingCreateConfig.Delete(req.NativeID)
		return &resource.ReadResult{ResourceType: req.ResourceType, ErrorCode: resource.OperationErrorCodeNotFound}, nil
	}
	props := apiResp.toProps()
	p.readConfigBlocks(ctx, req.NativeID, &props)
	return &resource.ReadResult{ResourceType: req.ResourceType, Properties: string(prov.MustMarshal(props))}, nil
}

func (p *Project) Update(ctx context.Context, req *resource.UpdateRequest) (*resource.UpdateResult, error) {
	if req.NativeID == "" {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, "native id required"), nil
	}
	var desired ProjectProperties
	if err := json.Unmarshal(req.DesiredProperties, &desired); err != nil {
		return prov.FailUpdate(resource.OperationErrorCodeInvalidRequest, err.Error()), nil
	}

	// Cap the whole Update call below the harness's 40s
	// "PluginOperatorMissingInAction" watchdog. Each underlying HTTP call
	// has its own 30s client-level timeout; if Supabase is sitting on a
	// transient state we'd otherwise chain four of them sequentially and
	// blow past two minutes.
	callCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()

	body := map[string]any{}
	if desired.Name != "" {
		body["name"] = desired.Name
	}
	var apiResp projectAPI
	if len(body) > 0 {
		// PATCH /v1/projects/{ref} returns a body whose `id` field is a
		// number rather than the ref string the rest of the API uses
		// (`{"id":1234,"name":"…"}`). Decoding that into projectAPI.ID
		// (string) fails. Discard the response and read fresh below.
		if err := p.Client.Do(callCtx, supatransport.Request{
			Method: "PATCH", Path: "/v1/projects/" + req.NativeID, Body: body,
		}, nil); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return p.inProgressUpdate(req.NativeID, "project metadata patch deadline; will retry"), nil
			}
			return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
		}
	}
	// Always GET fresh state — gives us a well-formed projectAPI (ref-string
	// id, status, created_at) for the response.
	if err := p.Client.Do(callCtx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + req.NativeID,
	}, &apiResp); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return p.inProgressUpdate(req.NativeID, "project read deadline; will retry"), nil
		}
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}

	if err := p.applyConfigBlocks(callCtx, req.NativeID, &desired); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return p.inProgressUpdate(req.NativeID, "config patch deadline; will retry"), nil
		}
		return prov.FailUpdate(supatransport.ClassifyError(err), err.Error()), nil
	}
	// applyConfigBlocks records managed keys incrementally per success;
	// no need to overwrite the whole cache here.

	// Mirror desired blocks directly into the response. We just PATCHed
	// them, so the cloud is authoritatively at those values; doing another
	// round of GETs only adds latency (and can re-trip the 35s deadline).
	props := apiResp.toProps()
	mirrorDesiredBlocks(&desired, &props)
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:          resource.OperationUpdate,
			OperationStatus:    resource.OperationStatusSuccess,
			NativeID:           req.NativeID,
			ResourceProperties: prov.MustMarshal(props),
		},
	}, nil
}

// mirrorDesiredBlocks copies any non-nil block from src into dst.
func mirrorDesiredBlocks(src, dst *ProjectProperties) {
	if src.Auth != nil {
		dst.Auth = src.Auth
	}
	if src.API != nil {
		dst.API = src.API
	}
	if src.Database != nil {
		dst.Database = src.Database
	}
	if src.NetworkRestriction != nil {
		dst.NetworkRestriction = src.NetworkRestriction
	}
}

func (p *Project) inProgressUpdate(nativeID, msg string) *resource.UpdateResult {
	return &resource.UpdateResult{
		ProgressResult: &resource.ProgressResult{
			Operation:       resource.OperationUpdate,
			OperationStatus: resource.OperationStatusInProgress,
			NativeID:        nativeID,
			RequestID:       nativeID,
			StatusMessage:   msg,
		},
	}
}

func (p *Project) Delete(ctx context.Context, req *resource.DeleteRequest) (*resource.DeleteResult, error) {
	if req.NativeID == "" {
		return prov.FailDelete(resource.OperationErrorCodeInvalidRequest, "native id required"), nil
	}
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "DELETE", Path: "/v1/projects/" + req.NativeID,
	}, nil); err != nil {
		if supatransport.IsNotFound(err) {
			managedKeysCache.Delete(req.NativeID)
			pendingCreateConfig.Delete(req.NativeID)
			return prov.SuccessDelete(req.NativeID), nil
		}
		return prov.FailDelete(supatransport.ClassifyError(err), err.Error()), nil
	}
	managedKeysCache.Delete(req.NativeID)
	pendingCreateConfig.Delete(req.NativeID)
	return prov.SuccessDelete(req.NativeID), nil
}

func (p *Project) Status(ctx context.Context, req *resource.StatusRequest) (*resource.StatusResult, error) {
	ref := req.RequestID
	if ref == "" {
		ref = req.NativeID
	}
	if ref == "" {
		return prov.FailStatus(resource.OperationErrorCodeInvalidRequest, "request id required"), nil
	}
	var apiResp projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{
		Method: "GET", Path: "/v1/projects/" + ref,
	}, &apiResp); err != nil {
		if supatransport.IsNotFound(err) || isProjectGone(err) {
			managedKeysCache.Delete(ref)
			pendingCreateConfig.Delete(ref)
			return &resource.StatusResult{
				ProgressResult: &resource.ProgressResult{
					Operation:       resource.OperationCheckStatus,
					OperationStatus: resource.OperationStatusSuccess,
					NativeID:        ref,
				},
			}, nil
		}
		return prov.FailStatus(supatransport.ClassifyError(err), err.Error()), nil
	}
	switch apiResp.Status {
	case projectStatusActive:
		// Drain any pending config from Create and apply it. Bound the
		// chain of HTTP calls below the harness's PluginOperatorMissingIn
		// Action watchdog; if we hit the deadline, return InProgress so
		// formae's reconciler comes back instead of failing the command.
		drainCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
		defer cancel()
		props := apiResp.toProps()
		if pending, ok := pendingCreateConfig.LoadAndDelete(ref); ok {
			pp := pending.(ProjectProperties)
			if err := p.applyConfigBlocks(drainCtx, ref, &pp); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					// Put it back so the next Status retry can finish the job.
					pendingCreateConfig.Store(ref, pp)
					return &resource.StatusResult{
						ProgressResult: &resource.ProgressResult{
							Operation:       resource.OperationCheckStatus,
							OperationStatus: resource.OperationStatusInProgress,
							NativeID:        ref,
							RequestID:       ref,
							StatusMessage:   "applying pending config; will retry",
						},
					}, nil
				}
				return prov.FailStatus(supatransport.ClassifyError(err), err.Error()), nil
			}
			// Mirror desired into the response: cloud is at those values
			// because applyConfigBlocks succeeded.
			mirrorDesiredBlocks(&pp, &props)
		} else {
			// No pending operation — read whatever cloud says, filtered
			// to the managed-keys cache.
			p.readConfigBlocks(drainCtx, ref, &props)
		}
		return &resource.StatusResult{
			ProgressResult: &resource.ProgressResult{
				Operation:          resource.OperationCheckStatus,
				OperationStatus:    resource.OperationStatusSuccess,
				NativeID:           ref,
				ResourceProperties: prov.MustMarshal(props),
			},
		}, nil
	case projectStatusInactive, projectStatusInitFailed, projectStatusRemoved:
		return prov.FailStatus(resource.OperationErrorCodeServiceInternalError,
			fmt.Sprintf("project entered terminal status %q", apiResp.Status)), nil
	default:
		return &resource.StatusResult{
			ProgressResult: &resource.ProgressResult{
				Operation:       resource.OperationCheckStatus,
				OperationStatus: resource.OperationStatusInProgress,
				NativeID:        ref,
				RequestID:       ref,
				StatusMessage:   "project status: " + apiResp.Status,
			},
		}, nil
	}
}

func (p *Project) List(ctx context.Context, req *resource.ListRequest) (*resource.ListResult, error) {
	var projects []projectAPI
	if err := p.Client.Do(ctx, supatransport.Request{Method: "GET", Path: "/v1/projects"}, &projects); err != nil {
		return &resource.ListResult{NativeIDs: []string{}}, nil
	}
	ids := make([]string, 0, len(projects))
	for _, pr := range projects {
		if pr.ID == "" || pr.Status == projectStatusRemoved {
			continue
		}
		ids = append(ids, pr.ID)
	}
	return &resource.ListResult{NativeIDs: ids}, nil
}
