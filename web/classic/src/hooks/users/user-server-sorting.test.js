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

import {
  DEFAULT_USER_SORT,
  getSemiUserSortOrder,
  resolveUserTableSort,
} from './user-server-sorting';

describe('classic user server sorting', () => {
  test('maps a supported Semi table sort to API parameters', () => {
    expect(
      resolveUserTableSort({
        sorter: { dataIndex: 'quota', sortOrder: 'ascend' },
      }),
    ).toEqual({ sortBy: 'quota', sortOrder: 'asc' });
  });

  test('falls back to deterministic id ordering for unsupported values', () => {
    expect(
      resolveUserTableSort({
        sorter: {
          dataIndex: 'quota desc; drop table users',
          sortOrder: 'ascend',
        },
      }),
    ).toEqual(DEFAULT_USER_SORT);
    expect(resolveUserTableSort({ sorter: {} })).toEqual(DEFAULT_USER_SORT);
  });

  test('keeps only the active column controlled', () => {
    const sorting = { sortBy: 'username', sortOrder: 'desc' };
    expect(getSemiUserSortOrder(sorting, 'username')).toBe('descend');
    expect(getSemiUserSortOrder(sorting, 'quota')).toBe(false);
  });
});
