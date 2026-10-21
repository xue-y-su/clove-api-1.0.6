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

test('keeps the group picker usable in short viewports', async () => {
  const source = await Bun.file(
    new URL('./api-key-group-combobox.tsx', import.meta.url)
  ).text()

  expect(source).toContain('collisionPadding={12}')
  expect(source).toContain('calc(100svh-10rem)')
  expect(source).toContain('max-w-[calc(100vw-24px)]')
  expect(source).toContain('min-w-0')
  expect(source).not.toContain('min-w-[var(--anchor-width)]')
})

test('offers the same group editor in mobile API key cards', async () => {
  const source = await Bun.file(
    new URL('./api-keys-table.tsx', import.meta.url)
  ).text()

  expect(source).toContain('groupOptions: ApiKeyGroupOption[]')
  expect(source).toContain(
    '<ApiKeyGroupEditor apiKey={apiKey} options={groupOptions} />'
  )
})
