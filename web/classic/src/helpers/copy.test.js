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
import { readFileSync } from 'node:fs';

const helperSource = readFileSync(
  new URL('./utils.jsx', import.meta.url),
  'utf8',
);
const copyStart = helperSource.indexOf('export async function copy(text)');
const copyEnd = helperSource.indexOf('\n}\n\n// isMobile', copyStart) + 2;
const copySource = helperSource
  .slice(copyStart, copyEnd)
  .replace('export ', '');

describe('copy', () => {
  test('does not run the fallback while the Clipboard API write is pending', async () => {
    let resolveClipboardWrite = () => undefined;
    const clipboardWrite = new Promise((resolve) => {
      resolveClipboardWrite = resolve;
    });
    let fallbackCalled = false;
    const navigator = {
      clipboard: { writeText: () => clipboardWrite },
    };
    const textArea = {
      value: '',
      style: {},
      setAttribute: () => undefined,
      select: () => undefined,
    };
    const window = {
      document: {
        body: {
          appendChild: () => undefined,
          removeChild: () => undefined,
        },
        createElement: () => textArea,
        execCommand: () => {
          fallbackCalled = true;
          return true;
        },
      },
    };
    const copy = new Function(
      'navigator',
      'window',
      `${copySource}; return copy;`,
    )(navigator, window);

    const resultPromise = copy('fresh-code');
    const fallbackStartedBeforeResolution = fallbackCalled;
    resolveClipboardWrite();

    expect(await resultPromise).toBe(true);
    expect(fallbackStartedBeforeResolution).toBe(false);
    expect(fallbackCalled).toBe(false);
  });

  test('uses the fallback after the Clipboard API rejects', async () => {
    let fallbackCalled = false;
    const navigator = {
      clipboard: {
        writeText: async () => {
          throw new Error('clipboard permission denied');
        },
      },
    };
    const textArea = {
      value: '',
      style: {},
      setAttribute: () => undefined,
      select: () => undefined,
    };
    const window = {
      document: {
        body: {
          appendChild: () => undefined,
          removeChild: () => undefined,
        },
        createElement: () => textArea,
        execCommand: () => {
          fallbackCalled = true;
          return true;
        },
      },
    };
    const copy = new Function(
      'navigator',
      'window',
      `${copySource}; return copy;`,
    )(navigator, window);

    expect(await copy('fallback-code')).toBe(true);
    expect(fallbackCalled).toBe(true);
    expect(textArea.value).toBe('fallback-code');
  });

  test('uses the synchronous fallback first on HTTP pages', async () => {
    let clipboardWriteCalled = false;
    let userActivationAvailable = true;
    const navigator = {
      clipboard: {
        writeText: async () => {
          clipboardWriteCalled = true;
          await Promise.resolve();
          userActivationAvailable = false;
          throw new Error('clipboard permission denied');
        },
      },
    };
    const textArea = {
      value: '',
      style: {},
      setAttribute: () => undefined,
      select: () => undefined,
      parentNode: true,
    };
    const window = {
      location: { protocol: 'http:' },
      document: {
        body: {
          appendChild: () => undefined,
          removeChild: () => undefined,
        },
        createElement: () => textArea,
        execCommand: () => userActivationAvailable,
      },
    };
    const copy = new Function(
      'navigator',
      'window',
      `${copySource}; return copy;`,
    )(navigator, window);

    expect(await copy('http-code')).toBe(true);
    expect(textArea.value).toBe('http-code');
    expect(clipboardWriteCalled).toBe(false);
  });
});
