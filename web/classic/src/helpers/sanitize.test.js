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
import { JSDOM } from 'jsdom';

beforeAll(() => {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', {
    url: 'https://app.example/',
  });
  globalThis.window = dom.window;
  globalThis.document = dom.window.document;
  globalThis.Node = dom.window.Node;
});

async function loadSanitizer() {
  return import('./sanitize.js');
}

describe('sanitizeRichHtml', () => {
  test('removes scripts, inline handlers, and active URLs', async () => {
    const { sanitizeRichHtml } = await loadSanitizer();
    const html = sanitizeRichHtml(
      '<p onclick="alert(1)">Hi</p><script>alert(1)</script><a href="javascript:alert(1)">bad</a>',
    );

    expect(html).toContain('<p>Hi</p>');
    expect(html).not.toContain('onclick');
    expect(html).not.toContain('<script');
    expect(html).not.toContain('javascript:');
  });

  test('adds safe attributes to external links', async () => {
    const { sanitizeRichHtml } = await loadSanitizer();
    const html = sanitizeRichHtml('<a href="https://example.com">docs</a>');

    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });
});

describe('sanitizeMarkdown', () => {
  test('sanitizes HTML produced by markdown', async () => {
    const { sanitizeMarkdown } = await loadSanitizer();
    const html = sanitizeMarkdown(
      '[safe](https://example.com) <img src=x onerror=alert(1)>',
    );

    expect(html).toContain('<a');
    expect(html).toContain('target="_blank"');
    expect(html).not.toContain('onerror');
  });
});
