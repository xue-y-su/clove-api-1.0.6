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

test('changes token groups through the narrow endpoint instead of full editing', async () => {
  const hookSource = await Bun.file(
    new URL('../../../hooks/tokens/useTokensData.jsx', import.meta.url),
  ).text();
  const columnsSource = await Bun.file(
    new URL('./TokensColumnDefs.jsx', import.meta.url),
  ).text();

  expect(hookSource).toContain("API.put('/api/token/?group_only=true'");
  expect(columnsSource).toContain('<Select');
  expect(columnsSource).toContain('updateTokenGroup');
});

test('keeps the quick group selector visible inside narrow viewports', async () => {
  const columnsSource = await Bun.file(
    new URL('./TokensColumnDefs.jsx', import.meta.url),
  ).text();
  const tableSource = await Bun.file(
    new URL('./TokensTable.jsx', import.meta.url),
  ).text();

  expect(columnsSource).toContain('getPopupContainer={() => document.body}');
  expect(columnsSource).toContain(
    "maxHeight: 'min(360px, calc(100svh - 32px))'",
  );
  expect(columnsSource).toContain("overflowY: 'auto'");
  expect(tableSource).toContain("scroll={{ x: 'max-content' }}");
});

test('imports the search filter used by the quick group selector', async () => {
  const columnsSource = await Bun.file(
    new URL('./TokensColumnDefs.jsx', import.meta.url),
  ).text();
  const helperImport = columnsSource.match(
    /import\s*\{([\s\S]*?)\}\s*from '\.\.\/\.\.\/\.\.\/helpers';/,
  );

  expect(helperImport?.[1]).toContain('selectFilter');
});
