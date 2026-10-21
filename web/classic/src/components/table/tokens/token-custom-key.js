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

export const TOKEN_STATUS_REASON = {
  COMPROMISED: 'custom_key_collision_suspected_compromise',
};

export const CUSTOM_TOKEN_KEY_PATTERN = /^[A-Za-z0-9_-]+$/;
export const CUSTOM_TOKEN_KEY_MIN_LENGTH = 16;
export const CUSTOM_TOKEN_KEY_MAX_LENGTH = 128;

export function getCustomTokenKeyValidationMessage(value) {
  const key = String(value || '').trim();
  if (key === '') return '';
  if (
    key.length < CUSTOM_TOKEN_KEY_MIN_LENGTH ||
    key.length > CUSTOM_TOKEN_KEY_MAX_LENGTH
  ) {
    return '自定义 API Key 长度必须为 16 到 128 个字符';
  }
  if (!CUSTOM_TOKEN_KEY_PATTERN.test(key)) {
    return '自定义 API Key 只能包含字母、数字、下划线和连字符';
  }
  return '';
}

export function resolveCustomTokenKeyForCreate(
  value,
  tokenCount,
  allowCustomKeys = true,
) {
  if (!allowCustomKeys || Number(tokenCount || 1) !== 1) return '';
  return String(value || '').trim();
}

export function isValidCustomTokenKey(value) {
  return getCustomTokenKeyValidationMessage(value) === '';
}
