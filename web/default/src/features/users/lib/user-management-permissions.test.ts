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

import { USER_ROLE } from '../constants'
import { getUserManagementPermissions } from './user-management-permissions'

describe('user row management permissions', () => {
  test('regular administrators cannot manage peer administrators or Root users', () => {
    const currentUser = { id: 8, role: USER_ROLE.ADMIN }

    expect(
      getUserManagementPermissions(currentUser, {
        id: 9,
        role: USER_ROLE.ADMIN,
      }).canManage
    ).toBe(false)
    expect(
      getUserManagementPermissions(currentUser, {
        id: 1,
        role: USER_ROLE.ROOT,
      }).canManage
    ).toBe(false)
  })

  test('regular administrators can manage common users but cannot promote them', () => {
    const permissions = getUserManagementPermissions(
      { id: 8, role: USER_ROLE.ADMIN },
      { id: 9, role: USER_ROLE.USER }
    )

    expect(permissions.canManage).toBe(true)
    expect(permissions.canPromoteAdmin).toBe(false)
    expect(permissions.canManageRootRole).toBe(false)
  })

  test('non-primary Root users can manage lower roles without changing Root roles', () => {
    const permissions = getUserManagementPermissions(
      { id: 2, role: USER_ROLE.ROOT },
      { id: 9, role: USER_ROLE.ADMIN }
    )

    expect(permissions.canManage).toBe(true)
    expect(permissions.canDemoteAdmin).toBe(true)
    expect(permissions.canManageRootRole).toBe(false)
  })

  test('non-primary Root users cannot manage themselves or peer Root users', () => {
    const currentUser = { id: 2, role: USER_ROLE.ROOT }

    expect(
      getUserManagementPermissions(currentUser, {
        id: 2,
        role: USER_ROLE.ROOT,
      }).canManage
    ).toBe(false)
    expect(
      getUserManagementPermissions(currentUser, {
        id: 3,
        role: USER_ROLE.ROOT,
      }).canManage
    ).toBe(false)
  })

  test('only the primary Root can promote or demote Root users', () => {
    expect(
      getUserManagementPermissions(
        { id: 1, role: USER_ROLE.ROOT },
        { id: 9, role: USER_ROLE.ADMIN }
      ).canPromoteRoot
    ).toBe(true)
    expect(
      getUserManagementPermissions(
        { id: 1, role: USER_ROLE.ROOT },
        { id: 2, role: USER_ROLE.ROOT }
      ).canDemoteRoot
    ).toBe(true)
    expect(
      getUserManagementPermissions(
        { id: 2, role: USER_ROLE.ROOT },
        { id: 3, role: USER_ROLE.ROOT }
      ).canDemoteRoot
    ).toBe(false)
  })

  test('matches backend object authorization for security audit reads', () => {
    expect(
      getUserManagementPermissions(
        { id: 8, role: USER_ROLE.ADMIN },
        { id: 9, role: USER_ROLE.USER }
      ).canViewSecurityAudit
    ).toBe(true)
    expect(
      getUserManagementPermissions(
        { id: 8, role: USER_ROLE.ADMIN },
        { id: 8, role: USER_ROLE.ADMIN }
      ).canViewSecurityAudit
    ).toBe(false)
    expect(
      getUserManagementPermissions(
        { id: 2, role: USER_ROLE.ROOT },
        { id: 3, role: USER_ROLE.ROOT }
      ).canViewSecurityAudit
    ).toBe(false)
    expect(
      getUserManagementPermissions(
        { id: 2, role: USER_ROLE.ROOT },
        { id: 2, role: USER_ROLE.ROOT }
      ).canViewSecurityAudit
    ).toBe(false)
    expect(
      getUserManagementPermissions(
        { id: 1, role: USER_ROLE.ROOT },
        { id: 1, role: USER_ROLE.ROOT }
      ).canViewSecurityAudit
    ).toBe(true)
    expect(
      getUserManagementPermissions(
        { id: 1, role: USER_ROLE.ROOT },
        { id: 3, role: USER_ROLE.ROOT }
      ).canViewSecurityAudit
    ).toBe(true)
  })

  test('renders the security audit action once and only behind its permission gate', async () => {
    const source = await Bun.file(
      new URL('../components/data-table-row-actions.tsx', import.meta.url)
    ).text()

    expect(source.match(/t\('Security & IP Audit'\)/g)).toHaveLength(1)
    expect(source).toContain('permissions.canViewSecurityAudit ?')
  })
})
