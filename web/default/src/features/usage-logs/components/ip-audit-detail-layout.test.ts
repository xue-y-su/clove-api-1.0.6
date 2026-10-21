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

test('renders IP details in one responsive sheet outside table rows and cards', async () => {
  const source = await Bun.file(
    new URL('./ip-audit-table.tsx', import.meta.url)
  ).text()

  expect(source).toContain('open={selectedIP !== null}')
  expect(source).toContain("className='w-full sm:max-w-2xl'")
  expect(source.match(/<IPAuditDetailPanel/g)).toHaveLength(1)
  expect(source).not.toContain('detailLoading: boolean')
  expect(source).not.toContain('expandedIP')
})
