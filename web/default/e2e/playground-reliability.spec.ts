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
import { expect, test, type Page } from 'playwright/test'

const authStorageState =
  process.env.E2E_STORAGE_STATE || process.env.E2E_ROOT_STORAGE_STATE

if (!authStorageState) {
  throw new Error(
    'E2E_ROOT_STORAGE_STATE or E2E_STORAGE_STATE must point to a local Playwright storage state'
  )
}

test.use({ storageState: authStorageState })

const imagePngBase64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M/wHwAF/gL+X8n9WQAAAABJRU5ErkJggg=='

type MockOptions = {
  modelDelayMs?: number
  webSearchAvailable?: boolean
}

async function installPlaygroundOptions(page: Page, options: MockOptions = {}) {
  if (options.webSearchAvailable) {
    await page.route('**/api/status', async (route) => {
      const response = await route.fetch()
      const payload = (await response.json()) as {
        data?: Record<string, unknown>
        [key: string]: unknown
      }

      await route.fulfill({
        response,
        json: {
          ...payload,
          data: {
            ...payload.data,
            playground_web_search: {
              available: true,
              native: false,
              tavily: true,
              reason: '',
            },
          },
        },
      })
    })
  }

  await page.route('**/api/user/self/groups*', async (route) => {
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        success: true,
        data: {
          default: { desc: 'Default test group', ratio: 1 },
          vip: { desc: 'VIP image group', ratio: 2 },
        },
      }),
    })
  })

  await page.route('**/api/user/models*', async (route) => {
    if (options.modelDelayMs) {
      await new Promise((resolve) => setTimeout(resolve, options.modelDelayMs))
    }

    const group = new URL(route.request().url()).searchParams.get('group')
    const models =
      group === 'vip'
        ? ['gpt-image-2', 'vip-chat-model']
        : ['gpt-4o', 'gpt-image-2', 'default-only-model']

    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ success: true, data: models }),
    })
  })
}

async function openPlayground(page: Page) {
  await page.goto('/playground', { waitUntil: 'domcontentloaded' })
  await expect(page).toHaveURL(/\/playground(?:\?|$)/)
  await expect(page.getByRole('tab', { name: 'Chat' })).toBeVisible()
}

async function openImageMode(page: Page) {
  await page.getByRole('tab', { name: 'Image' }).click()
  await expect(
    page.getByPlaceholder('Describe the image to generate')
  ).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Model' })).toBeEnabled()
}

async function selectImageModel(page: Page, model: string) {
  const modelInput = page.getByRole('combobox', { name: 'Model' })
  await modelInput.click()
  await modelInput.fill(model)
  await page.getByRole('option', { name: model, exact: true }).click()
  await expect(modelInput).toHaveValue(model)
}

function eventStream(...payloads: Array<Record<string, unknown> | '[DONE]'>) {
  return payloads
    .map((payload) =>
      payload === '[DONE]'
        ? 'data: [DONE]\n\n'
        : `data: ${JSON.stringify(payload)}\n\n`
    )
    .join('')
}

async function enableBasicWebSearch(page: Page) {
  await page.getByRole('button', { name: 'Web search' }).click()
  await page.getByRole('menuitem', { name: /Basic/ }).click()
  await expect(
    page.getByRole('button', { name: 'Web search' })
  ).toHaveAttribute('aria-pressed', 'true')
}

async function submitChat(page: Page, message: string) {
  await page.getByPlaceholder('Ask anything').fill(message)
  await page.getByRole('button', { name: 'Send' }).click()
}

async function expectNoHorizontalOverflow(page: Page) {
  const result = await page.evaluate(() => {
    const documentElement = document.documentElement
    const overflowingControls = Array.from(
      document.querySelectorAll<HTMLElement>(
        'button, input, textarea, [role="combobox"]'
      )
    )
      .filter((element) => {
        const style = window.getComputedStyle(element)
        const rect = element.getBoundingClientRect()
        return (
          style.visibility !== 'hidden' &&
          style.display !== 'none' &&
          rect.width > 0 &&
          rect.height > 0 &&
          (rect.left < -1 || rect.right > window.innerWidth + 1)
        )
      })
      .map((element) => ({
        label:
          element.getAttribute('aria-label') ||
          element.getAttribute('placeholder') ||
          element.textContent?.trim().slice(0, 80) ||
          element.tagName,
        rect: element.getBoundingClientRect().toJSON(),
      }))

    return {
      documentOverflow:
        documentElement.scrollWidth > documentElement.clientWidth + 1,
      overflowingControls,
    }
  })

  expect(result).toEqual({
    documentOverflow: false,
    overflowingControls: [],
  })
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('i18nextLng', 'en')
    window.localStorage.removeItem('playground_config')
    window.localStorage.removeItem('playground_messages')
    window.localStorage.removeItem('playground_parameter_enabled')
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

test('rapid chat and image switching does not surface cancellation errors', async ({
  page,
}) => {
  await installPlaygroundOptions(page, { modelDelayMs: 180 })
  const browserErrors: string[] = []
  page.on('pageerror', (error) => browserErrors.push(error.message))
  page.on('console', (message) => {
    if (message.type() === 'error') browserErrors.push(message.text())
  })

  await openPlayground(page)
  await page.getByRole('tab', { name: 'Image' }).click()
  await page.getByRole('tab', { name: 'Chat' }).click()
  await page.getByRole('tab', { name: 'Image' }).click()

  await expect(
    page.getByPlaceholder('Describe the image to generate')
  ).toBeVisible()
  await expect(page.getByText(/canceled/i)).toHaveCount(0)
  expect(browserErrors.filter((message) => /canceled/i.test(message))).toEqual(
    []
  )
})

test('image group changes refresh models and model search remains usable', async ({
  page,
}) => {
  await installPlaygroundOptions(page)
  await openPlayground(page)
  await openImageMode(page)

  await page.getByRole('combobox', { name: 'Group' }).click()
  await page.getByRole('option', { name: 'vip', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Group' })).toContainText(
    'vip'
  )

  await selectImageModel(page, 'gpt-image-2')
  await expect(page.getByRole('combobox', { name: 'Quality' })).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Background' })).toBeVisible()

  const modelInput = page.getByRole('combobox', { name: 'Model' })
  await modelInput.click()
  await modelInput.fill('default-only')
  await expect(
    page.getByRole('option', { name: 'default-only-model' })
  ).toHaveCount(0)
})

test('image 503 keeps the form and displays the upstream reason', async ({
  page,
}) => {
  await installPlaygroundOptions(page)
  await page.route('**/pg/images/generations', async (route) => {
    await route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({
        error: {
          code: 'upstream_unavailable',
          message:
            'Image provider is temporarily unavailable (request id: img-503)',
        },
      }),
    })
  })

  await openPlayground(page)
  await openImageMode(page)
  await selectImageModel(page, 'gpt-image-2')
  const prompt = 'A deterministic red square'
  await page.getByPlaceholder('Describe the image to generate').fill(prompt)
  await page.getByRole('button', { name: 'Generate' }).click()

  await expect(page.getByRole('alert')).toContainText(
    'Image provider is temporarily unavailable (request id: img-503)'
  )
  await expect(
    page.getByPlaceholder('Describe the image to generate')
  ).toHaveValue(prompt)
  await expect(page.getByRole('combobox', { name: 'Model' })).toHaveValue(
    'gpt-image-2'
  )
})

test('successful image response renders a preview without exposing a raw URL', async ({
  page,
}) => {
  await installPlaygroundOptions(page)
  await page.route('**/pg/images/generations', async (route) => {
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ data: [{ b64_json: imagePngBase64 }] }),
    })
  })

  await openPlayground(page)
  await openImageMode(page)
  await selectImageModel(page, 'gpt-image-2')
  const prompt = 'A generated preview'
  await page.getByPlaceholder('Describe the image to generate').fill(prompt)
  await page.getByRole('button', { name: 'Generate' }).click()

  await expect(page.getByRole('img', { name: prompt })).toBeVisible()
  await expect(page.getByText(/^https?:\/\//)).toHaveCount(0)
})

test('web search handles empty sources and hides reasoning while disabled', async ({
  page,
}) => {
  await installPlaygroundOptions(page, { webSearchAvailable: true })
  await page.route('**/pg/chat/completions', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: eventStream(
        {
          type: 'playground.metadata',
          search: { status: 'ok', path: 'tavily', sources: [] },
        },
        {
          choices: [
            {
              delta: {
                reasoning_content: 'hidden provider reasoning',
                content: '<think>hidden tagged reasoning</think>Visible answer',
              },
            },
          ],
        },
        '[DONE]'
      ),
    })
  })

  await openPlayground(page)
  await enableBasicWebSearch(page)
  await submitChat(page, 'Search without sources')

  await expect(page.getByText('Visible answer', { exact: true })).toBeVisible()
  await expect(page.getByText(/hidden provider reasoning/)).toHaveCount(0)
  await expect(page.getByText(/hidden tagged reasoning/)).toHaveCount(0)
  await expect(page.getByRole('link', { name: /source/i })).toHaveCount(0)
})

test('web search renders returned sources', async ({ page }) => {
  await installPlaygroundOptions(page, { webSearchAvailable: true })
  await page.route('**/pg/chat/completions', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: eventStream(
        {
          type: 'playground.metadata',
          search: {
            status: 'ok',
            path: 'tavily',
            sources: [
              {
                domain: 'example.com',
                snippet: 'Verified source snippet',
                title: 'Verified source',
                url: 'https://example.com/source',
              },
            ],
          },
        },
        { choices: [{ delta: { content: 'Answer with source' } }] },
        '[DONE]'
      ),
    })
  })

  await openPlayground(page)
  await enableBasicWebSearch(page)
  await submitChat(page, 'Search with a source')

  await expect(
    page.getByText('Answer with source', { exact: true })
  ).toBeVisible()
  await page.getByRole('button', { name: 'Used 1 sources' }).click()
  await expect(
    page.getByRole('link', { name: 'Verified source' })
  ).toHaveAttribute('href', 'https://example.com/source')
})

test('web search failure stays on the playground and shows a specific error', async ({
  page,
}) => {
  await installPlaygroundOptions(page, { webSearchAvailable: true })
  await page.route('**/pg/chat/completions', async (route) => {
    await route.fulfill({
      status: 503,
      contentType: 'text/event-stream',
      body: eventStream({
        error: {
          code: 'search_unavailable',
          message: 'Web search provider is temporarily unavailable',
        },
      }),
    })
  })

  await openPlayground(page)
  await enableBasicWebSearch(page)
  await submitChat(page, 'Search failure')

  await expect(page).toHaveURL(/\/playground(?:\?|$)/)
  await expect(
    page.getByText('Web search provider is temporarily unavailable').first()
  ).toBeVisible()
  await expect(page.getByText(/500 Internal Server Error/i)).toHaveCount(0)
})

for (const viewport of [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'mobile', width: 390, height: 844 },
]) {
  test(`image controls fit the ${viewport.name} viewport`, async ({ page }) => {
    await page.setViewportSize({
      width: viewport.width,
      height: viewport.height,
    })
    await installPlaygroundOptions(page)
    await openPlayground(page)
    await openImageMode(page)
    await selectImageModel(page, 'gpt-image-2')

    await expectNoHorizontalOverflow(page)
    await expect(page.getByRole('button', { name: 'Generate' })).toBeVisible()
  })
}
