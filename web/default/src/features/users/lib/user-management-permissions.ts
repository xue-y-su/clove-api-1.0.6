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
import { USER_ROLE } from '../constants'

type UserRoleIdentity = {
  id: number
  role: number
}

export function getUserManagementPermissions(
  currentUser: UserRoleIdentity | null | undefined,
  targetUser: UserRoleIdentity
) {
  const currentIsRoot = currentUser?.role === USER_ROLE.ROOT
  const currentIsPrimaryRoot = currentIsRoot && currentUser?.id === 1
  const targetIsRoot = targetUser.role === USER_ROLE.ROOT
  const targetIsAdmin = targetUser.role === USER_ROLE.ADMIN
  const canManage = Boolean(
    currentUser &&
    (currentIsPrimaryRoot ||
      (currentUser.id !== targetUser.id &&
        currentUser.role >= USER_ROLE.ADMIN &&
        currentUser.role > targetUser.role))
  )
  const canViewSecurityAudit = Boolean(
    currentUser &&
    (currentIsPrimaryRoot ||
      (currentUser.id !== targetUser.id &&
        currentUser.role >= USER_ROLE.ADMIN &&
        currentUser.role > targetUser.role))
  )

  return {
    canManage,
    canViewSecurityAudit,
    canPromoteAdmin:
      canManage && currentIsRoot && targetUser.role < USER_ROLE.ADMIN,
    canDemoteAdmin: canManage && currentIsRoot && targetIsAdmin,
    canManageRootRole: Boolean(currentIsPrimaryRoot),
    canPromoteRoot: canManage && Boolean(currentIsPrimaryRoot) && !targetIsRoot,
    canDemoteRoot:
      canManage &&
      Boolean(currentIsPrimaryRoot) &&
      targetIsRoot &&
      targetUser.id !== 1,
  }
}
