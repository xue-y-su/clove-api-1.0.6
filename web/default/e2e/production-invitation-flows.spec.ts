import { spawnSync } from 'node:child_process'

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
import {
  expect,
  test,
  type BrowserContext,
  type Download,
  type Locator,
  type Page,
  type Route,
} from 'playwright/test'

const baseURL = process.env.E2E_BASE_URL || 'http://127.0.0.1:3026'
const backendBaseURL = process.env.E2E_BACKEND_URL || 'http://127.0.0.1:3000'
const rootStorageState = requireStorageState('E2E_ROOT_STORAGE_STATE')
const adminStorageState = requireStorageState('E2E_ADMIN_STORAGE_STATE')
const userStorageState = requireStorageState('E2E_USER_STORAGE_STATE')
const rootUserId = Number(process.env.E2E_ROOT_USER_ID || 1)
const adminUserId = Number(process.env.E2E_ADMIN_USER_ID || 9400039)
const userId = Number(process.env.E2E_USER_ID || 9400040)
const userName = process.env.E2E_USER_NAME || 'qauser26'
const databaseFixtureAllowed = process.env.E2E_DB_FIXTURE_ALLOWED === '1'
const mysqlContainer = process.env.E2E_MYSQL_CONTAINER || ''
const mysqlDatabase = process.env.E2E_MYSQL_DATABASE || ''
const mysqlRootPassword = process.env.E2E_MYSQL_ROOT_PASSWORD || ''
const redisContainer = process.env.E2E_REDIS_CONTAINER || ''
const redisPassword = process.env.E2E_REDIS_PASSWORD || ''
const redisDatabase = process.env.E2E_REDIS_DATABASE || '0'
const invitationRewardQuota = 500_000

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data: T
}

type RegistrationSettings = {
  aff_required: boolean
  invite_limit_unlimited: boolean
  invite_limit_count: number
  invitation_applications_enabled: boolean
  invitation_reapply_cooldown_seconds: number
}

type InvitationState = {
  application?: {
    id: number
    status: string
  }
  code?: {
    id: number
    code: string
    status: string
  }
  can_apply: boolean
  reapply_at?: number
}

type InvitationRewardSummary = {
  invited_count: number
  total_earned: number
  available: number
  frozen: number
  pending_payout: number
  paid: number
  cash_commission_visible: boolean
  cash_commission_rate: string
}

type SelfUser = {
  id: number
  quota: number
}

type RewardFixture = {
  codeId: number
  useId: number
  rewardId: number
  baselineQuota: number
  baselineAffCount: number
  baselineAffHistory: number
}

type InvitationWorkflowBaseline = {
  totalCodes: number
  codeId: number
  totalApplications: number
  applicationId: number
  outboxId: number
  logId: number
}

type PosterTheme = 'default' | 'classic'

type PosterDatabaseState = {
  totalCodes: number
  activeUserCodes: number
  theme: string
}

type PosterThemeCase = {
  theme: PosterTheme
  walletPath: string
  qrButtonName: string
  downloadButtonName: string
}

const posterDimensions = {
  '4:5': { width: 1080, height: 1350 },
  '3:4': { width: 1080, height: 1440 },
  '9:16': { width: 1080, height: 1920 },
} as const

const posterThemeCases: PosterThemeCase[] = [
  {
    theme: 'default',
    walletPath: '/wallet',
    qrButtonName: 'Show invitation QR code',
    downloadButtonName: 'Download PNG',
  },
  {
    theme: 'classic',
    walletPath: '/console/topup',
    qrButtonName: '生成二维码',
    downloadButtonName: '下载 PNG',
  },
]

function requireStorageState(name: string): string {
  const value = process.env[name]
  if (!value) {
    throw new Error(`${name} must point to a local Playwright storage state`)
  }
  return value
}

async function apiRequest<T>(
  context: BrowserContext,
  actorId: number,
  method: string,
  path: string,
  data?: unknown
): Promise<ApiEnvelope<T>> {
  const response = await context.request.fetch(new URL(path, baseURL).href, {
    method,
    data,
    headers: { 'New-Api-User': String(actorId) },
  })
  expect(response.status(), `${method} ${path}`).toBeLessThan(500)
  return (await response.json()) as ApiEnvelope<T>
}

async function getInvitationState(context: BrowserContext) {
  const response = await apiRequest<InvitationState>(
    context,
    userId,
    'GET',
    '/api/invitation/self'
  )
  expect(response.success, response.message).toBe(true)
  return response.data
}

const announcementProbeCompleted = new WeakSet<Page>()

async function dismissAnnouncement(page: Page) {
  const authenticated = await page
    .evaluate(() => Boolean(window.localStorage.getItem('uid')))
    .catch(() => false)
  if (!authenticated) return

  const announcement = page.getByRole('dialog').filter({
    has: page.getByRole('button', { name: 'Got it' }),
  })
  const visible = announcementProbeCompleted.has(page)
    ? await announcement.isVisible().catch(() => false)
    : await announcement
        .waitFor({ state: 'visible', timeout: 5_000 })
        .then(() => true)
        .catch(() => false)
  announcementProbeCompleted.add(page)
  if (!visible) return
  await announcement.getByRole('button', { name: 'Got it' }).click()
  await expect(announcement).toBeHidden()
}

function dockerCommand(args: string[]): string {
  const result = spawnSync('docker', args, {
    encoding: 'utf8',
    windowsHide: true,
  })
  if (result.status !== 0) {
    throw new Error(result.stderr.trim() || result.stdout.trim())
  }
  return result.stdout.trim()
}

function mysqlQuery(sql: string): string {
  if (!mysqlContainer || !mysqlDatabase || !mysqlRootPassword) {
    throw new Error('MySQL fixture environment is incomplete')
  }
  return dockerCommand([
    'exec',
    mysqlContainer,
    'mysql',
    '-uroot',
    `-p${mysqlRootPassword}`,
    '--batch',
    '--skip-column-names',
    '-D',
    mysqlDatabase,
    '-e',
    sql,
  ])
}

function mysqlString(value: string): string {
  return `'${value.replaceAll('\\', '\\\\').replaceAll("'", "''")}'`
}

function readInvitationWorkflowBaseline(): InvitationWorkflowBaseline {
  const values = mysqlQuery(`
    SELECT COUNT(*), COALESCE(MAX(id), 0) FROM invite_codes;
    SELECT COUNT(*), COALESCE(MAX(id), 0) FROM invite_code_applications;
    SELECT COALESCE(MAX(id), 0) FROM audit_outbox_events;
    SELECT COALESCE(MAX(id), 0) FROM logs;
  `)
    .split(/\s+/)
    .filter(Boolean)
    .map(Number)
  if (
    values.length !== 6 ||
    values.some((value) => !Number.isSafeInteger(value))
  ) {
    throw new Error(
      `Unexpected invitation workflow baseline: ${values.join(',')}`
    )
  }
  return {
    totalCodes: values[0],
    codeId: values[1],
    totalApplications: values[2],
    applicationId: values[3],
    outboxId: values[4],
    logId: values[5],
  }
}

function readFixtureIds(sql: string): number[] {
  const ids = mysqlQuery(sql).split(/\r?\n/).filter(Boolean).map(Number)
  expect(ids.every((id) => Number.isSafeInteger(id) && id > 0)).toBe(true)
  return ids
}

function invitationAuditWhere(
  baseline: InvitationWorkflowBaseline,
  codeIds: number[],
  applicationIds: number[]
): string {
  const actorIds = [rootUserId, adminUserId, userId].join(',')
  const targets = [
    `user:${userId}`,
    ...codeIds.map((id) => `invite-code:${id}`),
    ...applicationIds.map((id) => `invite-application:${id}`),
  ]
  return (
    `id > ${baseline.outboxId} AND (` +
    `actor_id IN (${actorIds}) OR ` +
    `target IN (${targets.map(mysqlString).join(',')}))`
  )
}

async function cleanInvitationWorkflowFixtures(
  baseline: InvitationWorkflowBaseline
) {
  const codeIds = readFixtureIds(`
    SELECT id FROM invite_codes
    WHERE id > ${baseline.codeId} AND owner_user_id = ${userId}
    ORDER BY id;
  `)
  const applicationIds = readFixtureIds(`
    SELECT id FROM invite_code_applications
    WHERE id > ${baseline.applicationId} AND user_id = ${userId}
    ORDER BY id;
  `)
  const auditWhere = invitationAuditWhere(baseline, codeIds, applicationIds)

  await expect
    .poll(
      () =>
        Number(
          mysqlQuery(`
            SELECT COUNT(*) FROM audit_outbox_events
            WHERE ${auditWhere} AND status <> 'delivered';
          `)
        ),
      { timeout: 20_000 }
    )
    .toBe(0)

  const eventIds = mysqlQuery(`
    SELECT event_id FROM audit_outbox_events
    WHERE ${auditWhere}
    ORDER BY id;
  `)
    .split(/\r?\n/)
    .filter(Boolean)
  const eventIdList = eventIds.map(mysqlString).join(',')

  if (eventIdList) {
    await expect
      .poll(
        () =>
          Number(
            mysqlQuery(`
              SELECT COUNT(*) FROM ip_security_events
              WHERE event_id IN (${eventIdList})
                AND projection_status <> 'projected';
            `)
          ),
        { timeout: 20_000 }
      )
      .toBe(0)
  }

  const actorIds = [rootUserId, adminUserId, userId].join(',')
  const codeIdList = codeIds.join(',')
  const applicationIdList = applicationIds.join(',')
  mysqlQuery(`
    START TRANSACTION;
    DELETE FROM logs
    WHERE ${eventIdList ? `event_id IN (${eventIdList}) OR` : ''}
      (id > ${baseline.logId} AND user_id IN (${actorIds}));
    ${eventIdList ? `DELETE FROM ip_security_events WHERE event_id IN (${eventIdList});` : ''}
    DELETE FROM audit_outbox_events WHERE ${auditWhere};
    ${codeIdList ? `DELETE FROM invite_codes WHERE id IN (${codeIdList}) AND owner_user_id = ${userId};` : ''}
    ${applicationIdList ? `DELETE FROM invite_code_applications WHERE id IN (${applicationIdList}) AND user_id = ${userId};` : ''}
    COMMIT;
  `)

  const restored = mysqlQuery(`
    SELECT COUNT(*), COALESCE(MAX(id), 0) FROM invite_codes;
    SELECT COUNT(*), COALESCE(MAX(id), 0) FROM invite_code_applications;
    SELECT COUNT(*) FROM audit_outbox_events WHERE ${auditWhere};
    SELECT COUNT(*) FROM logs
    WHERE id > ${baseline.logId} AND user_id IN (${actorIds});
    ${eventIdList ? `SELECT COUNT(*) FROM ip_security_events WHERE event_id IN (${eventIdList});` : 'SELECT 0;'}
  `)
    .split(/\s+/)
    .filter(Boolean)
    .map(Number)
  expect(restored).toEqual([
    baseline.totalCodes,
    baseline.codeId,
    baseline.totalApplications,
    baseline.applicationId,
    0,
    0,
    0,
  ])
}

function readPosterDatabaseState(): PosterDatabaseState {
  const rows = mysqlQuery(`
    SELECT COUNT(*) FROM invite_codes;
    SELECT COUNT(*) FROM invite_codes
    WHERE owner_user_id = ${userId} AND status = 'active';
    SELECT value FROM options
    WHERE options.key = 'theme.frontend'
    LIMIT 1;
  `)
    .split(/\r?\n/)
    .filter(Boolean)
  if (rows.length !== 3) {
    throw new Error(`Unexpected poster fixture state: ${rows.join(' | ')}`)
  }
  return {
    totalCodes: Number(rows[0]),
    activeUserCodes: Number(rows[1]),
    theme: rows[2],
  }
}

function seedPosterFixture(code: string): number {
  const now = Math.floor(Date.now() / 1000)
  const output = mysqlQuery(`
    INSERT INTO invite_codes
      (owner_user_id, code, active_owner_slot, kind, max_uses, used_count,
       status, expires_at, issued_by, created_at, updated_at)
    VALUES
      (${userId}, '${code}', ${userId}, 'single', 1, 0,
       'active', ${now + 86400}, ${adminUserId}, ${now}, ${now});
    SELECT LAST_INSERT_ID();
  `)
  const codeId = Number(output.trim().split(/\r?\n/).at(-1))
  if (!Number.isSafeInteger(codeId) || codeId <= 0) {
    throw new Error(`Invitation poster fixture was not created: ${output}`)
  }
  return codeId
}

function deletePosterFixture(code: string, codeId: number) {
  const idPredicate = codeId > 0 ? `id = ${codeId} AND` : ''
  mysqlQuery(`
    DELETE FROM invite_codes
    WHERE ${idPredicate} code = '${code}' AND owner_user_id = ${userId};
  `)
}

async function waitForFrontendTheme(
  context: BrowserContext,
  theme: PosterTheme
) {
  await expect
    .poll(
      async () => {
        const response = await context.request.get(
          new URL(`/api/status?qa=${Date.now()}`, backendBaseURL).href,
          { timeout: 10_000 }
        )
        if (!response.ok()) return ''
        const payload = (await response.json()) as ApiEnvelope<{
          theme?: string
        }>
        return payload.data?.theme || ''
      },
      {
        message: `frontend theme should become ${theme}`,
        timeout: 90_000,
        intervals: [1_000, 2_000, 5_000],
      }
    )
    .toBe(theme)
}

async function setFrontendTheme(context: BrowserContext, theme: PosterTheme) {
  const apiResponse = await context.request.put(
    new URL('/api/option/', backendBaseURL).href,
    {
      data: { key: 'theme.frontend', value: theme },
      headers: { 'New-Api-User': String(rootUserId) },
      timeout: 10_000,
    }
  )
  expect(apiResponse.status()).toBe(200)
  const response = (await apiResponse.json()) as ApiEnvelope<unknown>
  expect(response.success, response.message).toBe(true)
  await waitForFrontendTheme(context, theme)
}

async function openInvitationPoster(page: Page, themeCase: PosterThemeCase) {
  const walletURL = new URL(
    themeCase.walletPath,
    themeCase.theme === 'classic' ? backendBaseURL : baseURL
  ).href
  await page.goto(walletURL, { waitUntil: 'domcontentloaded' })
  await dismissAnnouncement(page)
  const qrButton = page.getByRole('button', {
    name: themeCase.qrButtonName,
  })
  await expect(qrButton).toBeVisible()

  await qrButton.click()

  const dialog = page.getByRole('dialog').filter({
    has: page.getByText('Invitation poster', { exact: true }),
  })
  await expect(dialog).toBeVisible()
  await expect(
    dialog.getByRole('img', { name: 'Invitation poster preview' })
  ).toBeVisible()
  return dialog
}

async function assertPosterCanvas(
  canvas: Locator,
  dimensions: { width: number; height: number }
) {
  await expect
    .poll(
      async () => {
        if ((await canvas.count()) !== 1) return ''
        return canvas.evaluate((element) => {
          const poster = element as HTMLCanvasElement
          return `${poster.width}x${poster.height}`
        })
      },
      { timeout: 15_000 }
    )
    .toBe(`${dimensions.width}x${dimensions.height}`)

  const blob = await canvas.evaluate(async (element) => {
    const poster = element as HTMLCanvasElement
    const output = await new Promise<Blob | null>((resolve) => {
      poster.toBlob(resolve, 'image/png')
    })
    return output ? { type: output.type, size: output.size } : null
  })
  expect(blob).not.toBeNull()
  expect(blob?.type).toBe('image/png')
  expect(blob?.size).toBeGreaterThan(1_000)
}

async function readDownload(download: Download): Promise<Buffer> {
  const stream = await download.createReadStream()
  const chunks: Buffer[] = []
  for await (const chunk of stream) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
  }
  return Buffer.concat(chunks)
}

async function assertPosterDownload(
  page: Page,
  button: Locator,
  dimensions: { width: number; height: number }
) {
  await expect(button).toBeEnabled()
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    button.click(),
  ])
  expect(download.suggestedFilename()).toMatch(/\.png$/i)
  const png = await readDownload(download)
  expect(png.length).toBeGreaterThan(1_000)
  expect(
    png.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))
  ).toBe(true)
  expect(png.toString('ascii', 12, 16)).toBe('IHDR')
  expect(png.readUInt32BE(16)).toBe(dimensions.width)
  expect(png.readUInt32BE(20)).toBe(dimensions.height)
}

async function readBrandPixels(canvas: Locator): Promise<number[]> {
  return canvas.evaluate((element) => {
    const poster = element as HTMLCanvasElement
    const context = poster.getContext('2d')
    if (!context) throw new Error('Poster canvas has no 2D context')
    return [...context.getImageData(78, 70, 52, 52).data]
  })
}

function pixelDifference(left: number[], right: number[]): number {
  expect(right).toHaveLength(left.length)
  return left.reduce(
    (difference, value, index) => difference + Math.abs(value - right[index]),
    0
  )
}

async function verifyPosterTheme(page: Page, themeCase: PosterThemeCase) {
  const dialog = await openInvitationPoster(page, themeCase)
  const preview = dialog.getByRole('img', {
    name: 'Invitation poster preview',
  })
  const canvas = preview.locator('canvas')
  const downloadButton = dialog.getByRole('button', {
    name: themeCase.downloadButtonName,
  })

  for (const [size, dimensions] of Object.entries(posterDimensions)) {
    await dialog.getByRole('button', { name: 'Edit poster' }).click()
    await dialog.getByText(size, { exact: true }).click()
    await expect
      .poll(async () => {
        const editorCanvas = dialog.locator('[data-poster-stage] canvas')
        if ((await editorCanvas.count()) !== 1) return ''
        return editorCanvas.evaluate(
          (element) =>
            `${(element as HTMLCanvasElement).width}x${(element as HTMLCanvasElement).height}`
        )
      })
      .toBe(`${dimensions.width}x${dimensions.height}`)
    await dialog.getByRole('button', { name: 'Save', exact: true }).click()
    await expect(preview).toBeVisible()
    await assertPosterCanvas(canvas, dimensions)
    await assertPosterDownload(page, downloadButton, dimensions)
  }
  const realLogoPixels = await readBrandPixels(canvas)

  let interceptedLogoRequests = 0
  const logo404Handler = async (route: Route) => {
    interceptedLogoRequests += 1
    await route.fulfill({
      status: 404,
      contentType: 'text/plain',
      body: 'not found',
    })
  }
  await page.route('**/logo.png', logo404Handler)
  await page.route('**/api/invitation/poster-logo', logo404Handler)
  try {
    await page.reload({ waitUntil: 'domcontentloaded' })
    await dismissAnnouncement(page)
    const fallbackDialog = await openInvitationPoster(page, themeCase)
    const fallbackCanvas = fallbackDialog
      .getByRole('img', { name: 'Invitation poster preview' })
      .locator('canvas')
    await assertPosterCanvas(fallbackCanvas, posterDimensions['9:16'])
    expect(interceptedLogoRequests).toBeGreaterThan(0)
    const fallbackPixels = await readBrandPixels(fallbackCanvas)
    expect(pixelDifference(realLogoPixels, fallbackPixels)).toBeGreaterThan(
      1_000
    )
  } finally {
    await page.unroute('**/logo.png', logo404Handler)
    await page.unroute('**/api/invitation/poster-logo', logo404Handler)
  }
}

function seedRewardFixture(): RewardFixture {
  const baseline = mysqlQuery(
    `SELECT quota, aff_count, aff_history FROM users WHERE id = ${userId}; ` +
      `SELECT COUNT(*) FROM invite_code_uses WHERE inviter_user_id = ${userId}; ` +
      `SELECT COUNT(*) FROM invite_reward_payout_requests WHERE user_id = ${userId} AND status = 'pending';`
  )
    .split(/\r?\n/)
    .filter(Boolean)
  const [baselineQuota, baselineAffCount, baselineAffHistory] = baseline[0]
    .split('\t')
    .map(Number)
  expect(Number(baseline[1])).toBe(0)
  expect(Number(baseline[2])).toBe(0)

  const now = Math.floor(Date.now() / 1000)
  const fixtureCode = `QA${Date.now().toString(36).toUpperCase()}`
  const fixtureEvent = `qa-invitation-reward-${Date.now()}`
  const seedOutput = mysqlQuery(`
    START TRANSACTION;
    INSERT INTO invite_codes
      (code, owner_user_id, kind, max_uses, used_count, status, expires_at, issued_by, created_at, updated_at)
    VALUES
      ('${fixtureCode}', ${userId}, 'single', 1, 1, 'exhausted', ${now + 86400}, ${adminUserId}, ${now}, ${now});
    SET @code_id = LAST_INSERT_ID();
    INSERT INTO invite_code_uses
      (invite_code_id, inviter_user_id, registered_user_id, registered_at, source, registration_ip,
       inviter_reward, invitee_reward, inviter_reward_status, invitee_reward_status,
       reward_attempt_count, reward_last_error, reward_next_attempt_at, rewards_completed_at,
       audit_event_id, created_at, updated_at)
    VALUES
      (@code_id, ${userId}, ${adminUserId}, ${now}, 'automated_local_qa', '127.0.0.1',
       ${invitationRewardQuota}, 0, 'available', 'skipped', 0, '', 0, ${now},
       '${fixtureEvent}', ${now}, ${now});
    SET @use_id = LAST_INSERT_ID();
    INSERT INTO invite_reward_payouts
      (invite_code_use_id, trigger_type, trigger_id, recipient_kind, recipient_user_id,
       eligible_quota, rate_basis_points, amount, reversed_amount, reversal_state, status,
       reviewer_id, review_reason, reviewed_at, withdrawn_at, created_at, updated_at)
    VALUES
      (@use_id, 'registration_inviter', CAST(@use_id AS CHAR), 'inviter', ${userId},
       ${invitationRewardQuota}, 0, ${invitationRewardQuota}, 0, 'none', 'available',
       0, '', 0, 0, ${now}, ${now});
    SET @reward_id = LAST_INSERT_ID();
    SELECT @code_id, @use_id, @reward_id;
    COMMIT;
  `)
  const ids = seedOutput.split(/\r?\n/).at(-1)
  if (!ids) throw new Error('Invitation reward fixture was not created')
  const [codeId, useId, rewardId] = ids.split('\t').map(Number)
  return {
    codeId,
    useId,
    rewardId,
    baselineQuota,
    baselineAffCount,
    baselineAffHistory,
  }
}

function cleanUpRewardFixture(fixture: RewardFixture) {
  const requestRow = mysqlQuery(`
    SELECT requests.id, requests.operation_id
    FROM invite_reward_payout_requests AS requests
    JOIN invite_reward_payout_request_items AS items ON items.request_id = requests.id
    WHERE items.reward_id = ${fixture.rewardId}
    ORDER BY requests.id DESC
    LIMIT 1;
  `)
  let requestId = 0
  let operationId = ''
  if (requestRow) {
    const columns = requestRow.split('\t')
    requestId = Number(columns[0])
    operationId = columns[1] || ''
  }
  const eventIds = requestId
    ? `'invitation:reward-payout-request:${requestId}:submitted','invitation:reward-payout-request:${requestId}:reviewed'`
    : "''"
  mysqlQuery(`
    START TRANSACTION;
    DELETE FROM logs WHERE event_id IN (${eventIds});
    DELETE FROM audit_outbox_events WHERE event_id IN (${eventIds});
    ${operationId ? `DELETE FROM wallet_ledger_entries WHERE operation_id = '${operationId}';` : ''}
    DELETE FROM invite_reward_payout_request_items WHERE reward_id IN (
      SELECT reward_id FROM (
        SELECT id AS reward_id FROM invite_reward_payouts WHERE invite_code_use_id = ${fixture.useId}
      ) AS fixture_rewards
    );
    ${requestId ? `DELETE FROM invite_reward_payout_requests WHERE id = ${requestId};` : ''}
    DELETE FROM invite_reward_payouts WHERE invite_code_use_id = ${fixture.useId};
    DELETE FROM invite_code_uses WHERE id = ${fixture.useId};
    DELETE FROM invite_codes WHERE id = ${fixture.codeId};
    UPDATE users
    SET quota = ${fixture.baselineQuota},
        aff_count = ${fixture.baselineAffCount},
        aff_history = ${fixture.baselineAffHistory}
    WHERE id = ${userId};
    COMMIT;
  `)
  if (redisContainer && redisPassword) {
    dockerCommand([
      'exec',
      redisContainer,
      'redis-cli',
      '-a',
      redisPassword,
      '-n',
      redisDatabase,
      'DEL',
      `user:${userId}`,
    ])
  }
}

async function applyForInvitation(page: Page, reason: string) {
  await page.getByRole('button', { name: 'Apply for invitation code' }).click()
  const dialog = page.getByRole('dialog', { name: 'Apply for invitation code' })
  await dialog.getByLabel('Application reason').fill(reason)
  await dialog.getByRole('button', { name: 'Submit application' }).click()
  await expect(dialog).toBeHidden()
  await expect(page.getByText('Application pending')).toBeVisible()
}

async function searchApplication(adminPage: Page, reason: string) {
  await adminPage.goto('/invitations', { waitUntil: 'domcontentloaded' })
  await dismissAnnouncement(adminPage)
  const search = adminPage.getByPlaceholder('Search users or codes')
  await search.fill(userName)
  await search.press('Enter')
  const row = adminPage
    .getByRole('row')
    .filter({ hasText: reason })
    .filter({ hasText: 'pending' })
  await expect(row).toBeVisible()
  return row
}

test.describe('mutable production invitation acceptance', () => {
  test.skip(
    process.env.E2E_MUTATION_ALLOWED !== '1',
    'Set E2E_MUTATION_ALLOWED=1 only for a disposable local QA database.'
  )
  test.use({ storageState: userStorageState })

  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem('i18nextLng', 'en')
    })
  })

  test('application cooldown, approval, short code, and owner revoke remain usable', async ({
    browser,
    page,
  }) => {
    test.skip(
      !databaseFixtureAllowed,
      'Set E2E_DB_FIXTURE_ALLOWED=1 with disposable local MySQL fixture settings.'
    )
    test.setTimeout(180_000)
    const baseline = readInvitationWorkflowBaseline()
    const rootContext = await browser.newContext({
      baseURL,
      storageState: rootStorageState,
    })
    const adminContext = await browser.newContext({
      baseURL,
      storageState: adminStorageState,
    })
    const adminPage = await adminContext.newPage()
    let originalSettings: RegistrationSettings | undefined

    try {
      const settingsResponse = await apiRequest<RegistrationSettings>(
        rootContext,
        rootUserId,
        'GET',
        '/api/registration/settings'
      )
      expect(settingsResponse.success, settingsResponse.message).toBe(true)
      originalSettings = settingsResponse.data
      const noCooldownSettings = {
        ...originalSettings,
        invitation_reapply_cooldown_seconds: 0,
      }
      const preflightSettingsUpdate = await apiRequest<RegistrationSettings>(
        rootContext,
        rootUserId,
        'PUT',
        '/api/registration/settings',
        noCooldownSettings
      )
      expect(
        preflightSettingsUpdate.success,
        preflightSettingsUpdate.message
      ).toBe(true)

      const initialState = await getInvitationState(page.context())
      if (initialState.application?.status === 'pending') {
        await apiRequest(
          page.context(),
          userId,
          'POST',
          `/api/invitation/applications/${initialState.application.id}/withdraw`
        )
      }
      if (initialState.code?.status === 'active') {
        await apiRequest(
          page.context(),
          userId,
          'POST',
          `/api/invitation/codes/${initialState.code.id}/revoke`,
          { reason: 'Automated local QA pre-cleanup' }
        )
      }

      await page.goto('/wallet', { waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(page)

      await applyForInvitation(page, 'Automated withdrawal application')
      await page.getByRole('button', { name: 'Withdraw application' }).click()
      await expect(
        page.getByRole('button', { name: 'Apply for invitation code' })
      ).toBeVisible()

      const cooldownSettingsUpdate = await apiRequest<RegistrationSettings>(
        rootContext,
        rootUserId,
        'PUT',
        '/api/registration/settings',
        originalSettings
      )
      expect(
        cooldownSettingsUpdate.success,
        cooldownSettingsUpdate.message
      ).toBe(true)

      const rejectedReason = 'Automated rejection application'
      await applyForInvitation(page, rejectedReason)
      let row = await searchApplication(adminPage, rejectedReason)
      await row.getByRole('button', { name: 'Reject' }).click()
      let dialog = adminPage.getByRole('dialog', { name: 'Reject application' })
      await dialog.getByLabel('Rejection reason').fill('Wait before reapplying')
      await dialog.getByRole('button', { name: 'Reject application' }).click()
      await expect(dialog).toBeHidden()

      await page.reload({ waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(page)
      await expect(page.getByText('Application rejected')).toBeVisible()
      await expect(page.getByText('Wait before reapplying')).toBeVisible()
      await expect(page.getByText(/You can apply again after/)).toBeVisible()
      await expect(
        page.getByRole('button', { name: 'Apply for invitation code' })
      ).toBeHidden()

      const settingsUpdate = await apiRequest<RegistrationSettings>(
        rootContext,
        rootUserId,
        'PUT',
        '/api/registration/settings',
        noCooldownSettings
      )
      expect(settingsUpdate.success, settingsUpdate.message).toBe(true)

      await page.reload({ waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(page)
      await expect(page.getByText('Application rejected')).toBeHidden()
      await expect(
        page.getByRole('button', { name: 'Apply for invitation code' })
      ).toBeVisible()

      const approvedReason = 'Automated approval application'
      await applyForInvitation(page, approvedReason)
      row = await searchApplication(adminPage, approvedReason)
      await row.getByRole('button', { name: 'Approve' }).click()
      dialog = adminPage.getByRole('dialog', { name: 'Approve application' })
      await dialog.getByLabel('Review note').fill('Automated approval check')
      await dialog.getByRole('button', { name: 'Approve and issue' }).click()
      await expect(dialog).toBeHidden()

      await page.reload({ waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(page)
      const codeInput = page.getByRole('textbox', { name: 'Invitation code' })
      await expect(codeInput).toHaveValue(/^[0-9A-HJKMNP-TV-Z]{10}$/)
      const code = await codeInput.inputValue()
      await expect(
        page.getByRole('textbox', { name: 'Invitation link' })
      ).toHaveValue(`${baseURL}/register?aff=${code}`)

      await page.getByRole('button', { name: 'Revoke invitation code' }).click()
      dialog = page.getByRole('dialog', { name: 'Revoke invitation code?' })
      await dialog.getByLabel('Reason').fill('Automated owner revoke check')
      await dialog
        .getByRole('button', { name: 'Revoke invitation code' })
        .click()
      await expect(dialog).toBeHidden()
      await expect(
        page.getByText('Invitation code status: revoked')
      ).toBeVisible()
      await expect(
        page.getByRole('button', { name: 'Apply for invitation code' })
      ).toBeVisible()
    } finally {
      try {
        try {
          const finalState = await getInvitationState(page.context()).catch(
            () => undefined
          )
          if (finalState?.application?.status === 'pending') {
            await apiRequest(
              page.context(),
              userId,
              'POST',
              `/api/invitation/applications/${finalState.application.id}/withdraw`
            )
          }
          if (finalState?.code?.status === 'active') {
            await apiRequest(
              page.context(),
              userId,
              'POST',
              `/api/invitation/codes/${finalState.code.id}/revoke`,
              { reason: 'Automated local QA cleanup' }
            )
          }
        } finally {
          if (originalSettings) {
            await apiRequest(
              rootContext,
              rootUserId,
              'PUT',
              '/api/registration/settings',
              originalSettings
            )
          }
        }
      } finally {
        try {
          await Promise.all([
            page.close(),
            adminContext.close(),
            rootContext.close(),
          ])
        } finally {
          await cleanInvitationWorkflowFixtures(baseline)
        }
      }
    }
  })

  test('inviter reward requires payout approval before wallet credit', async ({
    browser,
    page,
  }) => {
    test.skip(
      !databaseFixtureAllowed,
      'Set E2E_DB_FIXTURE_ALLOWED=1 with disposable local MySQL fixture settings.'
    )
    test.setTimeout(120_000)
    const adminContext = await browser.newContext({
      baseURL,
      storageState: adminStorageState,
    })
    const adminPage = await adminContext.newPage()
    let fixture: RewardFixture | undefined

    try {
      fixture = seedRewardFixture()
      const baselineUser = await apiRequest<SelfUser>(
        page.context(),
        userId,
        'GET',
        '/api/user/self'
      )
      expect(baselineUser.success, baselineUser.message).toBe(true)
      expect(baselineUser.data.quota).toBe(fixture.baselineQuota)

      await page.goto('/wallet', { waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(page)
      const initialSummary = await apiRequest<InvitationRewardSummary>(
        page.context(),
        userId,
        'GET',
        '/api/invitation/rewards'
      )
      expect(initialSummary.success, initialSummary.message).toBe(true)
      expect(initialSummary.data).toMatchObject({
        invited_count: 1,
        total_earned: invitationRewardQuota,
        available: invitationRewardQuota,
        frozen: 0,
        pending_payout: 0,
        paid: 0,
      })

      const payoutResponsePromise = page.waitForResponse(
        (response) =>
          response.request().method() === 'POST' &&
          new URL(response.url()).pathname ===
            '/api/invitation/rewards/payout-requests'
      )
      const submitButton = page.getByRole('button', {
        name: 'Submit payout review',
      })
      await expect(submitButton).toBeEnabled()
      await submitButton.click()
      const payoutResponse = await payoutResponsePromise
      const payoutPayload = (await payoutResponse.json()) as ApiEnvelope<{
        id: number
      }>
      expect(payoutPayload.success, payoutPayload.message).toBe(true)

      const pendingSummary = await apiRequest<InvitationRewardSummary>(
        page.context(),
        userId,
        'GET',
        '/api/invitation/rewards'
      )
      expect(pendingSummary.success, pendingSummary.message).toBe(true)
      expect(pendingSummary.data.available).toBe(0)
      expect(pendingSummary.data.frozen).toBe(0)
      expect(pendingSummary.data.pending_payout).toBe(invitationRewardQuota)
      const pendingUser = await apiRequest<SelfUser>(
        page.context(),
        userId,
        'GET',
        '/api/user/self'
      )
      expect(pendingUser.data.quota).toBe(fixture.baselineQuota)

      await adminPage.goto('/invitation-rewards', {
        waitUntil: 'domcontentloaded',
      })
      await dismissAnnouncement(adminPage)
      const search = adminPage.getByPlaceholder('Search users or codes')
      await search.fill(userName)
      await search.press('Enter')
      const row = adminPage
        .getByRole('row')
        .filter({ hasText: userName })
        .filter({ hasText: 'Pending review' })
      await expect(row).toBeVisible()
      await row.getByRole('button', { name: 'Approve payout request' }).click()
      const dialog = adminPage.getByRole('dialog', {
        name: 'Approve payout request',
      })
      await dialog
        .getByPlaceholder('Optional review note')
        .fill('Automated reward approval check')
      await dialog
        .getByRole('button', { name: 'Approve payout request' })
        .click()
      await expect(dialog).toBeHidden()

      await expect
        .poll(async () => {
          const user = await apiRequest<SelfUser>(
            page.context(),
            userId,
            'GET',
            '/api/user/self'
          )
          return user.data.quota
        })
        .toBe(fixture.baselineQuota + invitationRewardQuota)
      const paidSummary = await apiRequest<InvitationRewardSummary>(
        page.context(),
        userId,
        'GET',
        '/api/invitation/rewards'
      )
      expect(paidSummary.success, paidSummary.message).toBe(true)
      expect(paidSummary.data.available).toBe(0)
      expect(paidSummary.data.frozen).toBe(0)
      expect(paidSummary.data.pending_payout).toBe(0)
      expect(paidSummary.data.paid).toBe(invitationRewardQuota)

      const reviewedRequest = await apiRequest<{
        items: Array<{ id: number; status: string }>
      }>(
        adminContext,
        adminUserId,
        'GET',
        `/api/invitation/admin/reward-payout-requests?page=1&page_size=20&status=paid&keyword=${userName}`
      )
      expect(reviewedRequest.success, reviewedRequest.message).toBe(true)
      expect(
        reviewedRequest.data.items.some(
          (item) => item.id === payoutPayload.data.id && item.status === 'paid'
        )
      ).toBe(true)
    } finally {
      if (fixture) cleanUpRewardFixture(fixture)
      await adminContext.close()
    }
  })

  test('invitation posters export branded PNGs in default and classic themes', async ({
    browser,
    page,
  }) => {
    test.skip(
      !databaseFixtureAllowed,
      'Set E2E_DB_FIXTURE_ALLOWED=1 with disposable local MySQL fixture settings.'
    )
    test.setTimeout(360_000)
    const fixtureCode = `P${String(Date.now()).slice(-9)}`
    let fixtureCodeId = 0
    let baseline: PosterDatabaseState | undefined
    const rootContext = await browser.newContext({
      baseURL,
      storageState: rootStorageState,
    })
    const userStorage = await page.context().storageState()
    const sourceOrigin = userStorage.origins.find(
      (origin) => origin.origin === new URL(baseURL).origin
    )
    const authStorage = (sourceOrigin?.localStorage ?? []).filter((entry) =>
      ['uid', 'user'].includes(entry.name)
    )
    expect(authStorage.map((entry) => entry.name).sort()).toEqual([
      'uid',
      'user',
    ])
    await page.context().addInitScript((entries) => {
      for (const entry of entries) {
        window.localStorage.setItem(entry.name, entry.value)
      }
    }, authStorage)

    try {
      baseline = readPosterDatabaseState()
      expect(baseline.activeUserCodes).toBe(0)
      expect(baseline.theme).toBe('default')
      fixtureCodeId = seedPosterFixture(fixtureCode)

      for (const themeCase of posterThemeCases) {
        await setFrontendTheme(rootContext, themeCase.theme)
        const themePage = await page.context().newPage()
        await themePage.addInitScript(() => {
          window.localStorage.setItem('i18nextLng', 'en')
          window.localStorage.removeItem('status')
          window.localStorage.removeItem('app:rev')
        })
        try {
          await verifyPosterTheme(themePage, themeCase)
        } finally {
          await themePage.close()
        }
      }
    } finally {
      deletePosterFixture(fixtureCode, fixtureCodeId)
      await setFrontendTheme(rootContext, 'default')
      await rootContext.close()

      if (baseline) {
        const restored = readPosterDatabaseState()
        expect(restored).toEqual({
          totalCodes: baseline.totalCodes,
          activeUserCodes: baseline.activeUserCodes,
          theme: 'default',
        })
        const fixtureRows = Number(
          mysqlQuery(`
            SELECT COUNT(*) FROM invite_codes
            WHERE code = '${fixtureCode}' AND owner_user_id = ${userId};
          `)
        )
        expect(fixtureRows).toBe(0)
      }
    }
  })
})
