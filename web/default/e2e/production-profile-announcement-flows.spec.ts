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
import { spawnSync } from 'node:child_process'

import { expect, test, type BrowserContext, type Page } from 'playwright/test'

const baseURL = process.env.E2E_BASE_URL || 'http://127.0.0.1:3026'
const rootStorageState = requireStorageState('E2E_ROOT_STORAGE_STATE')
const userStorageState = requireStorageState('E2E_USER_STORAGE_STATE')
const rootUserId = Number(process.env.E2E_ROOT_USER_ID || 1)
const userId = Number(process.env.E2E_USER_ID || 9400040)
const databaseFixtureAllowed = process.env.E2E_DB_FIXTURE_ALLOWED === '1'
const mysqlContainer = process.env.E2E_MYSQL_CONTAINER || ''
const mysqlDatabase = process.env.E2E_MYSQL_DATABASE || ''
const mysqlRootPassword = process.env.E2E_MYSQL_ROOT_PASSWORD || ''
const redisContainer = process.env.E2E_REDIS_CONTAINER || ''
const redisPassword = process.env.E2E_REDIS_PASSWORD || ''
const redisDatabase = process.env.E2E_REDIS_DATABASE || '0'
const announcementImageFixture =
  process.env.E2E_ANNOUNCEMENT_IMAGE_FIXTURE || ''

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data: T
}

type MutableFixtureBaseline = {
  acknowledgementId: number
  logId: number
  outboxId: number
}

type ProfileFixture = MutableFixtureBaseline & {
  displayNameUpdatedAt: number
  githubId: string | null
  githubFixture: string
  securityVersion: number
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

function flushFixtureRedis() {
  if (!redisContainer || !redisPassword) {
    throw new Error('Redis fixture environment is incomplete')
  }
  dockerCommand([
    'exec',
    redisContainer,
    'redis-cli',
    '-a',
    redisPassword,
    '-n',
    redisDatabase,
    'FLUSHDB',
  ])
}

function mysqlString(value: string): string {
  return `'${value.replaceAll('\\', '\\\\').replaceAll("'", "\\'")}'`
}

function readMutableBaseline(): MutableFixtureBaseline {
  const values = mysqlQuery(
    'SELECT COALESCE(MAX(id), 0) FROM announcement_acknowledgements; ' +
      'SELECT COALESCE(MAX(id), 0) FROM logs; ' +
      'SELECT COALESCE(MAX(id), 0) FROM audit_outbox_events;'
  )
    .split(/\r?\n/)
    .filter(Boolean)
    .map(Number)
  return {
    acknowledgementId: values[0] || 0,
    logId: values[1] || 0,
    outboxId: values[2] || 0,
  }
}

function cleanMutableAuditRows(
  baseline: MutableFixtureBaseline,
  actorIds: number[]
) {
  const actorList = actorIds.join(',')
  mysqlQuery(`
    DELETE FROM announcement_acknowledgements
      WHERE id > ${baseline.acknowledgementId} AND user_id IN (${actorList});
    DELETE FROM logs
      WHERE id > ${baseline.logId} AND user_id IN (${actorList});
    DELETE FROM audit_outbox_events
      WHERE id > ${baseline.outboxId}
        AND (actor_id IN (${actorList}) OR log_user_id IN (${actorList}));
  `)
}

function seedProfileFixture(): ProfileFixture {
  const mutableBaseline = readMutableBaseline()
  const userRow = mysqlQuery(
    `SELECT IF(github_id IS NULL, '__NULL__', CONCAT('__VALUE__', github_id)), ` +
      `COALESCE(display_name_updated_at, 0), ` +
      `COALESCE(security_version, 1) FROM users WHERE id = ${userId};`
  ).split('\t')
  const githubFixture = `qa-browser-${Date.now().toString(36)}`
  mysqlQuery(`
    UPDATE users
      SET github_id = ${mysqlString(githubFixture)},
          display_name_updated_at = UNIX_TIMESTAMP()
      WHERE id = ${userId};
  `)
  flushFixtureRedis()
  return {
    ...mutableBaseline,
    githubId:
      userRow[0] === '__NULL__' ? null : userRow[0].replace('__VALUE__', ''),
    displayNameUpdatedAt: Number(userRow[1]) || 0,
    securityVersion: Number(userRow[2]) || 1,
    githubFixture,
  }
}

function cleanProfileFixture(fixture: ProfileFixture) {
  const githubValue =
    fixture.githubId === null ? 'NULL' : mysqlString(fixture.githubId)
  mysqlQuery(`
    UPDATE users
      SET github_id = ${githubValue},
          display_name_updated_at = ${fixture.displayNameUpdatedAt},
          security_version = ${fixture.securityVersion}
      WHERE id = ${userId};
  `)
  cleanMutableAuditRows(fixture, [userId])
  flushFixtureRedis()
}

function readAnnouncements(): string {
  const encoded = mysqlQuery(
    "SELECT REPLACE(TO_BASE64(value), CHAR(10), '') FROM options " +
      "WHERE `key` = 'console_setting.announcements';"
  )
  return Buffer.from(encoded, 'base64').toString('utf8')
}

function writeAnnouncements(value: string) {
  const encoded = Buffer.from(value, 'utf8').toString('base64')
  mysqlQuery(`
    UPDATE options
      SET value = CONVERT(FROM_BASE64(${mysqlString(encoded)}) USING utf8mb4)
      WHERE \`key\` = 'console_setting.announcements';
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

async function dismissCurrentAnnouncement(page: Page) {
  const announcement = page
    .getByRole('dialog')
    .filter({ has: page.getByRole('button', { name: 'Got it' }) })
    .first()
  const visible = await announcement
    .waitFor({ state: 'visible', timeout: 5_000 })
    .then(() => true)
    .catch(() => false)
  if (!visible) return
  await announcement.getByRole('button', { name: 'Got it' }).click()
  await expect(announcement).toBeHidden()
}

test.describe('mutable profile and announcement acceptance', () => {
  test.skip(
    process.env.E2E_MUTATION_ALLOWED !== '1' || !databaseFixtureAllowed,
    'Enable mutation and database fixtures only for disposable local QA data.'
  )
  test.use({ storageState: userStorageState })

  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem('i18nextLng', 'en')
    })
  })

  test('nickname cooldown stays explicit and OAuth unbind preserves the session', async ({
    page,
  }) => {
    const fixture = seedProfileFixture()
    try {
      await page.goto('/profile', { waitUntil: 'domcontentloaded' })
      await dismissCurrentAnnouncement(page)

      await page.getByRole('button', { name: 'Edit profile' }).click()
      const editor = page.getByRole('dialog', { name: 'Edit profile' })
      await expect(editor).toBeVisible()
      await expect(editor.getByLabel('Display name')).toBeDisabled()
      await expect(
        editor.getByText(/^Display name can be changed /)
      ).toBeVisible()
      await expect(
        editor.getByRole('button', { name: 'Choose image' })
      ).toBeEnabled()
      await editor.getByRole('button', { name: 'Cancel' }).click()

      const githubValue = page.getByText(fixture.githubFixture, { exact: true })
      await expect(githubValue).toBeVisible()
      const githubRow = githubValue.locator(
        'xpath=ancestor::div[.//button[normalize-space()="Unbind"]][1]'
      )
      await githubRow.getByRole('button', { name: 'Unbind' }).click()
      const confirm = page.getByRole('alertdialog', { name: 'Confirm Unbind' })
      await expect(confirm).toBeVisible()
      const responsePromise = page.waitForResponse(
        (response) =>
          response.request().method() === 'DELETE' &&
          new URL(response.url()).pathname === '/api/user/self/bindings/github'
      )
      await confirm.getByRole('button', { name: 'Confirm Unbind' }).click()
      const response = await responsePromise
      expect(response.status()).toBe(200)
      await expect(page.getByText('Unbound GitHub')).toBeVisible()

      const self = await apiRequest<{ id: number }>(
        page.context(),
        userId,
        'GET',
        '/api/user/self'
      )
      expect(self.success, self.message).toBe(true)
      expect(self.data.id).toBe(userId)
      await page.reload({ waitUntil: 'domcontentloaded' })
      await expect(page).toHaveURL(/\/profile(?:\?|$)/)
      await expect(
        page.getByRole('button', { name: 'Edit profile' })
      ).toBeVisible()
    } finally {
      cleanProfileFixture(fixture)
    }
  })

  test('per-item popup supports HTML, managed images, preview, and acknowledgement', async ({
    browser,
  }) => {
    test.setTimeout(120_000)
    test.skip(
      !announcementImageFixture,
      'E2E_ANNOUNCEMENT_IMAGE_FIXTURE must point to an existing managed image.'
    )
    const baseline = readMutableBaseline()
    const originalAnnouncements = readAnnouncements()
    const title = `QA announcement ${Date.now().toString(36)}`
    const htmlText = 'QA HTML announcement'
    const rootContext = await browser.newContext({
      baseURL,
      storageState: rootStorageState,
    })
    const userContext = await browser.newContext({
      baseURL,
      storageState: userStorageState,
    })
    const anonymousContext = await browser.newContext({
      baseURL,
      storageState: { cookies: [], origins: [] },
    })
    for (const context of [rootContext, userContext, anonymousContext]) {
      await context.addInitScript(() => {
        window.localStorage.setItem('i18nextLng', 'en')
      })
    }
    const rootPage = await rootContext.newPage()
    const userPage = await userContext.newPage()
    const anonymousPage = await anonymousContext.newPage()

    try {
      await rootPage.goto('/system-settings/content/announcements', {
        waitUntil: 'domcontentloaded',
      })
      await dismissCurrentAnnouncement(rootPage)
      await rootPage.getByRole('button', { name: 'Add Announcement' }).click()
      const editor = rootPage.getByRole('dialog', { name: 'Add Announcement' })
      await editor.getByLabel('Announcement title').fill(title)
      await editor
        .getByLabel('Content')
        .fill(`<strong>${htmlText}</strong><p>Rendered safely.</p>`)
      await editor.getByText('Show this announcement as a popup').click()
      await editor
        .locator('input[type="file"]')
        .setInputFiles(announcementImageFixture)
      await expect(
        editor.getByRole('button', { name: 'Replace announcement image' })
      ).toBeVisible()
      await editor.getByRole('button', { name: 'Add', exact: true }).click()
      await expect(editor).toBeHidden()

      const savePromise = rootPage.waitForResponse(
        (response) =>
          response.request().method() === 'PUT' &&
          new URL(response.url()).pathname === '/api/option/'
      )
      await rootPage.getByRole('button', { name: 'Save Settings' }).click()
      const saveResponse = await savePromise
      const savePayload = (await saveResponse.json()) as ApiEnvelope<unknown>
      expect(savePayload.success, savePayload.message).toBe(true)

      await userPage.goto('/profile', { waitUntil: 'domcontentloaded' })
      const announcement = userPage.getByRole('dialog', { name: title })
      await expect(announcement).toBeVisible()
      const htmlElement = announcement.getByText(htmlText, { exact: true })
      await expect(htmlElement).toBeVisible()
      expect(await htmlElement.evaluate((element) => element.tagName)).toBe(
        'STRONG'
      )
      await announcement
        .getByRole('button', { name: 'View announcement image' })
        .click()
      const preview = userPage.getByRole('dialog', {
        name: 'Announcement image',
      })
      await expect(preview).toBeVisible()
      await expect
        .poll(() =>
          preview
            .getByRole('img', { name: 'Announcement image' })
            .evaluate((image) => (image as HTMLImageElement).naturalWidth)
        )
        .toBeGreaterThan(0)
      await userPage.keyboard.press('Escape')
      await expect(preview).toBeHidden()

      const acknowledgePromise = userPage.waitForResponse(
        (response) =>
          response.request().method() === 'POST' &&
          new URL(response.url()).pathname === '/api/announcement/acknowledge'
      )
      await announcement.getByRole('button', { name: 'Got it' }).click()
      const acknowledgeResponse = await acknowledgePromise
      const acknowledgePayload =
        (await acknowledgeResponse.json()) as ApiEnvelope<unknown>
      expect(acknowledgePayload.success, acknowledgePayload.message).toBe(true)
      await expect(announcement).toBeHidden()
      await userPage.reload({ waitUntil: 'domcontentloaded' })
      await expect(userPage.getByRole('dialog', { name: title })).toHaveCount(0)

      await anonymousPage.goto('/', { waitUntil: 'domcontentloaded' })
      await expect(
        anonymousPage.getByRole('dialog', { name: title })
      ).toHaveCount(0)
    } finally {
      writeAnnouncements(originalAnnouncements)
      cleanMutableAuditRows(baseline, [rootUserId, userId])
      flushFixtureRedis()
      await Promise.all([
        rootContext.close(),
        userContext.close(),
        anonymousContext.close(),
      ])
    }
  })
})
