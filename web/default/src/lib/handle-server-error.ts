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
import i18next from 'i18next'
import { toast } from 'sonner'

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null
    ? (value as Record<string, unknown>)
    : null
}

function nonEmptyString(value: unknown): string | null {
  return typeof value === 'string' && value.trim() ? value.trim() : null
}

function responseHeader(
  headers: Record<string, unknown> | null,
  name: string
): string | null {
  if (!headers) return null
  const getter = headers.get
  if (typeof getter === 'function') {
    return nonEmptyString(getter.call(headers, name))
  }
  return (
    nonEmptyString(headers[name]) ?? nonEmptyString(headers[name.toLowerCase()])
  )
}

export function getServerErrorMessage(
  error: unknown,
  fallback: string,
  requestIdLabel: string
): string {
  const errorRecord = asRecord(error)
  const response = asRecord(errorRecord?.response)
  const data = asRecord(response?.data)
  const nestedError = asRecord(data?.error)
  const publicMessage =
    nonEmptyString(data?.message) ??
    nonEmptyString(nestedError?.message) ??
    nonEmptyString(data?.title) ??
    (response ? null : nonEmptyString(errorRecord?.message)) ??
    fallback
  const headers = asRecord(response?.headers)
  const requestId =
    nonEmptyString(data?.request_id) ??
    nonEmptyString(data?.requestId) ??
    responseHeader(headers, 'x-request-id')

  if (!requestId || publicMessage.includes(requestId)) return publicMessage
  return `${publicMessage} (${requestIdLabel}: ${requestId})`
}

export function handleServerError(error: unknown) {
  let fallback = i18next.t('Something went wrong!')

  if (
    error &&
    typeof error === 'object' &&
    'status' in error &&
    Number(error.status) === 204
  ) {
    fallback = i18next.t('Content not found.')
  }

  toast.error(getServerErrorMessage(error, fallback, i18next.t('Request ID')))
}
