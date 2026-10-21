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
import { expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import createConfig from './rsbuild.config'

test('serves client-side routes through the development index', async () => {
  const config = await createConfig({ envMode: 'development' } as never)

  expect(config.server?.historyApiFallback).toBe(true)
})

test('proxies Snow home assets to the backend', async () => {
  const config = await createConfig({ envMode: 'development' } as never)

  expect(config.server?.proxy).toHaveProperty('/snow-assets')
})

test.each(['development', 'production'] as const)(
  'enables route splitting in %s builds',
  async (envMode) => {
    await createConfig({ envMode } as never)
    const source = readFileSync(
      fileURLToPath(new URL('./rsbuild.config.ts', import.meta.url)),
      'utf8'
    )

    expect(source).toContain('autoCodeSplitting: true')
  }
)

test('excludes colocated tests from route generation', () => {
  const source = readFileSync(
    fileURLToPath(new URL('./rsbuild.config.ts', import.meta.url)),
    'utf8'
  )

  expect(source).toContain("routeFileIgnorePattern: '\\\\.test\\\\.'")
})

test('keeps Playwright specs outside the Bun unit-test command', () => {
  const packageJson = JSON.parse(
    readFileSync(
      fileURLToPath(new URL('./package.json', import.meta.url)),
      'utf8'
    )
  ) as { scripts?: Record<string, string> }

  expect(packageJson.scripts?.test).toBe(
    'bun test ./src ./rsbuild.config.test.ts'
  )
})
