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
import { beforeEach, describe, expect, test } from 'bun:test';
import { JSDOM } from 'jsdom';
import { STORAGE_KEYS } from '../../constants/playground.constants';
import { loadConfig, saveMessages } from './configStorage';

describe('classic playground message storage', () => {
  beforeEach(() => {
    const dom = new JSDOM('', { url: 'https://example.test' });
    globalThis.localStorage = dom.window.localStorage;
  });

  test('does not persist attachment payloads or injected document context', () => {
    saveMessages([
      {
        role: 'user',
        content: [
          {
            type: 'text',
            text: 'Visible text\n\n<attachments>secret</attachments>',
          },
          {
            type: 'image_url',
            image_url: { url: 'data:image/png;base64,SECRET' },
          },
        ],
        displayContent: 'Visible text',
        attachments: [
          {
            id: 'attachment-1',
            kind: 'image',
            mimeType: 'image/png',
            name: 'proof.png',
            size: 42,
            dataUrl: 'data:image/png;base64,SECRET',
            text: 'secret',
          },
        ],
      },
    ]);

    const saved = localStorage.getItem(STORAGE_KEYS.MESSAGES);
    expect(saved).not.toContain('base64,SECRET');
    expect(saved).not.toContain('<attachments>secret</attachments>');
    expect(saved).not.toContain('"text":"secret"');
    expect(JSON.parse(saved).messages[0]).toMatchObject({
      content: 'Visible text',
      attachments: [
        {
          id: 'attachment-1',
          kind: 'image',
          mimeType: 'image/png',
          name: 'proof.png',
          size: 42,
        },
      ],
    });
  });

  test('migrates the legacy web search toggle to a search depth', () => {
    localStorage.setItem(
      STORAGE_KEYS.CONFIG,
      JSON.stringify({ inputs: { web_search: true } }),
    );

    expect(loadConfig().inputs.web_search).toBe('basic');
  });
});
