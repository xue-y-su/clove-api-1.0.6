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
function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null
    ? (value as Record<string, unknown>)
    : null
}

function asNonEmptyString(value: unknown): string | null {
  return typeof value === 'string' && value.trim() ? value.trim() : null
}

function asPositiveSeconds(value: unknown): number | null {
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) return null
  return Math.ceil(parsed)
}

export function extractGateRequestId(error: unknown): string | null {
  const errorRecord = asRecord(error)
  const response = asRecord(errorRecord?.response)
  const data = asRecord(response?.data)
  const bodyRequestId =
    asNonEmptyString(data?.request_id) ?? asNonEmptyString(data?.requestId)
  if (bodyRequestId) return bodyRequestId

  const headers = asRecord(response?.headers)
  const headerRequestId =
    asNonEmptyString(headers?.['x-request-id']) ??
    asNonEmptyString(headers?.['X-Request-Id'])
  if (headerRequestId) return headerRequestId

  const message =
    error instanceof Error
      ? error.message
      : asNonEmptyString(errorRecord?.message)
  const match = message?.match(/request(?:[\s_-]+)id\s*[:：]\s*([a-z\d._-]+)/i)
  return match?.[1] ?? null
}

export function extractGateRateLimit(
  error: unknown
): { retryAfterSeconds: number } | null {
  const errorRecord = asRecord(error)
  const response = asRecord(errorRecord?.response)
  if (response?.status !== 429) return null

  const data = asRecord(response.data)
  const headers = asRecord(response.headers)
  const retryAfterSeconds =
    asPositiveSeconds(data?.retry_after) ??
    asPositiveSeconds(headers?.['retry-after']) ??
    asPositiveSeconds(headers?.['Retry-After']) ??
    1

  return { retryAfterSeconds }
}
