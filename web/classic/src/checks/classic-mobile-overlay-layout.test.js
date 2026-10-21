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
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const sourceRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..',
);

function read(relativePath) {
  return fs.readFileSync(path.join(sourceRoot, relativePath), 'utf8');
}

describe('classic responsive overlays', () => {
  test('closes the user action menu before opening a dialog', () => {
    const source = read('components/table/users/UsersColumnDefs.jsx');
    expect(source).toMatch(
      /<Dropdown\s+menu=\{moreMenu\}\s+trigger='click'\s+position='bottomRight'\s+clickToHide\s*>/,
    );
  });

  test('stacks the deprecation banner title and description on mobile', () => {
    const source = read('index.css');
    expect(source).toContain(
      '.classic-frontend-deprecation-banner .semi-banner-content-body',
    );
    expect(source).toMatch(
      /\.classic-frontend-deprecation-banner \.semi-banner-content-body\s*\{[^}]*flex-direction:\s*column;/s,
    );
  });

  test('labels the user action menu and custom audit date range', () => {
    const userColumns = read('components/table/users/UsersColumnDefs.jsx');
    const rangeControls = read(
      'components/table/usage-logs/IPAuditRangeControls.jsx',
    );

    expect(userColumns).toMatch(
      /<Button[^>]*aria-label=\{t\('更多'\)\}[^>]*icon=\{<IconMore \/>\}/s,
    );
    expect(rangeControls).toMatch(
      /<DatePicker[\s\S]*?aria-label=\{t\('审计时间范围'\)\}/,
    );
  });
});
