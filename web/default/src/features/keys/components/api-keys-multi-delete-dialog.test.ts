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

test('closes the bulk-delete dialog before selection reset unmounts it', async () => {
  const source = await Bun.file(
    new URL('./api-keys-multi-delete-dialog.tsx', import.meta.url)
  ).text()
  const closeIndex = source.indexOf('onOpenChange(false)')
  const resetIndex = source.indexOf('table.resetRowSelection()')

  expect(closeIndex).toBeGreaterThan(-1)
  expect(resetIndex).toBeGreaterThan(-1)
  expect(closeIndex).toBeLessThan(resetIndex)
})
