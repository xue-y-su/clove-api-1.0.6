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
import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

test('loads known Lobe icons lazily and keeps unknown keys local', async () => {
  const source = readFileSync(
    fileURLToPath(new URL('./lobe-icon.tsx', import.meta.url)),
    'utf8'
  )
  const componentPath = fileURLToPath(
    new URL('./lobe-icon-component.tsx', import.meta.url)
  )

  expect(source).toContain("import { LobeIcon } from './lobe-icon-component'")
  expect(existsSync(componentPath)).toBe(true)
  const componentSource = readFileSync(componentPath, 'utf8')

  expect(source).not.toContain("import * as LobeIcons from '@lobehub/icons'")
  expect(source).toContain('LobeIcon')
  expect(componentSource).toContain('loadLobeIconModule')

  const modulePath = './lobe-icon-loaders'
  const loaders = await import(modulePath)
  const loaderSource = readFileSync(
    fileURLToPath(new URL('./lobe-icon-loaders.ts', import.meta.url)),
    'utf8'
  )
  expect(loaderSource).toContain('/components/Mono.js')
  expect(loaderSource).toContain('/components/Color.js')
  expect(loaderSource).not.toContain('/components/Avatar.js')
  expect(loaderSource).not.toContain("import('@lobehub/icons/es/OpenAI')")
  expect(componentSource).toContain('LobeAvatar')
  expect(loaders.resolveLobeIconLoader('OpenAI')).toBe(
    loaders.lobeIconLoaders.OpenAI
  )
  expect(loaders.resolveLobeIconLoader('Claude')).toBe(
    loaders.lobeIconLoaders.Claude
  )
  expect(loaders.resolveLobeIconLoader('UnknownVendor')).toBeUndefined()

  const first = loaders.loadLobeIconModule('OpenAI')
  const second = loaders.loadLobeIconModule('OpenAI')
  expect(first).toBe(second)
  await expect(first).resolves.toHaveProperty('default')
  await expect(
    loaders.loadLobeIconModule('Claude', 'Color')
  ).resolves.toHaveProperty('default')
  await expect(
    loaders.loadLobeIconModule('OpenAI', 'Avatar')
  ).resolves.toHaveProperty('default')
  expect(loaders.loadLobeIconModule('UnknownVendor')).toBeUndefined()
})
