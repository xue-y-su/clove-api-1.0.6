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
import { describe, expect, test } from 'bun:test';

import {
  getCustomTokenKeyValidationMessage,
  isValidCustomTokenKey,
  resolveCustomTokenKeyForCreate,
} from './token-custom-key';

describe('custom token key validation', () => {
  test('allows blank values because custom keys are optional', () => {
    expect(isValidCustomTokenKey('')).toBe(true);
    expect(isValidCustomTokenKey('   ')).toBe(true);
  });

  test('accepts supported custom key characters', () => {
    expect(isValidCustomTokenKey('custom_key-123456')).toBe(true);
  });

  test('rejects unsupported custom key values', () => {
    expect(isValidCustomTokenKey('short-key')).toBe(false);
    expect(isValidCustomTokenKey('bad key with spaces')).toBe(false);
    expect(isValidCustomTokenKey('含中文customkey123456')).toBe(false);
  });

  test('explains length and character failures separately', () => {
    expect(getCustomTokenKeyValidationMessage('short-key')).toBe(
      '自定义 API Key 长度必须为 16 到 128 个字符',
    );
    expect(getCustomTokenKeyValidationMessage('bad key with spaces')).toBe(
      '自定义 API Key 只能包含字母、数字、下划线和连字符',
    );
  });

  test('omits custom keys when creation is disabled by system policy', () => {
    expect(resolveCustomTokenKeyForCreate('custom_key-123456', 1, false)).toBe(
      '',
    );
    expect(resolveCustomTokenKeyForCreate('custom_key-123456', 1, true)).toBe(
      'custom_key-123456',
    );
    expect(resolveCustomTokenKeyForCreate('custom_key-123456', 2, true)).toBe(
      '',
    );
  });

  test('wires policy into the modal and keeps status text visible', async () => {
    const modalSource = await Bun.file(
      new URL('./modals/EditTokenModal.jsx', import.meta.url),
    ).text();
    const columnSource = await Bun.file(
      new URL('./TokensColumnDefs.jsx', import.meta.url),
    ).text();

    expect(modalSource).toContain('resolveCustomTokenKeyForCreate(');
    expect(modalSource).toContain('allowCustomKeys');
    expect(columnSource).toContain('whitespace-nowrap');
    expect(columnSource).not.toContain('text-overflow: ellipsis');
  });

  test('shows compromised keys as suspected leaks instead of generic disabled', async () => {
    const columnSource = await Bun.file(
      new URL('./TokensColumnDefs.jsx', import.meta.url),
    ).text();

    expect(columnSource).toContain("tagText = t('疑似泄露')");
    expect(columnSource).toContain(
      'record?.status_reason === TOKEN_STATUS_REASON.COMPROMISED',
    );
    expect(columnSource).toContain(
      'disabled={record.status_reason === TOKEN_STATUS_REASON.COMPROMISED}',
    );
  });

  test('localizes compromised key status and guidance in every locale', async () => {
    const locales = ['en', 'fr', 'ja', 'ru', 'vi', 'zh', 'zh-CN', 'zh-TW'];
    const message = '此 API Key 因疑似泄露已被系统禁用，请创建并替换新的 Key。';

    for (const locale of locales) {
      const catalog = JSON.parse(
        await Bun.file(
          new URL(`../../../i18n/locales/${locale}.json`, import.meta.url),
        ).text(),
      ).translation;

      expect(catalog['疑似泄露']).toBeTruthy();
      expect(catalog[message]).toBeTruthy();
    }
  });
});
