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

describe('classic header and channel responsive layout', () => {
  test('keeps header navigation compact below the desktop breakpoint', () => {
    const breakpoint = read('hooks/common/useIsMobile.js');
    const header = read('components/layout/headerbar/index.jsx');
    const navigation = read('components/layout/headerbar/Navigation.jsx');

    expect(breakpoint).toContain(
      'export const HEADER_DESKTOP_BREAKPOINT = 1024;',
    );
    expect(breakpoint).toContain('export const useIsCompactHeader = () =>');
    expect(header).toContain('const isCompactHeader = useIsCompactHeader();');
    expect(navigation).toContain(
      'isCompactHeader ? <MobileNavigationMenu /> : renderNavLinks()',
    );
  });

  test('wraps full-width channel action buttons on narrow screens', () => {
    const filters = read('components/table/channels/ChannelsFilters.jsx');

    expect(filters).toContain(
      "className='flex flex-wrap gap-2 w-full md:w-auto order-2 md:order-1'",
    );
  });
});
