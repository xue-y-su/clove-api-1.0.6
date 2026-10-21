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
import { describe, expect, test } from 'bun:test'

import * as sidebarConfig from './use-sidebar-config'
import * as sidebarData from './use-sidebar-data'

type SidebarConfigExports = {
  getSidebarModuleMapping?: (
    url: string
  ) => { section: string; module: string } | undefined
}

type SidebarDataExports = {
  SECURITY_AUDIT_ROUTE?: string
  USAGE_LOG_TASK_ROUTES?: readonly string[]
}

describe('IP audit sidebar mapping', () => {
  test('maps the standalone security audit page to admin user governance', () => {
    expect(
      (sidebarConfig as SidebarConfigExports).getSidebarModuleMapping?.(
        '/security-audit'
      )
    ).toEqual({ section: 'admin', module: 'user' })
  })

  test('keeps security audit outside the Task Logs route family', () => {
    expect((sidebarData as SidebarDataExports).SECURITY_AUDIT_ROUTE).toBe(
      '/security-audit'
    )
    expect(
      (sidebarData as SidebarDataExports).USAGE_LOG_TASK_ROUTES
    ).not.toContain('/usage-logs/ip-audit')
    expect(
      (sidebarData as SidebarDataExports).USAGE_LOG_TASK_ROUTES
    ).not.toContain('/security-audit')
  })

  test('renders security audit as its own admin navigation item', async () => {
    const source = await Bun.file(
      new URL('./use-sidebar-data.ts', import.meta.url)
    ).text()

    expect(source).toMatch(
      /title:\s*t\('Security & IP Audit'\)[\s\S]{0,160}url:\s*SECURITY_AUDIT_ROUTE/
    )
  })

  test('uses a standalone admin route and redirects the legacy log URL', async () => {
    const routeSource = await Bun.file(
      new URL(
        '../routes/_authenticated/security-audit/index.tsx',
        import.meta.url
      )
    ).text()
    const legacyRouteSource = await Bun.file(
      new URL(
        '../routes/_authenticated/usage-logs/$section.tsx',
        import.meta.url
      )
    ).text()
    const tableSource = await Bun.file(
      new URL(
        '../features/usage-logs/components/ip-audit-table.tsx',
        import.meta.url
      )
    ).text()

    expect(routeSource).toContain(
      "createFileRoute('/_authenticated/security-audit/')"
    )
    expect(routeSource).toContain('auth.user.role < ROLE.ADMIN')
    expect(legacyRouteSource).toMatch(
      /params\.section === 'ip-audit'[\s\S]{0,300}to: '\/security-audit'/
    )
    expect(tableSource).toContain(
      "getRouteApi('/_authenticated/security-audit/')"
    )
    expect(tableSource).toMatch(/to: '\/security-audit'/)
  })
})
