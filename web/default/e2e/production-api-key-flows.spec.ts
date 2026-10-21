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
import { expect, test, type BrowserContext, type Page } from 'playwright/test'

const baseURL = process.env.E2E_BASE_URL || 'http://127.0.0.1:3026'
const rootStorageState = requireStorageState('E2E_ROOT_STORAGE_STATE')
const adminStorageState = requireStorageState('E2E_ADMIN_STORAGE_STATE')
const userStorageState = requireStorageState('E2E_USER_STORAGE_STATE')
const rootUserId = Number(process.env.E2E_ROOT_USER_ID || 1)
const adminUserId = Number(process.env.E2E_ADMIN_USER_ID || 9400039)
const userId = Number(process.env.E2E_USER_ID || 9400040)
const databaseFixtureAllowed = process.env.E2E_DB_FIXTURE_ALLOWED === '1'
const mysqlContainer = process.env.E2E_MYSQL_CONTAINER || ''
const mysqlDatabase = process.env.E2E_MYSQL_DATABASE || ''
const mysqlRootPassword = process.env.E2E_MYSQL_ROOT_PASSWORD || ''
const redisContainer = process.env.E2E_REDIS_CONTAINER || ''
const redisPassword = process.env.E2E_REDIS_PASSWORD || ''
const redisDatabase = process.env.E2E_REDIS_DATABASE || '0'

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  code?: string
  data?: T
}

type ApiKey = {
  id: number
  name: string
  key: string
  status: number
  status_reason?: string
  group: string
}

type ApiKeyPage = {
  items: ApiKey[]
  total: number
}

type UserGroups = Record<string, { desc: string; ratio: number | string }>

type CreatedKeyRecord = {
  id: number
  key: string
  fingerprint: string
  group: string
  status: number
  statusReason: string
}

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
): Promise<{ status: number; payload: ApiEnvelope<T> }> {
  const response = await context.request.fetch(new URL(path, baseURL).href, {
    method,
    data,
    headers: { 'New-Api-User': String(actorId) },
  })
  expect(response.status(), `${method} ${path}`).toBeLessThan(500)
  return {
    status: response.status(),
    payload: (await response.json()) as ApiEnvelope<T>,
  }
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

function redisCommand(args: string[]): string {
  if (!redisContainer || !redisPassword) {
    throw new Error('Redis fixture environment is incomplete')
  }
  return dockerCommand([
    'exec',
    redisContainer,
    'redis-cli',
    '-a',
    redisPassword,
    '-n',
    redisDatabase,
    '--no-auth-warning',
    ...args,
  ])
}

function sqlString(value: string): string {
  return `'${value.replaceAll('\\', '\\\\').replaceAll("'", "''")}'`
}

function readOption(key: string): string {
  const encoded = mysqlQuery(
    `SELECT HEX(value) FROM options WHERE \`key\` = ${sqlString(key)} LIMIT 1;`
  )
  return encoded ? Buffer.from(encoded, 'hex').toString('utf8') : ''
}

function readUserGroup(ownerId: number): string {
  const encoded = mysqlQuery(
    `SELECT HEX(\`group\`) FROM users WHERE id = ${ownerId} LIMIT 1;`
  )
  return Buffer.from(encoded, 'hex').toString('utf8')
}

async function updateSystemOption(
  context: BrowserContext,
  key: string,
  value: string
) {
  const response = await apiRequest(
    context,
    rootUserId,
    'PUT',
    '/api/option/',
    { key, value }
  )
  expect(response.payload.success, response.payload.message).toBe(true)
}

function readCreatedKey(name: string, ownerId: number): CreatedKeyRecord {
  const output = mysqlQuery(
    `SELECT id, COALESCE(\`key\`, ''), COALESCE(key_fingerprint, ''), ` +
      `COALESCE(\`group\`, ''), status, COALESCE(status_reason, '') ` +
      `FROM tokens WHERE user_id = ${ownerId} AND name = ${sqlString(name)} ` +
      `ORDER BY id DESC LIMIT 1;`
  )
  expect(output, `token ${name} must exist`).not.toBe('')
  const record = output.split(/\r?\n/).at(-1)
  if (!record) throw new Error(`token ${name} must exist`)
  const [id, key, fingerprint, group, status, statusReason] = record.split('\t')
  return {
    id: Number(id),
    key,
    fingerprint,
    group,
    status: Number(status),
    statusReason,
  }
}

function cleanUpKeys(prefix: string, releasedFingerprints: string[] = []) {
  const pattern = `${prefix}%`
  const tokenRows = mysqlQuery(
    `SELECT id, COALESCE(key_fingerprint, '') FROM tokens ` +
      `WHERE user_id IN (${userId}, ${adminUserId}) AND name LIKE ${sqlString(pattern)};`
  )
    .split(/\r?\n/)
    .filter(Boolean)
  if (tokenRows.length === 0) return

  const tokenIds: number[] = []
  const fingerprints = [...releasedFingerprints]
  for (const row of tokenRows) {
    const [id, fingerprint] = row.split('\t')
    tokenIds.push(Number(id))
    if (fingerprint) fingerprints.push(fingerprint)
  }

  const tokenIdList = tokenIds.join(',')
  const fingerprintList = fingerprints.map(sqlString).join(',')
  const targetList = tokenIds.map((id) => sqlString(`token:${id}`)).join(',')
  mysqlQuery(`
    DELETE logs FROM logs
    JOIN audit_outbox_events ON BINARY audit_outbox_events.event_id = BINARY logs.event_id
    WHERE audit_outbox_events.target IN (${targetList});
    DELETE ip_security_events FROM ip_security_events
    JOIN audit_outbox_events ON BINARY audit_outbox_events.event_id = BINARY ip_security_events.event_id
    WHERE audit_outbox_events.target IN (${targetList});
    DELETE FROM audit_outbox_events WHERE target IN (${targetList});
    ${
      fingerprintList
        ? `DELETE FROM revoked_token_fingerprints WHERE fingerprint IN (${fingerprintList});`
        : ''
    }
    DELETE FROM tokens WHERE id IN (${tokenIdList});
  `)
  redisCommand(['DEL', `user:${userId}`, `user:${adminUserId}`])
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

async function getUserGroups(context: BrowserContext, actorId: number) {
  const response = await apiRequest<UserGroups>(
    context,
    actorId,
    'GET',
    '/api/user/self/groups'
  )
  expect(response.payload.success, response.payload.message).toBe(true)
  return response.payload.data || {}
}

async function createKey(
  context: BrowserContext,
  actorId: number,
  name: string,
  group: string,
  customKey?: string
) {
  return apiRequest(context, actorId, 'POST', '/api/token/', {
    name,
    remain_quota: 0,
    expired_time: -1,
    unlimited_quota: true,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    group,
    cross_group_retry: false,
    ...(customKey ? { custom_key: customKey } : {}),
  })
}

async function findKey(context: BrowserContext, actorId: number, name: string) {
  const response = await apiRequest<ApiKeyPage>(
    context,
    actorId,
    'GET',
    `/api/token/search?keyword=${encodeURIComponent(name)}&p=1&size=20`
  )
  expect(response.payload.success, response.payload.message).toBe(true)
  const key = response.payload.data?.items.find((item) => item.name === name)
  expect(key, `API key ${name} must be returned by search`).toBeTruthy()
  if (!key) throw new Error(`API key ${name} was not returned by search`)
  return key
}

async function deleteKey(context: BrowserContext, actorId: number, id: number) {
  const response = await apiRequest(
    context,
    actorId,
    'DELETE',
    `/api/token/${id}`
  )
  expect(response.payload.success, response.payload.message).toBe(true)
}

test.describe('mutable production API key acceptance', () => {
  test.skip(
    process.env.E2E_MUTATION_ALLOWED !== '1' || !databaseFixtureAllowed,
    'Set mutation and database fixture flags only for disposable local QA data.'
  )
  test.use({ storageState: userStorageState })

  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem('i18nextLng', 'en')
    })
  })

  test('automatic and custom keys preserve prefixes and support quick group changes', async ({
    browser,
    page,
  }) => {
    test.setTimeout(180_000)
    const suffix = Date.now().toString(36)
    const prefix = `qa-e2e-key-lifecycle-${suffix}`
    const autoName = `${prefix}-auto`
    const customName = `${prefix}-custom`
    const replacementName = `${prefix}-replacement`
    const customKey = `custom_${suffix}_key_value`
    const rootContext = await browser.newContext({
      baseURL,
      storageState: rootStorageState,
    })
    const specialOptionKey = 'group_ratio_setting.group_special_usable_group'
    const originalSpecialRaw = readOption(specialOptionKey) || '{}'

    try {
      const currentGroups = await getUserGroups(page.context(), userId)
      const groupRatios = JSON.parse(
        readOption('GroupRatio') || '{}'
      ) as Record<string, number>
      const fixtureGroup = Object.keys(groupRatios).find(
        (group) => !Object.hasOwn(currentGroups, group)
      )
      expect(
        fixtureGroup,
        'The fixture needs one priced group outside the user base groups'
      ).toBeTruthy()
      if (!fixtureGroup) {
        throw new Error('No alternate priced group is available')
      }
      const userGroup = readUserGroup(userId)
      const specialGroups = JSON.parse(originalSpecialRaw) as Record<
        string,
        Record<string, string>
      >
      specialGroups[userGroup] = {
        ...specialGroups[userGroup],
        [`+:${fixtureGroup}`]: 'E2E alternate API key group',
      }
      await updateSystemOption(
        rootContext,
        specialOptionKey,
        JSON.stringify(specialGroups)
      )
      await expect
        .poll(async () =>
          Object.keys(await getUserGroups(page.context(), userId))
        )
        .toContain(fixtureGroup)

      const groups = await getUserGroups(page.context(), userId)
      const groupNames = Object.keys(groups)
      expect(groupNames.length).toBeGreaterThan(1)
      const initialGroup = groupNames.includes('default')
        ? 'default'
        : groupNames[0]
      const targetGroup = groupNames.find((group) => group !== initialGroup)
      if (!initialGroup || !targetGroup) {
        throw new Error('At least two usable API key groups are required')
      }

      const automatic = await createKey(
        page.context(),
        userId,
        autoName,
        initialGroup
      )
      expect(automatic.payload.success, automatic.payload.message).toBe(true)
      const automaticRecord = readCreatedKey(autoName, userId)
      expect(automaticRecord.key).toMatch(/^sk-[A-Za-z0-9_-]+$/)

      await page.goto('/keys', { waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(page)
      await page.getByRole('button', { name: 'Create API Key' }).click()
      const drawer = page.getByRole('dialog', { name: 'Create API Key' })
      await expect(drawer).toBeVisible()
      await drawer.getByLabel('Name', { exact: true }).fill(customName)
      await drawer.getByRole('button', { name: 'Advanced Settings' }).click()
      const customKeyInput = drawer.getByLabel('Custom API Key', {
        exact: true,
      })
      await customKeyInput.fill('short-key')
      await drawer.getByRole('button', { name: 'Save changes' }).click()
      await expect(
        drawer.getByText('Custom API Key must be between 16 and 128 characters')
      ).toBeVisible()

      await customKeyInput.fill(customKey)
      const createResponsePromise = page.waitForResponse(
        (response) =>
          response.request().method() === 'POST' &&
          new URL(response.url()).pathname === '/api/token/'
      )
      await drawer.getByRole('button', { name: 'Save changes' }).click()
      const createResponse = await createResponsePromise
      const createPayload =
        (await createResponse.json()) as ApiEnvelope<unknown>
      expect(createPayload.success, createPayload.message).toBe(true)
      await expect(drawer).toBeHidden()

      const createdRecord = readCreatedKey(customName, userId)
      expect(createdRecord.key).toBe(customKey)
      expect(createdRecord.key).not.toMatch(/^sk-/)

      const search = page.getByPlaceholder('Filter by name...')
      await search.fill(customName)
      const row = page.getByRole('row').filter({ hasText: customName })
      await expect(row).toBeVisible()
      const groupSelector = row.getByRole('combobox')
      await groupSelector.click()
      const groupSearch = page.getByPlaceholder('Search...')
      await groupSearch.fill(targetGroup)
      const targetOption = page
        .getByRole('option')
        .filter({ hasText: targetGroup })
      await expect(targetOption).toBeVisible()
      const optionBounds = await targetOption.boundingBox()
      expect(optionBounds).not.toBeNull()
      const viewport = page.viewportSize()
      if (!optionBounds || !viewport) {
        throw new Error('Group option bounds and viewport must be available')
      }
      expect(optionBounds.x).toBeGreaterThanOrEqual(0)
      expect(optionBounds.y).toBeGreaterThanOrEqual(0)
      expect(optionBounds.x + optionBounds.width).toBeLessThanOrEqual(
        viewport.width
      )
      expect(optionBounds.y + optionBounds.height).toBeLessThanOrEqual(
        viewport.height
      )
      await targetOption.click()
      await expect
        .poll(
          async () => (await findKey(page.context(), userId, customName)).group
        )
        .toBe(targetGroup)

      await deleteKey(page.context(), userId, createdRecord.id)
      const recreated = await createKey(
        page.context(),
        userId,
        replacementName,
        initialGroup,
        customKey
      )
      expect(recreated.payload.success, recreated.payload.message).toBe(true)
      const replacement = readCreatedKey(replacementName, userId)
      expect(replacement.key).toBe(customKey)
      await deleteKey(page.context(), userId, replacement.id)
      await deleteKey(page.context(), userId, automaticRecord.id)
    } finally {
      try {
        cleanUpKeys(prefix)
      } finally {
        try {
          await updateSystemOption(
            rootContext,
            specialOptionKey,
            originalSpecialRaw
          )
        } finally {
          await rootContext.close()
        }
      }
    }
  })

  test('cross-user collision displays leaked status and permanently revokes the key', async ({
    browser,
    page,
  }) => {
    test.setTimeout(180_000)
    const suffix = Date.now().toString(36)
    const prefix = `qa-e2e-key-leak-${suffix}`
    const ownerName = `${prefix}-owner`
    const collisionName = `${prefix}-collision`
    const recreateName = `${prefix}-recreate`
    const customKey = `leaked_${suffix}_key_value`
    const revocationBaselineId = Number(
      mysqlQuery('SELECT COALESCE(MAX(id), 0) FROM revoked_token_fingerprints;')
    )
    const adminContext = await browser.newContext({
      baseURL,
      storageState: adminStorageState,
    })
    let ownerFingerprint = ''
    let revokedFingerprint = ''

    try {
      const userGroups = await getUserGroups(page.context(), userId)
      const adminGroups = await getUserGroups(adminContext, adminUserId)
      const userGroup = Object.keys(userGroups)[0]
      const adminGroup = Object.keys(adminGroups)[0]
      expect(userGroup).toBeTruthy()
      expect(adminGroup).toBeTruthy()

      const ownerCreate = await createKey(
        page.context(),
        userId,
        ownerName,
        userGroup,
        customKey
      )
      expect(ownerCreate.payload.success, ownerCreate.payload.message).toBe(
        true
      )
      const ownerRecord = readCreatedKey(ownerName, userId)
      ownerFingerprint = ownerRecord.fingerprint

      const collision = await createKey(
        adminContext,
        adminUserId,
        collisionName,
        adminGroup,
        customKey
      )
      expect(collision.payload.success).toBe(false)
      expect(collision.payload.code).toBe('token_custom_key_unavailable')
      expect(collision.payload.message).not.toContain('Duplicate entry')
      expect(collision.payload.message).not.toContain('idx_tokens_key')

      await expect
        .poll(() => readCreatedKey(ownerName, userId).statusReason)
        .toBe('custom_key_collision_suspected_compromise')
      revokedFingerprint = mysqlQuery(
        `SELECT fingerprint FROM revoked_token_fingerprints ` +
          `WHERE id > ${revocationBaselineId} AND reason = 'compromised' ` +
          `ORDER BY id DESC LIMIT 1;`
      )
      expect(revokedFingerprint).toMatch(/^[a-f0-9]{64}$/)

      const usageResponse = await page
        .context()
        .request.get(new URL('/api/usage/token/', baseURL).href, {
          headers: { Authorization: `Bearer ${customKey}` },
        })
      expect(usageResponse.status()).toBe(401)

      await page.goto('/keys', { waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(page)
      const search = page.getByPlaceholder('Filter by name...')
      await search.fill(ownerName)
      const leakedRow = page.getByRole('row').filter({ hasText: ownerName })
      await expect(leakedRow).toBeVisible()
      await expect(leakedRow.getByText('Leaked', { exact: true })).toBeVisible()
      await expect(
        leakedRow.getByRole('button', {
          name: 'This API key has been disabled because it may have been exposed. Create a new key and replace the old one.',
        })
      ).toBeDisabled()

      await deleteKey(page.context(), userId, ownerRecord.id)
      const recreate = await createKey(
        page.context(),
        userId,
        recreateName,
        userGroup,
        customKey
      )
      expect(recreate.payload.success).toBe(false)
      expect(recreate.payload.code).toBe('token_key_revoked')
      expect(recreate.payload.message).not.toContain('Duplicate entry')
      expect(recreate.payload.message).not.toContain('idx_tokens_key')
    } finally {
      cleanUpKeys(
        prefix,
        [ownerFingerprint, revokedFingerprint].filter(Boolean)
      )
      await adminContext.close()
    }
  })
})
