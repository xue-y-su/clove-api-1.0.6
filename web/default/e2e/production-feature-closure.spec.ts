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
  data: T
}

type PricingModel = {
  model_name: string
  quota_type: number
  model_ratio: number
  completion_ratio: number
  model_price?: number
  billing_mode?: string
  billing_expr?: string
  enable_groups: string[]
}

type PricingPayload = {
  success: boolean
  data: PricingModel[]
  group_ratio: Record<string, number>
  usable_group: Record<string, string>
  all_group: Record<string, string>
}

type CatalogVisibility = {
  version: 3
  enabled: boolean
  default_mode: 'all' | 'hidden'
  rules: Record<string, { mode: 'all' | 'selected'; user_groups: string[] }>
}

type CatalogVisibilityState = {
  config?: CatalogVisibility
  etag: string
}

type SubscriptionPlan = {
  id: number
  title: string
  price_amount: number
  enabled: boolean
  allow_balance_pay?: boolean
  upgrade_group?: string
}

type PlanRecord = {
  plan: SubscriptionPlan
}

type UserRedemptionCode = {
  id: number
  code: string
  reward_type: string
  max_claims: number
  total_quota: number
}

type DomainBaseline = {
  users: number
  tokens: number
  walletEntries: number
  walletDelta: number
  redemptionCodes: number
  redemptionClaims: number
  redemptionEntitlements: number
  userSubscriptions: number
  subscriptionOrders: number
  topUps: number
  acknowledgements: number
  complianceConfirmations: number
  sourceConfirmations: number
  inviteUses: number
  invitePayouts: number
}

type AuditBaseline = {
  outboxId: number
  logId: number
  ipEventId: number
  acknowledgementId: number
}

type UserBaseline = {
  quota: number
  group: string
}

type DatabaseBaseline = {
  domain: DomainBaseline
  audit: AuditBaseline
  creator: UserBaseline
  administrator: UserBaseline
  redemptionCodeId: number
}

type SubscriptionPlanFixture = {
  id: number
  title: string
  priceAmount: number
  upgradeGroup: string
}

type RegistrationSettings = {
  aff_required: boolean
  invite_limit_unlimited: boolean
  invite_limit_count: number
  invitation_applications_enabled: boolean
  invitation_reapply_cooldown_seconds: number
  invitation_poster_title: string
  invitation_poster_subtitle: string
}

function requireStorageState(name: string): string {
  const value = process.env[name]
  if (!value) {
    throw new Error(`${name} must point to a local Playwright storage state`)
  }
  return value
}

async function getCatalogVisibility(context: BrowserContext) {
  const response = await context.request.get(
    new URL('/api/option/model-catalog-visibility', baseURL).href,
    { headers: { 'New-Api-User': String(rootUserId) } }
  )
  expect(response.status()).toBe(200)
  const payload = (await response.json()) as ApiEnvelope<CatalogVisibilityState>
  expect(payload.success, payload.message).toBe(true)
  return payload.data
}

async function replaceCatalogVisibility(
  context: BrowserContext,
  config: CatalogVisibility
) {
  const current = await getCatalogVisibility(context)
  const response = await context.request.put(
    new URL('/api/option/model-catalog-visibility', baseURL).href,
    {
      data: { config, expected_etag: current.etag },
      headers: { 'New-Api-User': String(rootUserId) },
    }
  )
  expect(response.status()).toBe(200)
  const payload = (await response.json()) as ApiEnvelope<CatalogVisibilityState>
  expect(payload.success, payload.message).toBe(true)
  return payload.data
}

async function updateSystemOption(
  context: BrowserContext,
  key: string,
  value: string
) {
  const response = await context.request.put(
    new URL('/api/option/', baseURL).href,
    {
      data: { key, value },
      headers: { 'New-Api-User': String(rootUserId) },
    }
  )
  expect(response.status()).toBe(200)
  const payload = (await response.json()) as ApiEnvelope<unknown>
  expect(payload.success, payload.message).toBe(true)
}

async function getRegistrationSettings(context: BrowserContext) {
  const response = await context.request.get(
    new URL('/api/registration/settings', baseURL).href,
    { headers: { 'New-Api-User': String(rootUserId) } }
  )
  expect(response.status()).toBe(200)
  const payload = (await response.json()) as ApiEnvelope<RegistrationSettings>
  expect(payload.success, payload.message).toBe(true)
  return payload.data
}

async function updateRegistrationSettings(
  context: BrowserContext,
  settings: RegistrationSettings
) {
  const response = await context.request.put(
    new URL('/api/registration/settings', baseURL).href,
    {
      data: settings,
      headers: { 'New-Api-User': String(rootUserId) },
    }
  )
  expect(response.status()).toBe(200)
  const payload = (await response.json()) as ApiEnvelope<RegistrationSettings>
  expect(payload.success, payload.message).toBe(true)
  return payload.data
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
    '--default-character-set=utf8mb4',
    '--batch',
    '--raw',
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

function mysqlIdList(ids: number[]): string {
  const unique = [...new Set(ids)]
  expect(unique.length).toBeGreaterThan(0)
  expect(unique.every((id) => Number.isSafeInteger(id) && id > 0)).toBe(true)
  return unique.join(',')
}

function redisFlushFixtureDatabase() {
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

function decodeBase64(value: string): string {
  return Buffer.from(value || '', 'base64').toString('utf8')
}

function readDatabaseBaseline(): DatabaseBaseline {
  const values = mysqlQuery(`
    SELECT
      (SELECT COUNT(*) FROM users),
      (SELECT COUNT(*) FROM tokens),
      (SELECT COUNT(*) FROM wallet_ledger_entries),
      (SELECT COALESCE(SUM(delta), 0) FROM wallet_ledger_entries),
      (SELECT COUNT(*) FROM user_redemption_codes),
      (SELECT COUNT(*) FROM user_redemption_claims),
      (SELECT COUNT(*) FROM user_redemption_subscription_entitlements),
      (SELECT COUNT(*) FROM user_subscriptions),
      (SELECT COUNT(*) FROM subscription_orders),
      (SELECT COUNT(*) FROM top_ups),
      (SELECT COUNT(*) FROM announcement_acknowledgements),
      (SELECT COUNT(*) FROM compliance_confirmations),
      (SELECT COUNT(*) FROM source_attribution_confirmations),
      (SELECT COUNT(*) FROM invite_code_uses),
      (SELECT COUNT(*) FROM invite_reward_payouts),
      (SELECT COALESCE(MAX(id), 0) FROM audit_outbox_events),
      (SELECT COALESCE(MAX(id), 0) FROM logs),
      (SELECT COALESCE(MAX(id), 0) FROM ip_security_events),
      (SELECT COALESCE(MAX(id), 0) FROM announcement_acknowledgements),
      (SELECT COALESCE(MAX(id), 0) FROM user_redemption_codes),
      (SELECT quota FROM users WHERE id = ${userId}),
      (SELECT TO_BASE64(CAST(users.group AS CHAR)) FROM users WHERE id = ${userId}),
      (SELECT quota FROM users WHERE id = ${adminUserId}),
      (SELECT TO_BASE64(CAST(users.group AS CHAR)) FROM users WHERE id = ${adminUserId});
  `).split('\t')
  if (values.length !== 24) {
    throw new Error(`Unexpected baseline shape: ${values.length}`)
  }
  const numbers = values.slice(0, 21).map(Number)
  return {
    domain: {
      users: numbers[0],
      tokens: numbers[1],
      walletEntries: numbers[2],
      walletDelta: numbers[3],
      redemptionCodes: numbers[4],
      redemptionClaims: numbers[5],
      redemptionEntitlements: numbers[6],
      userSubscriptions: numbers[7],
      subscriptionOrders: numbers[8],
      topUps: numbers[9],
      acknowledgements: numbers[10],
      complianceConfirmations: numbers[11],
      sourceConfirmations: numbers[12],
      inviteUses: numbers[13],
      invitePayouts: numbers[14],
    },
    audit: {
      outboxId: numbers[15],
      logId: numbers[16],
      ipEventId: numbers[17],
      acknowledgementId: numbers[18],
    },
    redemptionCodeId: numbers[19],
    creator: {
      quota: numbers[20],
      group: decodeBase64(values[21]),
    },
    administrator: {
      quota: Number(values[22]),
      group: decodeBase64(values[23]),
    },
  }
}

function readDomainBaseline(): DomainBaseline {
  return readDatabaseBaseline().domain
}

function readSubscriptionPlanFixture(): SubscriptionPlanFixture {
  const row = mysqlQuery(`
    SELECT id, TO_BASE64(title), TO_BASE64(COALESCE(upgrade_group, '')), price_amount
    FROM subscription_plans
    WHERE enabled = 1
      AND COALESCE(allow_balance_pay, 1) = 1
      AND price_amount = 1
      AND COALESCE(upgrade_group, '') = ''
      AND (
        max_purchase_per_user = 0 OR
        (SELECT COUNT(*) FROM user_subscriptions
         WHERE user_id = ${adminUserId}
           AND plan_id = subscription_plans.id) < max_purchase_per_user
      )
    ORDER BY (id = 9) DESC, id ASC
    LIMIT 1;
  `).split('\t')
  if (row.length !== 4 || !row[0]) {
    throw new Error('No eligible price-1 balance subscription plan exists')
  }
  return {
    id: Number(row[0]),
    title: decodeBase64(row[1]),
    priceAmount: Number(row[3]),
    upgradeGroup: decodeBase64(row[2]),
  }
}

function fixtureAuditWhere(
  baseline: AuditBaseline,
  actorIds: number[],
  targets: string[]
): string {
  const clauses: string[] = []
  if (actorIds.length > 0) {
    const actorList = mysqlIdList(actorIds)
    clauses.push(`actor_id IN (${actorList})`, `log_user_id IN (${actorList})`)
  }
  if (targets.length > 0) {
    clauses.push(`target IN (${targets.map(mysqlString).join(',')})`)
  }
  if (clauses.length === 0) return '0 = 1'
  return `id > ${baseline.outboxId} AND (${clauses.join(' OR ')})`
}

async function settleAndDeleteAuditFixtures(
  baseline: AuditBaseline,
  actorIds: number[],
  targets: string[]
) {
  const where = fixtureAuditWhere(baseline, actorIds, targets)
  await expect
    .poll(
      () =>
        Number(
          mysqlQuery(
            `SELECT COUNT(*) FROM audit_outbox_events ` +
              `WHERE ${where} AND status <> 'delivered';`
          )
        ),
      { timeout: 20_000 }
    )
    .toBe(0)

  if (actorIds.length > 0) {
    const actorList = mysqlIdList(actorIds)
    await expect
      .poll(
        () =>
          Number(
            mysqlQuery(
              `SELECT COUNT(*) FROM ip_security_events ` +
                `WHERE id > ${baseline.ipEventId} ` +
                `AND user_id IN (${actorList}) ` +
                `AND projection_status <> 'projected';`
            )
          ),
        { timeout: 20_000 }
      )
      .toBe(0)
  }

  const eventIds = mysqlQuery(
    `SELECT event_id FROM audit_outbox_events WHERE ${where};`
  )
    .split(/\r?\n/)
    .filter(Boolean)
  const eventIdList = eventIds.map(mysqlString).join(',')
  const actorList = actorIds.length > 0 ? mysqlIdList(actorIds) : ''
  mysqlQuery(`
    START TRANSACTION;
    DELETE FROM logs
      WHERE ${eventIdList ? `event_id IN (${eventIdList}) OR` : ''}
        (id > ${baseline.logId} ${actorList ? `AND user_id IN (${actorList})` : 'AND 0 = 1'});
    DELETE FROM ip_security_events
      WHERE ${eventIdList ? `event_id IN (${eventIdList}) OR` : ''}
        (id > ${baseline.ipEventId} ${actorList ? `AND user_id IN (${actorList})` : 'AND 0 = 1'});
    DELETE FROM audit_outbox_events WHERE ${where};
    COMMIT;
  `)
}

function findSubscriptionFixtureCodeIds(baseline: DatabaseBaseline): number[] {
  return mysqlQuery(`
    SELECT id FROM user_redemption_codes
    WHERE id > ${baseline.redemptionCodeId}
      AND creator_user_id = ${userId}
      AND reward_type = 'subscription'
    ORDER BY id;
  `)
    .split(/\r?\n/)
    .filter(Boolean)
    .map(Number)
}

async function cleanSubscriptionFixture(
  baseline: DatabaseBaseline,
  knownCodeIds: number[]
) {
  const codeIds = [
    ...new Set([...knownCodeIds, ...findSubscriptionFixtureCodeIds(baseline)]),
  ]
  const targets = codeIds.map((id) => `user_redemption_code:${id}`)
  await settleAndDeleteAuditFixtures(
    baseline.audit,
    [userId, adminUserId],
    targets
  )

  if (codeIds.length > 0) {
    const codeList = mysqlIdList(codeIds)
    const claimIds = mysqlQuery(
      `SELECT id FROM user_redemption_claims WHERE code_id IN (${codeList});`
    )
      .split(/\r?\n/)
      .filter(Boolean)
      .map(Number)
    const subscriptionIds = mysqlQuery(
      `SELECT user_subscription_id FROM user_redemption_claims ` +
        `WHERE code_id IN (${codeList}) AND user_subscription_id > 0;`
    )
      .split(/\r?\n/)
      .filter(Boolean)
      .map(Number)
    const claimList = claimIds.length > 0 ? mysqlIdList(claimIds) : ''
    const subscriptionList =
      subscriptionIds.length > 0 ? mysqlIdList(subscriptionIds) : ''
    const referenceIds = codeIds.map((id) => mysqlString(String(id))).join(',')
    mysqlQuery(`
      START TRANSACTION;
      ${claimList ? `DELETE FROM invite_reward_payouts WHERE trigger_type = 'admin_subscription_redemption' AND trigger_id IN (${claimIds.map((id) => mysqlString(String(id))).join(',')});` : ''}
      ${subscriptionList ? `DELETE FROM user_subscriptions WHERE id IN (${subscriptionList});` : ''}
      DELETE FROM user_redemption_claims WHERE code_id IN (${codeList});
      DELETE FROM user_redemption_subscription_entitlements WHERE code_id IN (${codeList});
      DELETE FROM wallet_ledger_entries
        WHERE reference_type = 'user_redemption_code'
          AND reference_id IN (${referenceIds});
      DELETE FROM user_redemption_codes WHERE id IN (${codeList});
      UPDATE users
        SET quota = ${baseline.creator.quota},
            users.group = ${mysqlString(baseline.creator.group)}
        WHERE id = ${userId};
      UPDATE users
        SET quota = ${baseline.administrator.quota},
            users.group = ${mysqlString(baseline.administrator.group)}
        WHERE id = ${adminUserId};
      COMMIT;
    `)
  }

  mysqlQuery(`
    DELETE FROM announcement_acknowledgements
      WHERE id > ${baseline.audit.acknowledgementId}
        AND user_id IN (${userId}, ${adminUserId});
  `)
  redisFlushFixtureDatabase()
  expect(readDomainBaseline()).toEqual(baseline.domain)
}

async function cleanRegisteredUserFixture(
  baseline: DatabaseBaseline,
  username: string,
  knownUserId: number
) {
  const discovered = Number(
    mysqlQuery(
      `SELECT COALESCE(MAX(id), 0) FROM users WHERE username = ${mysqlString(username)};`
    )
  )
  const registeredUserId = knownUserId || discovered
  if (registeredUserId <= 0) {
    redisFlushFixtureDatabase()
    expect(readDomainBaseline()).toEqual(baseline.domain)
    return
  }

  await settleAndDeleteAuditFixtures(
    baseline.audit,
    [registeredUserId],
    [`user:${registeredUserId}`]
  )
  mysqlQuery(`
    START TRANSACTION;
    DELETE FROM announcement_acknowledgements WHERE user_id = ${registeredUserId};
    DELETE FROM compliance_confirmations WHERE user_id = ${registeredUserId};
    DELETE FROM source_attribution_confirmations WHERE user_id = ${registeredUserId};
    DELETE FROM ip_audit_rollups WHERE user_id = ${registeredUserId};
    DELETE FROM tokens WHERE user_id = ${registeredUserId};
    DELETE FROM wallet_ledger_entries WHERE user_id = ${registeredUserId};
    DELETE FROM user_subscriptions WHERE user_id = ${registeredUserId};
    DELETE FROM subscription_orders WHERE user_id = ${registeredUserId};
    DELETE FROM top_ups WHERE user_id = ${registeredUserId};
    DELETE FROM checkins WHERE user_id = ${registeredUserId};
    DELETE FROM source_attribution_confirmations
      WHERE target_inviter_id = ${registeredUserId};
    DELETE FROM users WHERE id = ${registeredUserId};
    COMMIT;
  `)
  redisFlushFixtureDatabase()

  expect(
    Number(
      mysqlQuery(
        `SELECT COUNT(*) FROM users WHERE id = ${registeredUserId} OR username = ${mysqlString(username)};`
      )
    )
  ).toBe(0)
  expect(
    Number(
      mysqlQuery(
        `SELECT COUNT(*) FROM tokens WHERE user_id = ${registeredUserId};`
      )
    )
  ).toBe(0)
  expect(readDomainBaseline()).toEqual(baseline.domain)
}

async function addEnglishLocale(context: BrowserContext) {
  await context.addInitScript(() => {
    window.localStorage.setItem('i18nextLng', 'en')
  })
}

const announcementProbeCompleted = new WeakSet<Page>()

async function dismissAnnouncement(page: Page) {
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

async function confirmComplianceGate(page: Page) {
  const dialog = page.getByRole('dialog', { name: 'Compliance Statement' })
  if (
    !(await dialog
      .waitFor({ state: 'visible', timeout: 8_000 })
      .then(() => true)
      .catch(() => false))
  ) {
    return
  }
  await dialog
    .getByRole('checkbox', {
      name: 'I have read and agree to the current terms',
    })
    .check()
  await dialog.getByRole('button', { name: 'Confirm current term' }).click()
  await expect(dialog).toBeHidden({ timeout: 10_000 })
}

async function confirmSourceAttributionGate(page: Page) {
  const dialog = page.getByRole('dialog', { name: 'Source Attribution' })
  if (
    !(await dialog
      .waitFor({ state: 'visible', timeout: 8_000 })
      .then(() => true)
      .catch(() => false))
  ) {
    return
  }
  await dialog.getByLabel('Attribution option').click()
  await page.getByRole('option').first().click()
  const description = dialog.getByLabel('Source description')
  if (await description.isVisible().catch(() => false)) {
    await description.fill('Automated local QA')
  }
  await dialog.getByRole('button', { name: 'Confirm', exact: true }).click()
  await expect(dialog).toBeHidden({ timeout: 10_000 })
}

async function passAuthenticatedGates(page: Page) {
  await confirmComplianceGate(page)
  await confirmSourceAttributionGate(page)
  await dismissAnnouncement(page)
}

async function signInThroughForm(
  page: Page,
  username: string,
  password: string
) {
  await page.goto('/sign-in', { waitUntil: 'domcontentloaded' })
  const form = page.locator('form').first()
  await form.getByLabel('Username or Email').fill(username)
  await form.getByLabel('Password', { exact: true }).fill(password)
  const consent = form.getByRole('checkbox')
  if (await consent.isVisible().catch(() => false)) {
    await consent.check()
  }
  const responsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' &&
      new URL(response.url()).pathname === '/api/user/login'
  )
  await form.getByRole('button', { name: 'Sign in', exact: true }).click()
  const response = await responsePromise
  const payload = (await response.json()) as ApiEnvelope<{ id?: number }>
  expect(payload.success, payload.message).toBe(true)
  await expect(page).not.toHaveURL(/\/sign-in(?:\?|$)/, { timeout: 15_000 })
}

test.describe('production feature closure acceptance', () => {
  test.describe.configure({ mode: 'serial' })
  test.skip(
    process.env.E2E_MUTATION_ALLOWED !== '1' || !databaseFixtureAllowed,
    'Enable mutation and database fixtures only for disposable local QA data.'
  )

  test('normal users see locked groups, models, prices, and the unavailable explanation', async ({
    browser,
  }) => {
    const rootContext = await browser.newContext({
      baseURL,
      storageState: rootStorageState,
    })
    const context = await browser.newContext({
      baseURL,
      storageState: userStorageState,
    })
    await addEnglishLocale(context)
    const page = await context.newPage()
    await page.setViewportSize({ width: 1440, height: 900 })
    let originalVisibility: CatalogVisibility | undefined
    try {
      const initialState = await getCatalogVisibility(rootContext)
      expect(initialState.config).toBeTruthy()
      if (!initialState.config) {
        throw new Error('Catalog visibility configuration is unavailable')
      }
      originalVisibility = structuredClone(initialState.config)
      await replaceCatalogVisibility(rootContext, {
        ...initialState.config,
        enabled: true,
        default_mode: 'all',
      })

      const pricingResponsePromise = page.waitForResponse(
        (response) =>
          response.request().method() === 'GET' &&
          new URL(response.url()).pathname === '/api/pricing'
      )
      await page.goto('/pricing', { waitUntil: 'domcontentloaded' })
      const pricingResponse = await pricingResponsePromise
      expect(pricingResponse.status()).toBe(200)
      const pricing = (await pricingResponse.json()) as PricingPayload
      expect(pricing.success).toBe(true)

      const lockedGroups = new Set(
        Object.keys(pricing.all_group).filter(
          (group) => !Object.hasOwn(pricing.usable_group, group)
        )
      )
      const candidate = pricing.data
        .flatMap((model) =>
          model.enable_groups
            .filter((group) => lockedGroups.has(group))
            .map((group) => ({ group, model }))
        )
        .sort((left, right) => {
          const leftScore =
            Number(left.model.enable_groups.length === 1) * 4 +
            Number(left.model.quota_type === 0) * 2 +
            Number(left.model.billing_mode !== 'tiered_expr')
          const rightScore =
            Number(right.model.enable_groups.length === 1) * 4 +
            Number(right.model.quota_type === 0) * 2 +
            Number(right.model.billing_mode !== 'tiered_expr')
          return rightScore - leftScore
        })[0]
      expect(
        candidate,
        'The live catalog must contain a locked model group'
      ).toBeTruthy()
      if (!candidate) {
        throw new Error('The live catalog must contain a locked model group')
      }
      const locked = candidate
      expect(pricing.all_group[locked.group]).toBeTruthy()
      expect(Object.hasOwn(pricing.usable_group, locked.group)).toBe(false)
      expect(pricing.group_ratio[locked.group]).toBeDefined()

      const groupButton = page
        .locator('button')
        .filter({ hasText: pricing.all_group[locked.group] })
        .filter({
          has: page.getByLabel('Unavailable to your account'),
        })
        .first()
      await expect(groupButton).toBeVisible()
      await groupButton.click()
      await page
        .getByPlaceholder('Search model name, provider, endpoint, or tag...')
        .fill(locked.model.model_name)

      const heading = page.getByRole('heading', {
        name: locked.model.model_name,
        exact: true,
      })
      await expect(heading).toBeVisible()
      const card = heading.locator(
        'xpath=ancestor::div[contains(concat(" ", normalize-space(@class), " "), " group ")][1]'
      )
      await expect(card).toContainText(/[¥$¤✿]\s*[\d,.]+/)
      const unavailable = card.getByLabel('Unavailable to your account').first()
      await expect(unavailable).toBeVisible()
      await unavailable.hover()
      await expect(
        page
          .locator('[data-slot="tooltip-content"]')
          .getByText('Unavailable to your account', { exact: true })
      ).toBeVisible()

      await card.getByRole('button', { name: 'Details' }).click()
      const details = page.getByRole('dialog', {
        name: locked.model.model_name,
      })
      await expect(details).toBeVisible()
      const groupRow = details
        .getByRole('row')
        .filter({ hasText: locked.group })
        .last()
      await expect(groupRow).toBeVisible()
      await expect(groupRow).toContainText(
        `${pricing.group_ratio[locked.group]}x`
      )
      await expect(groupRow).toContainText(/[¥$¤✿]\s*[\d,.]+/)
      await expect(
        groupRow.getByLabel('Unavailable to your account')
      ).toBeVisible()

      await replaceCatalogVisibility(rootContext, {
        ...initialState.config,
        enabled: false,
      })
      const originalRulesResponsePromise = page.waitForResponse(
        (response) =>
          response.request().method() === 'GET' &&
          new URL(response.url()).pathname === '/api/pricing'
      )
      await page.reload({ waitUntil: 'domcontentloaded' })
      const originalRulesResponse = await originalRulesResponsePromise
      const originalRulesPricing =
        (await originalRulesResponse.json()) as PricingPayload
      expect(originalRulesPricing.success).toBe(true)
      expect(Object.keys(originalRulesPricing.all_group).sort()).toEqual(
        Object.keys(originalRulesPricing.usable_group).sort()
      )
      await expect(page.getByLabel('Unavailable to your account')).toHaveCount(
        0
      )
    } finally {
      if (originalVisibility) {
        await replaceCatalogVisibility(rootContext, originalVisibility)
      }
      await context.close()
      await rootContext.close()
    }
  })

  test('a user buys one subscription code and an administrator redeems it', async ({
    browser,
  }) => {
    test.setTimeout(180_000)
    const baseline = readDatabaseBaseline()
    const planFixture = readSubscriptionPlanFixture()
    expect(planFixture.priceAmount).toBe(1)
    expect(planFixture.upgradeGroup).toBe('')
    const knownCodeIds: number[] = []
    const userContext = await browser.newContext({
      baseURL,
      storageState: userStorageState,
    })
    const adminContext = await browser.newContext({
      baseURL,
      storageState: adminStorageState,
    })
    await Promise.all([
      addEnglishLocale(userContext),
      addEnglishLocale(adminContext),
    ])
    const userPage = await userContext.newPage()
    const adminPage = await adminContext.newPage()

    try {
      await userPage.goto('/wallet', { waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(userPage)
      await userPage
        .getByRole('button', { name: 'Create Redemption Code' })
        .click()
      const drawer = userPage.getByRole('dialog', {
        name: 'Create Redemption Code',
      })
      await expect(drawer).toBeVisible()
      const plansResponsePromise = userPage.waitForResponse(
        (response) =>
          response.request().method() === 'GET' &&
          new URL(response.url()).pathname === '/api/subscription/plans'
      )
      await drawer
        .getByRole('group', { name: 'Reward type' })
        .getByRole('button', { name: 'Subscription' })
        .click()
      const plansResponse = await plansResponsePromise
      const plansPayload = (await plansResponse.json()) as ApiEnvelope<
        PlanRecord[]
      >
      expect(plansPayload.success, plansPayload.message).toBe(true)
      const eligiblePlans = plansPayload.data.filter(
        ({ plan }) => plan.enabled && plan.allow_balance_pay !== false
      )
      const optionIndex = eligiblePlans.findIndex(
        ({ plan }) => plan.id === planFixture.id
      )
      expect(optionIndex).toBeGreaterThanOrEqual(0)

      await drawer.getByLabel('Plan').click()
      await userPage.getByRole('option').nth(optionIndex).click()
      await expect(drawer.getByLabel('Plan')).toContainText(planFixture.title)
      await drawer.getByLabel('Units').fill('1')
      await expect(drawer.getByText('Total cost').locator('..')).toContainText(
        /\b1(?:\.0+)?\b/
      )

      const createResponsePromise = userPage.waitForResponse(
        (response) =>
          response.request().method() === 'POST' &&
          new URL(response.url()).pathname === '/api/user-redemption/'
      )
      await drawer.getByRole('button', { name: 'Create', exact: true }).click()
      const createResponse = await createResponsePromise
      const createPayload =
        (await createResponse.json()) as ApiEnvelope<UserRedemptionCode>
      expect(createPayload.success, createPayload.message).toBe(true)
      const created = createPayload.data
      knownCodeIds.push(created.id)
      expect(created.reward_type).toBe('subscription')
      expect(created.max_claims).toBe(1)
      await expect(
        drawer.getByRole('textbox', { name: 'Redemption code' })
      ).toHaveValue(created.code)

      const createdRow = mysqlQuery(`
        SELECT c.reward_type, c.max_claims, c.total_quota,
               e.plan_id, e.unit_paid_quota, e.upgrade_group,
               u.quota, TO_BASE64(CAST(u.group AS CHAR))
        FROM user_redemption_codes c
        JOIN user_redemption_subscription_entitlements e ON e.code_id = c.id
        JOIN users u ON u.id = c.creator_user_id
        WHERE c.id = ${created.id};
      `).split('\t')
      expect(createdRow[0]).toBe('subscription')
      expect(Number(createdRow[1])).toBe(1)
      expect(Number(createdRow[3])).toBe(planFixture.id)
      expect(Number(createdRow[2])).toBe(Number(createdRow[4]))
      expect(createdRow[5]).toBe('')
      expect(Number(createdRow[6])).toBe(
        baseline.creator.quota - Number(createdRow[2])
      )
      expect(decodeBase64(createdRow[7])).toBe(baseline.creator.group)

      await adminPage.goto('/wallet', { waitUntil: 'domcontentloaded' })
      await dismissAnnouncement(adminPage)
      await adminPage
        .getByPlaceholder('Enter your redemption code')
        .fill(created.code)
      await adminPage
        .getByRole('button', { name: 'Redeem Code', exact: true })
        .click()
      await expect(
        adminPage.getByText('Subscription redeemed successfully')
      ).toBeVisible()

      await expect
        .poll(
          () =>
            Number(
              mysqlQuery(
                `SELECT COUNT(*) FROM user_redemption_claims ` +
                  `WHERE code_id = ${created.id} AND claimer_user_id = ${adminUserId};`
              )
            ),
          { timeout: 10_000 }
        )
        .toBe(1)
      const claimRow = mysqlQuery(`
        SELECT c.reward_type, c.user_subscription_id,
               s.user_id, s.plan_id, s.status, s.source,
               e.plan_id
        FROM user_redemption_claims c
        JOIN user_subscriptions s ON s.id = c.user_subscription_id
        JOIN user_redemption_subscription_entitlements e ON e.code_id = c.code_id
        WHERE c.code_id = ${created.id}
          AND c.claimer_user_id = ${adminUserId};
      `).split('\t')
      expect(claimRow[0]).toBe('subscription')
      expect(Number(claimRow[1])).toBeGreaterThan(0)
      expect(Number(claimRow[2])).toBe(adminUserId)
      expect(Number(claimRow[3])).toBe(planFixture.id)
      expect(claimRow[4]).toBe('active')
      expect(claimRow[5]).toBe('redemption')
      expect(Number(claimRow[6])).toBe(planFixture.id)
    } finally {
      try {
        await cleanSubscriptionFixture(baseline, knownCodeIds)
      } finally {
        await Promise.all([userContext.close(), adminContext.close()])
      }
    }
  })

  test('a Chinese username survives registration, logout, and a second login', async ({
    browser,
  }) => {
    test.setTimeout(180_000)
    const baseline = readDatabaseBaseline()
    const originalEmailVerification = mysqlQuery(`
      SELECT value FROM options
      WHERE \`key\` = 'EmailVerificationEnabled'
      LIMIT 1;
    `)
    expect(originalEmailVerification).not.toBe('')
    const suffix = Date.now().toString(36).slice(-8)
    const username = `验收用户${suffix}`
    const password = `Qa#${suffix}9x`
    let registeredUserId = 0
    let emailVerificationChanged = false
    let originalRegistrationSettings: RegistrationSettings | undefined
    let registrationSettingsChanged = false
    const rootContext = await browser.newContext({
      baseURL,
      storageState: rootStorageState,
    })
    const context = await browser.newContext({
      baseURL,
      storageState: { cookies: [], origins: [] },
    })
    await addEnglishLocale(context)
    const page = await context.newPage()

    try {
      originalRegistrationSettings = await getRegistrationSettings(rootContext)
      await updateSystemOption(rootContext, 'EmailVerificationEnabled', 'false')
      emailVerificationChanged = true
      await updateRegistrationSettings(rootContext, {
        ...originalRegistrationSettings,
        aff_required: false,
      })
      registrationSettingsChanged = true
      await page.goto('/sign-up', { waitUntil: 'domcontentloaded' })
      const form = page.locator('form').first()
      await form.getByLabel('Username').fill(username)
      await form.getByLabel('Password', { exact: true }).fill(password)
      await form.getByLabel('Confirm password').fill(password)
      const consent = form.getByRole('checkbox')
      if (await consent.isVisible().catch(() => false)) {
        await consent.check()
      }
      const registerResponsePromise = page.waitForResponse(
        (response) =>
          response.request().method() === 'POST' &&
          new URL(response.url()).pathname === '/api/user/register'
      )
      await form.getByRole('button', { name: 'Create account' }).click()
      const registerResponse = await registerResponsePromise
      const registerPayload =
        (await registerResponse.json()) as ApiEnvelope<unknown>
      expect(registerPayload.success, registerPayload.message).toBe(true)
      await expect(page).toHaveURL(/\/sign-in(?:\?|$)/)

      registeredUserId = Number(
        mysqlQuery(
          `SELECT id FROM users WHERE username = ${mysqlString(username)} LIMIT 1;`
        )
      )
      expect(registeredUserId).toBeGreaterThan(0)
      expect(
        decodeBase64(
          mysqlQuery(
            `SELECT TO_BASE64(username) FROM users WHERE id = ${registeredUserId};`
          )
        )
      ).toBe(username)

      await signInThroughForm(page, username, password)
      await passAuthenticatedGates(page)
      await expect
        .poll(() =>
          page.evaluate(() => {
            const value = window.localStorage.getItem('user')
            if (!value) return ''
            return (JSON.parse(value) as { username?: string }).username || ''
          })
        )
        .toBe(username)

      const profileTrigger = page
        .locator('button[data-slot="dropdown-menu-trigger"]')
        .filter({ has: page.locator('[data-slot="avatar"]') })
        .first()
      await expect(profileTrigger).toBeVisible()
      await profileTrigger.click()
      await page.getByRole('menuitem', { name: 'Sign out' }).click()
      const signOut = page.getByRole('alertdialog', { name: 'Sign out' })
      await expect(signOut).toBeVisible()
      await Promise.all([
        page.waitForURL(/\/sign-in(?:\?|$)/),
        signOut.getByRole('button', { name: 'Sign out', exact: true }).click(),
      ])
      await expect
        .poll(() => page.evaluate(() => window.localStorage.getItem('uid')))
        .toBeNull()

      await signInThroughForm(page, username, password)
      await passAuthenticatedGates(page)
      await expect
        .poll(() =>
          page.evaluate(() => {
            const value = window.localStorage.getItem('user')
            if (!value) return ''
            return (JSON.parse(value) as { username?: string }).username || ''
          })
        )
        .toBe(username)
      await expect(
        page
          .locator('button[data-slot="dropdown-menu-trigger"]')
          .filter({ has: page.locator('[data-slot="avatar"]') })
          .first()
      ).toBeVisible()
    } finally {
      try {
        await cleanRegisteredUserFixture(baseline, username, registeredUserId)
      } finally {
        try {
          if (registrationSettingsChanged && originalRegistrationSettings) {
            await updateRegistrationSettings(
              rootContext,
              originalRegistrationSettings
            )
          }
          if (emailVerificationChanged) {
            await updateSystemOption(
              rootContext,
              'EmailVerificationEnabled',
              originalEmailVerification
            )
          }
        } finally {
          await Promise.all([context.close(), rootContext.close()])
        }
      }
    }
  })
})
