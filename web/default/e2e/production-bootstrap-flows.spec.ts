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
import { expect, test } from 'playwright/test'

const rootStorageState = process.env.E2E_ROOT_STORAGE_STATE

if (!rootStorageState) {
  throw new Error(
    'E2E_ROOT_STORAGE_STATE must point to a local Playwright storage state'
  )
}

test('shares one status request across the authenticated application shell', async ({
  browser,
}) => {
  const context = await browser.newContext({ storageState: rootStorageState })
  await context.addInitScript(() => {
    window.localStorage.removeItem('status')
  })
  const page = await context.newPage()
  let statusRequestCount = 0

  await page.route('**/api/status', async (route) => {
    statusRequestCount += 1
    await route.continue()
  })

  const statusResponse = page.waitForResponse(
    (response) =>
      response.request().method() === 'GET' &&
      new URL(response.url()).pathname === '/api/status'
  )
  await page.goto('/security-audit', { waitUntil: 'domcontentloaded' })
  await statusResponse
  await page.waitForLoadState('networkidle')

  expect(statusRequestCount).toBe(1)
  await context.close()
})
