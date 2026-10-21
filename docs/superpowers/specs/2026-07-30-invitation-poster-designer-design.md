# Invitation Poster Modular Designer Design

## Status And Scope

- Date: 2026-07-30
- Scope: browser-local invitation poster customization for Default and Classic
- Delivery: local source, local QA runtime, and the later verified local feature commit only
- Excluded: production deployment, remote push, GitHub Release, server-side poster templates, and account-synchronized designs

This design extends, rather than replaces, the existing invitation poster work:

- [`2026-07-29-invitation-poster-export-design.md`](./2026-07-29-invitation-poster-export-design.md) remains authoritative for the complete PNG export, the three output dimensions, the real invitation QR code, the site-level title/subtitle settings, nickname fallback, and filename behavior.
- [`2026-07-30-invitation-poster-brand-logo-design.md`](./2026-07-30-invitation-poster-brand-logo-design.md) remains authoritative for the fixed same-origin logo endpoint, SSRF and size controls, canvas-safe logo loading, the six-second browser deadline, cancellation, and the initial-letter fallback.

The new work adds a modular editor and local persistence around that proven export path. A missing or corrupt saved design must always resolve to the current poster appearance, so an upgrade cannot make an existing invitation poster unavailable.

## Goals

1. Open the invitation dialog in a clean preview state and require an explicit pencil-icon action before editing.
2. Let a user arrange poster modules, customize four copy fields, select a built-in or solid background, or crop a locally uploaded background.
3. Keep the current site identity, invitation code, domain, and real QR code trustworthy and permanently present.
4. Produce the same pixels for preview and PNG export from one shared schema and renderer in Default and Classic.
5. Keep all user customization inside the current browser profile. No design, copy, or uploaded background is sent to the application server.
6. Preserve reliable export, QR decoding, canvas origin cleanliness, and the configured-logo fallback across `4:5`, `3:4`, and `9:16`.

## Non-Goals

- There is no server-side template library, synchronization between browsers, public template marketplace, collaborative editing, or administrator inspection of user designs.
- Users cannot replace the site logo, site name, domain, invitation code, or QR payload.
- The first version does not add custom fonts, arbitrary font sizes, filters, stickers, freehand drawing, layer ordering, or overlapping modules.
- The editor does not modify the vendored Snow package.

## Architectural Decision

Use a shared scene schema, validator, storage adapter, and Canvas renderer, with thin theme-specific editor shells.

The shared code under `web/shared/` owns these boundaries:

1. **Poster schema and defaults:** version parsing, v1 migration, canonical layouts for all three sizes, per-field normalization, module constraints, and copy resolution.
2. **Poster storage:** the versioned localStorage document, IndexedDB background records, in-memory fallback, and orphan cleanup.
3. **Scene resolver:** combines a validated local design with current runtime identity data and produces immutable draw commands and hit-test rectangles.
4. **Renderer:** draws the background, site branding, copy, real QR code, invitation code, and domain to a full-resolution offscreen Canvas.

Default and Classic own only their dialog composition, theme-native form controls, toasts, and accessibility wiring. Both pass the same shared design and runtime data to the same scene resolver and renderer. Existing duplicate `poster-preferences` helpers become thin compatibility wrappers over the shared storage/schema module or are removed once their imports are migrated; they must not retain separate validation rules.

Snow is only a home surface and does not own the invitation route. Navigation from Snow enters the existing Default application route, which uses the Default editor. No file under `web/snow/dist/` is edited, regenerated, or used as a second implementation.

## Trust Boundary And Runtime Identity

The persisted design contains presentation preferences only. The following values are supplied fresh by the open invitation dialog and are never written to localStorage or IndexedDB:

- site name;
- configured site logo URL;
- current site domain;
- invitation code;
- registration URL and QR payload;
- authenticated user's display name or username fallback.

This prevents a saved design from displaying a stale code, another account's nickname, or an old deployment's identity. The scene resolver treats these values as protected runtime facts. The editor can move and resize their modules within constraints, but it cannot edit, clear, replace, or hide their content.

The configured site logo continues through `resolveInvitationPosterLogoUrl`. Cross-origin HTTP(S) logos use `/api/invitation/poster-logo`; a failure renders the existing site-name initial fallback. User-uploaded backgrounds are a different trust domain: they are read from a local `Blob`, never passed to the logo resolver, and never requested through `/api/invitation/poster-logo`.

## Versioned Design Schema

The existing localStorage key `invitation-poster-preferences` is retained. Its payload advances from version 1 to version 2 so both frontends observe one origin-local design.

Conceptually, version 2 has this shape:

```ts
type PosterSize = '4:5' | '3:4' | '9:16'
type ModuleId =
  | 'brand'
  | 'eyebrow'
  | 'inviter'
  | 'title'
  | 'subtitle'
  | 'qr'
  | 'inviteCode'
  | 'domain'
  | 'footer'

type CopyValue =
  | { source: 'site' }
  | { source: 'custom'; value: string }

type ModuleRect = {
  x: number
  y: number
  width: number
  height: number
}

type BackgroundCrop = {
  focalX: number // 0 through 1
  focalY: number // 0 through 1
  zoom: number   // 1 through 4; 1 means cover
}

type PosterBackground =
  | { kind: 'builtin'; id: 'clean-light' | 'ink-contrast' | 'coral-paper' }
  | { kind: 'solid'; color: `#${string}` }
  | {
      kind: 'upload'
      assetId: string
      cropBySize: Record<PosterSize, BackgroundCrop>
    }

interface InvitationPosterDesignV2 {
  version: 2
  size: PosterSize
  showNickname: boolean
  background: PosterBackground
  copy: {
    eyebrow: CopyValue
    title: CopyValue
    subtitle: CopyValue
    footer: CopyValue
  }
  layouts: Record<PosterSize, Record<ModuleId, ModuleRect>>
}
```

Coordinates are integer export pixels in each size's native 1080-pixel-wide canvas, not CSS preview pixels. Each ratio keeps an independent layout because a useful `4:5` arrangement does not map mechanically to `9:16`. Switching sizes loads that size's saved layout; a size never edited uses its canonical layout. Pointer coordinates in the scaled preview are transformed back into native canvas coordinates before validation.

The schema deliberately omits visibility, z-index, lock flags, and runtime identity strings. Visibility is policy-driven, drawing order is fixed, and protected status is defined by the module registry rather than user data.

### Copy Resolution

- `source: 'site'` means use the current administrator title/subtitle when present, otherwise the current localized defaults from the existing export design.
- The eyebrow and footer use their current localized labels when their source is `site`.
- `source: 'custom'` renders the supplied value as plain text. A custom empty string hides that optional copy module without deleting its saved rectangle.
- The title keeps the existing `{站点名称}` substitution. Custom content is never interpreted as HTML.
- Limits are 32 Unicode code points for the eyebrow, 80 for the title, 160 for the subtitle, and 64 for the footer. Counting uses Unicode code points, matching the existing title/subtitle behavior.
- `showNickname` remains the only visibility control for the inviter module. Its content is always derived from the current account and is not editable.

### Migration And Normalization

A valid v1 payload migrates in memory by preserving its valid `size` and `showNickname`, then adding the default background, site-sourced copy, and canonical layouts. It is written as v2 only after the next successful Save, avoiding an unsolicited storage write when the dialog is merely opened.

Normalization follows these deterministic rules:

- Missing JSON, malformed JSON, a non-object root, or an unknown version yields a complete default v2 design.
- In a v2 document, invalid scalar fields fall back individually. An invalid or colliding layout resets only the affected poster size to its canonical layout.
- Unknown module IDs are ignored; missing registered module IDs receive their canonical rectangles.
- Non-finite coordinates, out-of-range sizes, invalid colors, unknown built-in IDs, and invalid crop values are rejected by the normalizer.
- A missing, unreadable, or undecodable IndexedDB asset resets only the background to `clean-light`; valid text, size, nickname, and layouts remain active.
- Normalization never throws into the dialog render path. A repaired in-memory design is persisted only on a later explicit Save.

## Background Model

Three source-neutral built-in recipes are rendered locally by the shared Canvas renderer:

- `clean-light`: the current poster's light presentation and the global default;
- `ink-contrast`: a high-contrast dark treatment with accessible light text surfaces;
- `coral-paper`: a warm accent treatment distinct from the light and dark options.

These are stable renderer-owned IDs, require no network request, and retain sufficient contrast behind every standard content module. Solid backgrounds accept only canonical six-digit `#RRGGBB` colors; the UI uses a color swatch/picker and stores the normalized uppercase value.

Local uploads support PNG, JPEG, and WebP. SVG and animated formats are rejected to avoid external-resource, script, animation-frame, and export ambiguity. The browser validates both the declared file type and successful image decode. A file may be at most 10 MiB, each dimension must be between 256 and 8192 pixels, and decoded area may not exceed 32 megapixels. Images below the selected export dimensions remain usable but receive a non-blocking resolution warning.

Uploaded bytes are not destructively cropped or recompressed. The design stores normalized focal coordinates and zoom for each poster size. Zoom `1` is the minimum centered `cover` scale; zoom may increase to `4`. Dragging the selected background changes `focalX` and `focalY`, while a slider changes zoom. The renderer computes the source rectangle and clips it exactly to the poster canvas, so preview and export use identical cropping.

## Local Persistence

Structured design data stays in localStorage because it is small and synchronous to restore. Uploaded blobs use IndexedDB so large binary data is never serialized into localStorage.

The IndexedDB database is `clove-invitation-poster`, version 1, with a `backgrounds` object store keyed by an opaque random `assetId`. A record contains the blob, detected MIME type, decoded width and height, and creation timestamp. It contains no user ID, invitation code, URL, or server credential.

Save uses this order:

1. Validate and normalize the complete draft.
2. If a new upload is selected, put its decoded blob record into IndexedDB.
3. Write the v2 document referencing that `assetId` to localStorage.
4. Promote the draft to the active in-memory design.
5. Best-effort delete the previously referenced upload and any unreferenced record created by an interrupted save.

If step 2 fails, the uploaded background remains usable for the current open dialog but is marked session-only; localStorage is left at the previous complete design, so it never receives a dangling asset reference. If step 3 fails, the active in-memory design still updates and remains downloadable, but the UI reports that the entire edit will not survive a reload; the newly written IndexedDB record is removed when possible and the in-memory blob remains the render source. The application never falls back to uploading the image to the server.

Cancel discards the draft and any unsaved in-memory upload, revokes its object URL, and returns to the last active preview. Reset replaces the draft with the complete default design: `3:4`, nickname shown, `clean-light`, site-sourced copy, and canonical layouts for all sizes. Reset is reversible through Cancel and is not persisted until Save.

Object URLs are scoped to a loaded asset, reused across draft redraws, and revoked when replaced, cancelled, the dialog closes, or the component unmounts. A startup cleanup removes only IndexedDB assets not referenced by the current valid v2 document and older than 24 hours, protecting an in-progress save in another tab.

## Modules And Layout Constraints

Every size uses a 72-pixel safe inset on all four canvas edges. A module transform must remain fully inside that safe rectangle. Visible modules use a 16-pixel collision gutter, and no user module may overlap another visible module.

| Module | Content | Visibility | Resize rule | Minimum rectangle |
| --- | --- | --- | --- | --- |
| `brand` | Current logo/fallback plus site name | Mandatory | Width and height scale together | `240 x 72` |
| `eyebrow` | Site or custom copy | Optional when custom text is empty | Width only; height follows its text box | `160 x 40` |
| `inviter` | Current display name/username | Controlled only by `showNickname` | Width only | `240 x 44` |
| `title` | Site or custom copy | Optional when custom text is empty | Width only | `320 x 120` |
| `subtitle` | Site or custom copy | Optional when custom text is empty | Width only | `320 x 80` |
| `qr` | Real registration QR and white quiet-zone plate | Mandatory | Corner resize, square aspect locked | `360 x 360` |
| `inviteCode` | Current invitation code and label | Mandatory | Width only | `240 x 80` |
| `domain` | Current site domain | Mandatory | Width only | `240 x 48` |
| `footer` | Site or custom copy | Optional when custom text is empty | Width only | `240 x 48` |

Text modules have renderer-owned font sizes, line heights, and maximum line counts. Resizing changes the wrapping width, not the font scale. Overflow uses the existing line-aware ellipsis and never increases the rectangle into a neighbor.

The QR selection rectangle represents its white plate. The real QR is inset by at least 7% of the plate width and retains the QR generator's four-module encoded margin. Collision testing expands the plate by another 24 export pixels, forming a protected quiet-zone guard that no other module may enter. Decorations and background do not enter this guard. The QR payload, correction level, square aspect, and rendered modules are never editable.

Dragging and resizing use pointer capture for mouse, pen, and touch. The constraint solver clamps a proposed transform to the safe area, then stops it at the closest valid non-colliding position along the gesture path. An invalid resize remains at the last valid rectangle. There is no transient invalid layout that can be saved. Keyboard users move a selected module by 1 export pixel with arrow keys or 10 pixels with Shift+arrow; resize handles and module selectors have keyboard equivalents and at least 44 CSS-pixel touch targets.

## Interaction Design

### Preview Mode

Every dialog opening starts in preview mode, regardless of the mode used before it was closed. Preview mode contains the scaled poster, a compact pencil icon button with an `Edit poster` tooltip and accessible name, and the existing Download PNG action. Editing controls are not shown. The preview uses the last active local design and current runtime identity.

The pencil clones the active design into a draft and enters edit mode. Edit mode itself is ephemeral and never stored.

### Edit Mode

Desktop uses an unframed two-column work area: the bounded poster preview on the left and a compact inspector on the right. Mobile uses one vertical scroll surface with a preview no taller than 45% of the viewport, followed by segmented inspector sections and a sticky action row. The canvas retains its selected ratio through a stable `aspect-ratio` container, so loading states and toolbars do not shift the dialog.

The editor provides:

- a `4:5 / 3:4 / 9:16` segmented size control;
- a background source selector for built-in, solid, and local upload;
- named swatches for a built-in background, a color swatch and six-digit color input for a solid background, and pan plus zoom controls for an uploaded background;
- text inputs for eyebrow, title, subtitle, and footer with remaining character counts;
- the existing nickname switch;
- a module list with lock indicators for protected modules and selection synchronization with the canvas;
- icon actions for Save, Cancel, and Reset to defaults, each with an accessible name and tooltip where the icon is not self-explanatory.

Selecting a module on the canvas or in the module list shows its drag boundary and allowed resize handles. Protected means content and visibility are locked, not position: the brand, QR, invitation code, and domain may still be arranged within their constraints. Site name stays inside the protected brand module and is never independently hidden.

Draft changes redraw through one animation-frame scheduler and abort any obsolete asynchronous render. Save validates once more, commits locally, exits to preview, and returns focus to the pencil button. Cancel exits to preview without persistence and also returns focus. Closing the whole dialog with a dirty draft asks for discard confirmation; closing a clean editor or the preview closes immediately.

Download is available only in preview mode and always exports that visible active design. Edit mode replaces the download action with Save and Cancel, so an export cannot silently contain an unsaved draft.

## Rendering And Export

`renderInvitationPoster` is extended to accept the normalized design plus protected runtime input. Both preview and download call the same renderer at the selected native dimensions:

- `4:5`: `1080 x 1350`;
- `3:4`: `1080 x 1440`;
- `9:16`: `1080 x 1920`.

The browser displays that full-resolution canvas with CSS `object-contain`; it does not render a separate low-resolution approximation. Editor hit targets are DOM overlays derived from the same resolved scene rectangles and are excluded from export.

Rendering order is fixed: background, background contrast surface where required by the selected built-in recipe, optional text modules, protected brand/details modules, and the QR plate/QR. Because module overlap is disallowed, users do not need or receive z-index controls.

The resolver loads resources through distinct functions:

- site logo: the existing canvas-safe logo URL resolver and initial-letter fallback;
- built-in recipe or solid color: synchronous local drawing;
- uploaded background: an IndexedDB blob exposed as a temporary local object URL;
- QR: the existing high-correction real QR canvas, scaled on integer boundaries.

An uploaded background never enters the logo proxy or any fetch/XHR request. Canvas export re-renders decoded pixels and therefore does not preserve uploaded image metadata. `canvasToPngBlob`, filename sanitization, empty-blob failure handling, and download URL cleanup remain unchanged.

## Error Handling

- **Invalid saved JSON or schema:** use the normalized defaults without blocking the dialog.
- **Missing or corrupt uploaded asset:** keep all other preferences, switch the background to `clean-light`, and show one concise local warning.
- **Unsupported, oversized, or undecodable upload:** reject it before it becomes the draft background and keep the previous selection.
- **IndexedDB unavailable or quota exceeded:** allow a decoded upload for this open dialog, mark it session-only, and state that it will not persist. No server fallback is attempted.
- **localStorage unavailable or quota exceeded:** keep the saved result in memory for preview/download and state that it will reset after reload.
- **Built-in background failure:** render `clean-light` and continue.
- **Configured logo failure:** render the existing site-name initial and continue; this is not reported as proof that the configured image loaded.
- **QR or Canvas failure:** retain the render-error view and disable download, while leaving editor controls available so the user can cancel or save preferences.
- **Stale render or closed dialog:** abort resource loading, suppress stale state updates, and revoke temporary object URLs.
- **Invalid drag/resize:** retain the last valid rectangle and show the blocked boundary visually; do not emit repeated toast noise.

Errors remain local and concise. Successful redraws, drags, storage reads, cache hits, and downloads do not create application usage logs or server audit events.

## Accessibility And Responsive Behavior

- The pencil, reset, save, cancel, upload, and module actions have accessible names; unfamiliar icons also have tooltips.
- The poster preview exposes one descriptive image label in preview mode. Edit-mode module overlays expose module names and selected state without causing the underlying canvas to be announced repeatedly.
- Focus is trapped by the existing dialog implementation, enters the first editor control after the pencil action, and returns predictably on Save or Cancel.
- Color selection does not communicate state by color alone. Built-in background choices include names and selected state.
- Pointer-only operations have keyboard controls. Touch dragging disables browser panning only for the active handle, leaving the rest of the mobile dialog scrollable.
- Text and controls wrap within the dialog at 320 CSS pixels and at desktop widths. The poster's exported font sizes do not scale with viewport width.
- Loading and error overlays occupy the stable preview frame and cannot overlap the dialog action row.

## Test Strategy

Implementation follows test-first changes. The acceptance matrix includes the following layers.

### Shared Schema And Constraint Tests

- Migrate valid v1 `size` and `showNickname` into a complete v2 design.
- Fall back safely for malformed JSON, unknown versions, non-finite rectangles, unknown modules, bad colors, invalid crop values, collisions, and out-of-bounds layouts.
- Reset only the affected size when one saved size layout is invalid.
- Resolve `site` versus `custom` copy, deliberate empty optional text, Unicode limits, and `{站点名称}` substitution.
- Enforce mandatory module visibility, minimum rectangles, square QR resizing, the 72-pixel safe area, 16-pixel module gutters, and the expanded QR guard.
- Preserve independent layouts and upload crops while switching among all three sizes.

### Storage And Upload Tests

- Read and write the v2 localStorage document through the shared adapter in both frontends.
- Store a valid blob before its design reference, clean replaced/orphaned assets, and never delete a currently referenced asset.
- Verify Cancel and Reset do not persist and revoke unsaved object URLs.
- Simulate localStorage and IndexedDB exceptions, quota errors, missing blobs, corrupt decoded bytes, and future schema versions.
- Accept valid PNG/JPEG/WebP inputs and reject SVG, animated/unsupported formats, oversize bytes, oversize dimensions, and excessive decoded area.
- Inspect network requests and prove a local upload never reaches an API endpoint, the configured URL, or `/api/invitation/poster-logo`.

### Shared Renderer Tests

- Render all three exact output dimensions from the same schema used by Default and Classic.
- Exercise every built-in recipe, a normalized solid color, and uploaded-image cover/crop/zoom at focal-point boundaries.
- Prove protected site name, logo/fallback, invitation code, domain, and real QR remain present when optional copy is empty.
- Verify long site names, custom copy at each limit, nickname fallback, and resized text boxes use ellipsis without collision.
- Confirm a real proxied logo reaches `drawImage`, logo failure uses the initial, and uploaded background loading bypasses the logo resolver.
- Verify nonblank pixels, an origin-clean PNG, valid PNG headers, and successful QR decode to the exact registration URL for every size and background kind.

### Default And Classic Component Tests

- Open in preview mode with editing controls absent, then enter edit mode only through the pencil action.
- Cover size selection, background selection/upload, copy editing, nickname toggling, canvas/module-list selection, keyboard movement, pointer movement, constrained resizing, Save, Cancel, Reset, and dirty-close confirmation.
- Prove Save updates the preview, Cancel restores it, Reset is reversible until Save, and reopening always starts in preview mode.
- Prove both theme wrappers import the shared schema, storage adapter, scene resolver, and renderer rather than implementing their own layout or validation.
- Verify render cancellation and object URL cleanup when settings change or the dialog closes.

### Browser Verification

Run Playwright against the preserved local database for Default and Classic at representative desktop and 320-pixel mobile viewports. Capture preview and edit screenshots for all ratios, assert that controls do not overlap or overflow, and exercise mouse, touch-compatible pointer, and keyboard editing.

For each ratio, download a poster with a built-in background and an uploaded background, verify exact pixel dimensions and nonblank canvas regions, decode the real QR, and confirm the configured logo image is present. Force the logo endpoint to fail and repeat one export to prove the initial fallback remains downloadable. Corrupt localStorage and remove the referenced IndexedDB blob between openings to prove safe recovery.

When Snow home mode is active, navigate from the Snow surface to the invitation route and confirm the Default dialog/editor is used; do not test or modify a separate Snow implementation.

After targeted tests pass, run both frontend test suites and production builds, the existing invitation Playwright flow, canvas pixel checks, QR decode checks, and browser console/network error review. The Go suite remains required because the existing poster-logo endpoint must not regress, even though this feature adds no backend API or database migration.

## Delivery And Acceptance

- Work only in the local source tree and preserved local runtime database.
- Do not deploy to `103.205.240.82`, change production options, push a remote branch, or create a release for this feature.
- Do not add an API, database table, object-storage bucket, or proxy behavior for user backgrounds.
- Do not edit or regenerate vendored Snow assets.
- Keep the existing canvas-safe logo endpoint and initial fallback intact.
- The feature is ready for its local commit only after Default and Classic unit/build checks, shared renderer/storage tests, desktop/mobile Playwright coverage, QR decoding, canvas pixel checks, upload privacy assertions, and the existing backend logo tests all pass.

Acceptance is complete when a user can open a normal preview, explicitly enter the editor, customize every supported module and background on desktop or mobile, save locally, reopen the same design, and download a valid full poster while every protected identity element and the real QR code remain present and verifiable.
