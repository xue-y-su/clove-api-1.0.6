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

import type { TFunction } from 'i18next'

import { api } from '@/lib/api'

import { createApiKey, updateApiKey } from '../api'
import {
  getApiKeyFormSchema,
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
  type ApiKeyFormValues,
} from './api-key-form'

const t = ((key: string) => key) as TFunction
const originalPost = api.post
const originalPut = api.put

afterEach(() => {
  api.post = originalPost
  api.put = originalPut
})

const baseFormValues: ApiKeyFormValues = {
  name: 'custom-key',
  custom_key: '',
  remain_quota_dollars: 10,
  expired_time: undefined,
  unlimited_quota: true,
  model_limits: [],
  allow_ips: '',
  group: 'default',
  auto_groups_mode: 'inherit',
  auto_groups: [],
  cross_group_retry: false,
  tokenCount: 1,
}

describe('api key form custom key', () => {
  test('includes a trimmed custom key when provided', () => {
    expect(
      transformFormDataToPayload({
        ...baseFormValues,
        custom_key: '  custom_key-123456  ',
      })
    ).toMatchObject({
      name: 'custom-key',
      custom_key: 'custom_key-123456',
    })
  })

  test('omits custom key when the field is blank', () => {
    expect(
      transformFormDataToPayload({
        ...baseFormValues,
        custom_key: '   ',
      })
    ).not.toHaveProperty('custom_key')
  })

  test('omits custom key for batch creation', () => {
    expect(
      transformFormDataToPayload({
        ...baseFormValues,
        tokenCount: 2,
        custom_key: 'custom_key-123456',
      })
    ).not.toHaveProperty('custom_key')
  })

  test('omits custom key when creation is disabled by system policy', () => {
    expect(
      transformFormDataToPayload(
        {
          ...baseFormValues,
          custom_key: 'custom_key-123456',
        },
        false
      )
    ).not.toHaveProperty('custom_key')
  })

  test('ignores stale custom key validation for batch creation', () => {
    const schema = getApiKeyFormSchema(t)

    expect(
      schema.safeParse({
        ...baseFormValues,
        tokenCount: 2,
        custom_key: 'short-key',
      }).success
    ).toBe(true)
  })

  test('rejects invalid custom key values', () => {
    const schema = getApiKeyFormSchema(t)

    expect(
      schema.safeParse({
        ...baseFormValues,
        custom_key: 'bad key with spaces',
      }).success
    ).toBe(false)

    expect(
      schema.safeParse({
        ...baseFormValues,
        custom_key: 'short-key',
      }).success
    ).toBe(false)
  })
})

describe('api key mutation error ownership', () => {
  test('leaves business failure notifications to the form', async () => {
    const post = mock(async () => ({
      data: { success: false, message: 'custom key unavailable' },
    }))
    const put = mock(async () => ({
      data: { success: false, message: 'custom key unavailable' },
    }))
    api.post = post as typeof api.post
    api.put = put as typeof api.put
    const payload = transformFormDataToPayload(baseFormValues)

    await createApiKey(payload)
    await updateApiKey({ ...payload, id: 7 })

    expect(post).toHaveBeenCalledWith('/api/token/', payload, {
      skipBusinessError: true,
    })
    expect(put).toHaveBeenCalledWith(
      '/api/token/',
      { ...payload, id: 7 },
      {
        skipBusinessError: true,
      }
    )
  })
})

describe('api key Auto group form', () => {
  test('submits inheritance and custom snapshots distinctly', () => {
    const inherited = {
      ...baseFormValues,
      group: 'auto',
      cross_group_retry: true,
    }
    expect(transformFormDataToPayload(inherited).auto_groups).toEqual([])

    expect(
      transformFormDataToPayload({
        ...inherited,
        auto_groups_mode: 'custom',
        auto_groups: ['vip', 'default'],
      }).auto_groups
    ).toEqual(['vip', 'default'])
  })

  test('keeps a fully filtered stored snapshot custom until resolved', () => {
    const defaults = transformApiKeyToFormDefaults(
      {
        id: 1,
        name: 'filtered',
        key: 'sk-filtered',
        status: 1,
        status_reason: '',
        remain_quota: 0,
        used_quota: 0,
        unlimited_quota: true,
        expired_time: -1,
        created_time: 1,
        accessed_time: 1,
        group: 'auto',
        auto_groups: ['revoked'],
        cross_group_retry: true,
        model_limits_enabled: false,
        model_limits: '',
        allow_ips: '',
      },
      ['default'],
      2
    )

    expect(defaults.auto_groups_mode).toBe('custom')
    expect(defaults.auto_groups).toEqual([])
    expect(getApiKeyFormSchema(t, 2).safeParse(defaults).success).toBe(false)
  })

  test('rejects duplicate and over-limit custom snapshots', () => {
    const schema = getApiKeyFormSchema(t, 1)
    expect(
      schema.safeParse({
        ...baseFormValues,
        group: 'auto',
        auto_groups_mode: 'custom',
        auto_groups: ['vip', 'vip'],
      }).success
    ).toBe(false)
    expect(
      schema.safeParse({
        ...baseFormValues,
        group: 'auto',
        auto_groups_mode: 'custom',
        auto_groups: ['vip', 'default'],
      }).success
    ).toBe(false)
  })
})
