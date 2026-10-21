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

const read = (relativePath) =>
  fs.readFileSync(path.join(sourceRoot, relativePath), 'utf8');

describe('classic source attribution target user contract', () => {
  test('uses target_user_id in settings and confirmation records', () => {
    const editor = read(
      'components/settings/SourceAttributionOptionsEditor.jsx',
    );
    const settings = read('components/settings/SourceAttributionSetting.jsx');
    const confirmations = read(
      'components/source-attribution/SourceAttributionConfirmations.jsx',
    );

    expect(editor).toContain('target_user_id: 0');
    expect(editor).toContain("dataIndex: 'target_user_id'");
    expect(editor).not.toContain('record.target_aff_code');
    expect(settings).toContain('目标用户 ID');
    expect(settings).not.toContain('<Form.Switch');
    expect(confirmations).toContain('filters.target_user_id');
    expect(confirmations).toContain("dataIndex: 'target_user_id'");
    expect(confirmations).not.toContain('filters.target_aff_code');
  });
});
