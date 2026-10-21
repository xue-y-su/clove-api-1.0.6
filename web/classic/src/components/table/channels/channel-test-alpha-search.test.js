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

test('offers Alpha Search as a non-streaming channel diagnostic', async () => {
  const source = await Bun.file(
    new URL('./modals/ModelTestModal.jsx', import.meta.url),
  ).text();

  expect(source).toContain("value: 'openai-alpha-search'");
  expect(source).toContain('Alpha Search (/v1/alpha/search)');
  expect(source.match(/'openai-alpha-search'/g)).toHaveLength(2);
});
