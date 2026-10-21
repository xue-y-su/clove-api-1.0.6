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
  SCREEN_CAPTURE_ERROR_MESSAGES,
  buildScreenshotFileName,
  captureDisplayScreenshotFile,
} from './screen-capture';

describe('classic playground screen capture', () => {
  test('builds stable screenshot names', () => {
    expect(buildScreenshotFileName(new Date('2026-07-11T12:34:56Z'))).toBe(
      'playground-screenshot-20260711-123456.png',
    );
  });

  test('reports unsupported browsers without silently hanging', async () => {
    const previousNavigator = globalThis.navigator;
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: {},
    });

    try {
      await expect(captureDisplayScreenshotFile()).rejects.toThrow(
        SCREEN_CAPTURE_ERROR_MESSAGES.UNSUPPORTED,
      );
    } finally {
      Object.defineProperty(globalThis, 'navigator', {
        configurable: true,
        value: previousNavigator,
      });
    }
  });
});
