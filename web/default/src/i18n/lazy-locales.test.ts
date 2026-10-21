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

test('loads locale dictionaries on demand before mounting React', async () => {
  const configSource = readFileSync(
    fileURLToPath(new URL('./config.ts', import.meta.url)),
    'utf8'
  )
  const mainSource = readFileSync(
    fileURLToPath(new URL('../main.tsx', import.meta.url)),
    'utf8'
  )

  expect(configSource).not.toContain("import en from './locales/en.json'")
  expect(configSource).not.toContain("import zhCN from './locales/zh.json'")
  expect(configSource).toContain('lazyLocaleBackend')
  expect(configSource).toContain('export const i18nReady')
  expect(mainSource).toContain("import { i18nReady } from './i18n/config'")
  expect(mainSource).toContain('void i18nReady.then')

  const modulePath = './locale-loaders'
  const loaders = await import(modulePath)
  const first = loaders.loadLocaleResources('en')
  const second = loaders.loadLocaleResources('en')
  expect(first).toBe(second)
  await expect(first).resolves.toHaveProperty('Sign in')
  await expect(loaders.loadLocaleResources('zhCN')).resolves.toHaveProperty(
    'Sign in'
  )
  expect(loaders.loadLocaleResources('unknown')).toBeUndefined()
})
