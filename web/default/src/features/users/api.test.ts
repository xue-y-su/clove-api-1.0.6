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
import { afterEach, describe, expect, mock, test } from 'bun:test'

import { api } from '@/lib/api'

import { getUsers, searchUsers } from './api'

const originalGet = api.get

afterEach(() => {
  api.get = originalGet
})

describe('user list API sorting', () => {
  test('passes server sorting with the paginated list request', async () => {
    const get = mock(async () => ({ data: { success: true } }))
    api.get = get as typeof api.get

    await getUsers({
      p: 3,
      page_size: 20,
      sort_by: 'username',
      sort_order: 'asc',
    })

    expect(get).toHaveBeenCalledWith('/api/user/', {
      params: {
        p: 3,
        page_size: 20,
        sort_by: 'username',
        sort_order: 'asc',
      },
    })
  })

  test('passes server sorting with filtered user search', async () => {
    const get = mock(async () => ({ data: { success: true } }))
    api.get = get as typeof api.get

    await searchUsers({
      keyword: 'alice',
      group: 'default',
      p: 2,
      page_size: 10,
      sort_by: 'quota',
      sort_order: 'desc',
    })

    const [url] = get.mock.calls[0]
    const query = new URL(url as string, 'http://localhost').searchParams
    expect(query.get('keyword')).toBe('alice')
    expect(query.get('group')).toBe('default')
    expect(query.get('p')).toBe('2')
    expect(query.get('page_size')).toBe('10')
    expect(query.get('sort_by')).toBe('quota')
    expect(query.get('sort_order')).toBe('desc')
  })
})
