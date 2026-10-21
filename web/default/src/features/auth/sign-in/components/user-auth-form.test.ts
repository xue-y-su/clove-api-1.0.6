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

test('password login rejects duplicate submissions while one is in flight', async () => {
  const source = await Bun.file(
    new URL('./user-auth-form.tsx', import.meta.url)
  ).text()
  const submitSource = source.slice(
    source.indexOf('async function onSubmit'),
    source.indexOf('\n  const handleOpenWeChatDialog')
  )

  expect(source).toContain('useRef')
  expect(source).toContain('passwordLoginInFlight')
  expect(submitSource.indexOf('passwordLoginInFlight.current')).toBeLessThan(
    submitSource.indexOf('await login(')
  )
  expect(submitSource).toContain('passwordLoginInFlight.current = true')
  expect(submitSource).toContain('passwordLoginInFlight.current = false')
})
