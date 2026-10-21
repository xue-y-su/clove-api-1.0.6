# Invitation Poster Modular Designer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a browser-local modular invitation-poster editor that preserves protected identity content, produces identical preview/export pixels, and works in both Default and Classic at all three poster ratios.

**Architecture:** Put the versioned design schema, normalization, layout solver, upload validation, IndexedDB/localStorage transaction, scene resolver, and Canvas renderer under `web/shared/`. Keep Default and Classic as thin React shells that own only theme controls, focus/toast behavior, and dirty-close confirmation; both shells pass protected runtime identity into the same resolver and renderer.

**Tech Stack:** JavaScript/TypeScript, React 19, Base UI/Tailwind, Semi UI, Canvas 2D, IndexedDB, localStorage, Bun tests, Playwright, jsQR.

---

## File Structure

- Create `web/shared/invitation-poster-design.js`: v2 schema defaults, v1 migration, copy normalization, visibility policy, rectangle constraints, collision solving, and immutable updates.
- Create `web/shared/invitation-poster-design.d.ts`: shared public types and function signatures consumed by TypeScript and Classic JavaScript.
- Create `web/shared/invitation-poster-design.test.js`: migration, repair, Unicode copy, visibility, safe-area, collision, QR, pointer-coordinate, and keyboard-transform tests.
- Create `web/shared/invitation-poster-upload.js`: file signature/decode validation, animation rejection, dimension limits, and object-URL leases.
- Create `web/shared/invitation-poster-upload.d.ts`: upload result, dependency, and lease types.
- Create `web/shared/invitation-poster-upload.test.js`: PNG/JPEG/WebP acceptance and rejection/error/resource-lifetime tests.
- Create `web/shared/invitation-poster-storage.js`: localStorage v2 reads/writes, IndexedDB `backgrounds` store, save ordering, asset recovery, and 24-hour orphan cleanup.
- Create `web/shared/invitation-poster-storage.d.ts`: storage/store/state/result interfaces.
- Create `web/shared/invitation-poster-storage.test.js`: transaction ordering, degraded persistence, missing assets, and cleanup tests.
- Create `web/shared/invitation-poster-scene.js`: protected-runtime projection and immutable draw-command/hit-rectangle resolution.
- Create `web/shared/invitation-poster-scene.d.ts`: runtime identity and resolved scene command types.
- Create `web/shared/invitation-poster-scene.test.js`: protected values, optional copy, independent sizes/crops, fixed order, and crop math tests.
- Modify `web/shared/invitation-poster.js`: consume a resolved scene, draw three background recipes/upload crops, retain logo fallback/abort behavior, and render all modules.
- Modify `web/shared/invitation-poster.d.ts`: replace the legacy flat renderer input with `design`, `runtime`, `qrCanvas`, and optional local background asset.
- Modify `web/shared/invitation-poster.test.js`: renderer pixels/resources/PNG/QR-contract regressions for every size and background kind.
- Modify `web/default/src/features/invitations/lib/poster-preferences.ts`: turn the v1 helper into a typed compatibility wrapper over shared design/storage APIs.
- Modify `web/default/src/features/invitations/lib/poster-preferences.test.ts`: prove the wrapper exposes v2 behavior without a second validator.
- Create `web/default/src/features/invitations/components/invitation-poster-editor.tsx`: Default inspector, canvas overlays, pointer/keyboard transforms, upload/crop controls, and accessible module list.
- Create `web/default/src/features/invitations/components/invitation-poster-editor.test.ts`: Default editor source contracts and translation-key coverage.
- Modify `web/default/src/features/invitations/components/invitation-qr-dialog.tsx`: preview/edit state, persistence, render scheduling, save/cancel/reset, dirty close, and focus restoration.
- Modify `web/default/src/features/invitations/components/invitation-ui.test.ts`: preview-first and shared-boundary integration assertions.
- Modify all seven `web/default/src/i18n/locales/*.json` files: localized editor labels, warnings, and confirmations.
- Modify `web/classic/src/components/invitations/poster-preferences.js`: compatibility exports from shared design/storage.
- Modify `web/classic/src/components/invitations/poster-preferences.test.js`: Classic v2 wrapper contract.
- Create `web/classic/src/components/invitations/InvitationPosterEditor.jsx`: Semi UI inspector, overlays, pointer/keyboard transforms, and upload/crop controls.
- Create `web/classic/src/components/invitations/invitation-poster-editor.test.js`: Classic editor/shared-boundary/translation contracts.
- Modify `web/classic/src/components/invitations/InvitationQRModal.jsx`: the same preview/edit lifecycle and persistence semantics as Default.
- Modify `web/classic/src/components/invitations/invitation-ui.test.js`: preview-first and parity assertions.
- Modify all eight `web/classic/src/i18n/locales/*.json` files: the same localized user-facing editor text.
- Modify `web/default/package.json` and `web/bun.lock`: add `jsqr@1.4.0` as an E2E-only decoder.
- Modify `web/default/e2e/production-invitation-flows.spec.ts`: desktop/mobile editor, persistence/recovery, upload privacy, exact dimensions, pixels, logo fallback, and exact QR payload coverage.

### Task 1: Define And Normalize The V2 Design Schema

**Files:**
- Create: `web/shared/invitation-poster-design.js`
- Create: `web/shared/invitation-poster-design.d.ts`
- Create: `web/shared/invitation-poster-design.test.js`

- [ ] **Step 1: Write failing default, migration, repair, and copy tests**

```js
import { describe, expect, test } from 'bun:test';
import {
  POSTER_DESIGN_VERSION,
  POSTER_SIZES,
  createDefaultInvitationPosterDesign,
  normalizeInvitationPosterDesign,
  resolveInvitationPosterDesignCopy,
} from './invitation-poster-design.js';

test('migrates valid v1 values without writing runtime identity into the design', () => {
  const result = normalizeInvitationPosterDesign({
    version: 1,
    size: '4:5',
    showNickname: false,
  });
  expect(result.sourceVersion).toBe(1);
  expect(result.design).toMatchObject({
    version: 2,
    size: '4:5',
    showNickname: false,
    background: { kind: 'builtin', id: 'clean-light' },
  });
  expect(Object.keys(result.design.layouts)).toEqual(['4:5', '3:4', '9:16']);
  expect(JSON.stringify(result.design)).not.toMatch(/siteName|invitationCode|registrationUrl/);
});

test('repairs one invalid layout without replacing valid sizes or copy', () => {
  const saved = createDefaultInvitationPosterDesign();
  saved.copy.title = { source: 'custom', value: 'Join {站点名称}' };
  saved.layouts['4:5'].brand.x = Number.NaN;
  saved.layouts['9:16'].footer.y += 8;
  const result = normalizeInvitationPosterDesign(saved);
  expect(result.repairedSizes).toEqual(['4:5']);
  expect(result.design.copy.title).toEqual({ source: 'custom', value: 'Join {站点名称}' });
  expect(result.design.layouts['9:16'].footer.y).toBe(saved.layouts['9:16'].footer.y);
});

test('uses Unicode code-point limits and preserves intentional empty optional copy', () => {
  const design = createDefaultInvitationPosterDesign();
  design.copy.eyebrow = { source: 'custom', value: '😀'.repeat(33) };
  design.copy.footer = { source: 'custom', value: '' };
  const { design: normalized } = normalizeInvitationPosterDesign(design);
  expect(Array.from(normalized.copy.eyebrow.value)).toHaveLength(32);
  expect(normalized.copy.footer).toEqual({ source: 'custom', value: '' });
  expect(resolveInvitationPosterDesignCopy(normalized, {
    siteName: 'Clove',
    title: '',
    subtitle: '',
    defaultTitle: 'Join {站点名称}',
    defaultSubtitle: 'Scan now',
    labels: { eyebrow: 'Invitation', footer: 'Exclusive invitation', code: 'Code' },
  })).toMatchObject({ title: 'Join Clove', footer: '' });
});
```

- [ ] **Step 2: Run the design tests and verify the red state**

Run: `cd web && bun test shared/invitation-poster-design.test.js`

Expected: FAIL with `Cannot find module './invitation-poster-design.js'`.

- [ ] **Step 3: Add the exact public schema, registry, dimensions, and defaults**

```js
export const POSTER_DESIGN_VERSION = 2;
export const POSTER_DESIGN_STORAGE_KEY = 'invitation-poster-preferences';
export const POSTER_SIZES = Object.freeze({
  '4:5': Object.freeze({ width: 1080, height: 1350 }),
  '3:4': Object.freeze({ width: 1080, height: 1440 }),
  '9:16': Object.freeze({ width: 1080, height: 1920 }),
});
export const POSTER_COPY_LIMITS = Object.freeze({
  eyebrow: 32,
  title: 80,
  subtitle: 160,
  footer: 64,
});
export const POSTER_MODULE_REGISTRY = Object.freeze({
  brand: Object.freeze({ mandatory: true, resize: 'aspect', minWidth: 240, minHeight: 72 }),
  eyebrow: Object.freeze({ mandatory: false, resize: 'width', minWidth: 160, minHeight: 40 }),
  inviter: Object.freeze({ mandatory: false, resize: 'width', minWidth: 240, minHeight: 44 }),
  title: Object.freeze({ mandatory: false, resize: 'width', minWidth: 320, minHeight: 120 }),
  subtitle: Object.freeze({ mandatory: false, resize: 'width', minWidth: 320, minHeight: 80 }),
  qr: Object.freeze({ mandatory: true, resize: 'square', minWidth: 360, minHeight: 360 }),
  inviteCode: Object.freeze({ mandatory: true, resize: 'width', minWidth: 240, minHeight: 80 }),
  domain: Object.freeze({ mandatory: true, resize: 'width', minWidth: 240, minHeight: 48 }),
  footer: Object.freeze({ mandatory: false, resize: 'width', minWidth: 240, minHeight: 48 }),
});

const CANONICAL_LAYOUTS = Object.freeze({
  '4:5': Object.freeze({
    brand: { x: 72, y: 72, width: 640, height: 72 },
    eyebrow: { x: 124, y: 258, width: 300, height: 40 },
    inviter: { x: 124, y: 314, width: 832, height: 44 },
    title: { x: 124, y: 374, width: 832, height: 120 },
    subtitle: { x: 124, y: 510, width: 832, height: 80 },
    qr: { x: 124, y: 630, width: 420, height: 420 },
    inviteCode: { x: 604, y: 668, width: 340, height: 96 },
    domain: { x: 604, y: 780, width: 340, height: 64 },
    footer: { x: 124, y: 1090, width: 832, height: 48 },
  }),
  '3:4': Object.freeze({
    brand: { x: 72, y: 72, width: 640, height: 72 },
    eyebrow: { x: 124, y: 258, width: 300, height: 40 },
    inviter: { x: 124, y: 314, width: 832, height: 44 },
    title: { x: 124, y: 374, width: 832, height: 120 },
    subtitle: { x: 124, y: 510, width: 832, height: 80 },
    qr: { x: 124, y: 700, width: 420, height: 420 },
    inviteCode: { x: 604, y: 742, width: 340, height: 96 },
    domain: { x: 604, y: 854, width: 340, height: 64 },
    footer: { x: 124, y: 1172, width: 832, height: 48 },
  }),
  '9:16': Object.freeze({
    brand: { x: 72, y: 72, width: 640, height: 72 },
    eyebrow: { x: 124, y: 258, width: 300, height: 40 },
    inviter: { x: 124, y: 326, width: 832, height: 44 },
    title: { x: 124, y: 396, width: 832, height: 200 },
    subtitle: { x: 124, y: 620, width: 832, height: 120 },
    qr: { x: 330, y: 840, width: 420, height: 420 },
    inviteCode: { x: 124, y: 1320, width: 832, height: 80 },
    domain: { x: 124, y: 1416, width: 832, height: 48 },
    footer: { x: 124, y: 1580, width: 832, height: 48 },
  }),
});

export function createDefaultInvitationPosterDesign() {
  return structuredClone({
    version: POSTER_DESIGN_VERSION,
    size: '3:4',
    showNickname: true,
    background: { kind: 'builtin', id: 'clean-light' },
    copy: {
      eyebrow: { source: 'site' },
      title: { source: 'site' },
      subtitle: { source: 'site' },
      footer: { source: 'site' },
    },
    layouts: CANONICAL_LAYOUTS,
  });
}
```

Declare the public types in `invitation-poster-design.d.ts` with these exact names and fields:

```ts
export type InvitationPosterSize = '4:5' | '3:4' | '9:16';
export type PosterModuleId = 'brand' | 'eyebrow' | 'inviter' | 'title' | 'subtitle' | 'qr' | 'inviteCode' | 'domain' | 'footer';
export type PosterCopyField = 'eyebrow' | 'title' | 'subtitle' | 'footer';
export type CopyValue = { source: 'site' } | { source: 'custom'; value: string };
export type ModuleRect = { x: number; y: number; width: number; height: number };
export type BackgroundCrop = { focalX: number; focalY: number; zoom: number };
export type PosterBackground =
  | { kind: 'builtin'; id: 'clean-light' | 'ink-contrast' | 'coral-paper' }
  | { kind: 'solid'; color: `#${string}` }
  | { kind: 'upload'; assetId: string; cropBySize: Record<InvitationPosterSize, BackgroundCrop> };
export interface InvitationPosterDesignV2 {
  version: 2;
  size: InvitationPosterSize;
  showNickname: boolean;
  background: PosterBackground;
  copy: Record<PosterCopyField, CopyValue>;
  layouts: Record<InvitationPosterSize, Record<PosterModuleId, ModuleRect>>;
}
export interface PosterCopyRuntime {
  siteName: string;
  title: string;
  subtitle: string;
  defaultTitle: string;
  defaultSubtitle: string;
  labels: { eyebrow: string; code: string; footer: string };
}
export interface NormalizeInvitationPosterDesignResult {
  design: InvitationPosterDesignV2;
  sourceVersion: 0 | 1 | 2;
  repairedSizes: InvitationPosterSize[];
}
```

```ts
export interface ConstrainInvitationPosterTransformInput {
  design: InvitationPosterDesignV2;
  size: InvitationPosterSize;
  moduleId: PosterModuleId;
  from: ModuleRect;
  proposed: ModuleRect;
  resolvedCopy: Record<PosterCopyField, string>;
}
export const POSTER_DESIGN_VERSION: 2;
export const POSTER_DESIGN_STORAGE_KEY: 'invitation-poster-preferences';
export const POSTER_SIZES: Readonly<Record<InvitationPosterSize, Readonly<{ width: number; height: number }>>>;
export const POSTER_COPY_LIMITS: Readonly<Record<PosterCopyField, number>>;
export function createDefaultInvitationPosterDesign(): InvitationPosterDesignV2;
export function normalizeInvitationPosterDesign(value: unknown): NormalizeInvitationPosterDesignResult;
export function resolveInvitationPosterDesignCopy(design: InvitationPosterDesignV2, runtime: PosterCopyRuntime): Record<PosterCopyField, string>;
export function getVisibleInvitationPosterModules(design: InvitationPosterDesignV2, copy: Record<PosterCopyField, string>): PosterModuleId[];
export function constrainInvitationPosterTransform(input: ConstrainInvitationPosterTransformInput): { accepted: boolean; rect: ModuleRect };
export function updateInvitationPosterModuleRect(design: InvitationPosterDesignV2, size: InvitationPosterSize, moduleId: PosterModuleId, rect: ModuleRect): InvitationPosterDesignV2;
export function moveInvitationPosterModule(design: InvitationPosterDesignV2, size: InvitationPosterSize, moduleId: PosterModuleId, dx: number, dy: number, copy: Record<PosterCopyField, string>): InvitationPosterDesignV2;
export function clientPointToPosterPoint(clientX: number, clientY: number, bounds: Pick<DOMRect, 'left' | 'top' | 'width' | 'height'>, dimensions: { width: number; height: number }): { x: number; y: number };
```

- [ ] **Step 4: Implement deterministic migration, scalar normalization, layout repair, and copy resolution**

```js
export function normalizeInvitationPosterDesign(value) {
  const fallback = createDefaultInvitationPosterDesign();
  const sourceVersion = value?.version === 1 ? 1 : value?.version === 2 ? 2 : 0;
  if (sourceVersion === 0) return { design: fallback, sourceVersion, repairedSizes: [] };
  const candidate = sourceVersion === 1 ? {
    ...fallback,
    size: Object.hasOwn(POSTER_SIZES, value.size) ? value.size : fallback.size,
    showNickname: typeof value.showNickname === 'boolean' ? value.showNickname : true,
  } : value;
  const design = {
    ...fallback,
    size: Object.hasOwn(POSTER_SIZES, candidate.size) ? candidate.size : fallback.size,
    showNickname: typeof candidate.showNickname === 'boolean' ? candidate.showNickname : true,
    background: normalizePosterBackground(candidate.background, fallback.background),
    copy: normalizePosterCopy(candidate.copy, fallback.copy),
  };
  const repairedSizes = [];
  design.layouts = Object.fromEntries(Object.keys(POSTER_SIZES).map((size) => {
    const layout = normalizePosterLayout(candidate.layouts?.[size], fallback.layouts[size]);
    if (!layout || !isPosterLayoutWithinBounds(size, layout)) {
      repairedSizes.push(size);
      return [size, structuredClone(fallback.layouts[size])];
    }
    return [size, layout];
  }));
  return { design, sourceVersion, repairedSizes };
}

export function resolveInvitationPosterDesignCopy(design, runtime) {
  const sourceText = {
    eyebrow: runtime.labels.eyebrow,
    title: runtime.title || runtime.defaultTitle,
    subtitle: runtime.subtitle || runtime.defaultSubtitle,
    footer: runtime.labels.footer,
  };
  return Object.fromEntries(Object.keys(POSTER_COPY_LIMITS).map((key) => {
    const preference = design.copy[key];
    const text = preference.source === 'custom' ? preference.value : sourceText[key];
    return [key, String(text || '').split('{站点名称}').join(runtime.siteName)];
  }));
}

function normalizePosterCopy(value, fallback) {
  return Object.fromEntries(Object.entries(POSTER_COPY_LIMITS).map(([key, limit]) => {
    const item = value?.[key];
    if (item?.source === 'custom' && typeof item.value === 'string') {
      return [key, { source: 'custom', value: Array.from(item.value).slice(0, limit).join('') }];
    }
    return [key, item?.source === 'site' ? { source: 'site' } : { ...fallback[key] }];
  }));
}

function normalizePosterBackground(value, fallback) {
  if (value?.kind === 'builtin' && ['clean-light', 'ink-contrast', 'coral-paper'].includes(value.id)) return { kind: 'builtin', id: value.id };
  if (value?.kind === 'solid' && /^#[0-9A-Fa-f]{6}$/.test(value.color)) return { kind: 'solid', color: value.color.toUpperCase() };
  if (value?.kind === 'upload' && typeof value.assetId === 'string' && value.assetId.length > 0) {
    const cropBySize = Object.fromEntries(Object.keys(POSTER_SIZES).map((size) => [size, normalizeCrop(value.cropBySize?.[size])]));
    if (Object.values(cropBySize).every(Boolean)) return { kind: 'upload', assetId: value.assetId, cropBySize };
  }
  return structuredClone(fallback);
}

function normalizeCrop(value) {
  return Number.isFinite(value?.focalX) && value.focalX >= 0 && value.focalX <= 1 &&
    Number.isFinite(value?.focalY) && value.focalY >= 0 && value.focalY <= 1 &&
    Number.isFinite(value?.zoom) && value.zoom >= 1 && value.zoom <= 4
    ? { focalX: value.focalX, focalY: value.focalY, zoom: value.zoom }
    : null;
}

function normalizePosterLayout(value, canonical) {
  if (value !== undefined && (value === null || typeof value !== 'object')) return null;
  const layout = {};
  for (const moduleId of Object.keys(POSTER_MODULE_REGISTRY)) {
    const rect = value?.[moduleId];
    if (rect === undefined) { layout[moduleId] = { ...canonical[moduleId] }; continue; }
    if (!['x', 'y', 'width', 'height'].every((key) => Number.isFinite(rect[key]) && Number.isInteger(rect[key]))) return null;
    layout[moduleId] = { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
  }
  return layout;
}

function isPosterLayoutWithinBounds(size, layout) {
  const dimensions = POSTER_SIZES[size];
  return Object.entries(layout).every(([moduleId, rect]) => {
    const rule = POSTER_MODULE_REGISTRY[moduleId];
    return rect.width >= rule.minWidth && rect.height >= rule.minHeight &&
      rect.x >= 72 && rect.y >= 72 &&
      rect.x + rect.width <= dimensions.width - 72 &&
      rect.y + rect.height <= dimensions.height - 72;
  });
}
```

`normalizePosterBackground` must accept only the three built-in IDs, uppercase six-digit solid colors, or an upload with a non-empty `assetId` and valid `cropBySize` entries (`focalX/focalY` in `0..1`, `zoom` in `1..4`). `normalizePosterCopy` truncates with `Array.from(value).slice(0, limit).join('')`, ignores unknown keys, and keeps custom empty strings.

- [ ] **Step 5: Run schema tests and commit the schema**

Run: `cd web && bun test shared/invitation-poster-design.test.js`

Expected: PASS with no unhandled normalization exception.

```bash
git add web/shared/invitation-poster-design.js web/shared/invitation-poster-design.d.ts web/shared/invitation-poster-design.test.js
git commit -m "feat(invitation): define poster design schema"
```

### Task 2: Enforce Safe Dragging, Resizing, Collisions, And QR Protection

**Files:**
- Modify: `web/shared/invitation-poster-design.js`
- Modify: `web/shared/invitation-poster-design.d.ts`
- Modify: `web/shared/invitation-poster-design.test.js`

- [ ] **Step 1: Add failing visibility and transform tests**

```js
test('keeps mandatory modules visible and hides only empty optional copy or nickname', () => {
  const design = createDefaultInvitationPosterDesign();
  design.showNickname = false;
  design.copy.footer = { source: 'custom', value: '' };
  expect(getVisibleInvitationPosterModules(design, {
    eyebrow: 'Invitation', title: 'Join', subtitle: 'Scan', footer: '',
  })).toEqual(['brand', 'eyebrow', 'title', 'subtitle', 'qr', 'inviteCode', 'domain']);
});

test('stops a drag before the expanded QR guard and safe-area boundary', () => {
  const design = createDefaultInvitationPosterDesign();
  const from = design.layouts['3:4'].inviteCode;
  const result = constrainInvitationPosterTransform({
    design,
    size: '3:4',
    moduleId: 'inviteCode',
    from,
    proposed: { ...from, x: 300 },
    resolvedCopy: { eyebrow: 'Invitation', title: 'Join', subtitle: 'Scan', footer: 'Footer' },
  });
  expect(result.accepted).toBe(false);
  expect(result.rect.x).toBeGreaterThanOrEqual(584);
  expect(result.rect).toEqual(expect.objectContaining({ width: 340, height: 96 }));
});

test('locks QR to a square and maps pointer pixels into export coordinates', () => {
  const design = createDefaultInvitationPosterDesign();
  const qr = design.layouts['4:5'].qr;
  const resized = constrainInvitationPosterTransform({
    design,
    size: '4:5',
    moduleId: 'qr',
    from: qr,
    proposed: { ...qr, width: 500, height: 390 },
    resolvedCopy: { eyebrow: 'Invitation', title: 'Join', subtitle: 'Scan', footer: 'Footer' },
  });
  expect(resized.rect.width).toBe(resized.rect.height);
  expect(clientPointToPosterPoint(160, 200, { left: 10, top: 20, width: 300, height: 400 }, POSTER_SIZES['4:5']))
    .toEqual({ x: 540, y: 608 });
});
```

- [ ] **Step 2: Run the focused transform tests and verify missing exports**

Run: `cd web && bun test shared/invitation-poster-design.test.js -t 'mandatory|stops a drag|locks QR'`

Expected: FAIL because the visibility, constraint, and coordinate functions are not exported.

- [ ] **Step 3: Implement visibility, rectangle validity, path stopping, and immutable layout updates**

```js
const SAFE_INSET = 72;
const COLLISION_GUTTER = 16;
const QR_GUARD = 24;

export function getVisibleInvitationPosterModules(design, copy) {
  return Object.keys(POSTER_MODULE_REGISTRY).filter((moduleId) => {
    if (POSTER_MODULE_REGISTRY[moduleId].mandatory) return true;
    if (moduleId === 'inviter') return design.showNickname;
    return String(copy[moduleId] || '').length > 0;
  });
}

function rectanglesConflict(leftId, left, rightId, right) {
  const leftGuard = leftId === 'qr' ? QR_GUARD : 0;
  const rightGuard = rightId === 'qr' ? QR_GUARD : 0;
  return !(
    left.x + left.width + leftGuard + COLLISION_GUTTER <= right.x - rightGuard ||
    right.x + right.width + rightGuard + COLLISION_GUTTER <= left.x - leftGuard ||
    left.y + left.height + leftGuard + COLLISION_GUTTER <= right.y - rightGuard ||
    right.y + right.height + rightGuard + COLLISION_GUTTER <= left.y - leftGuard
  );
}

function normalizationCopy(design) {
  return Object.fromEntries(Object.keys(POSTER_COPY_LIMITS).map((key) => [
    key,
    design.copy[key].source === 'custom' ? design.copy[key].value : key,
  ]));
}

function isPosterLayoutValid(design, size, layout) {
  if (!isPosterLayoutWithinBounds(size, layout)) return false;
  const visible = getVisibleInvitationPosterModules(design, normalizationCopy(design));
  return visible.every((leftId, leftIndex) => visible.slice(leftIndex + 1).every((rightId) =>
    !rectanglesConflict(leftId, layout[leftId], rightId, layout[rightId])));
}

function normalizeTransformForModule(moduleId, from, proposed) {
  const rule = POSTER_MODULE_REGISTRY[moduleId];
  if (rule.resize === 'width') {
    return { ...proposed, width: Math.max(rule.minWidth, Math.round(proposed.width)), height: from.height, y: from.y };
  }
  if (rule.resize === 'square') {
    const size = Math.max(rule.minWidth, Math.round(Math.max(proposed.width, proposed.height)));
    return { ...proposed, width: size, height: size };
  }
  const aspect = from.width / from.height;
  const width = Math.max(rule.minWidth, Math.round(rule.minHeight * aspect), Math.round(proposed.width));
  return { ...proposed, width, height: Math.round(width / aspect) };
}

function clampRectToSafeArea(rect, dimensions, inset) {
  const maxX = dimensions.width - inset - rect.width;
  const maxY = dimensions.height - inset - rect.height;
  return {
    x: Math.round(maxX < inset ? inset : Math.min(Math.max(rect.x, inset), maxX)),
    y: Math.round(maxY < inset ? inset : Math.min(Math.max(rect.y, inset), maxY)),
    width: Math.round(rect.width),
    height: Math.round(rect.height),
  };
}

function interpolateRect(from, to, factor) {
  return Object.fromEntries(['x', 'y', 'width', 'height'].map((key) =>
    [key, Math.round(from[key] + (to[key] - from[key]) * factor)]));
}

function isModuleRectValid({ design, size, moduleId, rect, resolvedCopy }) {
  const layout = { ...design.layouts[size], [moduleId]: rect };
  if (!isPosterLayoutWithinBounds(size, layout)) return false;
  return getVisibleInvitationPosterModules(design, resolvedCopy)
    .filter((otherId) => otherId !== moduleId)
    .every((otherId) => !rectanglesConflict(moduleId, rect, otherId, layout[otherId]));
}

export function constrainInvitationPosterTransform(input) {
  const normalized = normalizeTransformForModule(input.moduleId, input.from, input.proposed);
  const target = clampRectToSafeArea(normalized, POSTER_SIZES[input.size], SAFE_INSET);
  if (isModuleRectValid({ ...input, rect: target })) return { accepted: true, rect: target };
  let lastValid = input.from;
  let validFactor = 0;
  let invalidFactor = 1;
  for (let index = 1; index <= 64; index += 1) {
    const factor = index / 64;
    const candidate = interpolateRect(input.from, target, factor);
    if (!isModuleRectValid({ ...input, rect: candidate })) { invalidFactor = factor; break; }
    lastValid = candidate;
    validFactor = factor;
  }
  for (let index = 0; index < 12; index += 1) {
    const factor = (validFactor + invalidFactor) / 2;
    const candidate = interpolateRect(input.from, target, factor);
    if (isModuleRectValid({ ...input, rect: candidate })) {
      validFactor = factor;
      lastValid = candidate;
    } else {
      invalidFactor = factor;
    }
  }
  return { accepted: false, rect: lastValid };
}

export function updateInvitationPosterModuleRect(design, size, moduleId, rect) {
  return {
    ...design,
    layouts: {
      ...design.layouts,
      [size]: { ...design.layouts[size], [moduleId]: { ...rect } },
    },
  };
}

export function clientPointToPosterPoint(clientX, clientY, bounds, dimensions) {
  return {
    x: Math.round(((clientX - bounds.left) / bounds.width) * dimensions.width),
    y: Math.round(((clientY - bounds.top) / bounds.height) * dimensions.height),
  };
}
```

In `normalizeInvitationPosterDesign`, replace the Task 1 call to `isPosterLayoutWithinBounds(size, layout)` with `isPosterLayoutValid(design, size, layout)` so colliding saved layouts reset only that size. `normalizeTransformForModule` must preserve height/y for `width`, preserve the starting aspect ratio for `aspect`, force equal width/height for `square`, and enforce registry minimums. `isModuleRectValid` checks finite integers, the 72-pixel safe rectangle, all visible modules with a 16-pixel gutter, and the QR's additional 24-pixel guard. Export `moveInvitationPosterModule(design, size, moduleId, dx, dy, copy)` for 1/10-pixel keyboard movement by composing the constraint and immutable update functions.

- [ ] **Step 4: Run the complete shared design suite and commit constraints**

Run: `cd web && bun test shared/invitation-poster-design.test.js`

Expected: PASS for all three sizes, including out-of-bounds, collision, hidden-module, width-only, brand-aspect, QR-square, and independent-layout cases.

```bash
git add web/shared/invitation-poster-design.js web/shared/invitation-poster-design.d.ts web/shared/invitation-poster-design.test.js
git commit -m "feat(invitation): constrain poster modules"
```

### Task 3: Validate Local Backgrounds And Persist Them Transactionally

**Files:**
- Create: `web/shared/invitation-poster-upload.js`
- Create: `web/shared/invitation-poster-upload.d.ts`
- Create: `web/shared/invitation-poster-upload.test.js`
- Create: `web/shared/invitation-poster-storage.js`
- Create: `web/shared/invitation-poster-storage.d.ts`
- Create: `web/shared/invitation-poster-storage.test.js`

- [ ] **Step 1: Write failing upload validation and object-URL lifetime tests**

```js
test.each([
  ['image/png', new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])],
  ['image/jpeg', new Uint8Array([255, 216, 255, 224])],
  ['image/webp', new TextEncoder().encode('RIFF0000WEBPVP8 ')],
])('accepts decoded %s within byte and pixel bounds', async (type, prefix) => {
  const file = new File([prefix, new Uint8Array(64)], 'background', { type });
  const result = await validateInvitationPosterUpload(file, {
    decodeImage: async () => ({ width: 1024, height: 1024, close() {} }),
  });
  expect(result).toMatchObject({ mimeType: type, width: 1024, height: 1024 });
});

test('rejects APNG, animated WebP, MIME/signature mismatch, and excessive decoded area', async () => {
  await expect(validateInvitationPosterUpload(apngFixture(), decoder(512, 512))).rejects.toThrow('Animated images are not supported');
  await expect(validateInvitationPosterUpload(animatedWebpFixture(), decoder(512, 512))).rejects.toThrow('Animated images are not supported');
  await expect(validateInvitationPosterUpload(jpegNamedAsPng(), decoder(512, 512))).rejects.toThrow('File type does not match its contents');
  await expect(validateInvitationPosterUpload(pngFixture(), decoder(8192, 8192))).rejects.toThrow('Image area exceeds 32 megapixels');
});

test('revokes an object URL exactly once', () => {
  const revoked = [];
  const lease = createInvitationPosterObjectUrlLease(new Blob(['image']), {
    createObjectURL: () => 'blob:poster',
    revokeObjectURL: (url) => revoked.push(url),
  });
  lease.release();
  lease.release();
  expect(revoked).toEqual(['blob:poster']);
});

test('warns only when decoded pixels are below the selected export size', () => {
  const asset = { width: 1080, height: 1440 };
  expect(isInvitationPosterResolutionLow(asset, '3:4')).toBe(false);
  expect(isInvitationPosterResolutionLow(asset, '9:16')).toBe(true);
});

function decoder(width, height) {
  return { decodeImage: async () => ({ width, height, close() {} }) };
}

function pngFixture(extra = new Uint8Array()) {
  return new File([new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]), extra], 'fixture.png', { type: 'image/png' });
}

function apngFixture() {
  return pngFixture(new TextEncoder().encode('0000acTL0000'));
}

function animatedWebpFixture() {
  return new File([new TextEncoder().encode('RIFF0000WEBPANIM')], 'fixture.webp', { type: 'image/webp' });
}

function jpegNamedAsPng() {
  return new File([new Uint8Array([255, 216, 255, 224])], 'fixture.png', { type: 'image/png' });
}
```

- [ ] **Step 2: Verify the upload tests fail before implementation**

Run: `cd web && bun test shared/invitation-poster-upload.test.js`

Expected: FAIL with `Cannot find module './invitation-poster-upload.js'`.

- [ ] **Step 3: Implement magic-byte, animation, decode, and resource validation**

```js
import { POSTER_SIZES } from './invitation-poster-design.js';

const MAX_BYTES = 10 * 1024 * 1024;
const MIN_DIMENSION = 256;
const MAX_DIMENSION = 8192;
const MAX_AREA = 32_000_000;
const SUPPORTED_TYPES = new Set(['image/png', 'image/jpeg', 'image/webp']);

export async function validateInvitationPosterUpload(file, dependencies = {}) {
  if (!(file instanceof Blob) || !SUPPORTED_TYPES.has(file.type)) throw new Error('Use a PNG, JPEG, or WebP image');
  if (file.size > MAX_BYTES) throw new Error('Image must be 10 MiB or smaller');
  const bytes = new Uint8Array(await file.arrayBuffer());
  const detectedType = detectRasterMimeType(bytes);
  if (detectedType !== file.type) throw new Error('File type does not match its contents');
  if (containsAnimation(bytes, detectedType)) throw new Error('Animated images are not supported');
  const decoded = await (dependencies.decodeImage || createImageBitmap)(file);
  try {
    if (decoded.width < MIN_DIMENSION || decoded.height < MIN_DIMENSION) throw new Error('Image dimensions must be at least 256 pixels');
    if (decoded.width > MAX_DIMENSION || decoded.height > MAX_DIMENSION) throw new Error('Image dimensions must not exceed 8192 pixels');
    if (decoded.width * decoded.height > MAX_AREA) throw new Error('Image area exceeds 32 megapixels');
    return { blob: file, mimeType: detectedType, width: decoded.width, height: decoded.height };
  } finally {
    decoded.close?.();
  }
}

export function createInvitationPosterObjectUrlLease(blob, urlApi = URL) {
  const url = urlApi.createObjectURL(blob);
  let released = false;
  return { url, release() { if (!released) { released = true; urlApi.revokeObjectURL(url); } } };
}

export function isInvitationPosterResolutionLow(asset, size) {
  const dimensions = POSTER_SIZES[size] || POSTER_SIZES['3:4'];
  return asset.width < dimensions.width || asset.height < dimensions.height;
}
```

Declare the upload boundary without any server/upload callback:

```ts
import type { InvitationPosterSize } from './invitation-poster-design.js';

export interface ValidatedInvitationPosterUpload {
  blob: Blob;
  mimeType: 'image/png' | 'image/jpeg' | 'image/webp';
  width: number;
  height: number;
}
export interface InvitationPosterObjectUrlLease { url: string; release(): void }
export function validateInvitationPosterUpload(file: File, dependencies?: { decodeImage?: (blob: Blob) => Promise<{ width: number; height: number; close?(): void }> }): Promise<ValidatedInvitationPosterUpload>;
export function createInvitationPosterObjectUrlLease(blob: Blob, urlApi?: Pick<typeof URL, 'createObjectURL' | 'revokeObjectURL'>): InvitationPosterObjectUrlLease;
export function isInvitationPosterResolutionLow(asset: Pick<ValidatedInvitationPosterUpload, 'width' | 'height'>, size: InvitationPosterSize): boolean;
```

Implement `detectRasterMimeType` from PNG's eight-byte signature, JPEG `FF D8 FF`, and WebP `RIFF....WEBP`. Implement `containsAnimation` by scanning PNG chunks for `acTL` and WebP chunks for `ANIM` or `ANMF`; JPEG always returns false. The editor calls `isInvitationPosterResolutionLow` after every size change and shows its warning without rejecting the upload.

- [ ] **Step 4: Write failing storage ordering, degradation, recovery, and cleanup tests**

```js
test('stores a new blob before publishing its v2 reference', async () => {
  const calls = [];
  const result = await saveInvitationPosterState({
    activeDesign: createDefaultInvitationPosterDesign(),
    draftDesign: uploadDesign('asset-new'),
    pendingUpload: backgroundRecord('asset-new', 100),
    storage: memoryStorage(calls),
    backgroundStore: memoryBackgroundStore(calls),
  });
  expect(calls.slice(0, 2)).toEqual(['idb.put:asset-new', 'localStorage.set']);
  expect(result.persistence).toBe('saved');
});

test('never publishes a dangling reference when IndexedDB fails', async () => {
  const storage = memoryStorage([]);
  const before = storage.getItem(POSTER_DESIGN_STORAGE_KEY);
  const result = await saveInvitationPosterState({
    activeDesign: createDefaultInvitationPosterDesign(),
    draftDesign: uploadDesign('asset-new'),
    pendingUpload: backgroundRecord('asset-new', 100),
    storage,
    backgroundStore: failingBackgroundStore(),
  });
  expect(storage.getItem(POSTER_DESIGN_STORAGE_KEY)).toBe(before);
  expect(result.persistence).toBe('session-only-upload');
});

test('repairs only a missing upload background and preserves other settings', async () => {
  const saved = uploadDesign('missing');
  saved.copy.title = { source: 'custom', value: 'Still here' };
  const result = await loadInvitationPosterState({
    storage: memoryStorageWith(saved),
    backgroundStore: memoryBackgroundStore([]),
  });
  expect(result.design.background).toEqual({ kind: 'builtin', id: 'clean-light' });
  expect(result.design.copy.title.value).toBe('Still here');
  expect(result.warning).toBe('missing-upload');
});

test('repairs an undecodable stored blob without erasing layout or copy', async () => {
  const saved = uploadDesign('corrupt');
  saved.copy.subtitle = { source: 'custom', value: 'Keep this copy' };
  const result = await loadInvitationPosterState({
    storage: memoryStorageWith(saved),
    backgroundStore: memoryBackgroundStore([], [backgroundRecord('corrupt', 100)]),
    validateBackground: async () => { throw new Error('decode failed'); },
  });
  expect(result.design.background).toEqual({ kind: 'builtin', id: 'clean-light' });
  expect(result.design.copy.subtitle.value).toBe('Keep this copy');
});

function uploadDesign(assetId) {
  const design = createDefaultInvitationPosterDesign();
  const crop = { focalX: 0.5, focalY: 0.5, zoom: 1 };
  design.background = {
    kind: 'upload',
    assetId,
    cropBySize: { '4:5': { ...crop }, '3:4': { ...crop }, '9:16': { ...crop } },
  };
  return design;
}

function backgroundRecord(assetId, createdAt) {
  return {
    assetId, blob: new Blob(['png'], { type: 'image/png' }), mimeType: 'image/png',
    width: 512, height: 512, createdAt, objectUrl: `blob:${assetId}`,
  };
}

function memoryStorage(calls = [], initial = null) {
  let value = initial;
  return {
    getItem: () => value,
    setItem: (_key, next) => { calls.push('localStorage.set'); value = next; },
  };
}

function memoryStorageWith(design) {
  return memoryStorage([], JSON.stringify(design));
}

function memoryBackgroundStore(calls, records = []) {
  const values = new Map(records.map((record) => [record.assetId, record]));
  return {
    persistent: true,
    get: async (id) => values.get(id),
    getAll: async () => [...values.values()],
    put: async (record) => { calls.push(`idb.put:${record.assetId}`); values.set(record.assetId, record); },
    delete: async (id) => { calls.push(`idb.delete:${id}`); values.delete(id); },
  };
}

function failingBackgroundStore() {
  return { ...memoryBackgroundStore([]), persistent: false, put: async () => { throw new Error('quota'); } };
}
```

- [ ] **Step 5: Run storage tests and verify the red state**

Run: `cd web && bun test shared/invitation-poster-storage.test.js`

Expected: FAIL because storage APIs are absent.

- [ ] **Step 6: Implement the IndexedDB adapter and exact save/load ordering**

```js
import { validateInvitationPosterUpload } from './invitation-poster-upload.js';

export const POSTER_DATABASE_NAME = 'clove-invitation-poster';
export const POSTER_DATABASE_VERSION = 1;
export const POSTER_BACKGROUND_STORE = 'backgrounds';
export const POSTER_ORPHAN_MAX_AGE_MS = 24 * 60 * 60 * 1000;

export async function openInvitationPosterBackgroundStore(factory = indexedDB) {
  const database = await openDatabase(factory, POSTER_DATABASE_NAME, POSTER_DATABASE_VERSION, (event) => {
    const db = event.target.result;
    if (!db.objectStoreNames.contains(POSTER_BACKGROUND_STORE)) db.createObjectStore(POSTER_BACKGROUND_STORE, { keyPath: 'assetId' });
  });
  return createBackgroundStore(database, POSTER_BACKGROUND_STORE);
}

export function createUnavailableInvitationPosterBackgroundStore() {
  return {
    persistent: false,
    get: async () => undefined,
    getAll: async () => [],
    put: async () => { throw new Error('IndexedDB is unavailable'); },
    delete: async () => {},
  };
}

function openDatabase(factory, name, version, onUpgrade) {
  return new Promise((resolve, reject) => {
    const request = factory.open(name, version);
    request.onupgradeneeded = onUpgrade;
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('IndexedDB open failed'));
    request.onblocked = () => reject(new Error('IndexedDB upgrade was blocked'));
  });
}

function requestResult(request) {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('IndexedDB request failed'));
  });
}

function createBackgroundStore(database, storeName) {
  const read = (method, argument) => {
    const transaction = database.transaction(storeName, 'readonly');
    return requestResult(transaction.objectStore(storeName)[method](argument));
  };
  const write = (method, argument) => new Promise((resolve, reject) => {
    const transaction = database.transaction(storeName, 'readwrite');
    transaction.objectStore(storeName)[method](argument);
    transaction.oncomplete = () => resolve();
    transaction.onerror = () => reject(transaction.error || new Error('IndexedDB write failed'));
    transaction.onabort = () => reject(transaction.error || new Error('IndexedDB write aborted'));
  });
  return {
    persistent: true,
    get: (assetId) => read('get', assetId),
    getAll: () => read('getAll'),
    put: (record) => write('put', record),
    delete: (assetId) => write('delete', assetId),
    close: () => database.close(),
  };
}

export async function saveInvitationPosterState(input) {
  const normalized = normalizeInvitationPosterDesign(input.draftDesign).design;
  const retainedAsset = normalized.background.kind === 'upload' &&
    input.activeDesign.background.kind === 'upload' &&
    normalized.background.assetId === input.activeDesign.background.assetId
      ? input.activeAsset
      : null;
  if (input.pendingUpload) {
    try {
      await input.backgroundStore.put(toStoredBackgroundRecord(input.pendingUpload));
    } catch {
      return { design: normalized, asset: input.pendingUpload, persistence: 'session-only-upload', warning: 'indexeddb-unavailable' };
    }
  }
  try {
    input.storage.setItem(POSTER_DESIGN_STORAGE_KEY, JSON.stringify(normalized));
  } catch {
    if (input.pendingUpload) await input.backgroundStore.delete(input.pendingUpload.assetId).catch(() => {});
    return { design: normalized, asset: input.pendingUpload || retainedAsset, persistence: 'memory-only', warning: 'localstorage-unavailable' };
  }
  await deleteReplacedUpload(input.activeDesign, normalized, input.backgroundStore);
  return { design: normalized, asset: input.pendingUpload || retainedAsset, persistence: 'saved', warning: null };
}

function toStoredBackgroundRecord(asset) {
  return {
    assetId: asset.assetId,
    blob: asset.blob,
    mimeType: asset.mimeType,
    width: asset.width,
    height: asset.height,
    createdAt: asset.createdAt,
  };
}

async function deleteReplacedUpload(previous, next, store) {
  if (previous.background.kind !== 'upload') return;
  if (next.background.kind === 'upload' && next.background.assetId === previous.background.assetId) return;
  await store.delete(previous.background.assetId).catch(() => {});
}

export async function loadInvitationPosterState(input) {
  let raw;
  try {
    raw = input.storage.getItem(POSTER_DESIGN_STORAGE_KEY);
  } catch {
    return { design: createDefaultInvitationPosterDesign(), asset: null, warning: 'storage-unavailable' };
  }
  let parsed = null;
  try { parsed = JSON.parse(raw || 'null'); } catch {}
  const design = normalizeInvitationPosterDesign(parsed).design;
  if (design.background.kind !== 'upload') return { design, asset: null, warning: null };
  try {
    const asset = await input.backgroundStore.get(design.background.assetId);
    if (asset?.blob instanceof Blob) {
      await (input.validateBackground || validateStoredInvitationPosterBackground)(asset);
      return { design, asset, warning: null };
    }
  } catch {}
  return {
    design: { ...design, background: { kind: 'builtin', id: 'clean-light' } },
    asset: null,
    warning: 'missing-upload',
  };
}

async function validateStoredInvitationPosterBackground(record) {
  const file = new File([record.blob], `poster-background.${record.mimeType.split('/')[1]}`, { type: record.mimeType });
  const decoded = await validateInvitationPosterUpload(file);
  if (decoded.width !== record.width || decoded.height !== record.height) throw new Error('Stored image dimensions changed');
}

export async function cleanupInvitationPosterBackgrounds(store, referencedAssetId, now = Date.now()) {
  const records = await store.getAll();
  await Promise.all(records
    .filter((record) => record.assetId !== referencedAssetId && now - record.createdAt > POSTER_ORPHAN_MAX_AGE_MS)
    .map((record) => store.delete(record.assetId).catch(() => {})));
}
```

`loadInvitationPosterState` must catch localStorage and IndexedDB errors, normalize without writing, fetch only the referenced asset, and change only `background` to `clean-light` on missing/unreadable bytes. `cleanupInvitationPosterBackgrounds` calls `getAll()`, retains the current `assetId`, retains records newer than 24 hours, and best-effort deletes only older unreferenced IDs. Save returns the normalized draft as active even when persistence degrades, so preview/download still work for the current dialog.

In `invitation-poster-storage.d.ts`, define `InvitationPosterBackgroundRecord` with exactly `assetId`, `blob`, `mimeType`, `width`, `height`, and `createdAt`; define `InvitationPosterBackgroundResource` as that record plus ephemeral `objectUrl`; define load results with `InvitationPosterBackgroundRecord | null` and save inputs/results with `InvitationPosterBackgroundResource | null`. `toStoredBackgroundRecord` is the only boundary that writes an asset to IndexedDB, preventing a `blob:` URL from entering persistent data.

```ts
import type { InvitationPosterDesignV2 } from './invitation-poster-design.js';

export interface InvitationPosterBackgroundRecord {
  assetId: string;
  blob: Blob;
  mimeType: 'image/png' | 'image/jpeg' | 'image/webp';
  width: number;
  height: number;
  createdAt: number;
}
export interface InvitationPosterBackgroundResource extends InvitationPosterBackgroundRecord { objectUrl: string }
export interface InvitationPosterBackgroundStore {
  persistent: boolean;
  get(assetId: string): Promise<InvitationPosterBackgroundRecord | undefined>;
  getAll(): Promise<InvitationPosterBackgroundRecord[]>;
  put(record: InvitationPosterBackgroundRecord): Promise<void>;
  delete(assetId: string): Promise<void>;
  close?(): void;
}
export type InvitationPosterStorageWarning = 'storage-unavailable' | 'missing-upload' | 'indexeddb-unavailable' | 'localstorage-unavailable';
export function openInvitationPosterBackgroundStore(factory?: IDBFactory): Promise<InvitationPosterBackgroundStore>;
export function createUnavailableInvitationPosterBackgroundStore(): InvitationPosterBackgroundStore;
export function loadInvitationPosterState(input: { storage: Pick<Storage, 'getItem'>; backgroundStore: InvitationPosterBackgroundStore; validateBackground?: (record: InvitationPosterBackgroundRecord) => Promise<void> }): Promise<{ design: InvitationPosterDesignV2; asset: InvitationPosterBackgroundRecord | null; warning: InvitationPosterStorageWarning | null }>;
export function saveInvitationPosterState(input: { activeDesign: InvitationPosterDesignV2; activeAsset: InvitationPosterBackgroundResource | null; draftDesign: InvitationPosterDesignV2; pendingUpload: InvitationPosterBackgroundResource | null; storage: Pick<Storage, 'setItem'>; backgroundStore: InvitationPosterBackgroundStore }): Promise<{ design: InvitationPosterDesignV2; asset: InvitationPosterBackgroundResource | null; persistence: 'saved' | 'memory-only' | 'session-only-upload'; warning: InvitationPosterStorageWarning | null }>;
export function cleanupInvitationPosterBackgrounds(store: InvitationPosterBackgroundStore, referencedAssetId?: string, now?: number): Promise<void>;
```

- [ ] **Step 7: Run upload/storage suites and commit browser-local persistence**

Run: `cd web && bun test shared/invitation-poster-upload.test.js shared/invitation-poster-storage.test.js`

Expected: PASS, including quota exceptions, future schema versions, interrupted-save orphans, and no deletion of the referenced asset.

```bash
git add web/shared/invitation-poster-upload.js web/shared/invitation-poster-upload.d.ts web/shared/invitation-poster-upload.test.js web/shared/invitation-poster-storage.js web/shared/invitation-poster-storage.d.ts web/shared/invitation-poster-storage.test.js
git commit -m "feat(invitation): persist local poster backgrounds"
```

### Task 4: Resolve One Protected Scene And Render It For Preview And Export

**Files:**
- Create: `web/shared/invitation-poster-scene.js`
- Create: `web/shared/invitation-poster-scene.d.ts`
- Create: `web/shared/invitation-poster-scene.test.js`
- Modify: `web/shared/invitation-poster.js`
- Modify: `web/shared/invitation-poster.d.ts`
- Modify: `web/shared/invitation-poster.test.js`

- [ ] **Step 1: Write failing scene tests for protected data, order, and upload crops**

```js
test('projects fresh protected runtime facts and fixed drawing order', () => {
  const design = createDefaultInvitationPosterDesign();
  design.copy.eyebrow = { source: 'custom', value: '' };
  const scene = resolveInvitationPosterScene(design, runtime({
    siteName: 'Current Clove', invitationCode: 'LIVE-CODE', siteDomain: 'api.example',
  }));
  expect(scene.modules.map((module) => module.id)).toEqual([
    'inviter', 'title', 'subtitle', 'footer', 'brand', 'inviteCode', 'domain', 'qr',
  ]);
  expect(scene.modules.find((module) => module.id === 'brand').text).toContain('Current Clove');
  expect(scene.modules.find((module) => module.id === 'inviteCode').text).toBe('LIVE-CODE');
  expect(scene.modules.some((module) => module.id === 'eyebrow')).toBe(false);
});

test('computes deterministic uploaded-image cover crop at focal boundaries', () => {
  expect(getInvitationPosterCoverSourceRect(
    { width: 2000, height: 1000 },
    POSTER_SIZES['3:4'],
    { focalX: 1, focalY: 0, zoom: 2 },
  )).toEqual({ sx: 1625, sy: 0, sw: 375, sh: 500 });
});

function runtime(overrides = {}) {
  return {
    siteName: 'Clove API', logoUrl: '', title: '', subtitle: '',
    defaultTitle: 'Join {站点名称}', defaultSubtitle: 'Scan to register',
    displayName: 'Alice', inviterText: 'Invitation from Alice',
    invitationCode: 'CLOVE-TEST', siteDomain: '127.0.0.1:18104',
    labels: { eyebrow: 'Invitation', code: 'Invitation code', footer: 'Exclusive invitation' },
    ...overrides,
  };
}
```

- [ ] **Step 2: Run scene tests and verify missing resolver exports**

Run: `cd web && bun test shared/invitation-poster-scene.test.js`

Expected: FAIL with `Cannot find module './invitation-poster-scene.js'`.

- [ ] **Step 3: Implement the immutable scene and exact protected-runtime interface**

```js
const DRAW_ORDER = ['eyebrow', 'inviter', 'title', 'subtitle', 'footer', 'brand', 'inviteCode', 'domain', 'qr'];

export function resolveInvitationPosterScene(designInput, runtime) {
  const design = normalizeInvitationPosterDesign(designInput).design;
  const copy = resolveInvitationPosterDesignCopy(design, runtime);
  const visible = new Set(getVisibleInvitationPosterModules(design, copy));
  const protectedText = {
    brand: runtime.siteName,
    inviter: runtime.inviterText,
    inviteCode: runtime.invitationCode,
    domain: runtime.siteDomain,
  };
  return Object.freeze({
    size: design.size,
    dimensions: POSTER_SIZES[design.size],
    background: structuredClone(design.background),
    modules: DRAW_ORDER.filter((id) => visible.has(id)).map((id) => Object.freeze({
      id,
      rect: Object.freeze({ ...design.layouts[design.size][id] }),
      text: protectedText[id] ?? copy[id] ?? '',
      protected: POSTER_MODULE_REGISTRY[id].mandatory,
    })),
  });
}

export function getInvitationPosterCoverSourceRect(image, canvas, crop) {
  const coverWidth = Math.min(image.width, image.height * (canvas.width / canvas.height));
  const coverHeight = coverWidth * (canvas.height / canvas.width);
  const sw = coverWidth / crop.zoom;
  const sh = coverHeight / crop.zoom;
  return {
    sx: Math.round((image.width - sw) * crop.focalX),
    sy: Math.round((image.height - sh) * crop.focalY),
    sw: Math.round(sw),
    sh: Math.round(sh),
  };
}
```

The runtime type contains `siteName`, `logoUrl`, `title`, `subtitle`, `defaultTitle`, `defaultSubtitle`, `displayName`, the already-localized `inviterText`, `invitationCode`, `siteDomain`, and `labels`. The scene keeps the QR payload outside persisted design data, marks mandatory modules protected, never exposes editability for protected content or visibility, and places `qr` last so no later drawing can enter its quiet-zone plate.

Declare the shared scene/renderer boundary exactly once:

```ts
// web/shared/invitation-poster-scene.d.ts
import type { BackgroundCrop, InvitationPosterDesignV2, InvitationPosterSize, ModuleRect, PosterBackground, PosterCopyRuntime, PosterModuleId } from './invitation-poster-design.js';

export interface InvitationPosterRuntime extends PosterCopyRuntime {
  logoUrl: string;
  displayName: string;
  inviterText: string;
  invitationCode: string;
  siteDomain: string;
}
export interface InvitationPosterSceneModule {
  id: PosterModuleId;
  rect: Readonly<ModuleRect>;
  text: string;
  protected: boolean;
}
export interface InvitationPosterScene {
  size: InvitationPosterSize;
  dimensions: Readonly<{ width: number; height: number }>;
  background: PosterBackground;
  modules: ReadonlyArray<Readonly<InvitationPosterSceneModule>>;
}
export function resolveInvitationPosterScene(design: InvitationPosterDesignV2, runtime: InvitationPosterRuntime): InvitationPosterScene;
export function getInvitationPosterCoverSourceRect(image: { width: number; height: number }, canvas: { width: number; height: number }, crop: BackgroundCrop): { sx: number; sy: number; sw: number; sh: number };

// web/shared/invitation-poster.d.ts
import type { InvitationPosterDesignV2 } from './invitation-poster-design.js';
import type { InvitationPosterRuntime } from './invitation-poster-scene.js';
import type { InvitationPosterBackgroundResource } from './invitation-poster-storage.js';

export interface InvitationPosterRenderInput {
  design: InvitationPosterDesignV2;
  runtime: InvitationPosterRuntime;
  qrCanvas: HTMLCanvasElement;
  backgroundAsset?: InvitationPosterBackgroundResource | null;
  signal?: AbortSignal;
}
export function renderInvitationPoster(input: InvitationPosterRenderInput, dependencies?: {
  createCanvas?: () => HTMLCanvasElement;
  loadLogoImage?: (url: string, timeoutMs?: number, signal?: AbortSignal) => Promise<CanvasImageSource>;
  loadBackgroundImage?: (objectUrl: string, timeoutMs?: number, signal?: AbortSignal) => Promise<CanvasImageSource>;
  readyFonts?: () => Promise<void>;
}): Promise<HTMLCanvasElement>;
```

- [ ] **Step 4: Write failing renderer tests for backgrounds, resources, optional copy, and dimensions**

```js
test.each([
  ['4:5', 1080, 1350], ['3:4', 1080, 1440], ['9:16', 1080, 1920],
])('renders the resolved %s scene at %ix%i', async (size, width, height) => {
  const design = createDefaultInvitationPosterDesign();
  design.size = size;
  const { canvas, calls } = createCanvasHarness();
  const qr = { width: 360, height: 360 };
  const result = await renderInvitationPoster({ design, runtime: runtime(), qrCanvas: qr }, {
    createCanvas: () => canvas,
    readyFonts: async () => {},
  });
  expect([result.width, result.height]).toEqual([width, height]);
  expect(calls.some((call) => call.type === 'drawImage' && call.args[0] === qr)).toBe(true);
});

test('loads an uploaded blob only through the background loader', async () => {
  const logoLoads = [];
  const backgroundLoads = [];
  const design = createDefaultInvitationPosterDesign();
  const crop = { focalX: 0.5, focalY: 0.5, zoom: 1 };
  design.background = {
    kind: 'upload',
    assetId: 'asset-1',
    cropBySize: { '4:5': { ...crop }, '3:4': { ...crop }, '9:16': { ...crop } },
  };
  const qr = { width: 360, height: 360 };
  const backgroundAsset = {
    assetId: 'asset-1', blob: new Blob(['png'], { type: 'image/png' }),
    mimeType: 'image/png', width: 512, height: 512, createdAt: 1,
    objectUrl: 'blob:poster-background',
  };
  await renderInvitationPoster({
    design, runtime: runtime({ logoUrl: '/api/invitation/poster-logo' }),
    qrCanvas: qr, backgroundAsset,
  }, {
    loadLogoImage: async (url) => { logoLoads.push(url); return { width: 64, height: 64 }; },
    loadBackgroundImage: async (url) => { backgroundLoads.push(url); return { width: 512, height: 512 }; },
  });
  expect(logoLoads).toEqual(['/api/invitation/poster-logo']);
  expect(backgroundLoads).toEqual(['blob:poster-background']);
});

function runtime(overrides = {}) {
  return {
    siteName: 'Clove API',
    logoUrl: '',
    title: '',
    subtitle: '',
    defaultTitle: 'Join {站点名称}',
    defaultSubtitle: 'Scan to register',
    displayName: 'Alice',
    inviterText: 'Invitation from Alice',
    invitationCode: 'CLOVE-TEST',
    siteDomain: '127.0.0.1:18104',
    labels: { eyebrow: 'Invitation', code: 'Invitation code', footer: 'Exclusive invitation' },
    ...overrides,
  };
}
```

- [ ] **Step 5: Run renderer tests and verify the legacy renderer contract fails**

Run: `cd web && bun test shared/invitation-poster.test.js`

Expected: FAIL until the renderer accepts `design`, `runtime`, `qrCanvas`, and `backgroundAsset`.

- [ ] **Step 6: Refactor the renderer around the resolved scene while retaining export helpers**

```js
export async function renderInvitationPoster(input, dependencies = {}) {
  if (!input?.qrCanvas) throw new Error('Invitation QR canvas is required');
  const scene = resolveInvitationPosterScene(input.design, input.runtime);
  await (dependencies.readyFonts || waitForBrowserFonts)();
  throwIfAborted(input.signal);
  const canvas = (dependencies.createCanvas || defaultCreateCanvas)();
  canvas.width = scene.dimensions.width;
  canvas.height = scene.dimensions.height;
  const context = canvas.getContext('2d');
  if (!context) throw new Error('Canvas 2D is unavailable');
  const logo = await loadLogoWithFallback(input.runtime.logoUrl, input.signal, dependencies.loadLogoImage);
  const background = input.backgroundAsset
    ? await (dependencies.loadBackgroundImage || loadBrowserImage)(input.backgroundAsset.objectUrl, undefined, input.signal)
    : null;
  drawInvitationPosterBackground(context, scene, background);
  for (const command of scene.modules) drawInvitationPosterModule(context, command, { ...input, logo });
  return canvas;
}
```

`drawInvitationPosterBackground` must implement `clean-light`, `ink-contrast`, `coral-paper`, canonical uppercase solid fill, and uploaded cover/crop/zoom clipped to the full canvas; wrap recipe selection so any unknown/failed recipe calls `drawCleanLightBackground` and continues. `drawInvitationPosterModule` uses each resolved `ModuleRect`, renderer-owned fonts/line counts, line-aware ellipsis, a white QR plate, at least 7% QR inset, and integer QR draw coordinates. Keep `resolveInvitationPosterLogoUrl`, six-second logo timeout, abort handling, site-initial fallback, `canvasToPngBlob`, filename sanitization, and download URL revocation unchanged in behavior.

- [ ] **Step 7: Run all shared poster tests and commit the unified rendering path**

Run: `cd web && bun test shared/invitation-poster-design.test.js shared/invitation-poster-upload.test.js shared/invitation-poster-storage.test.js shared/invitation-poster-scene.test.js shared/invitation-poster.test.js`

Expected: PASS; assertions prove a nonblank PNG path, exact dimensions, protected modules, separate logo/background loaders, built-in/solid/upload rendering, initial fallback, abort cleanup, and no optional-copy collision.

```bash
git add web/shared/invitation-poster-scene.js web/shared/invitation-poster-scene.d.ts web/shared/invitation-poster-scene.test.js web/shared/invitation-poster.js web/shared/invitation-poster.d.ts web/shared/invitation-poster.test.js
git commit -m "feat(invitation): render modular poster scenes"
```

### Task 5: Integrate The Preview-First Editor In Default

**Files:**
- Modify: `web/default/src/features/invitations/lib/poster-preferences.ts`
- Modify: `web/default/src/features/invitations/lib/poster-preferences.test.ts`
- Create: `web/default/src/features/invitations/components/invitation-poster-editor.tsx`
- Create: `web/default/src/features/invitations/components/invitation-poster-editor.test.ts`
- Modify: `web/default/src/features/invitations/components/invitation-qr-dialog.tsx`
- Modify: `web/default/src/features/invitations/components/invitation-ui.test.ts`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Modify: `web/default/src/i18n/locales/zh-TW.json`
- Modify: `web/default/src/i18n/locales/zh.json`

- [ ] **Step 1: Replace v1 helper tests with a failing shared-wrapper contract**

```ts
test('uses the shared v2 normalizer and storage key', () => {
  const result = readPosterPreferences(memoryStorage(JSON.stringify({
    version: 1, size: '9:16', showNickname: false,
  })))
  expect(result).toMatchObject({ version: 2, size: '9:16', showNickname: false })
  expect(POSTER_PREFERENCES_STORAGE_KEY).toBe('invitation-poster-preferences')
})

test('does not define a second size or schema validator', async () => {
  const source = await Bun.file(new URL('./poster-preferences.ts', import.meta.url)).text()
  expect(source).toContain("from '../../../../../shared/invitation-poster-design.js'")
  expect(source).not.toContain('VALID_POSTER_SIZES')
})
```

- [ ] **Step 2: Run Default invitation tests and verify they fail on the v1 wrapper/editor absence**

Run: `cd web/default && bun test src/features/invitations/lib/poster-preferences.test.ts src/features/invitations/components/invitation-poster-editor.test.ts src/features/invitations/components/invitation-ui.test.ts`

Expected: FAIL because preferences still serialize v1 and the editor test file/component do not exist.

- [ ] **Step 3: Convert `poster-preferences.ts` to a typed compatibility wrapper**

```ts
import {
  POSTER_DESIGN_STORAGE_KEY,
  createDefaultInvitationPosterDesign,
  normalizeInvitationPosterDesign,
  type InvitationPosterDesignV2,
} from '../../../../../shared/invitation-poster-design.js'

export const POSTER_PREFERENCES_STORAGE_KEY = POSTER_DESIGN_STORAGE_KEY
export type PosterPreferences = InvitationPosterDesignV2
export const DEFAULT_POSTER_PREFERENCES = createDefaultInvitationPosterDesign()

export function readPosterPreferences(storage: Pick<Storage, 'getItem'> | null = browserStorage()) {
  try {
    return normalizeInvitationPosterDesign(JSON.parse(storage?.getItem(POSTER_DESIGN_STORAGE_KEY) || 'null')).design
  } catch {
    return createDefaultInvitationPosterDesign()
  }
}
```

Keep `countPosterCharacters` and `limitPosterCharacters` only if registration settings still import them; remove `isInvitationPosterSize`, `writePosterPreferences`, and immediate `persistPosterPreferences` once the dialog uses the shared save transaction.

- [ ] **Step 4: Add failing Default editor source and state-contract assertions**

```ts
test('keeps editing behind a pencil and uses shared transforms', () => {
  expect(dialogSource).toContain("mode === 'preview'")
  expect(dialogSource).toContain("aria-label={t('Edit poster')}")
  expect(dialogSource).toContain('<Pencil')
  expect(editorSource).toContain('constrainInvitationPosterTransform')
  expect(editorSource).toContain('setPointerCapture')
  expect(editorSource).toContain("event.shiftKey ? 10 : 1")
})

test('keeps download out of edit mode and reset reversible', () => {
  expect(dialogSource).toContain("mode === 'preview' ?")
  expect(dialogSource).toContain('saveInvitationPosterState')
  expect(dialogSource).toContain('createDefaultInvitationPosterDesign')
  expect(dialogSource).toContain('setDraft(structuredClone(activeDesign))')
})
```

- [ ] **Step 5: Build the Default editor shell with stable overlays and inspector controls**

```tsx
type InvitationPosterEditorProps = {
  design: InvitationPosterDesignV2
  scene: InvitationPosterScene
  selectedModule: PosterModuleId | null
  pendingUpload: InvitationPosterBackgroundResource | null
  onDesignChange: (design: InvitationPosterDesignV2) => void
  onSelectedModuleChange: (moduleId: PosterModuleId) => void
  onUpload: (file: File) => Promise<void>
}

export function InvitationPosterEditor(props: InvitationPosterEditorProps) {
  const dimensions = POSTER_SIZES[props.design.size]
  const updateRect = (moduleId: PosterModuleId, from: ModuleRect, proposed: ModuleRect) => {
    const result = constrainInvitationPosterTransform({
      design: props.design,
      size: props.design.size,
      moduleId,
      from,
      proposed,
      resolvedCopy: Object.fromEntries(props.scene.modules.map((item) => [item.id, item.text])),
    })
    props.onDesignChange(updateInvitationPosterModuleRect(props.design, props.design.size, moduleId, result.rect))
  }
  return (
    <div className='grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]'>
      <PosterCanvasOverlays dimensions={dimensions} scene={props.scene} onTransform={updateRect} />
      <PosterInspector {...props} />
    </div>
  )
}
```

The preview frame uses the selected `aspect-ratio`, is at most `45vh` on mobile, and overlays modules with percentage `left/top/width/height` derived from native rectangles. Pointer down captures only the selected overlay/handle; pointer move converts CSS pixels through `clientPointToPosterPoint`; pointer up releases capture. Arrow keys call `moveInvitationPosterModule` by `1` or `10` export pixels. Every overlay and handle has a 44-CSS-pixel minimum target and `aria-selected`; the canvas itself is `aria-hidden` in edit mode.

The inspector uses `ToggleGroup` for sizes, named built-in swatches, native `type='color'` plus a six-digit text input, hidden PNG/JPEG/WebP file input, `Slider` for zoom `1..4`, focal pan on the background, rune-counted copy inputs, nickname `Switch`, and a module list with `Lock` icons for `brand`, `qr`, `inviteCode`, and `domain`. Copy updates pass through `normalizeInvitationPosterDesign`, so revealing text cannot save a collision.

Create a pending resource locally with this exact conversion; this is the only upload-file handler:

```ts
const validated = await validateInvitationPosterUpload(file)
const lease = createInvitationPosterObjectUrlLease(validated.blob)
const resource: InvitationPosterBackgroundResource = {
  ...validated,
  assetId: crypto.randomUUID(),
  createdAt: Date.now(),
  objectUrl: lease.url,
}
pendingUploadLease.current?.release()
pendingUploadLease.current = lease
setPendingUpload(resource)
setDraft((current) => current ? {
  ...current,
  background: {
    kind: 'upload',
    assetId: resource.assetId,
    cropBySize: Object.fromEntries((['4:5', '3:4', '9:16'] as const).map((size) => [size, { focalX: 0.5, focalY: 0.5, zoom: 1 }])) as Record<InvitationPosterSize, BackgroundCrop>,
  },
} : current)
```

- [ ] **Step 6: Refactor the Default dialog into explicit preview/edit lifecycle and transactional Save**

```tsx
const [mode, setMode] = useState<'preview' | 'edit'>('preview')
const [activeDesign, setActiveDesign] = useState(createDefaultInvitationPosterDesign)
const [activeAsset, setActiveAsset] = useState<InvitationPosterBackgroundResource | null>(null)
const [pendingUpload, setPendingUpload] = useState<InvitationPosterBackgroundResource | null>(null)
const [draft, setDraft] = useState<InvitationPosterDesignV2 | null>(null)
const pencilRef = useRef<HTMLButtonElement>(null)
const activeUploadLease = useRef<InvitationPosterObjectUrlLease | null>(null)
const pendingUploadLease = useRef<InvitationPosterObjectUrlLease | null>(null)
const STORAGE_WARNING_KEYS = {
  'indexeddb-unavailable': 'This background is available only until this dialog closes',
  'localstorage-unavailable': 'Poster changes will reset after reload',
  'storage-unavailable': 'Poster changes will reset after reload',
  'missing-upload': 'Saved background was unavailable and has been reset',
} as const

function enterEditMode() {
  setDraft(structuredClone(activeDesign))
  setMode('edit')
}

async function saveDraft() {
  if (!draft) return
  const result = await saveInvitationPosterState({
    activeDesign,
    activeAsset,
    draftDesign: draft,
    pendingUpload,
    storage: window.localStorage,
    backgroundStore,
  })
  setActiveDesign(result.design)
  setActiveAsset(result.asset)
  if (pendingUpload && result.asset?.assetId === pendingUpload.assetId) {
    activeUploadLease.current?.release()
    activeUploadLease.current = pendingUploadLease.current
    pendingUploadLease.current = null
  } else if (result.design.background.kind !== 'upload') {
    activeUploadLease.current?.release()
    activeUploadLease.current = null
  }
  setPendingUpload(null)
  setDraft(null)
  setMode('preview')
  requestAnimationFrame(() => pencilRef.current?.focus())
  if (result.warning) toast.warning(t(STORAGE_WARNING_KEYS[result.warning]))
}

function cancelEditMode() {
  pendingUploadLease.current?.release()
  setPendingUpload(null)
  setDraft(null)
  setMode('preview')
  requestAnimationFrame(() => pencilRef.current?.focus())
}

function resetDraft() {
  pendingUploadLease.current?.release()
  pendingUploadLease.current = null
  setPendingUpload(null)
  setDraft(createDefaultInvitationPosterDesign())
}

const dirty = Boolean(draft && (
  pendingUpload || JSON.stringify(draft) !== JSON.stringify(activeDesign)
))

function requestDialogClose() {
  if (mode === 'edit' && dirty) {
    setDiscardConfirmationOpen(true)
    return
  }
  props.onOpenChange(false)
}
```

On every open transition, obtain the store with `await openInvitationPosterBackgroundStore().catch(() => createUnavailableInvitationPosterBackgroundStore())`, load state and asset, run orphan cleanup, and force `mode` to `preview`; never persist migration or repair during open. Wrap a loaded or newly validated blob once with `createInvitationPosterObjectUrlLease`, keep `{ ...record, objectUrl: lease.url }` in component state, and reuse that URL across redraws; `saveInvitationPosterState` strips `objectUrl` through `toStoredBackgroundRecord`. Render `activeDesign` in preview and `draft` in edit using one `requestAnimationFrame` scheduler plus an `AbortController`, cancelling the previous render before starting another. Pass the active or pending resource directly as `backgroundAsset`; never call `fetch`, XHR, or `resolveInvitationPosterLogoUrl` for a user upload.

Preview footer contains pencil and Download PNG. Edit footer contains icon+accessible-name Save, Cancel, and Reset; Reset assigns the complete default design and remains a draft. Download is absent in edit mode. Intercept close while `draft` differs from `activeDesign` and show an `AlertDialog` with Discard/Keep editing; a clean edit or preview closes immediately. On close/unmount, abort render, cancel the animation frame, release upload object URLs, and reset mode.

Keep the stable preview frame mounted during loading and errors. A Canvas/QR failure sets `renderError`, clears only the downloadable canvas, disables Download in preview, and leaves the pencil or Save/Cancel/Reset controls operable; blocked drag/resize uses one visual boundary state and never emits repeated toasts.

- [ ] **Step 7: Add complete Default translations and run focused checks**

Use these exact English keys in all seven Default locale files, with meaningful translations rather than copied English outside `en.json`:

```json
{
  "Edit poster": "Edit poster",
  "Poster background": "Poster background",
  "Clean light": "Clean light",
  "Ink contrast": "Ink contrast",
  "Coral paper": "Coral paper",
  "Solid color": "Solid color",
  "Local image": "Local image",
  "Move background": "Move background",
  "Zoom": "Zoom",
  "Poster modules": "Poster modules",
  "Eyebrow": "Eyebrow",
  "Inviter": "Inviter",
  "Invitation code": "Invitation code",
  "QR code": "QR code",
  "Reset poster": "Reset poster",
  "Discard poster changes?": "Discard poster changes?",
  "Discard changes": "Discard changes",
  "Keep editing": "Keep editing",
  "This image may look soft at the selected size": "This image may look soft at the selected size",
  "This background is available only until this dialog closes": "This background is available only until this dialog closes",
  "Poster changes will reset after reload": "Poster changes will reset after reload",
  "Saved background was unavailable and has been reset": "Saved background was unavailable and has been reset",
  "Use a PNG, JPEG, or WebP image up to 10 MiB": "Use a PNG, JPEG, or WebP image up to 10 MiB"
}
```

Use the keys in the English block in the same order with these exact locale values:

```text
fr.json: Modifier l'affiche | Arrière-plan de l'affiche | Clair épuré | Contraste encre | Papier corail | Couleur unie | Image locale | Déplacer l'arrière-plan | Zoom | Modules de l'affiche | Surtitre | Invitant | Code d'invitation | Code QR | Réinitialiser l'affiche | Abandonner les modifications de l'affiche ? | Abandonner les modifications | Continuer la modification | Cette image peut manquer de netteté au format sélectionné | Cet arrière-plan est disponible uniquement jusqu'à la fermeture de cette boîte de dialogue | Les modifications de l'affiche seront réinitialisées après le rechargement | L'arrière-plan enregistré était indisponible et a été réinitialisé | Utilisez une image PNG, JPEG ou WebP de 10 Mio maximum
ja.json: ポスターを編集 | ポスターの背景 | クリーンライト | インクコントラスト | コーラルペーパー | 単色 | ローカル画像 | 背景を移動 | ズーム | ポスターのモジュール | 見出し | 招待者 | 招待コード | QRコード | ポスターをリセット | ポスターの変更を破棄しますか？ | 変更を破棄 | 編集を続ける | 選択したサイズでは画像がぼやける可能性があります | この背景はこのダイアログを閉じるまでのみ利用できます | ポスターの変更は再読み込み後にリセットされます | 保存した背景を利用できなかったためリセットしました | 10 MiB以下のPNG、JPEG、WebP画像を使用してください
ru.json: Редактировать плакат | Фон плаката | Светлый минимализм | Чернильный контраст | Коралловая бумага | Сплошной цвет | Локальное изображение | Переместить фон | Масштаб | Модули плаката | Надзаголовок | Пригласивший | Код приглашения | QR-код | Сбросить плакат | Отменить изменения плаката? | Отменить изменения | Продолжить редактирование | Изображение может выглядеть размытым в выбранном размере | Этот фон доступен только до закрытия диалога | Изменения плаката будут сброшены после перезагрузки | Сохранённый фон недоступен и был сброшен | Используйте PNG, JPEG или WebP размером до 10 МиБ
vi.json: Chỉnh sửa áp phích | Nền áp phích | Sáng tinh gọn | Tương phản mực | Giấy san hô | Màu đơn | Ảnh cục bộ | Di chuyển nền | Thu phóng | Mô-đun áp phích | Dòng dẫn | Người mời | Mã mời | Mã QR | Đặt lại áp phích | Hủy các thay đổi áp phích? | Hủy thay đổi | Tiếp tục chỉnh sửa | Ảnh này có thể bị mờ ở kích thước đã chọn | Nền này chỉ khả dụng cho đến khi đóng hộp thoại | Các thay đổi áp phích sẽ được đặt lại sau khi tải lại | Nền đã lưu không khả dụng và đã được đặt lại | Sử dụng ảnh PNG, JPEG hoặc WebP tối đa 10 MiB
zh.json: 编辑海报 | 海报背景 | 清爽浅色 | 墨色高对比 | 珊瑚纸张 | 纯色 | 本地图片 | 移动背景 | 缩放 | 海报模块 | 眉题 | 邀请人 | 邀请码 | 二维码 | 重置海报 | 放弃海报更改？ | 放弃更改 | 继续编辑 | 此图片在所选尺寸下可能不够清晰 | 此背景仅在本次弹窗打开期间可用 | 海报更改将在刷新后重置 | 已保存的背景不可用，现已重置 | 请使用不超过 10 MiB 的 PNG、JPEG 或 WebP 图片
zh-TW.json: 編輯海報 | 海報背景 | 清爽淺色 | 墨色高對比 | 珊瑚紙張 | 純色 | 本機圖片 | 移動背景 | 縮放 | 海報模組 | 眉題 | 邀請人 | 邀請碼 | 二維碼 | 重設海報 | 放棄海報變更？ | 放棄變更 | 繼續編輯 | 此圖片在所選尺寸下可能不夠清晰 | 此背景僅在本次對話框開啟期間可用 | 海報變更將在重新載入後重設 | 已儲存的背景無法使用，現已重設 | 請使用不超過 10 MiB 的 PNG、JPEG 或 WebP 圖片
```

Where a locale already contains one of these keys (for example `Zoom`), update the existing entry in place; do not append a duplicate JSON key.

Run: `cd web/default && bun test src/features/invitations && bun run typecheck && bun run lint`

Expected: PASS with no duplicate schema validator, no TypeScript error, and every new translation key populated.

- [ ] **Step 8: Build Default and commit its editor**

Run: `cd web/default && bun run build:check`

Expected: PASS and `dist` production bundle completes.

```bash
git add web/default/src/features/invitations web/default/src/i18n/locales
git commit -m "feat(web): edit invitation posters locally"
```

### Task 6: Add Classic Parity Through Shared Rules

**Files:**
- Modify: `web/classic/src/components/invitations/poster-preferences.js`
- Modify: `web/classic/src/components/invitations/poster-preferences.test.js`
- Create: `web/classic/src/components/invitations/InvitationPosterEditor.jsx`
- Create: `web/classic/src/components/invitations/invitation-poster-editor.test.js`
- Modify: `web/classic/src/components/invitations/InvitationQRModal.jsx`
- Modify: `web/classic/src/components/invitations/invitation-ui.test.js`
- Modify: `web/classic/src/i18n/locales/en.json`
- Modify: `web/classic/src/i18n/locales/fr.json`
- Modify: `web/classic/src/i18n/locales/ja.json`
- Modify: `web/classic/src/i18n/locales/ru.json`
- Modify: `web/classic/src/i18n/locales/vi.json`
- Modify: `web/classic/src/i18n/locales/zh-CN.json`
- Modify: `web/classic/src/i18n/locales/zh-TW.json`
- Modify: `web/classic/src/i18n/locales/zh.json`

- [ ] **Step 1: Write failing Classic parity/shared-import tests**

```js
test('delegates Classic preferences and transforms to shared modules', () => {
  expect(preferencesSource).toContain("shared/invitation-poster-design.js")
  expect(preferencesSource).not.toContain('VALID_POSTER_SIZES')
  expect(editorSource).toContain('constrainInvitationPosterTransform')
  expect(editorSource).toContain('clientPointToPosterPoint')
  expect(modalSource).toContain('saveInvitationPosterState')
});

test('opens in preview and reveals editor only from its pencil button', () => {
  expect(modalSource).toContain("const [mode, setMode] = useState('preview')")
  expect(modalSource).toContain("aria-label={t('Edit poster')}")
  expect(modalSource).toContain('<InvitationPosterEditor')
  expect(modalSource).toContain("mode === 'preview'")
});
```

- [ ] **Step 2: Run Classic invitation tests and verify the red state**

Run: `cd web/classic && bun test src/components/invitations/poster-preferences.test.js src/components/invitations/invitation-poster-editor.test.js src/components/invitations/invitation-ui.test.js`

Expected: FAIL because Classic still persists v1 immediately and has no editor shell.

- [ ] **Step 3: Replace Classic preferences with shared compatibility exports**

```js
import {
  POSTER_DESIGN_STORAGE_KEY,
  createDefaultInvitationPosterDesign,
  normalizeInvitationPosterDesign,
} from '../../../../shared/invitation-poster-design.js';

export const POSTER_PREFERENCES_STORAGE_KEY = POSTER_DESIGN_STORAGE_KEY;
export const DEFAULT_POSTER_PREFERENCES = createDefaultInvitationPosterDesign();
export function readPosterPreferences(storage = browserStorage()) {
  try {
    return normalizeInvitationPosterDesign(JSON.parse(storage?.getItem(POSTER_DESIGN_STORAGE_KEY) || 'null')).design;
  } catch {
    return createDefaultInvitationPosterDesign();
  }
}
```

Remove Classic's independent valid-size set and immediate write/persist functions after `InvitationQRModal` switches to the shared storage transaction.

- [ ] **Step 4: Implement the Semi UI editor on the shared transform APIs**

```jsx
export default function InvitationPosterEditor({
  design, scene, selectedModule, pendingUpload,
  onDesignChange, onSelectedModuleChange, onUpload,
}) {
  const dimensions = POSTER_SIZES[design.size];
  const updateRect = (moduleId, from, proposed) => {
    const result = constrainInvitationPosterTransform({
      design, size: design.size, moduleId, from, proposed,
      resolvedCopy: Object.fromEntries(scene.modules.map((item) => [item.id, item.text])),
    });
    onDesignChange(updateInvitationPosterModuleRect(design, design.size, moduleId, result.rect));
  };
  return (
    <div className='grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]'>
      <PosterCanvasOverlays dimensions={dimensions} scene={scene} onTransform={updateRect} />
      <ClassicPosterInspector
        design={design}
        scene={scene}
        selectedModule={selectedModule}
        pendingUpload={pendingUpload}
        onDesignChange={onDesignChange}
        onSelectedModuleChange={onSelectedModuleChange}
        onUpload={onUpload}
      />
    </div>
  );
}
```

Use Semi `RadioGroup type='button'`, `Switch`, `Slider`, `Input`, `Button`, and `Tooltip`; use a native color input and hidden constrained file input where Semi has no stronger semantic control. Keep the same percentage overlays, pointer capture, 1/10-pixel keyboard moves, protected lock icons, crop controls, 44-pixel hit targets, and mobile preview cap as Default.

- [ ] **Step 5: Refactor `InvitationQRModal` into an explicit preview/edit lifecycle**

```jsx
const [mode, setMode] = useState('preview');
const [activeDesign, setActiveDesign] = useState(createDefaultInvitationPosterDesign);
const [draft, setDraft] = useState(null);
const pencilRef = useRef(null);
const activeUploadLease = useRef(null);
const pendingUploadLease = useRef(null);
const STORAGE_WARNING_KEYS = {
  'indexeddb-unavailable': 'This background is available only until this dialog closes',
  'localstorage-unavailable': 'Poster changes will reset after reload',
  'storage-unavailable': 'Poster changes will reset after reload',
  'missing-upload': 'Saved background was unavailable and has been reset',
};

const enterEditMode = () => {
  setDraft(structuredClone(activeDesign));
  setMode('edit');
};

const cancelEditMode = () => {
  pendingUploadLease.current?.release();
  setPendingUpload(null);
  setDraft(null);
  setMode('preview');
  requestAnimationFrame(() => pencilRef.current?.focus());
};

const saveDraft = async () => {
  if (!draft) return;
  const result = await saveInvitationPosterState({
    activeDesign,
    activeAsset,
    draftDesign: draft,
    pendingUpload,
    storage: window.localStorage,
    backgroundStore,
  });
  setActiveDesign(result.design);
  setActiveAsset(result.asset);
  if (pendingUpload && result.asset?.assetId === pendingUpload.assetId) {
    activeUploadLease.current?.release();
    activeUploadLease.current = pendingUploadLease.current;
    pendingUploadLease.current = null;
  } else if (result.design.background.kind !== 'upload') {
    activeUploadLease.current?.release();
    activeUploadLease.current = null;
  }
  setPendingUpload(null);
  setDraft(null);
  setMode('preview');
  requestAnimationFrame(() => pencilRef.current?.focus());
  if (result.warning) showWarning(t(STORAGE_WARNING_KEYS[result.warning]));
};

const resetDraft = () => {
  pendingUploadLease.current?.release();
  pendingUploadLease.current = null;
  setPendingUpload(null);
  setDraft(createDefaultInvitationPosterDesign());
};

const requestModalClose = () => {
  const dirty = Boolean(draft && (pendingUpload || JSON.stringify(draft) !== JSON.stringify(activeDesign)));
  if (mode !== 'edit' || !dirty) { onCancel(); return; }
  Modal.confirm({
    title: t('Discard poster changes?'),
    okText: t('Discard changes'),
    cancelText: t('Keep editing'),
    onOk: () => { cancelEditMode(); onCancel(); },
  });
};
```

Open the background store with `openInvitationPosterBackgroundStore().catch(() => createUnavailableInvitationPosterBackgroundStore())`, then use `loadInvitationPosterState`, `saveInvitationPosterState`, orphan cleanup, upload validator, scene resolver, and renderer. Open always resets to preview; Save/Cancel return focus; Reset stays reversible; Download exists only in preview; render work uses one animation-frame scheduler and one current abort controller. Use `Modal.confirm` with translated Discard/Keep editing actions for a dirty whole-modal close, and release all object URLs on replacement, cancel, close, or unmount.

- [ ] **Step 6: Add native translations to every Classic locale and verify parity**

Add these exact keys to all eight Classic locale files, updating existing entries rather than creating duplicate JSON keys:

```js
const posterEditorKeys = [
  'Edit poster', 'Poster background', 'Clean light', 'Ink contrast', 'Coral paper',
  'Solid color', 'Local image', 'Move background', 'Zoom', 'Poster modules',
  'Eyebrow', 'Inviter', 'Invitation code', 'QR code', 'Reset poster',
  'Discard poster changes?', 'Discard changes', 'Keep editing',
  'This image may look soft at the selected size',
  'This background is available only until this dialog closes',
  'Poster changes will reset after reload',
  'Saved background was unavailable and has been reset',
  'Use a PNG, JPEG, or WebP image up to 10 MiB',
];
```

Use these values in that order:

```text
en.json: Edit poster | Poster background | Clean light | Ink contrast | Coral paper | Solid color | Local image | Move background | Zoom | Poster modules | Eyebrow | Inviter | Invitation code | QR code | Reset poster | Discard poster changes? | Discard changes | Keep editing | This image may look soft at the selected size | This background is available only until this dialog closes | Poster changes will reset after reload | Saved background was unavailable and has been reset | Use a PNG, JPEG, or WebP image up to 10 MiB
fr.json: Modifier l'affiche | Arrière-plan de l'affiche | Clair épuré | Contraste encre | Papier corail | Couleur unie | Image locale | Déplacer l'arrière-plan | Zoom | Modules de l'affiche | Surtitre | Invitant | Code d'invitation | Code QR | Réinitialiser l'affiche | Abandonner les modifications de l'affiche ? | Abandonner les modifications | Continuer la modification | Cette image peut manquer de netteté au format sélectionné | Cet arrière-plan est disponible uniquement jusqu'à la fermeture de cette boîte de dialogue | Les modifications de l'affiche seront réinitialisées après le rechargement | L'arrière-plan enregistré était indisponible et a été réinitialisé | Utilisez une image PNG, JPEG ou WebP de 10 Mio maximum
ja.json: ポスターを編集 | ポスターの背景 | クリーンライト | インクコントラスト | コーラルペーパー | 単色 | ローカル画像 | 背景を移動 | ズーム | ポスターのモジュール | 見出し | 招待者 | 招待コード | QRコード | ポスターをリセット | ポスターの変更を破棄しますか？ | 変更を破棄 | 編集を続ける | 選択したサイズでは画像がぼやける可能性があります | この背景はこのダイアログを閉じるまでのみ利用できます | ポスターの変更は再読み込み後にリセットされます | 保存した背景を利用できなかったためリセットしました | 10 MiB以下のPNG、JPEG、WebP画像を使用してください
ru.json: Редактировать плакат | Фон плаката | Светлый минимализм | Чернильный контраст | Коралловая бумага | Сплошной цвет | Локальное изображение | Переместить фон | Масштаб | Модули плаката | Надзаголовок | Пригласивший | Код приглашения | QR-код | Сбросить плакат | Отменить изменения плаката? | Отменить изменения | Продолжить редактирование | Изображение может выглядеть размытым в выбранном размере | Этот фон доступен только до закрытия диалога | Изменения плаката будут сброшены после перезагрузки | Сохранённый фон недоступен и был сброшен | Используйте PNG, JPEG или WebP размером до 10 МиБ
vi.json: Chỉnh sửa áp phích | Nền áp phích | Sáng tinh gọn | Tương phản mực | Giấy san hô | Màu đơn | Ảnh cục bộ | Di chuyển nền | Thu phóng | Mô-đun áp phích | Dòng dẫn | Người mời | Mã mời | Mã QR | Đặt lại áp phích | Hủy các thay đổi áp phích? | Hủy thay đổi | Tiếp tục chỉnh sửa | Ảnh này có thể bị mờ ở kích thước đã chọn | Nền này chỉ khả dụng cho đến khi đóng hộp thoại | Các thay đổi áp phích sẽ được đặt lại sau khi tải lại | Nền đã lưu không khả dụng và đã được đặt lại | Sử dụng ảnh PNG, JPEG hoặc WebP tối đa 10 MiB
zh-CN.json and zh.json: 编辑海报 | 海报背景 | 清爽浅色 | 墨色高对比 | 珊瑚纸张 | 纯色 | 本地图片 | 移动背景 | 缩放 | 海报模块 | 眉题 | 邀请人 | 邀请码 | 二维码 | 重置海报 | 放弃海报更改？ | 放弃更改 | 继续编辑 | 此图片在所选尺寸下可能不够清晰 | 此背景仅在本次弹窗打开期间可用 | 海报更改将在刷新后重置 | 已保存的背景不可用，现已重置 | 请使用不超过 10 MiB 的 PNG、JPEG 或 WebP 图片
zh-TW.json: 編輯海報 | 海報背景 | 清爽淺色 | 墨色高對比 | 珊瑚紙張 | 純色 | 本機圖片 | 移動背景 | 縮放 | 海報模組 | 眉題 | 邀請人 | 邀請碼 | 二維碼 | 重設海報 | 放棄海報變更？ | 放棄變更 | 繼續編輯 | 此圖片在所選尺寸下可能不夠清晰 | 此背景僅在本次對話框開啟期間可用 | 海報變更將在重新載入後重設 | 已儲存的背景無法使用，現已重設 | 請使用不超過 10 MiB 的 PNG、JPEG 或 WebP 圖片
```

Run: `cd web/classic && bun test src/components/invitations && bun run eslint && bun run build`

Expected: PASS with no copied schema/constraint/storage rules, no formatter or bundle failure, and translated controls in every locale.

- [ ] **Step 7: Run both themes together and commit Classic parity**

Run: `cd web/default && bun run build:check`

Run: `cd web/classic && bun run build`

Expected: both production builds PASS against the same shared modules.

```bash
git add web/classic/src/components/invitations web/classic/src/i18n/locales
git commit -m "feat(classic): add modular poster editor"
```

### Task 7: Prove Real Browser Editing, Upload Privacy, PNG Integrity, And QR Payloads

**Files:**
- Modify: `web/default/package.json`
- Modify: `web/bun.lock`
- Modify: `web/default/e2e/production-invitation-flows.spec.ts`

- [ ] **Step 1: Add the deterministic QR decoder dependency**

Run: `cd web && bun add --dev --cwd default jsqr@1.4.0`

Expected: `web/default/package.json` lists `jsqr` in `devDependencies` and `web/bun.lock` resolves version `1.4.0`.

- [ ] **Step 2: Extend the E2E helper with exact QR decoding and generated local PNG upload**

```ts
import jsQR from 'jsqr'

async function decodePosterQr(canvas: Locator, expectedUrl: string) {
  const pixels = await canvas.evaluate((element) => {
    const canvas = element as HTMLCanvasElement
    const context = canvas.getContext('2d')
    if (!context) throw new Error('Poster canvas has no 2D context')
    const image = context.getImageData(0, 0, canvas.width, canvas.height)
    return { data: Array.from(image.data), width: image.width, height: image.height }
  })
  const decoded = jsQR(Uint8ClampedArray.from(pixels.data), pixels.width, pixels.height)
  expect(decoded?.data).toBe(expectedUrl)
}

async function chooseGeneratedBackground(fileInput: Locator) {
  await fileInput.evaluate(async (element) => {
    const canvas = document.createElement('canvas')
    canvas.width = 512
    canvas.height = 512
    const context = canvas.getContext('2d')!
    context.fillStyle = '#1F6F62'
    context.fillRect(0, 0, 256, 512)
    context.fillStyle = '#E97557'
    context.fillRect(256, 0, 256, 512)
    const blob = await new Promise<Blob>((resolve, reject) => canvas.toBlob((value) => value ? resolve(value) : reject(new Error('PNG fixture failed')), 'image/png'))
    const transfer = new DataTransfer()
    transfer.items.add(new File([blob], 'poster-background.png', { type: 'image/png' }))
    ;(element as HTMLInputElement).files = transfer.files
    element.dispatchEvent(new Event('change', { bubbles: true }))
  })
}
```

- [ ] **Step 3: Add failing preview/edit/persistence/privacy/recovery cases for both themes**

```ts
type PosterThemeCase = {
  theme: PosterTheme
  walletPath: string
  qrButtonName: string
  downloadButtonName: string
  editButtonName: string
  saveButtonName: string
  zoomName: string
}

const posterThemeCases: PosterThemeCase[] = [
  { theme: 'default', walletPath: '/wallet', qrButtonName: 'Show invitation QR code', downloadButtonName: 'Download PNG', editButtonName: 'Edit poster', saveButtonName: 'Save', zoomName: 'Zoom' },
  { theme: 'classic', walletPath: '/console/topup', qrButtonName: '生成二维码', downloadButtonName: '下载 PNG', editButtonName: '编辑海报', saveButtonName: '保存', zoomName: '缩放' },
]

for (const themeCase of posterThemeCases) {
  const dialog = await openInvitationPoster(page, themeCase, true)
  await expect(dialog.getByRole('button', { name: themeCase.editButtonName })).toBeVisible()
  await expect(dialog.getByRole('button', { name: themeCase.saveButtonName })).toHaveCount(0)
  await dialog.getByRole('button', { name: themeCase.editButtonName }).click()
  await expect(dialog.getByRole('button', { name: themeCase.saveButtonName })).toBeVisible()
  await expect(dialog.getByRole('button', { name: themeCase.downloadButtonName })).toHaveCount(0)

  const backgroundRequests = []
  page.on('request', (request) => {
    const url = new URL(request.url())
    if ((request.method() !== 'GET' && url.pathname.startsWith('/api/')) ||
        (request.resourceType() === 'image' && !/^(blob:|data:)/.test(request.url()) && url.pathname !== '/api/invitation/poster-logo')) {
      backgroundRequests.push(request.url())
    }
  })
  await chooseGeneratedBackground(dialog.locator('input[type=file]'))
  await dialog.getByLabel(themeCase.zoomName).fill('2')
  expect(backgroundRequests).toEqual([])
  await dialog.getByRole('button', { name: themeCase.saveButtonName }).click()
  await page.reload({ waitUntil: 'domcontentloaded' })
  const reopened = await openInvitationPoster(page, themeCase, true)
  await expect(reopened.getByRole('button', { name: themeCase.editButtonName })).toBeVisible()
}
```

Also cover Cancel restoring the prior preview, Reset followed by Cancel remaining reversible, dirty close requiring confirmation, keyboard 1/10-pixel movement, pointer drag collision stopping, square QR resize, independent `4:5`/`3:4`/`9:16` layouts and crops, and mandatory protected modules after emptying all four optional fields. Assert no page error, console error, overlap, horizontal overflow, or action-row occlusion at `1440x900`, `834x912`, and `320x700`.

- [ ] **Step 4: Extend export assertions across every ratio/background and failure path**

For each theme and ratio, export once with `clean-light` and once with the uploaded background. Reuse `assertPosterCanvas` and `assertPosterDownload` for exact dimensions/nonblank bytes, call `decodePosterQr(canvas, expectedRegistrationUrl)`, and compare background/brand pixel regions. Force `/api/invitation/poster-logo` to 404 and verify initial fallback export; corrupt `invitation-poster-preferences` and remove the referenced IndexedDB record between opens and verify default recovery plus one concise warning.

- [ ] **Step 5: Run the invitation browser suite against the preserved local service**

Run from `web/default`:

```powershell
$env:E2E_BASE_URL='http://127.0.0.1:18104'
bunx playwright test e2e/production-invitation-flows.spec.ts --project=chromium --reporter=line
```

Expected: PASS for Default and Classic; downloaded QR payloads equal the fixture registration URLs, upload bytes cause no HTTP request, all canvases are nonblank/origin-clean, and screenshots show no overflow at desktop/tablet/320px mobile.

- [ ] **Step 6: Commit executable browser acceptance**

```bash
git add web/default/package.json web/bun.lock web/default/e2e/production-invitation-flows.spec.ts
git commit -m "test(invitation): verify modular poster editor"
```

### Task 8: Run Full Regression And Preserve The Local Database Runtime

**Files:**
- Modify production or test files only when a new failing regression test identifies a defect in Tasks 1-7.

- [ ] **Step 1: Run the complete shared and frontend unit suites**

Run: `cd web && bun test shared`

Run: `cd web/default && bun test ./src ./rsbuild.config.test.ts && bun run typecheck && bun run lint && bun run build`

Run: `cd web/classic && bun test && bun run eslint && bun run build`

Expected: every command PASS; Default and Classic resolve the same v2 types/storage/scene/renderer and Snow files remain unchanged.

- [ ] **Step 2: Run backend logo-proxy and full Go regression tests**

Run: `go test ./controller ./service -run 'InvitationPosterLogo' -count=1`

Run: `go test ./... -count=1`

Expected: PASS with the canvas-safe proxy, SSRF/redirect/size controls, and initial fallback contract unchanged.

- [ ] **Step 3: Rebuild the local image while preserving the attached database volume**

Record baseline counts before rebuild:

```powershell
docker exec cloveapi-remote-copy-mysql sh -lc 'mysql -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse "SELECT COUNT(*) FROM users; SELECT COUNT(*) FROM logs;"'
git rev-parse --short HEAD
pwsh -File scripts/rebuild-local-mysql-qa.ps1 -ContainerName cloveapi-current-source-mysql-qa -ImageTag clove-api:local-20260730-poster-designer -HostPort 18104
```

Expected: the script reuses the existing `/data` volume, health returns HTTP 200 at `http://127.0.0.1:18104/api/status`, and no volume deletion/recreation occurs.

- [ ] **Step 4: Repeat database counts and run final browser smoke checks**

```powershell
docker exec cloveapi-remote-copy-mysql sh -lc 'mysql -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -Nse "SELECT COUNT(*) FROM users; SELECT COUNT(*) FROM logs;"'
$env:E2E_BASE_URL='http://127.0.0.1:18104'
Set-Location web/default
bunx playwright test e2e/production-invitation-flows.spec.ts --project=chromium --reporter=line
```

Expected: user/log baseline counts are unchanged by the rebuild itself, the invitation suite PASSes against the preserved database, and the service remains healthy on port `18104` for user testing.

- [ ] **Step 5: Verify repository scope and create the final local implementation commit only if fixes remain**

Run:

```powershell
git diff --check
git status --short
git diff --name-only HEAD -- web/snow/dist
```

Expected: `git diff --check` is clean, no file under `web/snow/dist` is listed, no backend schema/API file was added for user backgrounds, and no production/server/release artifact changed. If Task 8 exposed a defect, add its focused regression test and owning source file, rerun the failing command plus all Task 8 commands, then commit those residual fixes:

```bash
git add web/shared web/default/src/features/invitations web/classic/src/components/invitations web/default/e2e/production-invitation-flows.spec.ts
git commit -m "fix(invitation): close poster editor regressions"
```

Do not push, deploy to `103.205.240.82`, publish a GitHub Release, edit production settings, reset a database, or delete a Docker volume.
