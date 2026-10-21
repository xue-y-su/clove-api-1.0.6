# Model Catalog Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show every currently available model, group, and effective price in the model square while preserving backend call authorization.

**Architecture:** Extend the existing `/api/pricing` response rather than creating a second catalog. Return the full cached pricing list plus separate `all_group` display metadata and unchanged `usable_group` authorization metadata; both frontends derive lock state and prices from those explicit sets.

**Tech Stack:** Go/Gin, in-memory pricing cache, React 19/TypeScript, React 18/Semi UI, Bun tests.

---

## File Structure

- Modify `controller/pricing.go`: stop visibility filtering and build complete group metadata/effective ratios.
- Create `controller/pricing_test.go`: public and authenticated response contracts.
- Modify `web/default/src/features/pricing/types.ts`, `hooks/use-pricing-data.ts`, and `lib/model-helpers.ts`: normalized groups, identity-aware query data, lock-aware prices.
- Modify default pricing sidebar, cards, columns, and detail group table to expose lock state.
- Modify Classic `useModelPricingData.jsx`, group filters, cards, table columns, and detail pricing table for parity.
- Add focused behavior tests in each pricing feature directory.

### Task 1: Return a Full Pricing Catalog

**Files:**
- Modify: `controller/pricing.go`
- Create: `controller/pricing_test.go`

- [ ] **Step 1: Write failing response contract tests**

```go
func TestGetPricingKeepsModelsFromLockedGroups(t *testing.T) {
    setupPricingControllerTest(t)
    setPricingFixture(t, []model.Pricing{
        {ModelName: "public-model", EnableGroup: []string{"default"}},
        {ModelName: "locked-model", EnableGroup: []string{"enterprise"}},
    })
    setGroups(t, map[string]float64{"default": 1, "enterprise": 0.8})
    ctx, recorder := authenticatedPricingContext(t, userInGroup(t, "default"))
    GetPricing(ctx)
    payload := decodePricingResponse(t, recorder)
    assert.ElementsMatch(t, []string{"public-model", "locked-model"}, pricingNames(payload.Data))
    assert.Contains(t, payload.AllGroup, "enterprise")
    assert.NotContains(t, payload.UsableGroup, "enterprise")
    assert.Equal(t, 0.8, payload.GroupRatio["enterprise"])
}
```

Add an anonymous case that receives both models with base ratios and a special group-to-group ratio case that receives the effective override.

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./controller -run 'GetPricing.*Locked|GetPricing.*Anonymous|GetPricing.*Effective' -count=1`

Expected: FAIL because the controller filters models/ratios and omits `all_group`.

- [ ] **Step 3: Replace authorization filtering with explicit metadata**

```go
func pricingDisplayGroups(pricing []model.Pricing) map[string]string {
    groups := make(map[string]string)
    for group := range ratio_setting.GetGroupRatioCopy() {
        groups[group] = setting.GetUsableGroupDescription(group)
    }
    for _, item := range pricing {
        for _, group := range item.EnableGroup {
            if group != "" && group != "all" {
                if _, exists := groups[group]; !exists {
                    groups[group] = setting.GetUsableGroupDescription(group)
                }
            }
        }
    }
    return groups
}
```

Return `model.GetPricing()` unchanged, populate ratios for every display group, retain `usable_group`, and add `all_group`. Remove `filterPricingByUsableGroups` after tests no longer reference it.

- [ ] **Step 4: Run controller tests**

Run: `go test ./controller -run GetPricing -count=1`

Expected: PASS.

- [ ] **Step 5: Commit backend contract**

```bash
git add controller/pricing.go controller/pricing_test.go
git commit -m "feat(pricing): expose full model catalog"
```

### Task 2: Normalize Groups and Lock-Aware Prices in the Default Frontend

**Files:**
- Modify: `web/default/src/features/pricing/types.ts`
- Modify: `web/default/src/features/pricing/hooks/use-pricing-data.ts`
- Modify: `web/default/src/features/pricing/lib/model-helpers.ts`
- Create: `web/default/src/features/pricing/lib/model-visibility.test.ts`

- [ ] **Step 1: Write failing pure-logic tests**

```ts
test('expands all and marks groups outside usable_group as locked', () => {
  expect(getModelDisplayGroups(['all'], ['default', 'vip'], { default: 'Default' })).toEqual([
    { group: 'default', locked: false },
    { group: 'vip', locked: true },
  ])
})

test('marks an all-group minimum as locked when the cheapest group is unavailable', () => {
  expect(resolveCatalogPrice(model, { default: 1, vip: 0.5 }, { default: 'Default' }, 'all')).toMatchObject({
    group: 'vip', locked: true,
  })
})
```

- [ ] **Step 2: Run tests and verify failure**

Run: `cd web/default && bun test src/features/pricing/lib/model-visibility.test.ts`

Expected: FAIL because normalization helpers do not exist.

- [ ] **Step 3: Add explicit types and pure helpers**

```ts
export type PricingGroup = { name: string; description: string; usable: boolean }

export function isPricingGroupUsable(
  group: string,
  usableGroups: Record<string, string>
): boolean {
  return Object.hasOwn(usableGroups, group)
}
```

Extend `PricingData` with `all_group: Record<string, string>`, normalize sorted group objects in `usePricingData`, and have price resolution return `{ value, group, locked }`. Expand model `enable_groups: ['all']` against all display groups.

- [ ] **Step 4: Run pricing logic tests and typecheck**

Run: `cd web/default && bun test src/features/pricing && bun run typecheck`

Expected: PASS.

- [ ] **Step 5: Commit default data model**

```bash
git add web/default/src/features/pricing/types.ts web/default/src/features/pricing/hooks/use-pricing-data.ts web/default/src/features/pricing/lib/model-helpers.ts web/default/src/features/pricing/lib/model-visibility.test.ts
git commit -m "feat(pricing): model catalog visibility state"
```

### Task 3: Render Full Groups and Locked Prices in the Default UI

**Files:**
- Modify: `web/default/src/features/pricing/index.tsx`
- Modify: `web/default/src/features/pricing/components/pricing-sidebar.tsx`
- Modify: `web/default/src/features/pricing/components/model-card.tsx`
- Modify: `web/default/src/features/pricing/components/pricing-columns.tsx`
- Modify: `web/default/src/features/pricing/components/model-details.tsx`
- Test: `web/default/src/features/pricing/lib/model-visibility.test.ts`

- [ ] **Step 1: Add failing source/behavior assertions for locked state**

```ts
test('keeps locked groups selectable and labels their prices', () => {
  const options = buildPricingGroupOptions(groups, models)
  expect(options.find((item) => item.value === 'enterprise')).toMatchObject({ locked: true })
})
```

- [ ] **Step 2: Verify the UI contract fails**

Run: `cd web/default && bun test src/features/pricing`

Expected: FAIL because filter options do not carry lock metadata.

- [ ] **Step 3: Implement lock icons and price provenance**

Use `LockKeyhole` from `lucide-react` in group chips and beside prices whose selected/minimum group is unavailable. Keep chips clickable, expose `aria-label`/tooltip state, and render all supported groups in the detail table. Anonymous data has no per-group lock decoration.

- [ ] **Step 4: Run default frontend checks**

Run: `cd web/default && bun test src/features/pricing && bun run typecheck && bun run lint && bun run build`

Expected: PASS with no overflow or type errors.

- [ ] **Step 5: Commit default UI**

```bash
git add web/default/src/features/pricing
git commit -m "feat(web): show locked model groups and prices"
```

### Task 4: Add Classic Parity

**Files:**
- Modify: `web/classic/src/hooks/model-pricing/useModelPricingData.jsx`
- Modify: `web/classic/src/hooks/model-pricing/usePricingFilterCounts.js`
- Modify: `web/classic/src/components/table/model-pricing/filter/PricingGroups.jsx`
- Modify: `web/classic/src/components/table/model-pricing/view/card/PricingCardView.jsx`
- Modify: `web/classic/src/components/table/model-pricing/view/table/PricingTableColumns.jsx`
- Modify: `web/classic/src/components/table/model-pricing/modal/components/ModelPricingTable.jsx`
- Create: `web/classic/src/hooks/model-pricing/model-visibility.test.js`

- [ ] **Step 1: Add failing Classic normalization tests**

```js
test('does not intersect model groups with usable groups', () => {
  const result = normalizePricingGroups(['default', 'enterprise'], { default: 'Default' });
  expect(result).toEqual([
    { group: 'default', locked: false },
    { group: 'enterprise', locked: true },
  ]);
});
```

- [ ] **Step 2: Verify Classic tests fail**

Run: `cd web/classic && bun test src/hooks/model-pricing/model-visibility.test.js`

Expected: FAIL because the current modal explicitly intersects model and usable groups.

- [ ] **Step 3: Implement Classic full-group display**

Read `all_group`, preserve `usable_group`, expand `all`, add lock state to group filters, cards, table prices, and model detail rows. Use existing Semi UI Tooltip/Tag and `LockKeyhole` icon without disabling filter controls.

- [ ] **Step 4: Run all pricing checks**

Run: `go test ./controller -run GetPricing -count=1`

Run: `cd web/default && bun test src/features/pricing && bun run build:check`

Run: `cd web/classic && bun test src/hooks/model-pricing src/components/table/model-pricing && bun run build`

Expected: all commands PASS.

- [ ] **Step 5: Commit Classic parity**

```bash
git add web/classic/src/hooks/model-pricing web/classic/src/components/table/model-pricing
git commit -m "feat(classic): show full model group catalog"
```

### Task 5: Local Browser Acceptance

**Files:**
- Modify only files required by defects found during acceptance.

- [ ] **Step 1: Open the local model square as an anonymous visitor**

Verify all enabled models and base prices appear at `http://127.0.0.1:18104/pricing`.

- [ ] **Step 2: Sign in as a restricted user**

Verify locked groups remain selectable, their prices are visible, and a model that exists only in a locked group remains in the catalog.

- [ ] **Step 3: Verify authorization remains enforced**

Attempt an API request using the locked group and confirm the backend rejects it through the existing authorization path.

- [ ] **Step 4: Capture desktop/mobile screenshots**

Use Playwright at `1440x900` and `390x844`; inspect group chips, cards, table, and model detail for lock visibility, text overflow, and overlap.

- [ ] **Step 5: Close acceptance findings through their owning task**

If acceptance finds a defect, return to Task 1 for response-contract defects, Task 2 for price/group logic, Task 3 for default UI, or Task 4 for Classic UI. Add the regression test and use that task's exact verification and staging commands. If no defect is found, do not create an empty acceptance commit.

Do not push, deploy, publish a Release, or change the production version.
