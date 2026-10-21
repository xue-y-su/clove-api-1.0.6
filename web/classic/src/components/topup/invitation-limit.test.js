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

import { getInvitationLimitMetrics } from './invitation-limit';

describe('classic wallet invitation limit metrics', () => {
  test('shows finite zero limits without treating them as unlimited', () => {
    expect(
      getInvitationLimitMetrics({
        aff_count: 3,
        invite_limit: {
          unlimited: false,
          exempt: false,
          limit: 0,
          used: 3,
          remaining: 0,
        },
      }),
    ).toEqual({ used: 3, limit: '0', remaining: '0', exempt: false });
  });

  test('labels explicit unlimited limits', () => {
    expect(
      getInvitationLimitMetrics(
        { invite_limit: { unlimited: true, used: 2 } },
        'Unlimited',
      ),
    ).toEqual({
      used: 2,
      limit: 'Unlimited',
      remaining: 'Unlimited',
      exempt: false,
    });
  });
});
