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
import { expect, test } from 'bun:test';

test('keeps the exchange-rate prefix and currency suffix readable on mobile', async () => {
  const source = await Bun.file(
    new URL('./SettingsGeneral.jsx', import.meta.url),
  ).text();

  expect(source).toContain('[&_.semi-input-prefix]:!shrink-0');
  expect(source).toContain('[&_.semi-input-suffix]:!shrink-0');
});
