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
import { describe, expect, test } from 'bun:test'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  buildSettingJSON,
  channelFormSchema,
  normalizeHttp2ConnectionShards,
  normalizeHttpProtocol,
} from './channel-form'

describe('channel HTTP transport form', () => {
  test('normalizes legacy and out-of-range stored values', () => {
    expect(normalizeHttpProtocol(undefined)).toBe('auto')
    expect(normalizeHttpProtocol('HTTP1')).toBe('http1')
    expect(normalizeHttpProtocol('http3')).toBe('auto')
    expect(normalizeHttp2ConnectionShards(0)).toBe(1)
    expect(normalizeHttp2ConnectionShards(99)).toBe(8)
  })

  test('omits defaults and stores only effective policy fields', () => {
    const defaultSetting = JSON.parse(
      buildSettingJSON(CHANNEL_FORM_DEFAULT_VALUES)
    )
    expect(defaultSetting.http_protocol).toBeUndefined()
    expect(defaultSetting.http2_connection_shards).toBeUndefined()

    const http1Setting = JSON.parse(
      buildSettingJSON({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        http_protocol: 'http1',
        http2_connection_shards: 8,
      })
    )
    expect(http1Setting.http_protocol).toBe('http1')
    expect(http1Setting.http2_connection_shards).toBeUndefined()

    const shardedSetting = JSON.parse(
      buildSettingJSON({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        http2_connection_shards: 4,
      })
    )
    expect(shardedSetting.http_protocol).toBeUndefined()
    expect(shardedSetting.http2_connection_shards).toBe(4)
  })

  test('rejects invalid proxies and HTTP1 shard combinations', () => {
    const proxyResult = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      proxy: 'ftp://proxy.example',
    })
    expect(proxyResult.success).toBe(false)
    if (!proxyResult.success) {
      expect(
        proxyResult.error.issues.some((issue) => issue.path[0] === 'proxy')
      ).toBe(true)
    }

    const shardResult = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      http_protocol: 'http1',
      http2_connection_shards: 2,
    })
    expect(shardResult.success).toBe(false)
    if (!shardResult.success) {
      expect(
        shardResult.error.issues.some(
          (issue) => issue.path[0] === 'http2_connection_shards'
        )
      ).toBe(true)
    }
  })
})
