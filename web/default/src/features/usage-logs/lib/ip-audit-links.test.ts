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
import { describe, expect, test } from 'vitest'

import { buildIPThreatURL, buildUsageLogSearch } from './ip-audit-links'

describe('IP audit links', () => {
  test('builds a Baidu threat lookup only for authorized public addresses', () => {
    expect(buildIPThreatURL('8.8.8.8', true)).toBe(
      'https://cloud.baidu.com/product-s/afd_s/ip-threat.html?s=8.8.8.8'
    )
    expect(buildIPThreatURL('2001:4860:4860::8888', true)).toContain(
      encodeURIComponent('2001:4860:4860::8888')
    )
    expect(buildIPThreatURL('127.0.0.1', false)).toBeNull()
    expect(buildIPThreatURL('', true)).toBeNull()
  })

  test('turns timeline request identifiers into usage log search state', () => {
    expect(buildUsageLogSearch('req-401', 1_720_000_000)).toEqual({
      requestId: 'req-401',
      startTime: 1_719_999_700_000,
      endTime: 1_720_000_300_000,
    })
    expect(buildUsageLogSearch('   ')).toEqual({})
  })
})
