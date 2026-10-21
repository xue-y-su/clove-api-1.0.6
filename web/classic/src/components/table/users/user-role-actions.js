/*
Copyright (C) 2025 QuantumNous

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
const ROLE_ADMIN = 10;
const ROLE_ROOT = 100;

export function getUserRoleActions(targetUser, currentUser) {
  if (!targetUser || !currentUser || targetUser.id === currentUser.id) {
    return [];
  }

  const actions = [];
  const currentIsRoot = currentUser.role >= ROLE_ROOT;
  const currentIsPrimaryRoot = currentUser.id === 1 && currentIsRoot;
  const targetIsRoot = targetUser.role >= ROLE_ROOT;
  const targetIsAdmin = targetUser.role >= ROLE_ADMIN;

  if (currentIsRoot && !targetIsRoot) {
    actions.push(targetIsAdmin ? 'demote' : 'promote');
  }
  if (currentIsPrimaryRoot) {
    actions.push(targetIsRoot ? 'demote_root' : 'promote_root');
  }

  return actions;
}

export function canViewUserSecurityAudit(targetUser, currentUser) {
  if (!targetUser || !currentUser) {
    return false;
  }
  if (currentUser.id === 1 && currentUser.role === ROLE_ROOT) {
    return true;
  }
  if (currentUser.id === targetUser.id || currentUser.role < ROLE_ADMIN) {
    return false;
  }
  return currentUser.role > targetUser.role;
}
