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
import { expect, mock, test } from 'bun:test';

mock.module('@douyinfe/semi-ui', () => ({
  Modal: { confirm: () => undefined },
}));
mock.module('./api', () => ({ API: {} }));
mock.module('./utils', () => ({
  showError: () => undefined,
  showSuccess: () => undefined,
}));

const frontendTheme = await import('./frontendTheme');

function createStorage(initial = {}) {
  const values = new Map(Object.entries(initial));
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, String(value)),
    removeItem: (key) => values.delete(key),
  };
}

test('syncs the classic user id for default frontend session validation', () => {
  expect(frontendTheme.syncDefaultFrontendUserId).toBeFunction();

  const storage = createStorage({ user: JSON.stringify({ id: 7 }) });
  expect(frontendTheme.syncDefaultFrontendUserId(storage)).toBe(true);
  expect(storage.getItem('uid')).toBe('7');
});

test('clears a stale default frontend user id when classic user data is invalid', () => {
  expect(frontendTheme.syncDefaultFrontendUserId).toBeFunction();

  const storage = createStorage({ user: '{invalid', uid: '99' });
  expect(frontendTheme.syncDefaultFrontendUserId(storage)).toBe(false);
  expect(storage.getItem('uid')).toBeNull();
});
