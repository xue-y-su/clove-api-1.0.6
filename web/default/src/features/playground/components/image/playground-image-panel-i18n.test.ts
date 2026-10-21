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

test('translates image quality, style, and background option labels', () => {
  const source = readFileSync(
    new URL('./playground-image-panel.tsx', import.meta.url),
    'utf8'
  )

  expect(source).toContain('IMAGE_OPTION_LABELS')
  expect(source).toContain('t(getImageOptionLabel(value))')
  expect(source).not.toContain("<SelectItem value='auto'>auto</SelectItem>")
  expect(source).not.toContain("<SelectItem value='opaque'>opaque</SelectItem>")
  expect(source).not.toContain(
    "<SelectItem value='transparent'>transparent</SelectItem>"
  )
  expect(source).not.toContain('outputFormat')
  expect(source).not.toContain('b64_json')
})

test('loads and searches only models available to the selected group', () => {
  const apiSource = readFileSync(
    new URL('../../api.ts', import.meta.url),
    'utf8'
  )
  const panelSource = readFileSync(
    new URL('./playground-image-panel.tsx', import.meta.url),
    'utf8'
  )

  expect(apiSource).not.toContain("endpoint_type: 'image-generation'")
  expect(apiSource).toContain('return getUserModels(group, signal)')
  expect(panelSource).toMatch(
    /getUserImageModels\(\s*group,\s*abortController\.signal\s*\)/
  )
  expect(panelSource).toContain('modelAbortRef.current?.abort()')
  expect(panelSource).toContain('<Combobox')
  expect(panelSource).toContain("searchPlaceholder={t('Search models...')}")
  expect(panelSource).not.toContain('No routable image generation models')
  expect(panelSource).toContain('No models are available for this group.')
})

test('uses the shared clipboard feedback flow for image URLs', () => {
  const source = readFileSync(
    new URL('./playground-image-panel.tsx', import.meta.url),
    'utf8'
  )

  expect(source).toContain('useCopyToClipboard()')
  expect(source).toContain('copyToClipboard(url)')
  expect(source).not.toContain('navigator.clipboard')
})

test('keeps image failures visible with a retry action', () => {
  const source = readFileSync(
    new URL('./playground-image-panel.tsx', import.meta.url),
    'utf8'
  )

  expect(source).toContain('generationError')
  expect(source).toContain('getImageGenerationErrorMessage')
  expect(source).toContain("t('Retry')")
})
