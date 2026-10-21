# Console Return and Admin UI Design

## Goal

Deliver three focused frontend corrections without changing API contracts or
business behavior:

1. Returning from the public home page to the authenticated dashboard must not
   remain on the full-screen loading indicator.
2. The user redemption audit page must use the standard New API page layout,
   select controls, and round selection controls.
3. Wallet preset amount buttons must remain comfortably readable when the app
   sidebar leaves a narrow content area.

## Design

### Authenticated dashboard gate lifecycle

`AuthenticatedLayout` will retain its existing compliance and source
attribution gate orchestration. Its local gate resolutions will only be reset
when the authenticated user id actually changes after mount. The first layout
mount will not overwrite resolution callbacks from already-cached gate queries.

This preserves current loading, required, satisfied, error, and retry behavior.
No endpoint, query key, or gate order changes are included.

### User redemption audit page

The page will use `SectionPageLayout` with the title in the standard title slot
and total/batch settlement controls in the actions slot. The existing filters,
pagination, governance dialogs, responsive table/card behavior, and API calls
remain unchanged.

The three native `select` elements will be replaced with the shared `Select`
family. Native checkbox inputs in the mobile cards, table header, and table rows
will be replaced with the shared `Checkbox` and given a page-local
`rounded-full` style. Disabled and checked semantics remain unchanged.

### Wallet preset layout

Preset columns will respond to the authenticated content container rather than
the full browser viewport. A narrow sidebar-constrained content area will use
three columns; four columns will only appear when the content container is wide
enough. Buttons will use automatic height with a stable minimum, and payment
and savings metadata will use deliberate line structure instead of allowing the
bullet separator to wrap awkwardly.

## Error Handling

Existing API errors, gate error screens, retries, and redemption loading errors
remain authoritative. These changes add no new network operations and no new
error states.

## Verification

- Add a regression assertion that initial cached gate resolutions are not reset
  on layout mount, while a real user-id change still resets them.
- Assert the redemption page uses `SectionPageLayout` and shared `Select`, and
  contains no native `select` or checkbox inputs.
- Assert every redemption selection control uses the round page-local style.
- Assert wallet presets use content-container breakpoints and stable auto-height
  metadata layout.
- Run the focused Bun tests, the default frontend test suite, typecheck, lint,
  format check, and production build.
- Build a new production image, replace the local app container while retaining
  the existing MySQL, Redis, network, and data volume, then verify the home to
  dashboard return flow plus the 795x912, 834x912, mobile, and desktop layouts.

## Out of Scope

- Migrating the redemption page to the generic DataTable implementation.
- Changing redemption, settlement, wallet pricing, compliance, or source
  attribution business rules.
- Changing backend APIs or database data.
