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
import * as dashboardTimeRange from './dashboard-time-range';

describe('dashboard initial time range', () => {
  test.each([
    ['hour', 24 * 60 * 60],
    ['day', 7 * 24 * 60 * 60],
    ['week', 30 * 24 * 60 * 60],
  ])('%s range ends now and stays within the API limit', (mode, seconds) => {
    const now = 1_800_000_000;
    const range = dashboardTimeRange.getInitialDashboardTimestamps?.(mode, now);

    expect(range.endTimestamp).toBe(now);
    expect(range.endTimestamp - range.startTimestamp).toBe(seconds);
  });
});
