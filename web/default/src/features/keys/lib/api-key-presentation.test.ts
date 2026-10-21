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
import { expect, test } from 'bun:test'

import { API_KEY_STATUS, API_KEY_STATUS_REASON } from '../constants'
import type { ApiKey } from '../types'
import { getApiKeyStatusPresentation } from './api-key-presentation'

function createApiKey(overrides: Partial<ApiKey> = {}): ApiKey {
  return {
    id: 1,
    name: 'mobile-key',
    key: 'sk-test',
    status: API_KEY_STATUS.DISABLED,
    status_reason: '',
    remain_quota: 0,
    used_quota: 0,
    unlimited_quota: false,
    expired_time: -1,
    created_time: 0,
    accessed_time: 0,
    group: 'default',
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    ...overrides,
  }
}

test('shows a compromised API key as leaked instead of disabled', () => {
  const presentation = getApiKeyStatusPresentation(
    createApiKey({
      status_reason: API_KEY_STATUS_REASON.COMPROMISED,
    })
  )

  expect(presentation).toEqual({
    label: 'Leaked',
    variant: 'danger',
    compromised: true,
  })
})

test('keeps the regular status presentation for other API keys', () => {
  const presentation = getApiKeyStatusPresentation(createApiKey())

  expect(presentation).toEqual({
    label: 'Disabled',
    variant: 'neutral',
    compromised: false,
  })
})
