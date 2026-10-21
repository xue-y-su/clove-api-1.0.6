# Source Attribution Confirmations Scroll Design

## Problem

The source attribution confirmations page is rendered inside `SectionPageLayout fixedContent`. That layout clips overflowing content, while `SourceAttributionConfirmationsTable` neither fills the available height nor provides a vertical scroll container. When more records are rendered than fit in the viewport, the lower rows and pagination controls are clipped and mouse-wheel scrolling has no effect.

## Design

Keep the filters and pagination controls visible while making only the records region vertically scrollable:

- Make the confirmations component fill the available content height with `h-full min-h-0`.
- Keep the filter row and pagination row as non-scrolling flex children.
- Make the bordered records wrapper the flexible `min-h-0 flex-1 overflow-y-auto` region.
- Preserve the shared table component's existing horizontal scrolling for narrow viewports.
- Do not change API requests, pagination size, filtering behavior, or source attribution data.

## Scope

Only the default-theme source attribution confirmations table is changed. The compliance confirmations table and shared layout primitives remain untouched to avoid widening the behavior change beyond the reported page.

## Verification

1. Add a focused structural regression test that requires a full-height component and a vertically scrollable flexible records region.
2. Confirm the regression test fails before the component change and passes afterward.
3. Run the relevant frontend test suite and production build.
4. Verify scrolling at desktop and mobile viewport sizes against the local production-data copy.
