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
  buildUserEditPayload,
  normalizeUserEditValues,
} from './user-edit-values';

describe('classic user invitation override values', () => {
  test('preserves an explicit zero-person limit', () => {
    expect(
      normalizeUserEditValues({
        username: 'alice',
        invite_limit_mode: 'limited',
        invite_limit_count: 0,
      }),
    ).toMatchObject({
      username: 'alice',
      invite_limit_mode: 'limited',
      invite_limit_count: 0,
    });
  });

  test('includes the per-user override in update payloads', () => {
    expect(
      buildUserEditPayload(
        {
          username: 'alice',
          quota: 100,
          quota_amount: 1,
          invite_limit_mode: 'unlimited',
          invite_limit_count: 9,
        },
        7,
      ),
    ).toEqual({
      id: 7,
      username: 'alice',
      invite_limit_mode: 'unlimited',
      invite_limit_count: 9,
    });
  });

  test('keeps username edits in update payloads', () => {
    expect(
      buildUserEditPayload(
        {
          username: 'renamed-user',
          display_name: 'Renamed User',
          quota: 100,
          quota_amount: 1,
        },
        8,
      ),
    ).toMatchObject({
      id: 8,
      username: 'renamed-user',
      display_name: 'Renamed User',
    });
  });
});
