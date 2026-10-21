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
  createTokenRevealMemory,
  normalizeApiKeyCredential,
} from './token-reveal-memory';

describe('classic token reveal memory', () => {
  test('clears keys at the exact server expiry', () => {
    let now = 1000;
    const memory = createTokenRevealMemory(() => now);
    memory.setVerifiedUntil(2);
    memory.setKey(1, 'secret-key');
    now = 1999;
    expect(memory.getKey(1)).toBe('secret-key');
    now = 2000;
    expect(memory.getKey(1)).toBeUndefined();
    expect(memory.snapshot()).toEqual({});
  });

  test('preserves the complete credential without adding a prefix', () => {
    expect(normalizeApiKeyCredential('  custom-key  ')).toBe('custom-key');
    expect(normalizeApiKeyCredential('sk-custom-key')).toBe('sk-custom-key');
  });
});
