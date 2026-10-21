# Invitation Poster Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace raw invitation QR downloads with configurable, responsive, full-resolution invitation poster PNGs in both frontend themes.

**Architecture:** Store only title/subtitle templates on the server, keep user display preferences in versioned browser storage, and render deterministic fixed-pixel posters through a shared Canvas utility that accepts an existing QR canvas. The dialog shows a scaled preview while export always renders at 1080px width.

**Tech Stack:** Go settings API, React/TypeScript and React/Semi UI, Canvas 2D, qrcode.react, Bun tests, Playwright.

---

## File Structure

- Modify `setting/registration_setting/config.go`: poster title/subtitle settings, normalization, length validation.
- Modify `controller/registration.go` and related tests: persist and audit poster settings through the existing registration settings API.
- Modify `controller/misc.go`: expose resolved poster settings needed by the authenticated frontend if the registration settings endpoint is admin-only.
- Create `web/shared/invitation-poster.js`: dependency-free Canvas layout/render/download utility usable by both bundles.
- Create `web/shared/invitation-poster.test.js`: fixed dimensions, wrapping, fallback, and PNG contract.
- Modify default and Classic QR dialogs/cards to provide code, site identity, display name, controls, and preview.
- Modify registration settings pages in both themes for title/subtitle controls.

### Task 1: Add Configurable Poster Text Settings

**Files:**
- Modify: `setting/registration_setting/config.go`
- Modify: `setting/registration_setting/config_test.go`
- Modify: `controller/registration.go`
- Modify: `controller/feature_setting_test.go`
- Modify: `controller/misc.go`
- Modify: `controller/misc_test.go`

- [ ] **Step 1: Write failing normalization and API tests**

```go
func TestInvitationPosterSettingsTrimAndLimitText(t *testing.T) {
    settings := NormalizeSettings(Settings{
        InvitationPosterTitle: "  Welcome to {站点名称}  ",
        InvitationPosterSubtitle: " Scan to register ",
    })
    assert.Equal(t, "Welcome to {站点名称}", settings.InvitationPosterTitle)
    assert.Equal(t, "Scan to register", settings.InvitationPosterSubtitle)
    settings.InvitationPosterTitle = strings.Repeat("x", 81)
    require.Error(t, ValidateSettings(settings))
}
```

Add response assertions for both admin settings and the public/authenticated status payload used by the dialog.

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./setting/registration_setting ./controller -run 'InvitationPoster' -count=1`

Expected: FAIL because poster fields and keys do not exist.

- [ ] **Step 3: Add keys, defaults, validation, persistence, and audit fields**

```go
const (
    InvitationPosterTitleKey = "registration.invitation_poster_title"
    InvitationPosterSubtitleKey = "registration.invitation_poster_subtitle"
    InvitationPosterTitleMaxRunes = 80
    InvitationPosterSubtitleMaxRunes = 160
)
```

Store empty strings as empty strings so the frontend can select localized defaults. Validate by Unicode rune count and expose plain text only. Include before/after values in the existing registration settings audit event.

- [ ] **Step 4: Run settings/controller tests**

Run: `go test ./setting/registration_setting ./controller -run 'Registration|InvitationPoster|GetStatus' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit backend settings**

```bash
git add setting/registration_setting controller/registration.go controller/feature_setting_test.go controller/misc.go controller/misc_test.go
git commit -m "feat(invitation): configure poster copy"
```

### Task 2: Build the Shared Canvas Renderer

**Files:**
- Create: `web/shared/invitation-poster.js`
- Create: `web/shared/invitation-poster.test.js`
- Modify: `web/package.json` only if a test script path must include `shared`.

- [ ] **Step 1: Write failing renderer tests**

```js
test.each([
  ['4:5', 1080, 1350],
  ['3:4', 1080, 1440],
  ['9:16', 1080, 1920],
])('renders %s at exact dimensions', async (size, width, height) => {
  const canvas = await renderInvitationPoster(fixture({ size }));
  expect(canvas.width).toBe(width);
  expect(canvas.height).toBe(height);
});
```

Add tests that long text stays outside the QR rectangle, nickname is omitted when disabled, and logo load rejection uses the text-brand path.

- [ ] **Step 2: Verify renderer tests fail**

Run: `cd web && bun test shared/invitation-poster.test.js`

Expected: FAIL because the shared renderer does not exist.

- [ ] **Step 3: Implement a dependency-free rendering interface**

```js
export const POSTER_SIZES = {
  '4:5': { width: 1080, height: 1350 },
  '3:4': { width: 1080, height: 1440 },
  '9:16': { width: 1080, height: 1920 },
};

export async function renderInvitationPoster(input) {
  const dimensions = POSTER_SIZES[input.size] || POSTER_SIZES['3:4'];
  const canvas = document.createElement('canvas');
  canvas.width = dimensions.width;
  canvas.height = dimensions.height;
  const context = canvas.getContext('2d');
  if (!context) throw new Error('Canvas 2D is unavailable');
  // Draw the confirmed B information-frame composition, then draw the real QR canvas.
  return canvas;
}
```

Implement explicit text wrapping with maximum line counts, integer QR scaling, quiet-zone preservation, logo timeout/fallback, `canvasToPngBlob`, and sanitized filename generation. Keep layout constants internal and data inputs explicit.

- [ ] **Step 4: Run shared tests**

Run: `cd web && bun test shared/invitation-poster.test.js`

Expected: PASS.

- [ ] **Step 5: Commit the renderer**

```bash
git add web/shared web/package.json
git commit -m "feat(invitation): render full poster canvases"
```

### Task 3: Integrate the Default Frontend

**Files:**
- Modify: `web/default/src/features/invitations/components/invitation-qr-dialog.tsx`
- Modify: `web/default/src/features/invitations/components/user-invitation-card.tsx`
- Create: `web/default/src/features/invitations/lib/poster-preferences.ts`
- Create: `web/default/src/features/invitations/lib/poster-preferences.test.ts`
- Modify: `web/default/src/features/invitations/components/invitation-ui.test.ts`
- Modify: `web/default/src/features/system-settings/site/registration-section.tsx`
- Modify: `web/default/src/features/system-settings/types.ts`

- [ ] **Step 1: Write failing preference and component contract tests**

```ts
test('uses enabled nickname and 3:4 for first-time preferences', () => {
  expect(readPosterPreferences(emptyStorage)).toEqual({ size: '3:4', showNickname: true })
})

test('ignores an unknown stored size', () => {
  expect(readPosterPreferences(storageWith('{"version":1,"size":"square","showNickname":false}'))).toEqual({
    size: '3:4', showNickname: false,
  })
})
```

- [ ] **Step 2: Verify default frontend tests fail**

Run: `cd web/default && bun test src/features/invitations`

Expected: FAIL because poster preferences and controls are absent.

- [ ] **Step 3: Implement preview, controls, and settings fields**

Pass `code`, `value`, `systemName`, `logo`, and `displayName` to the dialog. Render a stable preview container, a three-option segmented control, and a checked-by-default nickname switch. Use a hidden `QRCodeCanvas` as the real QR input and call the shared renderer for preview/download. Disable download while rendering and show translated errors through `toast`.

Add poster title/subtitle inputs to the registration settings section with rune-aware limits and `{站点名称}` helper text in the admin form.

- [ ] **Step 4: Run default frontend checks**

Run: `cd web/default && bun test src/features/invitations src/features/system-settings/site && bun run typecheck && bun run lint && bun run build`

Expected: PASS.

- [ ] **Step 5: Commit default integration**

```bash
git add web/default/src/features/invitations web/default/src/features/system-settings
git commit -m "feat(web): download invitation posters"
```

### Task 4: Add Classic Frontend Parity

**Files:**
- Modify: `web/classic/src/components/invitations/InvitationQRModal.jsx`
- Modify: `web/classic/src/components/invitations/UserInvitationCard.jsx`
- Modify: `web/classic/src/components/invitations/invitation-ui.test.js`
- Modify: `web/classic/src/components/settings/RegistrationSetting.jsx`
- Modify: `web/classic/src/components/settings/registration-settings.js`
- Modify: `web/classic/src/components/settings/registration-settings.test.js`

- [ ] **Step 1: Add failing Classic payload and UI assertions**

```js
test('round-trips poster copy in registration settings', () => {
  const form = fromRegistrationSettings({ invitation_poster_title: 'Join {站点名称}', invitation_poster_subtitle: 'Scan now' });
  expect(toRegistrationPayload(form)).toMatchObject({
    invitation_poster_title: 'Join {站点名称}',
    invitation_poster_subtitle: 'Scan now',
  });
});
```

- [ ] **Step 2: Verify Classic tests fail**

Run: `cd web/classic && bun test src/components/invitations src/components/settings/registration-settings.test.js`

Expected: FAIL because Classic still downloads only the QR canvas.

- [ ] **Step 3: Implement Semi UI controls using the shared renderer**

Use Semi UI `RadioGroup` or button group for size, `Switch` for nickname, and the same shared renderer/data inputs. Keep the modal within the viewport with a scrollable body and stable preview aspect ratio. Add title/subtitle admin fields to Classic registration settings.

- [ ] **Step 4: Run both frontend builds**

Run: `cd web/default && bun run build:check`

Run: `cd web/classic && bun test src/components/invitations src/components/settings/registration-settings.test.js && bun run build`

Expected: PASS.

- [ ] **Step 5: Commit Classic parity**

```bash
git add web/classic/src/components/invitations web/classic/src/components/settings
git commit -m "feat(classic): download invitation posters"
```

### Task 5: Decode and Visually Verify Exported Posters

**Files:**
- Create or modify a focused Playwright acceptance script under `web/default/e2e/` only if the repository's existing browser harness needs one.
- Modify production files only for defects found during acceptance.

- [ ] **Step 1: Start the local app**

Use the existing local Docker/dev workflow with the attached local database and confirm `http://127.0.0.1:18104` is healthy. Do not deploy production.

- [ ] **Step 2: Export every size**

Download `4:5`, `3:4`, and `9:16` posters with nickname on/off. Assert PNG dimensions are exactly `1080x1350`, `1080x1440`, and `1080x1920`.

- [ ] **Step 3: Decode the QR from exported PNGs**

Use an existing QR decoding dependency or a temporary test-only decoder to assert each exported image resolves to the exact registration URL containing the invitation code.

- [ ] **Step 4: Inspect desktop and mobile dialogs**

Capture Playwright screenshots at `1440x900`, `834x912`, and `390x844`. Verify the dialog stays usable, preview is not clipped, controls do not overlap, and long custom copy wraps inside the frame.

- [ ] **Step 5: Close acceptance findings through their owning task**

If acceptance finds a renderer defect, return to Task 2; use Task 3 for default UI defects and Task 4 for Classic UI defects. Add a failing regression test, rerun that task's verification command, and use its exact staging paths. If no defect is found, do not create an empty acceptance commit.

Do not push, deploy, publish a Release, or change the production version.
