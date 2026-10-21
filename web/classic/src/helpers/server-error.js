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

function asServerErrorRecord(value) {
  return typeof value === 'object' && value !== null ? value : null;
}

function nonEmptyServerErrorString(value) {
  return typeof value === 'string' && value.trim() ? value.trim() : null;
}

export function getServerErrorMessage(
  error,
  fallback = '请求失败，请稍后重试。',
  requestIdLabel = '请求 ID',
) {
  const errorRecord = asServerErrorRecord(error);
  const response = asServerErrorRecord(errorRecord?.response);
  const data = asServerErrorRecord(response?.data);
  const nestedError = asServerErrorRecord(data?.error);
  const message =
    nonEmptyServerErrorString(data?.message) ??
    nonEmptyServerErrorString(nestedError?.message) ??
    nonEmptyServerErrorString(data?.title) ??
    (response ? null : nonEmptyServerErrorString(errorRecord?.message)) ??
    fallback;
  const responseHeaders = asServerErrorRecord(response?.headers);
  const headerRequestId =
    typeof responseHeaders?.get === 'function'
      ? nonEmptyServerErrorString(responseHeaders.get('x-request-id'))
      : (nonEmptyServerErrorString(responseHeaders?.['x-request-id']) ??
        nonEmptyServerErrorString(responseHeaders?.['X-Request-Id']));
  const requestId =
    nonEmptyServerErrorString(data?.request_id) ??
    nonEmptyServerErrorString(data?.requestId) ??
    headerRequestId;

  if (!requestId || message.includes(requestId)) return message;
  return `${message} (${requestIdLabel}: ${requestId})`;
}
