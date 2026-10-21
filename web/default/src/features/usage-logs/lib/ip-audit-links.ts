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
const BAIDU_IP_THREAT_URL =
  'https://cloud.baidu.com/product-s/afd_s/ip-threat.html?s='

export function buildIPThreatURL(ip: string, allowed: boolean): string | null {
  const value = ip.trim()
  if (!allowed || value === '') return null
  return `${BAIDU_IP_THREAT_URL}${encodeURIComponent(value)}`
}

export function buildUsageLogSearch(
  requestID?: string,
  createdAt?: number
): {
  requestId?: string
  startTime?: number
  endTime?: number
} {
  const value = requestID?.trim() ?? ''
  if (value === '') return {}
  if (!Number.isFinite(createdAt) || (createdAt ?? 0) <= 0) {
    return { requestId: value }
  }

  const createdAtMilliseconds = Math.trunc((createdAt ?? 0) * 1000)
  const searchWindowMilliseconds = 5 * 60 * 1000
  return {
    requestId: value,
    startTime: Math.max(0, createdAtMilliseconds - searchWindowMilliseconds),
    endTime: createdAtMilliseconds + searchWindowMilliseconds,
  }
}
