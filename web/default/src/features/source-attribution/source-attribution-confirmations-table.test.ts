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

test('keeps source attribution controls visible while records scroll vertically', async () => {
  const source = await Bun.file(
    new URL('./source-attribution-confirmations-table.tsx', import.meta.url)
  ).text()

  expect(source).toContain("className='flex h-full min-h-0 flex-col gap-4'")
  expect(source).toContain("className='grid shrink-0 gap-3 md:grid-cols-4'")
  expect(source).toContain(
    "className='min-h-0 flex-1 overflow-y-auto rounded-md border'"
  )
  expect(source).toContain(
    "className='flex shrink-0 items-center justify-between gap-3'"
  )
})
