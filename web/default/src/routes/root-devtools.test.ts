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

test('loads TanStack development tools only through an explicitly enabled development lazy chunk', () => {
  const rootSource = readFileSync(
    fileURLToPath(new URL('./__root.tsx', import.meta.url)),
    'utf8'
  )

  expect(rootSource).not.toContain(
    "import { ReactQueryDevtools } from '@tanstack/react-query-devtools'"
  )
  expect(rootSource).not.toContain(
    "import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'"
  )
  expect(rootSource).toContain('import.meta.env.DEV')
  expect(rootSource).toContain(
    "import.meta.env.VITE_ENABLE_DEVTOOLS === 'true'"
  )
  expect(rootSource).toContain("lazy(() => import('@/components/devtools'))")

  const devtoolsSource = readFileSync(
    fileURLToPath(new URL('../components/devtools.tsx', import.meta.url)),
    'utf8'
  )
  expect(devtoolsSource).toContain('@tanstack/react-query-devtools')
  expect(devtoolsSource).toContain('@tanstack/react-router-devtools')
  expect(devtoolsSource).toContain('<ReactQueryDevtools')
  expect(devtoolsSource).toContain('<TanStackRouterDevtools')
})
