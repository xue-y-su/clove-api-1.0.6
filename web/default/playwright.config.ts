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
import { defineConfig } from 'playwright/test'

export default defineConfig({
  expect: {
    timeout: 8_000,
  },
  forbidOnly: Boolean(process.env.CI),
  fullyParallel: false,
  outputDir: '../../.verification-output/playwright',
  projects: [
    {
      name: 'chromium',
      use: {
        channel: process.env.PLAYWRIGHT_CHANNEL || 'msedge',
      },
    },
  ],
  reporter: [['line']],
  retries: process.env.CI ? 1 : 0,
  testDir: './e2e',
  timeout: 40_000,
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://127.0.0.1:3026',
    colorScheme: 'light',
    headless: true,
    locale: 'en-US',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    video: 'off',
  },
  workers: 1,
})
