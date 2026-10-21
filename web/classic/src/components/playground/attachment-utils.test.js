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
  buildPendingPlaygroundAttachments,
  buildPlaygroundMessageContent,
  buildPromptWithTextAttachments,
  readPlaygroundAttachments,
} from './attachment-utils';

describe('classic playground attachments', () => {
  test('always sends attached images as OpenAI-compatible content parts', () => {
    expect(
      buildPlaygroundMessageContent('Describe this', [
        {
          id: 'image-1',
          kind: 'image',
          mimeType: 'image/png',
          name: 'capture.png',
          size: 3,
          dataUrl: 'data:image/png;base64,abc',
        },
      ]),
    ).toEqual([
      { type: 'text', text: 'Describe this' },
      {
        type: 'image_url',
        image_url: { url: 'data:image/png;base64,abc' },
      },
    ]);
  });

  test('adds text files as bounded prompt context', async () => {
    const result = await readPlaygroundAttachments([
      new File(['hello'], 'notes.txt', { type: 'text/plain' }),
    ]);
    expect(result.rejected).toEqual([]);
    expect(
      buildPromptWithTextAttachments('Summarize', result.attachments),
    ).toContain('<attachment name="notes.txt" type="text/plain">\nhello');
  });

  test('keeps uploaded attachments when manual image URLs are disabled', () => {
    const uploaded = [
      { id: 'upload-1', kind: 'image', dataUrl: 'data:image/png;base64,AA' },
    ];

    expect(
      buildPendingPlaygroundAttachments(uploaded, false, [
        'https://example.com/manual.png',
      ]),
    ).toEqual(uploaded);
  });

  test('adds only non-empty manual image URLs when enabled', () => {
    const result = buildPendingPlaygroundAttachments([], true, [
      '',
      '  ',
      'https://example.com/manual.png',
    ]);

    expect(result).toHaveLength(1);
    expect(result[0].dataUrl).toBe('https://example.com/manual.png');
  });
});
