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
const asRecord = (value) =>
  typeof value === 'object' && value !== null ? value : null;

const asNonEmptyString = (value) =>
  typeof value === 'string' && value.trim() ? value.trim() : null;

const asPositiveSeconds = (value) => {
  const parsed = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return null;
  return Math.ceil(parsed);
};

export const extractGateRequestId = (error) => {
  const errorRecord = asRecord(error);
  const response = asRecord(errorRecord?.response);
  const data = asRecord(response?.data);
  const bodyRequestId =
    asNonEmptyString(data?.request_id) ?? asNonEmptyString(data?.requestId);
  if (bodyRequestId) return bodyRequestId;

  const headers = asRecord(response?.headers);
  const headerRequestId =
    asNonEmptyString(headers?.['x-request-id']) ??
    asNonEmptyString(headers?.['X-Request-Id']);
  if (headerRequestId) return headerRequestId;

  const message =
    error instanceof Error
      ? error.message
      : asNonEmptyString(errorRecord?.message);
  const match = message?.match(/request(?:[\s_-]+)id\s*[:：]\s*([a-z\d._-]+)/i);
  return match?.[1] ?? null;
};

export const extractGateRateLimit = (error) => {
  const errorRecord = asRecord(error);
  const response = asRecord(errorRecord?.response);
  if (response?.status !== 429) return null;

  const data = asRecord(response.data);
  const headers = asRecord(response.headers);
  const retryAfterSeconds =
    asPositiveSeconds(data?.retry_after) ??
    asPositiveSeconds(headers?.['retry-after']) ??
    asPositiveSeconds(headers?.['Retry-After']) ??
    1;
  return { retryAfterSeconds };
};
