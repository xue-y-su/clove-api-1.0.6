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

const source = fs.readFileSync(
  new URL('../components/settings/RegistrationSetting.jsx', import.meta.url),
  'utf8',
);

describe('classic invitation registration settings', () => {
  test('hides legacy invitation-limit controls while preserving payload values', () => {
    expect(source).not.toContain("field='invite_limit_unlimited'");
    expect(source).not.toContain("field='invite_limit_count'");
    expect(source).toContain('inviteLimitUnlimited,');
    expect(source).toContain('inviteLimitCount,');
    expect(source).toContain('settingsLoaded');
    expect(source).toContain('disabled={!settingsLoaded}');
  });

  test('exposes application enablement and reapply cooldown controls', () => {
    expect(source).toContain('invitationApplicationsEnabled');
    expect(source).toContain('invitationReapplyCooldownSeconds');
    expect(source).toContain("t('Allow users to apply for invitation codes')");
    expect(source).toContain('InputNumber');
  });
});
