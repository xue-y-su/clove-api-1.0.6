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

import { readFileSync } from 'node:fs';

import { expect, test } from 'bun:test';

const addUserSource = readFileSync(
  new URL('./modals/AddUserModal.jsx', import.meta.url),
  'utf8',
);
const editUserSource = readFileSync(
  new URL('./modals/EditUserModal.jsx', import.meta.url),
  'utf8',
);
const registerSource = readFileSync(
  new URL('../../auth/RegisterForm.jsx', import.meta.url),
  'utf8',
);

test('classic user forms enforce the shared username length limit', () => {
  for (const source of [addUserSource, editUserSource, registerSource]) {
    expect(source).toContain('maxLength={20}');
    expect(source).toContain('max: 20');
    expect(source).toContain("t('用户名最多 20 个字符')");
  }
});

test('classic registration accepts Chinese usernames and rejects unsupported symbols', () => {
  expect(registerSource).toContain('USERNAME_PATTERN');
  expect(registerSource).toContain('\\p{Script=Han}');
  expect(registerSource).toContain(
    "t('用户名仅支持中文、英文字母、数字、下划线和连字符')",
  );
});
