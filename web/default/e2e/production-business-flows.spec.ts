/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { spawnSync } from 'node:child_process'

import { expect, test, type BrowserContext, type Page } from 'playwright/test'

const baseURL = process.env.E2E_BASE_URL || 'http://127.0.0.1:3026'
const adminStorageState = requireStorageState('E2E_ADMIN_STORAGE_STATE')
const userStorageState = requireStorageState('E2E_USER_STORAGE_STATE')
const adminUserId = Number(process.env.E2E_ADMIN_USER_ID || 9400039)
const userId = Number(process.env.E2E_USER_ID || 9400040)
const databaseFixtureAllowed = process.env.E2E_DB_FIXTURE_ALLOWED === '1'
const mysqlContainer = process.env.E2E_MYSQL_CONTAINER || ''
const mysqlDatabase = process.env.E2E_MYSQL_DATABASE || ''
const mysqlRootPassword = process.env.E2E_MYSQL_ROOT_PASSWORD || ''

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data: T
}

type SelfUser = {
  id: number
  quota: number
}

type RuntimeStatus = {
  quota_per_unit: number
}

type UserRedemption = {
  id: number
  code: string
  status: string
  remaining_quota: number
}

function requireStorageState(name: string): string {
  const value = process.env[name]
  if (!value) {
    throw new Error(`${name} must point to a local Playwright storage state`)
  }
  return value
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

function sqlString(value: string): string {
  return `'${value.replaceAll('\\', '\\\\').replaceAll("'", "''")}'`
}

async function hardDeleteRedemptionFixtures(redemptionIds: number[]) {
  if (!databaseFixtureAllowed || redemptionIds.length === 0) return

  expect(redemptionIds.every((id) => Number.isSafeInteger(id) && id > 0)).toBe(
    true
  )
  const ids = [...new Set(redemptionIds)]
  const idList = ids.join(',')
  const referenceIdList = ids.map((id) => sqlString(String(id))).join(',')
  const targetList = ids.map((id) => `'user_redemption_code:${id}'`).join(',')

  await expect
    .poll(
      () =>
        Number(
          mysqlQuery(
            `SELECT COUNT(*) FROM audit_outbox_events ` +
              `WHERE target IN (${targetList}) AND status <> 'delivered';`
          )
        ),
      { timeout: 10_000 }
    )
    .toBe(0)
  const eventIds = mysqlQuery(
    `SELECT event_id FROM audit_outbox_events WHERE target IN (${targetList});`
  )
    .split(/\r?\n/)
    .filter(Boolean)
  const eventIdList = eventIds.map(sqlString).join(',')

  mysqlQuery(`
    START TRANSACTION;
    ${eventIdList ? `DELETE FROM logs WHERE event_id IN (${eventIdList});` : ''}
    ${
      eventIdList
        ? `DELETE FROM ip_security_events WHERE event_id IN (${eventIdList});`
        : ''
    }
    DELETE FROM audit_outbox_events WHERE target IN (${targetList});
    DELETE FROM user_redemption_subscription_entitlements WHERE code_id IN (${idList});
    DELETE FROM user_redemption_claims WHERE code_id IN (${idList});
    DELETE FROM wallet_ledger_entries
    WHERE reference_type = 'user_redemption_code' AND reference_id IN (${referenceIdList});
    DELETE FROM user_redemption_codes WHERE id IN (${idList});
    COMMIT;
  `)
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

async function getSelf(context: BrowserContext, actorId: number) {
  const response = await apiRequest<SelfUser>(
    context,
    actorId,
    'GET',
    '/api/user/self'
  )
  expect(response.success, response.message).toBe(true)
  expect(response.data.id).toBe(actorId)
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

async function createRedemptionThroughWallet(
  page: Page,
  onCreated: (redemptionId: number) => void
) {
  await page.getByRole('button', { name: 'Create Redemption Code' }).click()
  const drawer = page.getByRole('dialog', { name: 'Create Redemption Code' })
  await expect(drawer).toBeVisible()
  await drawer.getByLabel('Amount', { exact: true }).fill('1')

  const responsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' &&
      new URL(response.url()).pathname === '/api/user-redemption/'
  )
  await drawer.getByRole('button', { name: 'Create', exact: true }).click()
  const response = await responsePromise
  const payload = (await response.json()) as ApiEnvelope<UserRedemption>
  expect(payload.success, payload.message).toBe(true)
  onCreated(payload.data.id)
  await expect(
    drawer.getByRole('textbox', { name: 'Redemption code', exact: true })
  ).toHaveValue(payload.data.code)
  await drawer
    .getByRole('button', { name: 'Close', exact: true })
    .first()
    .click()
  await expect(drawer).toBeHidden()
  return payload.data
}

async function cleanUpRedemption(
  adminContext: BrowserContext,
  redemptionId: number
) {
  const detail = await apiRequest<{ code: UserRedemption }>(
    adminContext,
    adminUserId,
    'GET',
    `/api/user-redemption/admin/${redemptionId}`
  )
  if (!detail.success || detail.data.code.status === 'deleted') return

  const liveStatuses = new Set(['active', 'disabled', 'expired_pending'])
  const settlement = liveStatuses.has(detail.data.code.status) ? 'refund' : ''
  await apiRequest(
    adminContext,
    adminUserId,
    'POST',
    `/api/user-redemption/admin/${redemptionId}/delete`,
    {
      reason: 'Automated local QA cleanup',
      settlement,
    }
  )
}

async function getAdminRedemption(
  adminContext: BrowserContext,
  redemptionId: number
) {
  const detail = await apiRequest<{ code: UserRedemption }>(
    adminContext,
    adminUserId,
    'GET',
    `/api/user-redemption/admin/${redemptionId}`
  )
  expect(detail.success, detail.message).toBe(true)
  return detail.data.code
}

async function cleanUpCreatorRedemptions(adminContext: BrowserContext) {
  const list = await apiRequest<{ items: UserRedemption[] }>(
    adminContext,
    adminUserId,
    'GET',
    `/api/user-redemption/admin/?creator_user_id=${userId}&deleted=false&page_size=100`
  )
  expect(list.success, list.message).toBe(true)
  for (const item of list.data.items) {
    await cleanUpRedemption(adminContext, item.id)
  }
}

test.describe('mutable production business acceptance', () => {
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

  test('user self-claim and administrator governance preserve wallet balance', async ({
    browser,
    page,
  }) => {
    test.setTimeout(240_000)
    const adminContext = await browser.newContext({
      baseURL,
      storageState: adminStorageState,
    })
    const adminPage = await adminContext.newPage()
    const createdIds: number[] = []

    try {
      await cleanUpCreatorRedemptions(adminContext)
      const baseline = await getSelf(page.context(), userId)
      const runtimeStatus = await apiRequest<RuntimeStatus>(
        page.context(),
        userId,
        'GET',
        '/api/status'
      )
      expect(runtimeStatus.success, runtimeStatus.message).toBe(true)
      const standardQuotaUnit = runtimeStatus.data.quota_per_unit
      expect(standardQuotaUnit).toBeGreaterThan(0)
      await page.goto('/wallet', { waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(page)

      const selfClaimCode = await createRedemptionThroughWallet(page, (id) =>
        createdIds.push(id)
      )
      await expect
        .poll(async () => (await getSelf(page.context(), userId)).quota)
        .toBe(baseline.quota - standardQuotaUnit)

      await page
        .getByPlaceholder('Enter your redemption code')
        .fill(selfClaimCode.code)
      await page
        .getByRole('button', { name: 'Redeem Code', exact: true })
        .click()
      await expect(page.getByText(/Redemption successful!/)).toBeVisible()
      await expect
        .poll(async () => (await getSelf(page.context(), userId)).quota)
        .toBe(baseline.quota)

      const governedCode = await createRedemptionThroughWallet(page, (id) =>
        createdIds.push(id)
      )
      await expect
        .poll(async () => (await getSelf(page.context(), userId)).quota)
        .toBe(baseline.quota - standardQuotaUnit)

      await adminPage.goto('/user-redemptions', {
        waitUntil: 'domcontentloaded',
      })
      await dismissAnnouncement(adminPage)
      await adminPage.getByPlaceholder('Creator ID').fill(String(userId))
      await adminPage
        .getByRole('main')
        .getByRole('button', { name: 'Search' })
        .click()

      const row = adminPage.getByRole('row').filter({
        hasText: governedCode.code,
      })
      await expect(row).toBeVisible()

      await row.getByRole('button', { name: 'Disable' }).click()
      let dialog = adminPage.getByRole('dialog', { name: 'Disable' })
      await dialog.getByLabel('Reason').fill('Automated governance check')
      await dialog.getByRole('button', { name: 'Confirm' }).click()
      await expect(dialog).toBeHidden()
      await expect
        .poll(
          async () =>
            (await getAdminRedemption(adminContext, governedCode.id)).status
        )
        .toBe('disabled')

      await row.getByRole('button', { name: 'Enable' }).click()
      dialog = adminPage.getByRole('dialog', { name: 'Enable' })
      await dialog.getByLabel('Reason').fill('Automated governance check')
      await dialog.getByRole('button', { name: 'Confirm' }).click()
      await expect(dialog).toBeHidden()
      await expect
        .poll(
          async () =>
            (await getAdminRedemption(adminContext, governedCode.id)).status
        )
        .toBe('active')

      await row.getByRole('button', { name: 'Terminate' }).click()
      dialog = adminPage.getByRole('dialog', { name: 'Terminate' })
      await dialog
        .getByRole('button', { name: 'Refund remaining balance' })
        .click()
      await dialog.getByLabel('Reason').fill('Automated refund check')
      await dialog.getByRole('button', { name: 'Confirm' }).click()
      await expect(dialog).toBeHidden()
      await expect
        .poll(
          async () =>
            (await getAdminRedemption(adminContext, governedCode.id)).status
        )
        .toBe('refunded')
      await expect
        .poll(async () => (await getSelf(page.context(), userId)).quota)
        .toBe(baseline.quota)

      await row.getByRole('button', { name: 'Delete' }).click()
      dialog = adminPage.getByRole('dialog', { name: 'Delete' })
      await dialog.getByLabel('Reason').fill('Automated cleanup check')
      await dialog.getByRole('button', { name: 'Confirm' }).click()
      await expect(dialog).toBeHidden()
      await expect
        .poll(
          async () =>
            (await getAdminRedemption(adminContext, governedCode.id)).status
        )
        .toBe('deleted')
      await expect
        .poll(async () => (await getSelf(page.context(), userId)).quota)
        .toBe(baseline.quota)
    } finally {
      try {
        for (const redemptionId of createdIds) {
          await cleanUpRedemption(adminContext, redemptionId)
        }
      } finally {
        await hardDeleteRedemptionFixtures(createdIds)
        await adminContext.close()
      }
    }
  })
})
