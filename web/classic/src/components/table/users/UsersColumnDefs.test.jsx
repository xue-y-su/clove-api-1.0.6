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
import { describe, expect, test } from 'bun:test';

import * as userRoleActions from './user-role-actions';

describe('classic user role actions', () => {
  test('matches backend Root role authorization rules', () => {
    const getActions = userRoleActions.getUserRoleActions;
    const cases = [
      {
        current: { id: 1, role: 100 },
        target: { id: 1, role: 100 },
        expected: [],
      },
      {
        current: { id: 1, role: 100 },
        target: { id: 2, role: 100 },
        expected: ['demote_root'],
      },
      {
        current: { id: 1, role: 100 },
        target: { id: 3, role: 10 },
        expected: ['demote', 'promote_root'],
      },
      {
        current: { id: 1, role: 100 },
        target: { id: 4, role: 1 },
        expected: ['promote', 'promote_root'],
      },
      {
        current: { id: 2, role: 100 },
        target: { id: 3, role: 10 },
        expected: ['demote'],
      },
      {
        current: { id: 2, role: 100 },
        target: { id: 4, role: 1 },
        expected: ['promote'],
      },
      {
        current: { id: 5, role: 10 },
        target: { id: 4, role: 1 },
        expected: [],
      },
    ];

    const actual =
      typeof getActions === 'function'
        ? cases.map(({ current, target }) => getActions(target, current))
        : null;

    expect(actual).toEqual(cases.map(({ expected }) => expected));
  });

  test('matches backend object authorization for security audit reads', () => {
    const canView = userRoleActions.canViewUserSecurityAudit;
    const cases = [
      [{ id: 1, role: 100 }, { id: 1, role: 100 }, true],
      [{ id: 1, role: 100 }, { id: 2, role: 100 }, true],
      [{ id: 2, role: 100 }, { id: 3, role: 10 }, true],
      [{ id: 2, role: 100 }, { id: 4, role: 1 }, true],
      [{ id: 2, role: 100 }, { id: 2, role: 100 }, false],
      [{ id: 2, role: 100 }, { id: 5, role: 100 }, false],
      [{ id: 5, role: 10 }, { id: 4, role: 1 }, true],
      [{ id: 5, role: 10 }, { id: 6, role: 10 }, false],
      [{ id: 7, role: 1 }, { id: 4, role: 1 }, false],
    ];

    expect(
      typeof canView === 'function'
        ? cases.map(([actor, target]) => canView(target, actor))
        : null,
    ).toEqual(cases.map(([, , expected]) => expected));
  });
});
