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

test('resolves anonymous branding after one successful status request', async ({
  page,
}) => {
  let statusRequestCount = 0
  page.on('request', (request) => {
    const url = new URL(request.url())
    if (request.method() === 'GET' && url.pathname === '/api/status') {
      statusRequestCount += 1
    }
  })

  await page.goto('/sign-in', { waitUntil: 'domcontentloaded' })

  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
  const brandLink = page.locator('a[href="/"]').first()
  await expect(brandLink.locator('[data-slot="skeleton"]')).toHaveCount(0)
  await expect(brandLink.locator('img')).toBeVisible()
  expect(statusRequestCount).toBe(1)
})

test('ends anonymous branding loading when status request fails', async ({
  page,
}) => {
  await page.route('**/api/status', async (route) => {
    await route.fulfill({
      body: JSON.stringify({
        success: false,
        message: 'status fixture unavailable',
      }),
      contentType: 'application/json',
      status: 503,
    })
  })

  await page.goto('/sign-in', { waitUntil: 'domcontentloaded' })

  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
  const brandLink = page.locator('a[href="/"]').first()
  await expect(brandLink.locator('[data-slot="skeleton"]')).toHaveCount(0, {
    timeout: 15_000,
  })
  await expect(brandLink.locator('img')).toBeVisible()
})
