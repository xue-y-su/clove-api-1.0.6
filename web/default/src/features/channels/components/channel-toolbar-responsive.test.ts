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

test('allows the channel toolbar to wrap cleanly at tablet width', async () => {
  const channelSource = await Bun.file(
    new URL('./channels-table.tsx', import.meta.url)
  ).text()
  const toolbarSource = await Bun.file(
    new URL(
      '../../../components/data-table/toolbar/toolbar.tsx',
      import.meta.url
    )
  ).text()

  expect(channelSource).toContain("className: 'min-w-0 max-lg:items-stretch'")
  expect(channelSource).toContain("className='w-full min-w-0")
  expect(toolbarSource).toContain('max-w-full')
  expect(toolbarSource).toContain('shrink-0')
  expect(toolbarSource).toContain('flex-wrap')
  expect(toolbarSource).toContain('items-center')
  expect(toolbarSource).toContain('justify-end')
})
