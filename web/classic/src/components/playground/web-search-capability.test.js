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
import { beforeAll, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

let subject = {};

beforeAll(async () => {
  try {
    subject = await import('./web-search-capability.js');
  } catch {
    subject = {};
  }
});

describe('classic playground web search capability', () => {
  test('reads the public capability from the shared status context shape', () => {
    expect(
      subject.getPlaygroundWebSearchCapability?.({
        data: {
          playground_web_search: {
            available: true,
            native: true,
            tavily: false,
            reason: '',
          },
        },
      }),
    ).toEqual({
      available: true,
      native: true,
      tavily: false,
      reason: '',
    });
  });

  test('defaults to unavailable for absent or malformed status data', () => {
    expect(subject.getPlaygroundWebSearchCapability?.(null)).toEqual({
      available: false,
      native: false,
      tavily: false,
      reason: 'not_configured',
    });
    expect(
      subject.getPlaygroundWebSearchCapability?.({
        data: { playground_web_search: { available: true } },
      }),
    ).toEqual({
      available: false,
      native: false,
      tavily: false,
      reason: 'not_configured',
    });
  });

  test('disables stale search state and leaves mobile message actions reachable', () => {
    const pageSource = readFileSync(
      new URL('../../pages/Playground/index.jsx', import.meta.url),
      'utf8',
    );
    const settingsSource = readFileSync(
      new URL('./SettingsPanel.jsx', import.meta.url),
      'utf8',
    );
    const chatSource = readFileSync(
      new URL('./ChatArea.jsx', import.meta.url),
      'utf8',
    );

    expect(pageSource).toContain('useContext(StatusContext)');
    expect(pageSource).toContain('getPlaygroundWebSearchCapability');
    expect(pageSource).toContain("handleInputChange('web_search', 'off')");
    expect(pageSource).toContain(
      'webSearchAvailable={webSearchCapability.available}',
    );
    expect(settingsSource).toContain('webSearchAvailable');
    expect(settingsSource).toContain(
      'disabled={customRequestMode || !webSearchAvailable}',
    );
    expect(chatSource).toContain("'[&_.semi-chat-container]:pb-24'");
    expect(chatSource).toContain("'[&_.semi-chat-container]:pb-0'");
  });

  test('localizes the unavailable capability message in every active locale', async () => {
    const localeNames = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW', 'zh'];
    const key = '管理员未配置游乐场联网搜索。';

    for (const localeName of localeNames) {
      const locale = await Bun.file(
        new URL(`../../i18n/locales/${localeName}.json`, import.meta.url),
      ).json();
      expect(locale.translation[key]).toBeTruthy();
      if (!localeName.startsWith('zh')) {
        expect(locale.translation[key]).not.toBe(key);
      }
    }
  });
});
