# Source Attribution Confirmations Scroll Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore vertical scrolling for source attribution confirmation records without moving the filters or pagination controls out of view.

**Architecture:** Keep `SectionPageLayout fixedContent` unchanged and make `SourceAttributionConfirmationsTable` own its constrained-height scrolling. The component fills the available height, while only the bordered records region flexes and scrolls vertically; the shared table continues to own horizontal overflow.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, Bun test, Rsbuild, Docker, Docker Compose

---

### Task 1: Add the scrolling regression test

**Files:**
- Create: `web/default/src/features/source-attribution/source-attribution-confirmations-table.test.ts`
- Test: `web/default/src/features/source-attribution/source-attribution-confirmations-table.test.ts`

- [ ] **Step 1: Write the failing structural test**

```ts
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { expect, test } from 'bun:test'

test('keeps source attribution controls visible while records scroll vertically', async () => {
  const source = await Bun.file(
    new URL('./source-attribution-confirmations-table.tsx', import.meta.url)
  ).text()

  expect(source).toContain("className='flex h-full min-h-0 flex-col gap-4'")
  expect(source).toContain("className='grid shrink-0 gap-3 md:grid-cols-4'")
  expect(source).toContain(
    "className='min-h-0 flex-1 overflow-y-auto rounded-md border'"
  )
  expect(source).toContain(
    "className='flex shrink-0 items-center justify-between gap-3'"
  )
})
```

- [ ] **Step 2: Run the test and verify RED**

Run from `web/default`:

```powershell
bun test src/features/source-attribution/source-attribution-confirmations-table.test.ts
```

Expected: FAIL because the current component lacks `h-full`, `shrink-0`, and `overflow-y-auto` layout classes.

### Task 2: Implement the constrained records scroller

**Files:**
- Modify: `web/default/src/features/source-attribution/source-attribution-confirmations-table.tsx`
- Test: `web/default/src/features/source-attribution/source-attribution-confirmations-table.test.ts`

- [ ] **Step 1: Apply the minimal class changes**

Change the component root to:

```tsx
<div className='flex h-full min-h-0 flex-col gap-4'>
```

Change the filter row to:

```tsx
<div className='grid shrink-0 gap-3 md:grid-cols-4'>
```

Change the bordered table wrapper to:

```tsx
<div className='min-h-0 flex-1 overflow-y-auto rounded-md border'>
```

Change the pagination row to:

```tsx
<div className='flex shrink-0 items-center justify-between gap-3'>
```

- [ ] **Step 2: Run the focused test and verify GREEN**

Run from `web/default`:

```powershell
bun test src/features/source-attribution/source-attribution-confirmations-table.test.ts
```

Expected: PASS.

- [ ] **Step 3: Run formatting, frontend tests, and the production build**

Run from `web/default`:

```powershell
bun run format:check
bun test ./src
bun run build:check
```

Expected: all commands exit 0 with no TypeScript or build errors.

- [ ] **Step 4: Verify desktop and mobile scrolling against the local production-data copy**

Use a temporary local admin account against `http://127.0.0.1:18104`, then open `/usage-logs/source-attribution` at desktop and mobile viewport sizes. At mobile height, verify the records wrapper has `overflow-y: auto`, `scrollHeight > clientHeight`, and a wheel gesture increases `scrollTop`; verify the filter controls and pagination remain in place. Remove the temporary account, audit rows, session artifacts, and Redis rate-limit keys afterward.

- [ ] **Step 5: Commit the implementation**

```powershell
git add web/default/src/features/source-attribution/source-attribution-confirmations-table.tsx web/default/src/features/source-attribution/source-attribution-confirmations-table.test.ts
git commit -m "fix(web): restore source attribution record scrolling"
```

### Task 3: Build and deploy the verified image

**Files:**
- Update on server: `/opt/1panel/apps/new-api/new-api/docker-compose.yml`
- Create locally: `D:/cloveapi/.runtime-build/clove-api-production-20260729-source-scroll.tar`
- Create on server: `/opt/1panel/apps/new-api/new-api/backups/pre-source-scroll-20260729/`

- [ ] **Step 1: Build and smoke-test the production image locally**

Run from the repository root:

```powershell
docker build -t clove-api:production-20260729-source-scroll .
docker image inspect clove-api:production-20260729-source-scroll
```

Expected: the image builds successfully and reports a valid image ID.

- [ ] **Step 2: Export and upload the image**

```powershell
docker save -o D:\cloveapi\.runtime-build\clove-api-production-20260729-source-scroll.tar clove-api:production-20260729-source-scroll
pscp.exe -batch -pw $env:CLOVE_PROD_PASSWORD D:\cloveapi\.runtime-build\clove-api-production-20260729-source-scroll.tar root@103.205.240.82:/opt/1panel/apps/new-api/new-api/
```

Expected: the archive exists on the server and its SHA-256 matches the local file.

- [ ] **Step 3: Stop only the application and back up production data**

Run through `plink.exe -batch -pw $env:CLOVE_PROD_PASSWORD root@103.205.240.82`:

```sh
cd /opt/1panel/apps/new-api/new-api
mkdir -p backups/pre-source-scroll-20260729
cp docker-compose.yml backups/pre-source-scroll-20260729/docker-compose.yml.pre
docker compose stop new-api
docker exec 1Panel-mysql-CQx4 sh -lc 'mysqldump --single-transaction --quick --routines --triggers -uroot -p"$MYSQL_ROOT_PASSWORD" new_api' | gzip -9 > backups/pre-source-scroll-20260729/new_api.sql.gz
docker exec 1Panel-mysql-CQx4 sh -lc 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" --batch --skip-column-names new_api -e "SELECT COUNT(*),COALESCE(SUM(quota),0),COALESCE(SUM(used_quota),0) FROM users; SELECT COUNT(*) FROM tokens; SELECT COUNT(*) FROM channels; SELECT COUNT(*) FROM source_attribution_confirmations;"' > backups/pre-source-scroll-20260729/baseline.tsv
```

Expected: the application is stopped, MySQL remains running, and both backup files are non-empty.

- [ ] **Step 4: Load the image and restart the application**

```sh
cd /opt/1panel/apps/new-api/new-api
docker load -i clove-api-production-20260729-source-scroll.tar
sed -i 's#image: clove-api:production-20260729-c059330d#image: clove-api:production-20260729-source-scroll#' docker-compose.yml
docker compose config --quiet
docker compose up -d new-api
```

Expected: Compose validates and the `new-api` container reaches `healthy`.

- [ ] **Step 5: Verify service and data preservation**

```sh
cd /opt/1panel/apps/new-api/new-api
docker compose ps
docker exec new-api /healthcheck
curl -fsS https://gaea.dpdns.org/api/status
docker exec 1Panel-mysql-CQx4 sh -lc 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" --batch --skip-column-names new_api -e "SELECT COUNT(*),COALESCE(SUM(quota),0),COALESCE(SUM(used_quota),0) FROM users; SELECT COUNT(*) FROM tokens; SELECT COUNT(*) FROM channels; SELECT COUNT(*) FROM source_attribution_confirmations;"'
```

Expected: both status requests return HTTP 200, the container is healthy, and all post-deploy counts and quota sums match `baseline.tsv`.

- [ ] **Step 6: Push Git commits and refresh the source archive**

```powershell
git push origin recovery/production-restore-20260719
git archive --format=zip --output=D:\cloveapi\clove-api-v1.0.0-beta-source.zip HEAD
```

Expected: the remote branch contains the design and implementation commits, and the source archive is regenerated from the deployed source.
