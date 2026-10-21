# Model Catalog Group Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a root administrator publish all model-catalog groups or an explicit group whitelist while ensuring hidden group metadata never reaches `/api/pricing`, preserving locked public groups, Relay authorization, billing, and Sub2API publication gates.

**Architecture:** Keep `model.GetPricing()` as the private Relay/pricing source and derive a separately cached, immutable catalog projection from a versioned visibility Option. A dedicated RootAuth endpoint validates, compare-and-swaps, audits, and atomically publishes the visibility document plus its prepared projection; `/api/pricing` then adds only user-specific usable groups and special ratios to a deep-copied base snapshot. Default and Classic share the same HTTP schema and behavior, while Snow remains a homepage shell that reaches the host `/pricing` route.

**Tech Stack:** Go 1.25, Gin, GORM, SQLite/MySQL/PostgreSQL-compatible Option persistence, `atomic.Pointer`, React 19/TypeScript/TanStack Query, React/Semi UI, Bun tests, Playwright, Docker Compose with the retained MySQL volume.

---

## Scope And Execution Dependency

This plan implements only the group-level catalog control described in `docs/superpowers/specs/2026-07-30-model-catalog-group-control-design.md`. It extends the existing locked-group behavior from `docs/superpowers/plans/2026-07-29-model-catalog-visibility.md`; it does not reimplement that completed work.

Execute backend Tasks 1-3 immediately. Execute the Sub2API portion of Task 4 after the rc22 integration plan has added `constant.ChannelTypeSub2API = 59`, the confirmed model-fetch flow, and `channel_model_publications`. An enabled `AbilityWithChannel` proves only that the model is routable; `model.IsChannelModelPublicationApproved(channelID, publicModel)` is the durable administrator-publication evidence. This plan adds the remaining catalog gates: that approval, a real group, and an explicitly persisted price/ratio/tiered expression. It does not automatically publish discovery results.

No task changes `web/snow`, deploys production, pushes a remote branch, or creates a GitHub Release.

## File Structure

### New backend files

- `setting/model_catalog_setting/config.go`: versioned visibility document, structural parsing, normalization, stale-group-aware update validation, and ETag generation.
- `setting/model_catalog_setting/config_test.go`: defaults, malformed persisted values, sentinels, sorting, unknown groups, and stale-group update tests.
- `model/catalog_pricing.go`: deep-copied source snapshot, group universe, Sub2API publication gate, immutable projection, stable digest, cache, and CAS persistence.
- `model/catalog_pricing_test.go`: projection, non-mutation, metadata pruning, fail-closed startup, cache invalidation, Sub2API, concurrency, and pricing-version tests.
- `controller/model_catalog.go`: RootAuth read/update endpoints and concise manage-audit event.
- `controller/model_catalog_test.go`: endpoint authorization contract, conflict handling, failed-save rollback, and audit tests.

### Existing backend files

- `model/pricing.go`: expose a lock-protected deep source snapshot and increment its generation without changing Relay-facing cache semantics.
- `model/pricing_refresh.go`: make forced refresh invalidate/rebuild the catalog projection.
- `model/option.go`: register the default Option, protect it from generic writes, and invalidate pricing/catalog caches for all relevant runtime setting changes.
- `model/model_meta.go`: invalidate pricing/catalog data after successful model metadata mutations.
- `model/vendor_meta.go`: invalidate pricing/catalog data after successful vendor metadata mutations.
- `controller/pricing.go`: serialize the catalog projection and intersect every user-specific group field with visible groups.
- `controller/pricing_test.go`: public response secrecy, locked visibility, empty catalog, `all` expansion, and permission/billing isolation.
- `controller/option.go`: reject `model_catalog_setting.group_visibility` through the generic update endpoint.
- `router/api-router.go`: mount the dedicated RootAuth model-catalog setting routes.

### Default frontend files

- `web/default/src/features/system-settings/types.ts`: admin state, visibility document, group option, summary, and update request types.
- `web/default/src/features/system-settings/api.ts`: dedicated get/update API functions.
- `web/default/src/features/system-settings/models/model-catalog-visibility-state.ts`: pure form normalization, stale-group preservation, dirty state, and empty-selection confirmation rules.
- `web/default/src/features/system-settings/models/model-catalog-visibility-state.test.ts`: pure form behavior tests.
- `web/default/src/features/system-settings/models/model-catalog-visibility-section.tsx`: responsive all/selected editor with search, checkboxes, conflicts, and fixed mobile save actions.
- `web/default/src/features/system-settings/models/model-catalog-visibility-section.test.ts`: source-level endpoint/control wiring tests; interaction behavior remains in pure-state tests and Playwright.
- `web/default/src/features/system-settings/models/section-registry.tsx`: add the catalog-visibility section.
- `web/default/src/features/pricing/types.ts`: keep `pricing_version` at response level and type the projected endpoint map.
- `web/default/src/features/pricing/hooks/use-pricing-data.ts`: identity-scoped query key and projected response consumption.
- `web/default/src/features/pricing/hooks/use-filters.ts`: reconcile a hidden group filter back to `all`.
- `web/default/src/features/pricing/lib/pricing-view-state.ts`: pure valid-group reconciliation helper.
- `web/default/src/features/pricing/lib/pricing-view-state.test.ts`: hidden-filter and empty-catalog tests.
- `web/default/src/features/pricing/index.tsx`: apply filter reconciliation after catalog changes.
- `web/default/src/i18n/locales/en.json`, `fr.json`, `ja.json`, `ru.json`, `vi.json`, `zh-TW.json`, and `zh.json`: translated catalog-editor labels and errors.

### Classic frontend files

- `web/classic/src/pages/Setting/Ratio/model-catalog-visibility-state.js`: pure form helpers matching the Default schema.
- `web/classic/src/pages/Setting/Ratio/model-catalog-visibility-state.test.js`: Classic state parity tests.
- `web/classic/src/pages/Setting/Ratio/ModelCatalogGroupVisibility.jsx`: responsive Semi UI editor.
- `web/classic/src/components/settings/RatioSetting.jsx`: add the group-visibility tab without changing existing ratio tabs.
- `web/classic/src/hooks/model-pricing/pricing-request-state.js`: pure selected-group reconciliation.
- `web/classic/src/hooks/model-pricing/pricing-request-state.test.js`: hidden-filter reset tests.
- `web/classic/src/hooks/model-pricing/useModelPricingData.jsx`: reload on identity changes and reset only invalid group filters.
- `web/classic/src/i18n/locales/en.json`, `fr.json`, `ja.json`, `ru.json`, `vi.json`, `zh-CN.json`, `zh-TW.json`, and `zh.json`: the same translated catalog-editor keys.

### Browser acceptance file

- `web/default/e2e/model-catalog-group-control.spec.ts`: API secrecy, Default/Classic behavior, mobile layout, empty catalog, and Snow host-routing acceptance.

## Invariants To Preserve During Implementation

1. `model.GetPricing()`, `model.GetModelEnableGroups()`, Relay selection, token group checks, subscriptions, and billing never read catalog visibility.
2. Public JSON contains no hidden group key in `data[].enable_groups`, `all_group`, `group_ratio`, `usable_group`, or `auto_groups`.
3. A mixed model survives with visible groups only; a hidden-only model and its orphan vendor/endpoint display metadata disappear.
4. `all` expands to the current visible groups and is never serialized.
5. Default `all` mode includes future real groups. `selected: []` is a valid successful empty catalog.
6. Missing/legacy-empty Option means `all`; malformed non-empty persisted data means fail-closed empty catalog plus one warning per malformed digest.
7. Save builds the candidate projection before persistence and publishes one immutable pointer after persistence. Readers see a complete old or complete new state; a transient raw-pricing refresh failure retains the last valid source/projection.
8. A stale group already present in the stored selected list may remain until an administrator removes it; a newly submitted unknown group is rejected.
9. Sub2API discovery is not public until a persisted channel/model publication approval exists, an enabled ability has a real group, and its public model name is explicitly priced in persisted configuration.
10. Default and Classic use `all_group` as the sole group-filter source and do not reconstruct hidden groups from models, local constants, or `usable_group`.
11. Every new TypeScript/JavaScript/JSX/TSX file begins with the repository's existing AGPL header; all user-facing strings use English i18n source keys and are translated in every shipped locale.

### Task 1: Parse And Validate The Atomic Visibility Document

**Files:**
- Create: `setting/model_catalog_setting/config.go`
- Create: `setting/model_catalog_setting/config_test.go`

- [ ] **Step 1: Write failing parser and validation tests**

```go
package model_catalog_setting

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestParseStoredDefaultsOnlyForMissingOrLegacyEmpty(t *testing.T) {
    for _, raw := range []string{"", "   "} {
        got, err := ParseStored(raw)
        require.NoError(t, err)
        assert.Equal(t, Visibility{Version: 1, Mode: ModeAll, Groups: []string{}}, got)
    }
    _, err := ParseStored(`{"version":2,"mode":"all","groups":[]}`)
    require.ErrorContains(t, err, "version")
    _, err = ParseStored(`{"version":1,"mode":"broken","groups":[]}`)
    require.ErrorContains(t, err, "mode")
}

func TestValidateUpdateNormalizesAndRejectsNewUnknownGroups(t *testing.T) {
    current := Visibility{Version: 1, Mode: ModeSelected, Groups: []string{"deleted"}}
    universe := map[string]string{"default": "Default", "vip": "VIP"}
    got, err := ValidateUpdate(
        Visibility{Version: 1, Mode: ModeSelected, Groups: []string{"vip", "deleted", "vip"}},
        current,
        universe,
    )
    require.NoError(t, err)
    assert.Equal(t, []string{"deleted", "vip"}, got.Groups)

    _, err = ValidateUpdate(
        Visibility{Version: 1, Mode: ModeSelected, Groups: []string{"invented"}},
        current,
        universe,
    )
    require.ErrorContains(t, err, "invented")
}

func TestValidateUpdateRejectsEmptyAndSentinelGroups(t *testing.T) {
    universe := map[string]string{"default": "Default"}
    for _, group := range []string{"", "all", "auto"} {
        _, err := ValidateUpdate(
            Visibility{Version: 1, Mode: ModeSelected, Groups: []string{group}},
            DefaultVisibility(),
            universe,
        )
        require.Error(t, err, group)
    }
}
```

- [ ] **Step 2: Run the focused package test and verify RED**

Run: `go test ./setting/model_catalog_setting -count=1`

Expected: FAIL because the package and `Visibility`, `ParseStored`, `ValidateUpdate`, and `DefaultVisibility` do not exist.

- [ ] **Step 3: Implement the complete versioned parser and validator**

```go
package model_catalog_setting

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "sort"
    "strings"
)

const (
    OptionKey    = "model_catalog_setting.group_visibility"
    ModeAll      = "all"
    ModeSelected = "selected"
)

type Visibility struct {
    Version int      `json:"version"`
    Mode    string   `json:"mode"`
    Groups  []string `json:"groups"`
}

func DefaultVisibility() Visibility {
    return Visibility{Version: 1, Mode: ModeAll, Groups: []string{}}
}

func ParseStored(raw string) (Visibility, error) {
    if strings.TrimSpace(raw) == "" {
        return DefaultVisibility(), nil
    }
    var value Visibility
    decoder := json.NewDecoder(strings.NewReader(raw))
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(&value); err != nil {
        return Visibility{}, fmt.Errorf("decode group visibility: %w", err)
    }
    var trailing interface{}
    if err := decoder.Decode(&trailing); err != io.EOF {
        return Visibility{}, fmt.Errorf("group visibility must contain exactly one JSON object")
    }
    return normalizeStructural(value)
}

func normalizeStructural(value Visibility) (Visibility, error) {
    if value.Version != 1 {
        return Visibility{}, fmt.Errorf("unsupported group visibility version %d", value.Version)
    }
    if value.Mode != ModeAll && value.Mode != ModeSelected {
        return Visibility{}, fmt.Errorf("unsupported group visibility mode %q", value.Mode)
    }
    if value.Mode == ModeAll {
        value.Groups = []string{}
        return value, nil
    }
    seen := make(map[string]struct{}, len(value.Groups))
    normalized := make([]string, 0, len(value.Groups))
    for _, group := range value.Groups {
        if group == "" || strings.TrimSpace(group) != group {
            return Visibility{}, fmt.Errorf("group identifiers must be non-empty exact values")
        }
        if group == "all" || group == "auto" {
            return Visibility{}, fmt.Errorf("group %q is an internal sentinel", group)
        }
        if _, ok := seen[group]; ok {
            continue
        }
        seen[group] = struct{}{}
        normalized = append(normalized, group)
    }
    sort.Strings(normalized)
    value.Groups = normalized
    return value, nil
}

func ValidateUpdate(next, current Visibility, universe map[string]string) (Visibility, error) {
    normalized, err := normalizeStructural(next)
    if err != nil {
        return Visibility{}, err
    }
    current, err = normalizeStructural(current)
    if err != nil {
        current = DefaultVisibility()
    }
    staleAllowed := make(map[string]struct{}, len(current.Groups))
    for _, group := range current.Groups {
        staleAllowed[group] = struct{}{}
    }
    for _, group := range normalized.Groups {
        if _, exists := universe[group]; exists {
            continue
        }
        if _, existedBefore := staleAllowed[group]; !existedBefore {
            return Visibility{}, fmt.Errorf("unknown catalog group %q", group)
        }
    }
    return normalized, nil
}

func Marshal(value Visibility) (string, error) {
    normalized, err := normalizeStructural(value)
    if err != nil {
        return "", err
    }
    encoded, err := json.Marshal(normalized)
    return string(encoded), err
}

func ETag(raw string) string {
    sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
    return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run tests and verify GREEN**

Run: `go test ./setting/model_catalog_setting -count=1`

Expected: PASS, including stable sorting, duplicate removal, empty selected list, and stale-group preservation.

- [ ] **Step 5: Commit the visibility value object**

```bash
git add setting/model_catalog_setting/config.go setting/model_catalog_setting/config_test.go
git commit -m "feat(catalog): validate group visibility policy"
```

### Task 2: Build A Deep-Copied, Fail-Closed Catalog Projection

**Files:**
- Create: `model/catalog_pricing.go`
- Create: `model/catalog_pricing_test.go`
- Modify: `model/pricing.go`
- Modify: `model/pricing_refresh.go`

- [ ] **Step 1: Write failing pure projection tests**

Define fixtures in `model/catalog_pricing_test.go` and assert the full response-bearing snapshot, not only model count:

```go
func TestBuildCatalogProjectionHidesStructuralGroupData(t *testing.T) {
    source := PricingSourceSnapshot{
        Pricing: []Pricing{
            {ModelName: "mixed", VendorID: 1, EnableGroup: []string{"default", "secret"}, SupportedEndpointTypes: []constant.EndpointType{"openai"}},
            {ModelName: "secret-only", VendorID: 2, EnableGroup: []string{"secret"}, SupportedEndpointTypes: []constant.EndpointType{"claude"}},
            {ModelName: "expanded", EnableGroup: []string{"all"}},
        },
        Vendors: []PricingVendor{{ID: 1, Name: "Visible Vendor"}, {ID: 2, Name: "Secret Vendor"}},
        SupportedEndpoints: map[string]common.EndpointInfo{
            "openai": {Path: "/v1/chat/completions", Method: "POST"},
            "claude": {Path: "/v1/messages", Method: "POST"},
        },
        GroupRatio: map[string]float64{"default": 1, "secret": 3},
        GroupDescriptions: map[string]string{"default": "Default", "secret": "Secret"},
    }
    got, err := buildCatalogProjection(source, model_catalog_setting.Visibility{
        Version: 1, Mode: model_catalog_setting.ModeSelected, Groups: []string{"default"},
    })
    require.NoError(t, err)
    assert.Equal(t, []string{"expanded", "mixed"}, pricingNames(got.Pricing))
    assert.Equal(t, []string{"default"}, got.Pricing[0].EnableGroup)
    assert.Equal(t, []string{"default"}, got.Pricing[1].EnableGroup)
    assert.Equal(t, map[string]string{"default": "Default"}, got.AllGroup)
    assert.Equal(t, map[string]float64{"default": 1}, got.GroupRatio)
    assert.Equal(t, []PricingVendor{{ID: 1, Name: "Visible Vendor"}}, got.Vendors)
    assert.Equal(t, map[string]common.EndpointInfo{"openai": {Path: "/v1/chat/completions", Method: "POST"}}, got.SupportedEndpoints)
    assert.NotContains(t, mustJSON(t, got), "secret")
}

func TestBuildCatalogProjectionDoesNotMutateSource(t *testing.T) {
    source := catalogProjectionFixture()
    before := mustJSON(t, source)
    _, err := buildCatalogProjection(source, selectedVisibility("default"))
    require.NoError(t, err)
    assert.JSONEq(t, before, mustJSON(t, source))
}

func TestBuildCatalogProjectionAllowsExplicitEmptyCatalog(t *testing.T) {
    got, err := buildCatalogProjection(catalogProjectionFixture(), selectedVisibility())
    require.NoError(t, err)
    assert.Empty(t, got.Pricing)
    assert.Empty(t, got.AllGroup)
    assert.Empty(t, got.Vendors)
    assert.NotEmpty(t, got.Version)
}
```

Add cases for `all` mode admitting a newly added group, case-sensitive identifiers, stable hashes across map iteration order, version changes after hide/restore, and malformed persisted configuration returning a successful empty snapshot rather than the raw source.

Add `TestCatalogProjectionRefreshFailureKeepsLastValidSnapshot`: build version A, force `GetAllEnableAbilityWithChannels` to fail on the next dirty rebuild through the model test seam, and assert `GetCatalogPricingSnapshot()` returns version A. Add the no-history counterpart asserting an error, so a source outage is not confused with a deliberate `selected: []` catalog.

- [ ] **Step 2: Run the focused model tests and verify RED**

Run: `go test ./model -run 'CatalogProjection|CatalogPricingSource' -count=1`

Expected: FAIL because `PricingSourceSnapshot`, `CatalogPricingSnapshot`, source deep-copy support, and `buildCatalogProjection` do not exist.

- [ ] **Step 3: Refactor the private pricing cache behind a deep snapshot reader**

Add a generation counter to `model/pricing.go`, increment it only after a successful `updatePricing()`, and centralize refresh-under-lock so the catalog builder never reads slices or maps while they are changing:

```go
type PricingSourceSnapshot struct {
    Generation          uint64
    Pricing             []Pricing
    Vendors             []PricingVendor
    SupportedEndpoints  map[string]common.EndpointInfo
    GroupRatio          map[string]float64
    GroupDescriptions   map[string]string
}

var pricingGeneration atomic.Uint64

func ensurePricingLocked() error {
    if time.Since(lastGetPricingTime) <= time.Minute && len(pricingMap) > 0 {
        return nil
    }
    modelSupportEndpointsLock.Lock()
    defer modelSupportEndpointsLock.Unlock()
    return updatePricing()
}

func GetPricingSourceSnapshot() (PricingSourceSnapshot, error) {
    updatePricingLock.Lock()
    defer updatePricingLock.Unlock()
    if err := ensurePricingLocked(); err != nil {
        return PricingSourceSnapshot{}, err
    }
    return clonePricingSourceSnapshot(PricingSourceSnapshot{
        Generation:         pricingGeneration.Load(),
        Pricing:            pricingMap,
        Vendors:            vendorsList,
        SupportedEndpoints: supportedEndpointMap,
        GroupRatio:         ratio_setting.GetGroupRatioCopy(),
    }), nil
}
```

Change `updatePricing()` to return its initial ability-query error and return `nil` only after all new globals are complete. Define `var loadPricingAbilities = GetAllEnableAbilityWithChannels` and call that variable from `updatePricing`; tests temporarily replace it and restore it with `t.Cleanup`, while production behavior is identical. `InvalidatePricingCache()` marks the source stale by zeroing `lastGetPricingTime` but retains the last valid maps; a failed refresh therefore leaves Relay and catalog readers on the prior complete source. `clonePricingSourceSnapshot` must allocate new pricing/vendor slices, clone every `Pricing.EnableGroup` and `Pricing.SupportedEndpointTypes` slice, and clone all maps. At the end of successful `updatePricing()`, call `pricingGeneration.Add(1)`. Keep `GetPricing()` and `GetVendors()` behavior compatible for Relay and existing callers by logging a refresh error and returning the retained cache when one exists.

`CatalogGroupUniverse()` must not infer the administrator list only from surviving pricing rows. It calls `GetAllEnableAbilityWithChannels()`, unions every real explicit ability group with `ratio_setting.GetGroupRatioCopy()`, excludes empty/`all`/`auto`, and fills descriptions with `setting.GetUsableGroupDescription`. `GetPricingSourceSnapshot()` attaches that universe as `GroupDescriptions` and adds `ratio_setting.GetGroupRatio(group)` for a capability group missing from the configured ratio map. This keeps a real enabled group manageable even when all of its model metadata is disabled or its Sub2API candidates are not yet publishable.

- [ ] **Step 4: Implement immutable projection construction and stable versioning**

Create `model/catalog_pricing.go` with these public types and entry points:

```go
type CatalogPricingSnapshot struct {
    Pricing            []Pricing
    Vendors            []PricingVendor
    GroupRatio         map[string]float64
    AllGroup           map[string]string
    SupportedEndpoints map[string]common.EndpointInfo
    Version            string
    Visibility         model_catalog_setting.Visibility
    VisibilityETag     string
    SourceGeneration   uint64
    InvalidationEpoch  uint64
}

func GetCatalogPricingSnapshot() (CatalogPricingSnapshot, error)
func InvalidateCatalogPricing()
func CatalogGroupUniverse() (map[string]string, error)
```

Use `atomic.Pointer[CatalogPricingSnapshot]` for last-valid publication, an atomic invalidation epoch, and a mutex only around rebuilding. `GetCatalogPricingSnapshot` reads `common.OptionMap[model_catalog_setting.OptionKey]`, interprets missing/empty as `all`, and reuses a snapshot only when its source generation, visibility ETag, and invalidation epoch are current. On a transient source refresh error, return the retained last-valid snapshot; return an error only when none exists. On malformed non-empty visibility input, deliberately publish a new allocated empty fail-closed snapshot, hash that public empty state, and log one redacted warning per ETag using `sync.Map.LoadOrStore`.

The core projector must be deterministic:

```go
func visibleGroupSet(source PricingSourceSnapshot, visibility model_catalog_setting.Visibility) map[string]struct{} {
    universe := make(map[string]struct{})
    for group := range source.GroupRatio {
        if isRealCatalogGroup(group) { universe[group] = struct{}{} }
    }
    for _, item := range source.Pricing {
        for _, group := range item.EnableGroup {
            if isRealCatalogGroup(group) { universe[group] = struct{}{} }
        }
    }
    if visibility.Mode == model_catalog_setting.ModeAll { return universe }
    visible := make(map[string]struct{}, len(visibility.Groups))
    for _, group := range visibility.Groups {
        if _, exists := universe[group]; exists { visible[group] = struct{}{} }
    }
    return visible
}

func projectModelGroups(groups []string, visible map[string]struct{}) []string {
    projected := make([]string, 0, len(groups))
    if slices.Contains(groups, "all") {
        for group := range visible { projected = append(projected, group) }
    } else {
        for _, group := range groups {
            if _, ok := visible[group]; ok { projected = append(projected, group) }
        }
    }
    slices.Sort(projected)
    return slices.Compact(projected)
}
```

Sort models by `ModelName`, vendors by `ID`, and every group slice before hashing a canonical struct with SHA-256. Filter vendors by surviving `VendorID`; filter endpoint metadata by the surviving models' `SupportedEndpointTypes`. `CatalogPricingSnapshot.Version` hashes the canonical projected public fields and effective visible-group set; it excludes the raw visibility JSON, stale nonexistent identifiers, ETag, and process-local generation so an equivalent no-op save keeps the version stable. Return a deep clone from `GetCatalogPricingSnapshot` so request-level overrides cannot mutate the atomic base.

- [ ] **Step 5: Verify GREEN and run the race-focused model tests**

Run: `go test ./model -run 'CatalogProjection|CatalogPricingSource' -count=1`

Run: `go test -race ./model -run 'CatalogProjectionConcurrent|CatalogPricingSource' -count=1`

Expected: PASS; the source fixture remains byte-for-byte equivalent, hidden-only metadata is absent, and concurrent readers observe only complete versions.

- [ ] **Step 6: Commit the immutable projection**

```bash
git add model/pricing.go model/pricing_refresh.go model/catalog_pricing.go model/catalog_pricing_test.go
git commit -m "feat(catalog): derive immutable visible group projection"
```

### Task 3: Add Root-Only Atomic Settings APIs And Public Projection Serialization

**Files:**
- Create: `controller/model_catalog.go`
- Create: `controller/model_catalog_test.go`
- Modify: `controller/pricing.go`
- Modify: `controller/pricing_test.go`
- Modify: `controller/option.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write failing administrator endpoint tests**

```go
func TestUpdateModelCatalogVisibilityPublishesOnceAndAuditsSummary(t *testing.T) {
    setupModelCatalogControllerTest(t)
    current := getModelCatalogAdminState(t)
    body := modelCatalogVisibilityUpdateRequest{
        Config: model_catalog_setting.Visibility{Version: 1, Mode: "selected", Groups: []string{"default"}},
        ExpectedETag: current.ETag,
    }
    recorder := performRootJSON(t, http.MethodPut, "/api/option/model-catalog-visibility", body)
    require.Equal(t, http.StatusOK, recorder.Code)
    state := decodeModelCatalogAdminState(t, recorder)
    assert.Equal(t, []string{"default"}, state.Config.Groups)
    assert.Equal(t, 1, state.Summary.VisibleGroupCount)

    var audit model.Log
    require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeManage).Order("id desc").First(&audit).Error)
    assert.Contains(t, audit.Other, "model_catalog.visibility_update")
    assert.Contains(t, audit.Other, `"added_count":0`)
    assert.Contains(t, audit.Other, `"removed_count":`)
    assert.NotContains(t, audit.Other, "secret")
}

func TestUpdateModelCatalogVisibilityRejectsStaleETagWithoutChangingSnapshot(t *testing.T) {
    setupModelCatalogControllerTest(t)
    before := mustCatalogSnapshot(t)
    recorder := performRootJSON(t, http.MethodPut, "/api/option/model-catalog-visibility", modelCatalogVisibilityUpdateRequest{
        Config: model_catalog_setting.Visibility{Version: 1, Mode: "selected", Groups: []string{}},
        ExpectedETag: "stale",
    })
    require.Equal(t, http.StatusConflict, recorder.Code)
    after := mustCatalogSnapshot(t)
    assert.Equal(t, before.Version, after.Version)
}
```

Also test RootAuth on both routes, field-level errors for unknown group/version/mode/sentinel, legal empty selection, persistence failure retaining the old Option and pointer, invalid cold-start state exposing `config_error` only to Root, and generic `PUT /api/option/` rejecting the dedicated key.

- [ ] **Step 2: Write failing public response secrecy tests**

Extend `pricingResponse` with `Vendors`, `SupportedEndpoint`, `AutoGroups`, and `PricingVersion`, then add:

```go
func TestGetPricingNeverSerializesHiddenGroupForAnyIdentity(t *testing.T) {
    setupPricingVisibilityFixture(t)
    for _, userID := range []int{0, 1901, 1902} {
        recorder := performPricingRequest(t, userID)
        require.Equal(t, http.StatusOK, recorder.Code)
        assert.NotContains(t, recorder.Body.String(), "secret")
        payload := decodePricingResponse(t, recorder)
        assert.Equal(t, []string{"default"}, payload.Data[0].EnableGroup)
        assert.Equal(t, map[string]string{"default": "Default"}, payload.AllGroup)
        assert.NotEmpty(t, payload.PricingVersion)
    }
}

func TestGetPricingSelectedEmptyIsSuccessNotFailure(t *testing.T) {
    setCatalogVisibility(t, selectedVisibility())
    recorder := performPricingRequest(t, 0)
    payload := decodePricingResponse(t, recorder)
    assert.Equal(t, http.StatusOK, recorder.Code)
    assert.True(t, payload.Success)
    assert.Empty(t, payload.Data)
    assert.Empty(t, payload.AllGroup)
    assert.Empty(t, payload.GroupRatio)
}
```

Add a mixed model, an `all` model, a hidden-authorized user, a public-unauthorized user, special user-to-target ratios, auto groups, orphan vendors/endpoints, and assertions against every group-bearing field. Every authenticated response test also asserts `Cache-Control: private, no-store`, preventing a shared intermediary from serving one user's `usable_group` or special ratios to another user.

- [ ] **Step 3: Run controller tests and verify RED**

Run: `go test ./controller -run 'ModelCatalogVisibility|GetPricing.*Hidden|GetPricing.*SelectedEmpty' -count=1`

Expected: FAIL because the dedicated endpoints are not mounted and `GetPricing` still serializes raw cache fields.

- [ ] **Step 4: Implement CAS persistence and the administrator state contract**

In `model/catalog_pricing.go`, add:

```go
var ErrCatalogVisibilityConflict = errors.New("catalog visibility changed")

type CatalogAdminState struct {
    Config      *model_catalog_setting.Visibility `json:"config,omitempty"`
    ETag        string                            `json:"etag"`
    ConfigError string                            `json:"config_error,omitempty"`
    Groups      []CatalogAdminGroup               `json:"groups"`
    Summary     CatalogSummary                    `json:"summary"`
}

type CatalogAdminGroup struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Exists      bool   `json:"exists"`
}

type CatalogSummary struct {
    VisibleGroupCount int    `json:"visible_group_count"`
    VisibleModelCount int    `json:"visible_model_count"`
    PricingVersion    string `json:"pricing_version"`
}

func GetCatalogAdminState() (CatalogAdminState, error)
func UpdateCatalogVisibility(expectedETag string, next model_catalog_setting.Visibility) (CatalogAdminState, CatalogVisibilityChange, error)
```

`UpdateCatalogVisibility` must load the actual Option row, compute its ETag, validate against the current structural config and current universe, build the candidate projection first, then use one GORM transaction. For an existing row use `WHERE key = ? AND value = ?` and require `RowsAffected == 1`; for an absent row insert and translate a duplicate-key race to `ErrCatalogVisibilityConflict`. After commit, write the normalized raw value to `common.OptionMap` and atomically publish the prepared snapshot while holding `catalogProjectionMu`. `CatalogVisibilityChange` contains only old/new mode, added/removed counts, and the resulting config/version digest.

In `controller/model_catalog.go`, decode the structured request and return `409` for `ErrCatalogVisibilityConflict`, `400` for field validation, and a generic `500` with request ID for persistence/build errors. On success call:

```go
recordManageAuditFor(c, c.GetInt("id"), "model_catalog.visibility_update", map[string]interface{}{
    "old_mode":      change.OldMode,
    "new_mode":      change.NewMode,
    "added_count":   change.AddedCount,
    "removed_count": change.RemovedCount,
    "config_digest": change.ConfigDigest,
})
```

Do not put group names or the raw JSON document in the audit row.

- [ ] **Step 5: Mount dedicated RootAuth routes and block generic writes**

Under the existing `optionRoute.Use(middleware.RootAuth())` group in `router/api-router.go`, add:

```go
optionRoute.GET("/model-catalog-visibility", controller.GetModelCatalogVisibility)
optionRoute.PUT("/model-catalog-visibility", controller.UpdateModelCatalogVisibility)
```

Add `model_catalog_setting.OptionKey` to `isProtectedDedicatedOptionKey` and its dedicated-message map in `controller/option.go`. This prevents bypassing validation with generic `PUT /api/option/` and removes the raw document from generic `GET /api/option/`; Root reads the normalized state, stale entries, ETag, and repair warning only through the dedicated endpoint.

- [ ] **Step 6: Serialize only the projected catalog and user intersections**

Replace raw reads in `controller/pricing.go` with `model.GetCatalogPricingSnapshot()`. Build user fields only over `snapshot.AllGroup`:

```go
func intersectCatalogGroups(source map[string]string, visible map[string]string) map[string]string {
    result := make(map[string]string)
    for group, description := range source {
        if _, ok := visible[group]; ok { result[group] = description }
    }
    return result
}

func catalogGroupRatios(userGroup string, snapshot model.CatalogPricingSnapshot) map[string]float64 {
    result := maps.Clone(snapshot.GroupRatio)
    for target := range snapshot.AllGroup {
        if ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, target); ok {
            result[target] = ratio
        }
    }
    return result
}
```

Intersect `service.GetUserUsableGroups(userGroup)` and `service.GetUserAutoGroup(userGroup)` with visible keys. Return the projected data, vendors, endpoints, `all_group`, ratios, and `snapshot.Version`. If projection construction has no last valid source snapshot, return `success:false` with a generic message; a valid empty projection remains `200` and `success:true`.

Before serializing, set `c.Header("Cache-Control", "private, no-store")` and `c.Header("Vary", "Cookie, Authorization")`. This complements, rather than replaces, identity-scoped frontend query keys.

- [ ] **Step 7: Run controller and router tests and verify GREEN**

Run: `go test ./controller ./router -run 'ModelCatalogVisibility|GetPricing' -count=1`

Expected: PASS; public JSON lacks the hidden fixture string for anonymous, locked, and hidden-authorized identities; rejected saves retain the prior version.

- [ ] **Step 8: Commit the API contract**

```bash
git add controller/model_catalog.go controller/model_catalog_test.go controller/pricing.go controller/pricing_test.go controller/option.go router/api-router.go model/catalog_pricing.go
git commit -m "feat(catalog): add atomic group visibility API"
```

### Task 4: Wire Every Cache Invalidation Source And The Sub2API Publication Gate

**Files:**
- Modify: `model/catalog_pricing.go`
- Modify: `model/catalog_pricing_test.go`
- Modify: `model/option.go`
- Modify: `model/model_meta.go`
- Modify: `model/vendor_meta.go`
- Modify: `model/channel_cache.go` only if the rc22 confirmed-model path does not already call `InvalidatePricingCache()`

- [ ] **Step 1: Write failing invalidation and Sub2API eligibility tests**

```go
func TestCatalogCacheInvalidatesForEveryPublicInput(t *testing.T) {
    fixture := setupCatalogCacheFixture(t)
    cases := []struct {
        name   string
        mutate func(t *testing.T)
    }{
        {"group ratio", func(t *testing.T) { require.NoError(t, UpdateOption("GroupRatio", `{"default":1,"vip":2}`)) }},
        {"model ratio", func(t *testing.T) { require.NoError(t, UpdateOption("ModelRatio", `{"catalog-model":2}`)) }},
        {"model metadata", func(t *testing.T) { fixture.updateModelDescription(t, "changed") }},
        {"vendor metadata", func(t *testing.T) { fixture.updateVendorName(t, "changed") }},
        {"channel ability", func(t *testing.T) { fixture.addAbility(t, "vip", "catalog-model") }},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            before := mustCatalogSnapshot(t).Version
            tc.mutate(t)
            after := mustCatalogSnapshot(t).Version
            assert.NotEqual(t, before, after)
        })
    }
}

func TestSub2APIAbilityRequiresPublicationRealGroupAndPersistedPrice(t *testing.T) {
    setupSub2APICatalogFixture(t)
    assert.NotContains(t, pricingNames(mustCatalogSnapshot(t).Pricing), "sub2-new")
    approveChannelModelPublication(t, sub2ChannelID, "sub2-new")
    persistExplicitModelRatio(t, "sub2-new", 1.25)
    assert.Contains(t, pricingNames(mustCatalogSnapshot(t).Pricing), "sub2-new")
    replaceAbilityGroup(t, "sub2-new", "auto")
    assert.NotContains(t, pricingNames(mustCatalogSnapshot(t).Pricing), "sub2-new")
}
```

Add cases showing non-Sub2API hand-maintained models preserve existing fallback-price compatibility, a routable but unapproved Sub2API model stays absent, approval without an ability stays absent, persisted fixed price and tiered expression both qualify, code-default ratio without a persisted Option entry does not qualify, and removing a model upstream with the default “retain” choice does not remove its catalog entry.

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./model -run 'CatalogCacheInvalidates|Sub2APIAbility' -count=1`

Expected: FAIL because model/vendor and ratio changes do not consistently invalidate the current pricing cache, and Sub2API abilities are not gated.

- [ ] **Step 3: Centralize pricing-plus-catalog invalidation**

Make `InvalidatePricingCache()` also call an internal catalog invalidation while preserving the existing lock order. Do not acquire `catalogProjectionMu` while holding `updatePricingLock`; invalidation increments the atomic catalog epoch and retains the last valid pointer for refresh-failure fallback.

Register the upgrade-compatible default in `InitOptionMap()` before database Options are loaded:

```go
defaultVisibilityRaw, err := model_catalog_setting.Marshal(model_catalog_setting.DefaultVisibility())
if err != nil { panic(err) }
common.OptionMap[model_catalog_setting.OptionKey] = defaultVisibilityRaw
```

The database loader then replaces it when a persisted row exists. A legacy empty persisted row remains semantically `all`; a malformed non-empty row stays in `OptionMap` so the projection can fail closed and the Root editor can report and repair it.

After successful `Insert`, `Update`, and `Delete` in `model/model_meta.go` and `model/vendor_meta.go`, call `InvalidatePricingCache()`. In `model/option.go`, call it after successfully applying any of these keys:

```go
var catalogAffectingOptionKeys = map[string]struct{}{
    "ModelPrice": {}, "ModelRatio": {}, "CompletionRatio": {},
    "CacheRatio": {}, "CreateCacheRatio": {}, "ImageRatio": {},
    "AudioRatio": {}, "AudioCompletionRatio": {}, "GroupRatio": {},
    "GroupGroupRatio": {}, "UserUsableGroups": {}, "AutoGroups": {},
    "billing_setting.billing_mode": {}, "billing_setting.billing_expr": {},
}
```

The `model_catalog_setting.OptionKey` runtime sync path calls `InvalidateCatalogPricing()` only; the dedicated update path publishes its already prepared candidate.

- [ ] **Step 4: Gate confirmed Sub2API abilities before raw pricing aggregation**

At the start of `updatePricing()`, load explicitly persisted public pricing names from Option rows `ModelPrice`, `ModelRatio`, `billing_setting.billing_mode`, and `billing_setting.billing_expr`. Normalize model names with `ratio_setting.FormatMatchingModelName`. Filter abilities before constructing `modelGroupsMap`, endpoint support, and catalog pricing:

```go
func catalogEligibleAbility(ability AbilityWithChannel, explicitlyPriced map[string]struct{}) bool {
    if ability.ChannelType != constant.ChannelTypeSub2API {
        return true
    }
    if !isRealCatalogGroup(ability.Group) {
        return false
    }
    if !IsChannelModelPublicationApproved(ability.ChannelId, ability.Model) {
        return false
    }
    _, ok := explicitlyPriced[ratio_setting.FormatMatchingModelName(ability.Model)]
    return ok
}
```

For tiered pricing, require both persisted mode `tiered_expr` and a non-empty persisted expression for the same public model. The rc22 integration writes publication approval only after the administrator confirms the diff; neither fetch preview nor saving `Channel.Models` is approval. This gate affects catalog-source construction only and must not silently alter the administrator's pending diff list.

- [ ] **Step 5: Verify cache, Sub2API, and race tests are GREEN**

Run: `go test ./model ./controller -run 'Catalog|Pricing' -count=1`

Run: `go test -race ./model ./controller -run 'Catalog|Pricing' -count=1`

Expected: PASS with a changed version for each public input, no partial snapshots, and the four Sub2API states (unconfirmed, ungrouped, unpriced, approved) matching the specification.

- [ ] **Step 6: Commit invalidation and candidate gating**

```bash
git add model/catalog_pricing.go model/catalog_pricing_test.go model/option.go model/model_meta.go model/vendor_meta.go model/channel_cache.go
git commit -m "fix(catalog): invalidate projections and gate discovered models"
```

### Task 5: Add The Default Administrator Editor

**Files:**
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/api.ts`
- Create: `web/default/src/features/system-settings/models/model-catalog-visibility-state.ts`
- Create: `web/default/src/features/system-settings/models/model-catalog-visibility-state.test.ts`
- Create: `web/default/src/features/system-settings/models/model-catalog-visibility-section.tsx`
- Create: `web/default/src/features/system-settings/models/model-catalog-visibility-section.test.ts`
- Modify: `web/default/src/features/system-settings/models/section-registry.tsx`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Modify: `web/default/src/i18n/locales/zh-TW.json`
- Modify: `web/default/src/i18n/locales/zh.json`

- [ ] **Step 1: Write failing pure state tests**

```ts
import { describe, expect, test } from 'bun:test'
import {
  buildCatalogVisibilityDraft,
  filterCatalogGroups,
  needsEmptyCatalogConfirmation,
} from './model-catalog-visibility-state'

describe('model catalog visibility editor state', () => {
  test('keeps stale stored groups visible and sorts existing groups', () => {
    const draft = buildCatalogVisibilityDraft({
      config: { version: 1, mode: 'selected', groups: ['removed', 'vip'] },
      etag: 'etag-1',
      groups: [
        { name: 'vip', description: 'VIP', exists: true },
        { name: 'default', description: 'Default', exists: true },
        { name: 'removed', description: '', exists: false },
      ],
      summary: { visible_group_count: 1, visible_model_count: 2, pricing_version: 'v1' },
    })
    expect(draft.selectedGroups).toEqual(['removed', 'vip'])
    expect(draft.groups.at(-1)).toMatchObject({ name: 'removed', exists: false })
  })

  test('requires confirmation only for an explicit empty selected catalog', () => {
    expect(needsEmptyCatalogConfirmation('selected', [])).toBe(true)
    expect(needsEmptyCatalogConfirmation('all', [])).toBe(false)
    expect(needsEmptyCatalogConfirmation('selected', ['default'])).toBe(false)
  })

  test('searches both real identifiers and descriptions', () => {
    const groups = [{ name: 'enterprise-cn', description: '企业', exists: true }]
    expect(filterCatalogGroups(groups, 'enterprise')).toHaveLength(1)
    expect(filterCatalogGroups(groups, '企业')).toHaveLength(1)
  })
})
```

- [ ] **Step 2: Run the state test and verify RED**

Run: `cd web/default && bun test src/features/system-settings/models/model-catalog-visibility-state.test.ts`

Expected: FAIL because the editor state module and API types do not exist.

- [ ] **Step 3: Add exact API types and functions**

Add these contracts to `types.ts`:

```ts
export type ModelCatalogVisibility = {
  version: 1
  mode: 'all' | 'selected'
  groups: string[]
}

export type ModelCatalogAdminState = {
  config?: ModelCatalogVisibility
  etag: string
  config_error?: string
  groups: Array<{ name: string; description: string; exists: boolean }>
  summary: {
    visible_group_count: number
    visible_model_count: number
    pricing_version: string
  }
}

export type UpdateModelCatalogVisibilityRequest = {
  config: ModelCatalogVisibility
  expected_etag: string
}
```

Add `getModelCatalogVisibility` and `updateModelCatalogVisibility` in `api.ts`, both using `/api/option/model-catalog-visibility` and `FeatureSettingsResponse<ModelCatalogAdminState>`.

- [ ] **Step 4: Implement the pure form state and make its tests GREEN**

`buildCatalogVisibilityDraft` must preserve `exists:false` selected entries, sort existing entries by `description || name`, put stale entries last, and clone every array. `filterCatalogGroups` performs case-insensitive matching without altering the source. Export `toVisibilityUpdate(draft)` so the React component sends exactly version/mode/groups/ETag.

Run: `cd web/default && bun test src/features/system-settings/models/model-catalog-visibility-state.test.ts`

Expected: PASS.

- [ ] **Step 5: Write failing source-wiring tests using the repository's Bun pattern**

The Default package does not currently install a React DOM testing library. Follow its existing source-contract tests and leave browser interactions to Task 8:

```ts
import { expect, test } from 'bun:test'

const section = await Bun.file(
  new URL('./model-catalog-visibility-section.tsx', import.meta.url)
).text()
const api = await Bun.file(new URL('../api.ts', import.meta.url)).text()
const registry = await Bun.file(new URL('./section-registry.tsx', import.meta.url)).text()

test('wires the dedicated atomic visibility endpoint', () => {
  expect(api).toContain("'/api/option/model-catalog-visibility'")
  expect(section).toContain('expected_etag')
  expect(section).toContain('updateModelCatalogVisibility')
  expect(section).toContain('getModelCatalogVisibility')
  expect(section).toContain('needsEmptyCatalogConfirmation')
  expect(section).toContain('status === 409')
  expect(section).toContain('break-words')
  expect(registry).toContain("id: 'catalog-visibility'")
})

test('uses group controls and never renders model-level choices', () => {
  expect(section).toContain("t('All groups')")
  expect(section).toContain("t('Specified groups')")
  expect(section).toContain("t('Select all groups')")
  expect(section).toContain("t('Clear selected groups')")
  expect(section).not.toContain('model_name')
})
```

The pure-state tests cover sorting, searching, stale entries, request payloads, and empty selection. Task 8 Playwright covers all/selected controls, select-all/clear, confirmation, `409` edit retention, failed-save feedback, invalid persisted state, and long names at narrow viewports.

- [ ] **Step 6: Implement the responsive Default section**

Use the existing design-system `SettingsSection`, segmented/radio control, input, checkbox, alert, dialog, and icon button components. The visible controls are:

- mode control: “All groups” / “Specified groups”;
- searchable single-column checklist with real identifier and description;
- icon buttons with tooltips for select-all and clear;
- stale rows marked “No longer exists” and removable;
- summary showing only visible group/model counts;
- fixed mobile bottom action row inside the section, normal inline actions on desktop.

On save success, call the GET endpoint again and replace the draft with the server response. On `409`, show the conflict inline and provide a refresh icon button; do not overwrite local edits. An empty selected list opens the confirmation dialog before PUT. Use `break-words`, `min-w-0`, and a one-column grid below `768px`; no horizontal scrolling.

Register it in `MODELS_SECTIONS` as `id: 'catalog-visibility'` with `titleKey: 'Model Catalog Group Visibility'`. The section fetches its dedicated state and does not add the visibility JSON to `ModelSettings` defaults.

- [ ] **Step 7: Run Default tests, typecheck, lint, and build**

Run `bun run i18n:sync`, then provide real translations for these extracted keys in every Default locale: `Model Catalog Group Visibility`, `All groups`, `Specified groups`, `Search groups`, `Select all groups`, `Clear selected groups`, `No longer exists`, `Save visibility`, `Catalog visibility changed; refresh before saving`, `Invalid stored catalog visibility`, `Hide all catalog groups?`, `Visible groups`, `Visible models`, `Failed to load catalog visibility`, and `Failed to save catalog visibility`. Extend `model-catalog-visibility-section.test.ts` to load every locale and assert each key exists; for non-English locales assert its value differs from the English value.

Run: `cd web/default && bun test src/features/system-settings/models/model-catalog-visibility-state.test.ts src/features/system-settings/models/model-catalog-visibility-section.test.ts`

Run: `cd web/default && bun run i18n:sync && bun run typecheck && bun run lint && bun run copyright:check && bun run build`

Expected: PASS; no type errors, missing imports, source-audit errors, or production-build failures.

- [ ] **Step 8: Commit the Default administrator editor**

```bash
git add web/default/src/features/system-settings/types.ts web/default/src/features/system-settings/api.ts web/default/src/features/system-settings/models/model-catalog-visibility-state.ts web/default/src/features/system-settings/models/model-catalog-visibility-state.test.ts web/default/src/features/system-settings/models/model-catalog-visibility-section.tsx web/default/src/features/system-settings/models/model-catalog-visibility-section.test.ts web/default/src/features/system-settings/models/section-registry.tsx web/default/src/i18n/locales
git commit -m "feat(default): manage catalog group visibility"
```

### Task 6: Add The Classic Administrator Editor With Contract Parity

**Files:**
- Create: `web/classic/src/pages/Setting/Ratio/model-catalog-visibility-state.js`
- Create: `web/classic/src/pages/Setting/Ratio/model-catalog-visibility-state.test.js`
- Create: `web/classic/src/pages/Setting/Ratio/ModelCatalogGroupVisibility.jsx`
- Modify: `web/classic/src/components/settings/RatioSetting.jsx`
- Modify: `web/classic/src/i18n/locales/en.json`
- Modify: `web/classic/src/i18n/locales/fr.json`
- Modify: `web/classic/src/i18n/locales/ja.json`
- Modify: `web/classic/src/i18n/locales/ru.json`
- Modify: `web/classic/src/i18n/locales/vi.json`
- Modify: `web/classic/src/i18n/locales/zh-CN.json`
- Modify: `web/classic/src/i18n/locales/zh-TW.json`
- Modify: `web/classic/src/i18n/locales/zh.json`

- [ ] **Step 1: Write failing Classic state tests using the same fixtures**

```js
import { describe, expect, test } from 'bun:test';
import {
  buildCatalogVisibilityDraft,
  filterCatalogGroups,
  needsEmptyCatalogConfirmation,
  toVisibilityUpdate,
} from './model-catalog-visibility-state';

describe('classic catalog visibility state', () => {
  test('matches the versioned backend update contract', () => {
    const draft = buildCatalogVisibilityDraft(adminState());
    draft.mode = 'selected';
    draft.selectedGroups = ['vip', 'default'];
    expect(toVisibilityUpdate(draft)).toEqual({
      config: { version: 1, mode: 'selected', groups: ['default', 'vip'] },
      expected_etag: 'etag-1',
    });
  });

  test('preserves stale groups and confirms selected empty', () => {
    const draft = buildCatalogVisibilityDraft(adminStateWithStaleGroup());
    expect(draft.groups.find((group) => group.name === 'removed').exists).toBe(false);
    expect(needsEmptyCatalogConfirmation('selected', [])).toBe(true);
    expect(filterCatalogGroups(draft.groups, 'removed')).toHaveLength(1);
  });
});
```

- [ ] **Step 2: Run the Classic test and verify RED**

Run: `cd web/classic && bun test src/pages/Setting/Ratio/model-catalog-visibility-state.test.js`

Expected: FAIL because the Classic parity helper does not exist.

- [ ] **Step 3: Implement state parity and the Semi UI editor**

Implement the same sorting, stale preservation, search, and request shape as Default. `ModelCatalogGroupVisibility.jsx` uses `API.get`/`API.put` against the dedicated endpoint, Semi `RadioGroup` or `Segmented`, `Input`, `Checkbox`, `Banner`, `Modal.confirm`, `Button`, `Tooltip`, and Lucide icons already installed in Classic.

The component must:

- load independently of the generic Ratio options request;
- keep edits on a `409` or network failure;
- refetch after success;
- require explicit confirmation for `selected: []`;
- use a single column on mobile with `wordBreak: 'break-word'` and a sticky action row that does not cover the final checkbox;
- show saved counts and invalid-config warning without exposing hidden group data outside this Root page.

Add a new `Tabs.TabPane` in `RatioSetting.jsx`:

```jsx
<Tabs.TabPane tab={t('Model Catalog Group Visibility')} itemKey='catalog_visibility'>
  <ModelCatalogGroupVisibility />
</Tabs.TabPane>
```

- [ ] **Step 4: Run Classic focused tests and production build**

Use the same 15 English source keys listed in Task 5, run `bun run i18n:sync`, and provide real values in all eight Classic locale files. Extend `model-catalog-visibility-state.test.js` to read each locale, assert all 15 keys exist, and assert every non-English value differs from English.

Run: `cd web/classic && bun test src/pages/Setting/Ratio/model-catalog-visibility-state.test.js`

Run: `cd web/classic && bun run i18n:sync && bun run i18n:lint && bun run lint && bun run build`

Expected: PASS with no formatting error, missing translation fallback, or production-build failure.

- [ ] **Step 5: Commit Classic parity**

```bash
git add web/classic/src/pages/Setting/Ratio/model-catalog-visibility-state.js web/classic/src/pages/Setting/Ratio/model-catalog-visibility-state.test.js web/classic/src/pages/Setting/Ratio/ModelCatalogGroupVisibility.jsx web/classic/src/components/settings/RatioSetting.jsx web/classic/src/i18n/locales
git commit -m "feat(classic): manage catalog group visibility"
```

### Task 7: Reconcile Public Filters And Identity-Scoped Pricing State

**Files:**
- Modify: `web/default/src/features/pricing/types.ts`
- Modify: `web/default/src/features/pricing/hooks/use-pricing-data.ts`
- Modify: `web/default/src/features/pricing/hooks/use-filters.ts`
- Modify: `web/default/src/features/pricing/lib/pricing-view-state.ts`
- Modify: `web/default/src/features/pricing/lib/pricing-view-state.test.ts`
- Modify: `web/default/src/features/pricing/index.tsx`
- Modify: `web/classic/src/hooks/model-pricing/pricing-request-state.js`
- Modify: `web/classic/src/hooks/model-pricing/pricing-request-state.test.js`
- Modify: `web/classic/src/hooks/model-pricing/useModelPricingData.jsx`

- [ ] **Step 1: Write failing valid-filter reconciliation tests for both frontends**

Default:

```ts
test('resets a group removed by a new catalog projection', () => {
  expect(reconcilePricingGroup('secret', [{ name: 'default', description: 'Default', usable: true }])).toBe('all')
  expect(reconcilePricingGroup('default', [{ name: 'default', description: 'Default', usable: true }])).toBe('default')
  expect(reconcilePricingGroup('all', [])).toBe('all')
})
```

Classic:

```js
test('resets only a group absent from all_group', () => {
  expect(reconcilePricingGroup('secret', [{ name: 'default' }])).toBe('all');
  expect(reconcilePricingGroup('default', [{ name: 'default' }])).toBe('default');
});
```

- [ ] **Step 2: Run tests and verify RED**

Run: `cd web/default && bun test src/features/pricing/lib/pricing-view-state.test.ts`

Run: `cd web/classic && bun test src/hooks/model-pricing/pricing-request-state.test.js`

Expected: FAIL because neither request-state module exports `reconcilePricingGroup`.

- [ ] **Step 3: Add the pure helper and response-level version type**

Both helpers return the existing selection only if it is `all` or exactly matches a group from `all_group`; otherwise return `all`. Add `pricing_version: string` to `PricingData` in Default and type `supported_endpoint` as `Record<string, { path: string; method: string }>` rather than `Record<string, string>`. Keep model-level `pricing_version?` compatibility until the broader rc22 cleanup removes it.

- [ ] **Step 4: Scope Default Query data by login identity and reconcile on projection changes**

Read the current user ID from `useAuthStore` and change:

```ts
const userId = useAuthStore((state) => state.auth.user?.id ?? 'anonymous')
const query = useQuery({
  queryKey: ['pricing', userId],
  queryFn: getPricing,
  staleTime: 5 * 60 * 1000,
})
```

Return `pricingVersion` from `usePricingData`. Add `reconcileGroupFilter(validGroups)` to `useFilters`, implemented with `setFilterState` so a hidden URL-derived key is removed. In `Pricing`, run an effect keyed by `pricingVersion`, `groups`, and `groupFilter`; if reconciliation yields `all`, call `setGroupFilter('all')`. Do not reconstruct groups from `models` or `usableGroup`.

- [ ] **Step 5: Reload Classic on identity change and preserve valid selection**

Derive `identityKey = userState?.user?.id ?? 'anonymous'`. Make the pricing load effect depend on `identityKey`. When a response arrives, normalize only `all_group`, then compute:

```js
const nextGroup = reconcilePricingGroup(selectedGroup, normalizedGroups);
setSelectedGroup(nextGroup);
setFilterGroup(nextGroup);
```

Remove the unconditional `setSelectedGroup('all')`. A visibility update resets a now-hidden group, while ordinary refresh retains a still-public locked group.

- [ ] **Step 6: Run all pricing tests and builds**

Run: `cd web/default && bun test src/features/pricing && bun run typecheck && bun run build`

Run: `cd web/classic && bun test src/hooks/model-pricing src/components/table/model-pricing && bun run build`

Expected: PASS; login/logout cannot reuse another identity's `usable_group`, and selected hidden groups return to `all` without treating an intentionally empty catalog as a network error.

- [ ] **Step 7: Commit public frontend reconciliation**

```bash
git add web/default/src/features/pricing/types.ts web/default/src/features/pricing/hooks/use-pricing-data.ts web/default/src/features/pricing/hooks/use-filters.ts web/default/src/features/pricing/lib/pricing-view-state.ts web/default/src/features/pricing/lib/pricing-view-state.test.ts web/default/src/features/pricing/index.tsx web/classic/src/hooks/model-pricing/pricing-request-state.js web/classic/src/hooks/model-pricing/pricing-request-state.test.js web/classic/src/hooks/model-pricing/useModelPricingData.jsx
git commit -m "fix(pricing): refresh identity and hidden group filters"
```

### Task 8: Prove Authorization Isolation, Cross-Database CAS, And End-To-End Secrecy

**Files:**
- Modify: `controller/pricing_test.go`
- Modify: `model/catalog_pricing_test.go`
- Create: `web/default/e2e/model-catalog-group-control.spec.ts`

- [ ] **Step 1: Add failing Relay isolation and CAS tests**

The controller regression captures authorization and pricing results before and after hiding the group:

```go
func TestCatalogVisibilityDoesNotChangeRelayAuthorizationOrPrice(t *testing.T) {
    fixture := setupCatalogRelayFixture(t, "secret", "secret-model")
    beforeAllowed := model.IsChannelEnabledForGroupModel("secret", "secret-model", fixture.ChannelID)
    beforeRatio := service.GetUserGroupRatio("secret", "secret")
    setCatalogVisibility(t, selectedVisibility("default"))
    afterAllowed := model.IsChannelEnabledForGroupModel("secret", "secret-model", fixture.ChannelID)
    afterRatio := service.GetUserGroupRatio("secret", "secret")
    assert.True(t, beforeAllowed)
    assert.Equal(t, beforeAllowed, afterAllowed)
    assert.Equal(t, beforeRatio, afterRatio)
    assert.NotContains(t, performPricingRequest(t, fixture.UserID).Body.String(), "secret")
}
```

Add a denied user case proving hiding does not grant access, and a billing calculation fixture proving the same group/model price before and after visibility changes. In model tests, run two concurrent `UpdateCatalogVisibility` calls with the same ETag and assert exactly one success and one `ErrCatalogVisibilityConflict`.

- [ ] **Step 2: Run SQLite tests and verify GREEN after implementation**

Run: `go test ./setting/model_catalog_setting ./model ./controller ./router -run 'Catalog|Pricing' -count=1`

Run: `go test -race ./model ./controller -run 'Catalog|Pricing' -count=1`

Expected: PASS with one CAS winner, immutable user responses, unchanged Relay permission and billing values, and no hidden group in serialized JSON.

- [ ] **Step 3: Run disposable MySQL and PostgreSQL CAS profiles**

Add `TestCatalogVisibilityCASMySQL` and `TestCatalogVisibilityCASPostgres` beside the SQLite concurrency test. Each reads the repository-standard `TEST_MYSQL_DSN` or `TEST_POSTGRES_DSN`, migrates only its isolated test database, runs two same-ETag writers, and cleans its rows; it skips when the corresponding variable is absent.

Run the MySQL profile from PowerShell:

```powershell
docker run --detach --rm --name clove-catalog-mysql-test --publish 127.0.0.1:13306:3306 --env MYSQL_ROOT_PASSWORD=catalog-test --env MYSQL_DATABASE=catalog_test mysql:8.4
while (-not (docker exec clove-catalog-mysql-test mysqladmin ping -uroot -pcatalog-test --silent)) { Start-Sleep -Seconds 2 }
$env:TEST_MYSQL_DSN='root:catalog-test@tcp(127.0.0.1:13306)/catalog_test?charset=utf8mb4&parseTime=True&loc=Local'
go test ./model -run 'CatalogVisibilityCASMySQL' -count=1
$testExit = $LASTEXITCODE
docker stop clove-catalog-mysql-test
Remove-Item Env:TEST_MYSQL_DSN
if ($testExit -ne 0) { exit $testExit }
```

Run the PostgreSQL profile from PowerShell:

```powershell
docker run --detach --rm --name clove-catalog-postgres-test --publish 127.0.0.1:15432:5432 --env POSTGRES_USER=root --env POSTGRES_PASSWORD=catalog-test --env POSTGRES_DB=catalog_test postgres:17-alpine
while (-not (docker exec clove-catalog-postgres-test pg_isready -U root -d catalog_test)) { Start-Sleep -Seconds 2 }
$env:TEST_POSTGRES_DSN='postgresql://root:catalog-test@127.0.0.1:15432/catalog_test?sslmode=disable'
go test ./model -run 'CatalogVisibilityCASPostgres' -count=1
$testExit = $LASTEXITCODE
docker stop clove-catalog-postgres-test
Remove-Item Env:TEST_POSTGRES_DSN
if ($testExit -ne 0) { exit $testExit }
```

Expected: PASS on SQLite, MySQL, and PostgreSQL. Conditional update row counts and duplicate-key conflict translation match; exactly one same-ETag writer succeeds on each backend.

- [ ] **Step 4: Write Playwright acceptance around seeded local fixtures**

Create `web/default/e2e/model-catalog-group-control.spec.ts` with API-assisted setup/cleanup and these named cases:

```ts
test('hidden group is absent from raw pricing JSON for every user class', async ({ request }) => {})
test('Default keeps public unauthorized groups locked and priced', async ({ page }) => {})
test('Classic keeps public unauthorized groups locked and priced', async ({ page }) => {})
test('selected empty is an intentional empty catalog', async ({ page }) => {})
test('catalog editor fits 360px 768px and desktop', async ({ page }) => {})
test('Snow root enters the host pricing projection', async ({ page }) => {})
```

The raw-response case must test anonymous, a user authorized only for `default`, and a user authorized for hidden `secret`, asserting `JSON.stringify(response).includes('secret') === false`. The Snow case sets `HomePageMode=snow`, verifies the iframe at `/snow-assets/`, follows the host navigation to `/pricing`, and checks the same visible model list. It must not query a Snow-owned catalog endpoint or modify files under `web/snow`.

- [ ] **Step 5: Rebuild locally with the retained database and execute browser acceptance**

Record database baselines first: users, channels, abilities, models, options, wallet ledger rows, subscription rows, and total balances. Then run `scripts/rebuild-local-mysql-qa.ps1` with container `cloveapi-current-source-mysql-qa`, host port `18104`, and existing volume `cloveapi-current-source-mysql-qa-data`. Do not delete, recreate, or rename that volume; keep the script's rollback container until health succeeds.

Run: `cd web/default && bunx playwright test e2e/model-catalog-group-control.spec.ts --project=chromium`

Expected: PASS at `1440x900`, `768x1024`, and `360x800`. Check no horizontal overflow, clipped long group names, save-row overlap, hidden response strings, console errors, failed network requests, or Snow data-source divergence.

- [ ] **Step 6: Restore fixture settings and compare database baselines**

Restore the original visibility Option, `HomePageMode`, theme, test models, channels, abilities, and users through the same APIs/transactions used to create them. Recount all baseline tables and total balances.

Expected: baseline counts and balances match exactly, except the intended new visibility Option row if the baseline had no row. Wallet ledger and subscription rows are byte-for-byte unchanged because catalog visibility never invokes billing.

- [ ] **Step 7: Commit integration evidence**

```bash
git add controller/pricing_test.go model/catalog_pricing_test.go web/default/e2e/model-catalog-group-control.spec.ts
git commit -m "test(catalog): prove group visibility isolation"
```

### Task 9: Full Local Release Gate

**Files:**
- Modify only files required by a reproducible defect found by the commands below; add a regression test in that defect's owning task before changing implementation.

- [ ] **Step 1: Run complete backend validation**

Run: `go test ./... -count=1`

Run: `go vet ./...`

Expected: PASS with no race, panic, SQL error, duplicate audit, or new package cycle.

- [ ] **Step 2: Run complete Default validation**

Run: `cd web/default && bun test ./src ./rsbuild.config.test.ts && bun run typecheck && bun run lint && bun run format:check && bun run copyright:check && bun run audit:source && bun run build`

Expected: PASS; generated output uses the projected `/api/pricing` contract and the admin section is included in the production bundle.

- [ ] **Step 3: Run complete Classic validation**

Run: `cd web/classic && bun test && bun run lint && bun run build`

Expected: PASS; the visibility tab and public filter reconciliation build without changing the completed locked-group behavior.

- [ ] **Step 4: Inspect runtime health and logs**

Verify `http://127.0.0.1:18104/api/status` and `http://127.0.0.1:18104/api/pricing` return `200`. Inspect container logs from startup through all acceptance requests.

Expected: no panic, data race, deadlock, malformed-option repetition, duplicate manage log, database migration error, billing row, or full hidden configuration in logs. A malformed persisted visibility value produces one redacted warning for its digest.

- [ ] **Step 5: Review the final diff and create the implementation commit only when clean**

Run: `git diff --check`

Run: `git status --short`

Run: `git diff --stat HEAD`

Expected: no whitespace errors, no modifications under `web/snow`, no production/server scripts changed, and no generated secrets or database files tracked. If defect-fix commits already contain every implementation file, do not create an empty commit. Otherwise stage only this feature's tested files and commit:

```bash
git commit -m "feat(catalog): control public groups"
```

Do not deploy to `103.205.240.82`, push, publish a source archive, or create a Release in this plan.

## Self-Review Checklist Before Execution

- Every requirement in the group-control design maps to a task: settings/migration (Task 1), projection/privacy/cache (Tasks 2-4), both admin themes (Tasks 5-6), both public themes and identity cache (Task 7), Sub2API/Relay/Snow/mobile/database proof (Tasks 4 and 8), and full local verification (Task 9).
- The public controller never receives the raw visibility document or management group universe; only RootAuth endpoints do.
- The dedicated write endpoint is the only writer, uses an expected ETag, builds before persistence, and publishes after persistence.
- Invalid non-empty cold-start state is fail-closed; missing and legacy-empty state preserve the existing all-visible catalog.
- `selected: []` is a valid empty directory, distinct from a failed source snapshot.
- Group identifiers remain case-sensitive; wildcards, empty identifiers, `all`, and `auto` are rejected in selected lists.
- Stale stored identifiers remain visible only to Root and may be removed; newly invented identifiers fail validation.
- All response-bearing slices/maps are cloned, all structural group fields are intersected, and source/Relay caches are not mutated.
- Sub2API publication approval, enabled real group, and persisted explicit pricing are all required before catalog publication.
- Default, Classic, and Snow host navigation consume the same `/api/pricing` projection.
- No production deployment, remote push, Release, Snow vendored edit, destructive volume command, or balance mutation belongs to this plan.
