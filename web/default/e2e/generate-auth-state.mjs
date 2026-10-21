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
import { mkdir } from 'node:fs/promises'
import { resolve } from 'node:path'

import { chromium } from 'playwright'

function requireEnvironment(name) {
  const value = process.env[name]?.trim()
  if (!value) throw new Error(`${name} is required`)
  return value
}

async function verifySession(context, baseURL, userId) {
  const response = await context.request.get(`${baseURL}/api/user/self`, {
    headers: { 'New-Api-User': userId },
  })
  const result = await response.json()

  if (!response.ok() || !result.success) {
    throw new Error(
      result.message || `session verification returned ${response.status()}`
    )
  }
}

async function createStorageState(context, baseURL, credentials) {
  const page = await context.newPage()
  process.stdout.write(`${credentials.name}: opening sign-in page\n`)
  await page.goto(`${baseURL}/sign-in`, { waitUntil: 'domcontentloaded' })
  process.stdout.write(`${credentials.name}: submitting credentials\n`)
  await page
    .getByRole('textbox', { name: 'Username or Email' })
    .fill(credentials.username)
  await page.getByPlaceholder('Enter password').fill(credentials.password)

  const legalConsent = page.getByRole('checkbox')
  if (
    (await legalConsent.isVisible().catch(() => false)) &&
    !(await legalConsent.isChecked())
  ) {
    await legalConsent.check()
  }

  const loginResponsePromise = page.waitForResponse(
    (response) =>
      response.url().includes('/api/user/login') &&
      response.request().method() === 'POST'
  )
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  const loginResponse = await loginResponsePromise
  const loginResult = await loginResponse.json()

  if (!loginResponse.ok() || !loginResult.success) {
    throw new Error(
      loginResult.message || `login returned ${loginResponse.status()}`
    )
  }
  if (loginResult.data?.require_2fa) {
    throw new Error(`${credentials.name} requires two-factor authentication`)
  }

  process.stdout.write(`${credentials.name}: waiting for browser auth state\n`)
  await page.waitForFunction(
    () =>
      Boolean(
        window.localStorage.getItem('uid') &&
        window.localStorage.getItem('user')
      ),
    undefined,
    { timeout: 15_000 }
  )
  const userId = await page.evaluate(
    () => window.localStorage.getItem('uid') || ''
  )
  await verifySession(context, baseURL, userId)
  await context.storageState({ path: credentials.outputFile })
  process.stdout.write(`${credentials.name}: storage state verified\n`)
  await page.close()
}

const baseURL = (process.env.E2E_BASE_URL || 'http://127.0.0.1:3026').replace(
  /\/$/,
  ''
)
const outputDirectory = resolve(
  process.env.E2E_AUTH_OUTPUT_DIR || '../../../.verification-output/auth'
)
const roles = [
  {
    name: 'root',
    outputFile: resolve(outputDirectory, 'root.json'),
    username: requireEnvironment('E2E_ROOT_USERNAME'),
    password: requireEnvironment('E2E_ROOT_PASSWORD'),
  },
  {
    name: 'administrator',
    outputFile: resolve(outputDirectory, 'admin.json'),
    username: requireEnvironment('E2E_ADMIN_USERNAME'),
    password: requireEnvironment('E2E_ADMIN_PASSWORD'),
  },
  {
    name: 'user',
    outputFile: resolve(outputDirectory, 'user.json'),
    username: requireEnvironment('E2E_USER_USERNAME'),
    password: requireEnvironment('E2E_USER_PASSWORD'),
  },
]

await mkdir(outputDirectory, { recursive: true })
const browser = await chromium.launch({
  channel: process.env.PLAYWRIGHT_CHANNEL || 'msedge',
  headless: true,
})

try {
  for (const role of roles) {
    const context = await browser.newContext({ baseURL, locale: 'en-US' })
    context.setDefaultTimeout(15_000)
    context.setDefaultNavigationTimeout(20_000)
    try {
      await createStorageState(context, baseURL, role)
      process.stdout.write(`created ${role.name} storage state\n`)
    } finally {
      await context.close()
    }
  }
} finally {
  await browser.close()
}
