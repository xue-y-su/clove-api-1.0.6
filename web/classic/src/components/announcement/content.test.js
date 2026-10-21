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

import { normalizeAnnouncementContent } from './content';

describe('normalizeAnnouncementContent', () => {
  test('dedents formatted HTML blocks mixed with plain text', () => {
    const content = [
      '欢迎页面',
      '',
      '    <div>',
      '        <h1>欢迎来到本站</h1>',
      '    </div>',
    ].join('\n');

    expect(normalizeAnnouncementContent(content)).toBe(
      ['欢迎页面', '', '<div>', '    <h1>欢迎来到本站</h1>', '</div>'].join(
        '\n',
      ),
    );
  });

  test('preserves fenced code and ordinary indented text', () => {
    const content = [
      '```html',
      '<div>示例</div>',
      '```',
      '',
      '    普通缩进文本',
    ].join('\n');

    expect(normalizeAnnouncementContent(content)).toBe(content);
  });

  test('stops dedenting after the formatted HTML block closes', () => {
    const content = [
      '    <div>',
      '        <p>公告</p>',
      '    </div>',
      '',
      '    const example = 1',
    ].join('\n');

    expect(normalizeAnnouncementContent(content)).toBe(
      ['<div>', '    <p>公告</p>', '</div>', '', '    const example = 1'].join(
        '\n',
      ),
    );
  });
});
