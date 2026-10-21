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
import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { expect, test, type Page, type TestInfo } from 'playwright/test'

const rootStorageState = requireStorageState('E2E_ROOT_STORAGE_STATE')
const adminStorageState = requireStorageState('E2E_ADMIN_STORAGE_STATE')
const userStorageState = requireStorageState('E2E_USER_STORAGE_STATE')

const avatarFixturePath = fileURLToPath(
  new URL('../public/logo.png', import.meta.url)
)
const screenshotDirectory = fileURLToPath(
  new URL('../../../.verification-output/screenshots/', import.meta.url)
)

type RuntimeWatch = {
  consoleErrors: string[]
  pageErrors: string[]
  serverErrors: string[]
}

type RouteEvidence = {
  gateResolutionMs: number
  navigationDurationMs: number
  path: string
  transferredScriptBytes: number
}

function requireStorageState(name: string): string {
  const value = process.env[name]
  if (!value) {
    throw new Error(`${name} must point to a local Playwright storage state`)
  }
  return value
}

function watchRuntime(page: Page): RuntimeWatch {
  const watch: RuntimeWatch = {
    consoleErrors: [],
    pageErrors: [],
    serverErrors: [],
  }

  page.on('console', (message) => {
    if (message.type() === 'error') watch.consoleErrors.push(message.text())
  })
  page.on('pageerror', (error) => watch.pageErrors.push(error.message))
  page.on('response', (response) => {
    if (response.status() >= 500) {
      watch.serverErrors.push(`${response.status()} ${response.url()}`)
    }
  })

  return watch
}

const announcementProbeCompleted = new WeakSet<Page>()

async function dismissAnnouncement(page: Page) {
  const authenticated = await page
    .evaluate(() => Boolean(window.localStorage.getItem('uid')))
    .catch(() => false)
  if (!authenticated) return false

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
  if (!visible) return false

  await announcement.getByRole('button', { name: 'Got it' }).click()
  await expect(announcement).toBeHidden()
  await expect(
    page.getByText('Announcement confirmation failed. Please try again.')
  ).toHaveCount(0)
  return true
}

async function confirmComplianceStatement(page: Page) {
  const authenticated = await page.evaluate(() =>
    Boolean(window.localStorage.getItem('uid'))
  )
  if (!authenticated) return false

  const compliance = page.getByRole('dialog', {
    name: 'Compliance Statement',
  })
  const attribution = page.getByRole('dialog', { name: 'Source Attribution' })
  const main = page.locator('main').first()

  await expect
    .poll(
      async () =>
        (await compliance.isVisible().catch(() => false)) ||
        (await attribution.isVisible().catch(() => false)) ||
        (await main.isVisible().catch(() => false)),
      { timeout: 10_000 }
    )
    .toBe(true)
  if (!(await compliance.isVisible().catch(() => false))) return false

  await compliance
    .getByRole('checkbox', {
      name: 'I have read and agree to the current terms',
    })
    .check()
  await compliance.getByRole('button', { name: 'Confirm current term' }).click()
  await expect(compliance).toBeHidden({ timeout: 10_000 })
  await expect(page.getByText('Failed to confirm compliance')).toHaveCount(0)
  return true
}

async function confirmSourceAttribution(page: Page) {
  const authenticated = await page.evaluate(() =>
    Boolean(window.localStorage.getItem('uid'))
  )
  if (!authenticated) return false

  const attribution = page.getByRole('dialog', { name: 'Source Attribution' })
  const main = page.locator('main').first()

  await expect
    .poll(
      async () =>
        (await attribution.isVisible().catch(() => false)) ||
        (await main.isVisible().catch(() => false)),
      { timeout: 10_000 }
    )
    .toBe(true)
  if (!(await attribution.isVisible().catch(() => false))) return false

  await attribution
    .getByRole('combobox', { name: 'Attribution option' })
    .click()
  await page.getByRole('option').first().click()

  const sourceDescription = attribution.getByRole('textbox', {
    name: 'Source description',
  })
  if (await sourceDescription.isVisible().catch(() => false)) {
    await sourceDescription.fill('Local QA acceptance')
  }

  await attribution
    .getByRole('button', { name: 'Confirm', exact: true })
    .click()
  await expect(attribution).toBeHidden({ timeout: 10_000 })
  await expect(
    page.getByText('Failed to confirm source attribution')
  ).toHaveCount(0)
  return true
}

async function visibleSpinners(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll<HTMLElement>('.animate-spin'))
      .filter((element) => {
        if (element.closest('[data-nonblocking-progress="true"]')) {
          return false
        }
        const rect = element.getBoundingClientRect()
        const style = window.getComputedStyle(element)
        return (
          rect.width > 0 &&
          rect.height > 0 &&
          style.display !== 'none' &&
          style.visibility !== 'hidden'
        )
      })
      .map(
        (element) =>
          element.getAttribute('aria-label') ||
          element.closest('button')?.textContent?.trim() ||
          element.parentElement?.textContent?.trim().slice(0, 80) ||
          element.tagName
      )
  )
}

async function assertViewportFit(page: Page) {
  const fit = await page.evaluate(() => {
    const root = document.documentElement
    const main = document.querySelector<HTMLElement>('main')
    const mainRect = main?.getBoundingClientRect()

    return {
      documentOverflow: root.scrollWidth > root.clientWidth + 1,
      mainEscapesViewport: Boolean(
        mainRect &&
        mainRect.width > 0 &&
        (mainRect.left < -1 || mainRect.right > window.innerWidth + 1)
      ),
    }
  })

  expect(fit).toEqual({
    documentOverflow: false,
    mainEscapesViewport: false,
  })
}

async function readRouteEvidence(
  page: Page,
  path: string,
  gateResolutionMs: number
): Promise<RouteEvidence> {
  return page.evaluate(
    ({ currentPath, resolutionMs }) => {
      const navigation = performance.getEntriesByType('navigation')[0] as
        | PerformanceNavigationTiming
        | undefined
      const scriptBytes = performance
        .getEntriesByType('resource')
        .filter((entry) => entry.initiatorType === 'script')
        .reduce((total, entry) => {
          const resource = entry as PerformanceResourceTiming
          return total + Math.max(resource.transferSize, 0)
        }, 0)

      return {
        gateResolutionMs: resolutionMs,
        navigationDurationMs: Math.round(navigation?.duration ?? 0),
        path: currentPath,
        transferredScriptBytes: scriptBytes,
      }
    },
    { currentPath: path, resolutionMs: gateResolutionMs }
  )
}

async function navigateAndAssert(
  page: Page,
  path: string,
  watch: RuntimeWatch
): Promise<RouteEvidence> {
  watch.consoleErrors.length = 0
  watch.pageErrors.length = 0
  watch.serverErrors.length = 0
  const startedAt = Date.now()

  await page.goto(path, { waitUntil: 'domcontentloaded' })
  await confirmComplianceStatement(page)
  await confirmSourceAttribution(page)
  await expect(page.locator('main').first()).toBeVisible({ timeout: 10_000 })
  const gateResolutionMs = Date.now() - startedAt
  expect(gateResolutionMs).toBeLessThanOrEqual(10_000)
  await dismissAnnouncement(page)

  await expect
    .poll(() => visibleSpinners(page), { timeout: 10_000 })
    .toEqual([])
  await assertViewportFit(page)
  await expect(page.getByText('TanStack Router', { exact: true })).toHaveCount(
    0
  )
  await expect(page.getByText('TanStack Query', { exact: true })).toHaveCount(0)

  const body = await page.locator('body').innerText()
  expect(body).not.toMatch(
    /500 Internal Server Error|Something went wrong|Unexpected route error|页面加载失败/
  )
  expect(watch.pageErrors).toEqual([])
  expect(watch.serverErrors).toEqual([])
  expect(
    watch.consoleErrors.filter((message) =>
      /uncaught|internal server error|status code 500|failed to fetch/i.test(
        message
      )
    )
  ).toEqual([])

  return readRouteEvidence(page, path, gateResolutionMs)
}

async function attachScreenshot(page: Page, testInfo: TestInfo, name: string) {
  mkdirSync(screenshotDirectory, { recursive: true })
  const screenshotPath = join(
    screenshotDirectory,
    `${name}-${testInfo.project.name}.png`
  )
  await page.screenshot({
    animations: 'disabled',
    path: screenshotPath,
    scale: 'css',
  })
  await testInfo.attach(name, { path: screenshotPath })
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('i18nextLng', 'en')
  })
  await page.route('**/api/announcement/popup-state', async (route) => {
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        success: true,
        data: {
          enabled: true,
          announcement: null,
          signature: '',
          acknowledged: true,
        },
      }),
    })
  })
})

test.describe('anonymous acceptance', () => {
  test.use({ storageState: { cookies: [], origins: [] } })

  test('homepage is healthy and never opens an authenticated announcement', async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    const watch = watchRuntime(page)
    const evidence = await navigateAndAssert(page, '/', watch)

    await expect(
      page.getByRole('dialog').filter({
        has: page.getByRole('button', { name: 'Got it' }),
      })
    ).toHaveCount(0)
    await attachScreenshot(page, testInfo, 'anonymous-home-desktop')
    console.log(`[acceptance-metrics] ${JSON.stringify(evidence)}`)
  })
})

test.describe('root acceptance', () => {
  test.use({ storageState: rootStorageState })

  test('root routes stay healthy at desktop width', async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    const watch = watchRuntime(page)
    const routes = [
      '/profile',
      '/security-audit',
      '/user-redemptions',
      '/invitations',
      '/invitation-rewards',
      '/system-settings/content/announcements',
      '/playground',
      '/keys',
      '/wallet',
    ]
    const evidence: RouteEvidence[] = []

    for (const path of routes) {
      evidence.push(await navigateAndAssert(page, path, watch))
      if (path === '/security-audit') {
        await attachScreenshot(page, testInfo, 'root-security-audit-desktop')
      }
      if (path === '/system-settings/content/announcements') {
        await attachScreenshot(page, testInfo, 'root-announcements-desktop')
      }
    }

    await page.goto('/profile', { waitUntil: 'domcontentloaded' })
    await dismissAnnouncement(page)
    const profileTrigger = page
      .locator('button[data-slot="dropdown-menu-trigger"]')
      .filter({ has: page.locator('[data-slot="avatar"]') })
      .first()
    await expect(profileTrigger).toBeVisible()
    const headerAvatar = profileTrigger.locator('[data-slot="avatar"]')
    const headerAvatarImage = headerAvatar.locator('img')
    if ((await headerAvatarImage.count()) > 0) {
      await expect(headerAvatarImage).toBeVisible()
      await expect
        .poll(() =>
          headerAvatarImage.evaluate(
            (image) => (image as HTMLImageElement).naturalWidth
          )
        )
        .toBeGreaterThan(0)
    } else {
      await expect(headerAvatar).toContainText(/\S/)
    }

    console.log(`[acceptance-metrics] ${JSON.stringify(evidence)}`)
  })
})

test.describe('administrator acceptance', () => {
  test.use({ storageState: adminStorageState })

  test('administrator routes stay healthy at tablet width', async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width: 1024, height: 768 })
    const watch = watchRuntime(page)
    const routes = [
      '/users',
      '/security-audit',
      '/user-redemptions',
      '/invitations',
      '/invitation-rewards',
      '/redemption-codes',
    ]

    for (const path of routes) {
      await navigateAndAssert(page, path, watch)
      if (path === '/user-redemptions') {
        await attachScreenshot(
          page,
          testInfo,
          'administrator-user-redemptions-tablet'
        )
      }
    }

    await page.goto('/system-settings/content/announcements', {
      waitUntil: 'domcontentloaded',
    })
    await expect(page).toHaveURL(/\/403(?:\?|$)/)
    await expect(
      page.getByRole('link', { name: 'System Settings' })
    ).toHaveCount(0)
  })
})

test.describe('normal user acceptance', () => {
  test.use({ storageState: userStorageState })

  test('user routes and avatar synchronization work at mobile width', async ({
    page,
  }, testInfo) => {
    test.setTimeout(120_000)
    await page.setViewportSize({ width: 390, height: 844 })
    await page.route(/^https:\/\//, async (route) => {
      if (route.request().method() === 'POST') {
        await route.fulfill({
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            data: { url: 'https://img.scdn.io/e2e-avatar.png' },
          }),
        })
        return
      }
      if (route.request().resourceType() === 'image') {
        await route.fulfill({
          contentType: 'image/png',
          path: avatarFixturePath,
        })
        return
      }
      await route.continue()
    })
    const watch = watchRuntime(page)
    const routes = [
      '/profile',
      '/wallet',
      '/keys',
      '/playground',
      '/usage-logs/common',
    ]

    for (const path of routes) {
      await navigateAndAssert(page, path, watch)
      if (path === '/wallet') {
        await attachScreenshot(page, testInfo, 'normal-user-wallet-mobile')
      }
    }

    await page.goto('/profile', { waitUntil: 'domcontentloaded' })
    await dismissAnnouncement(page)
    const userIdentity = await page.evaluate(() => {
      const storedUser = window.localStorage.getItem('user')
      if (!storedUser) return { displayName: '', username: '' }
      const user = JSON.parse(storedUser) as {
        display_name?: string
        username?: string
      }
      return {
        displayName: user.display_name ?? '',
        username: user.username ?? '',
      }
    })
    expect(userIdentity.username).not.toBe('')
    const accountLabel = userIdentity.displayName || userIdentity.username
    await page.getByRole('button', { name: 'Edit profile' }).click()
    const editor = page.getByRole('dialog', { name: 'Edit profile' })
    await expect(editor).toBeVisible()
    await editor.locator('input[type="file"]').setInputFiles(avatarFixturePath)
    await editor.getByRole('button', { name: 'Save profile' }).click()
    await expect(editor).toBeHidden({ timeout: 60_000 })

    const headerAvatar = page
      .getByRole('button', { name: `@${accountLabel}` })
      .getByRole('img', { name: `@${accountLabel}` })
    await expect(headerAvatar).toBeVisible()
    await expect(headerAvatar).toHaveAttribute('src', /^https:\/\//)
    await expect
      .poll(() =>
        headerAvatar.evaluate(
          (image) => (image as HTMLImageElement).naturalWidth
        )
      )
      .toBeGreaterThan(0)
    await attachScreenshot(page, testInfo, 'normal-user-avatar-synced-mobile')

    await page.goto('/security-audit', { waitUntil: 'domcontentloaded' })
    await expect(page).toHaveURL(/\/403(?:\?|$)/)
    await page.goto('/user-redemptions', { waitUntil: 'domcontentloaded' })
    await expect(page).toHaveURL(/\/wallet\?show_redemptions=true$/)
  })
})
