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

test('keeps notification and timeline panels mounted while switching tabs', () => {
  const source = readFileSync(
    new URL('./notification-popover.tsx', import.meta.url),
    'utf8'
  )

  expect(source).toContain("className='mt-2 min-h-[min(52vh,28rem)]'")
  expect(source).toContain("hidden={activeTab !== 'notice'}")
  expect(source).toContain("hidden={activeTab !== 'announcements'}")
  expect(source).not.toContain('<TabsContent')
})
