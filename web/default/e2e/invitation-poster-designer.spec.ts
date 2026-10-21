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
import { readFileSync } from 'node:fs'

import {
  expect,
  test,
  type BrowserContextOptions,
  type Page,
} from 'playwright/test'

const baseURL = process.env.E2E_BASE_URL || 'http://127.0.0.1:3026'
const storageState = normalizedStorageState('E2E_USER_STORAGE_STATE')

function normalizedStorageState(
  name: string
): NonNullable<BrowserContextOptions['storageState']> {
  const file = process.env[name]
  if (!file) throw new Error(`${name} must point to a local storage state`)

  const state = JSON.parse(readFileSync(file, 'utf8')) as Exclude<
    NonNullable<BrowserContextOptions['storageState']>,
    string
  >
  const origin = new URL(baseURL).origin
  return {
    ...state,
    origins: state.origins.map((entry) => ({ ...entry, origin })),
  }
}

async function mockActiveInvitation(page: Page) {
  await page.route('**/api/invitation/self', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        success: true,
        message: '',
        data: {
          application: null,
          code: {
            id: 990_001,
            code: 'POSTER-QA-CODE',
            kind: 'multi',
            max_uses: 10,
            used_count: 2,
            status: 'active',
            expires_at: 0,
            created_at: 1_785_420_000,
            updated_at: 1_785_420_000,
          },
          uses: [],
          can_apply: false,
        },
      }),
    })
  })
}

async function dismissAnnouncement(page: Page) {
  const announcement = page.getByRole('dialog').filter({
    has: page.getByRole('button', { name: 'Got it' }),
  })
  const visible = await announcement
    .waitFor({ state: 'visible', timeout: 5_000 })
    .then(() => true)
    .catch(() => false)
  if (!visible) return
  await announcement.getByRole('button', { name: 'Got it' }).click()
  await expect(announcement).toBeHidden()
}

async function openPoster(page: Page) {
  await page.addInitScript(() => {
    window.localStorage.setItem('i18nextLng', 'en')
  })
  await mockActiveInvitation(page)
  await page.goto('/wallet', { waitUntil: 'domcontentloaded' })
  await dismissAnnouncement(page)

  const qrButton = page.getByRole('button', {
    name: 'Show invitation QR code',
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

async function expectCanvasSize(page: Page, width: number, height: number) {
  const canvas = page
    .getByRole('img', { name: 'Invitation poster preview' })
    .locator('canvas')
  await expect
    .poll(async () =>
      canvas.evaluate((element) => {
        const poster = element as HTMLCanvasElement
        return `${poster.width}x${poster.height}`
      })
    )
    .toBe(`${width}x${height}`)
}

test.describe('invitation poster designer', () => {
  test.use({ storageState })

  test('keeps preview first and saves a modular desktop poster', async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width: 1440, height: 1000 })
    const dialog = await openPoster(page)

    await expect(
      dialog.getByRole('button', { name: 'Edit poster' })
    ).toBeVisible()
    await expect(
      dialog.getByRole('button', { name: 'Download PNG' })
    ).toBeEnabled()
    await expect(dialog.getByText('Poster size', { exact: true })).toHaveCount(
      0
    )
    await expectCanvasSize(page, 1080, 1440)
    await dialog.screenshot({
      path: testInfo.outputPath('desktop-preview.png'),
    })

    await dialog.getByRole('button', { name: 'Edit poster' }).click()
    await expect(dialog.getByText('Poster size', { exact: true })).toBeVisible()
    await expect(
      dialog.getByText('Poster modules', { exact: true })
    ).toBeVisible()
    await expect(
      dialog.getByRole('button', { name: 'Download PNG' })
    ).toHaveCount(0)
    await dialog.getByText('9:16', { exact: true }).click()
    await dialog.getByRole('button', { name: 'Save' }).click()

    await expect(
      dialog.getByRole('button', { name: 'Download PNG' })
    ).toBeEnabled()
    await expectCanvasSize(page, 1080, 1920)
    await dialog.screenshot({ path: testInfo.outputPath('desktop-saved.png') })
  })

  test('keeps every editor module usable without mobile overflow', async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 844 })
    const dialog = await openPoster(page)
    await dialog.getByRole('button', { name: 'Edit poster' }).click()

    await expect(dialog.getByText('Poster size', { exact: true })).toBeVisible()
    await expect(
      dialog.getByText('Poster background', { exact: true })
    ).toBeVisible()
    await expect(
      dialog.getByText('Poster modules', { exact: true })
    ).toBeVisible()
    await expect(
      dialog.getByRole('button', { name: 'Local image' })
    ).toBeVisible()
    await expect(dialog.getByRole('button', { name: 'Save' })).toBeVisible()

    const overflow = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    }))
    expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth)
    await page.screenshot({
      path: testInfo.outputPath('mobile-editor.png'),
      fullPage: true,
    })

    const modulesHeading = dialog.getByText('Poster modules', { exact: true })
    await modulesHeading.scrollIntoViewIfNeeded()
    await expect(modulesHeading).toBeInViewport()
    const moduleBox = await modulesHeading.boundingBox()
    const saveBox = await dialog
      .getByRole('button', { name: 'Save' })
      .boundingBox()
    expect(moduleBox).not.toBeNull()
    expect(saveBox).not.toBeNull()
    expect((moduleBox?.y || 0) + (moduleBox?.height || 0)).toBeLessThan(
      saveBox?.y || 0
    )
    await page.screenshot({
      path: testInfo.outputPath('mobile-modules.png'),
      fullPage: true,
    })
  })
})
