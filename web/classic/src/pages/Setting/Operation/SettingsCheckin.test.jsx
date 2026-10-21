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

const sectionSource = await Bun.file(
  new URL('./SettingsCheckin.jsx', import.meta.url),
).text();
const operationSource = await Bun.file(
  new URL('../../../components/settings/OperationSetting.jsx', import.meta.url),
).text();
const profileSource = await Bun.file(
  new URL(
    '../../../components/settings/personal/cards/CheckinCalendar.jsx',
    import.meta.url,
  ),
).text();

test('wires the classic daily expiration option into settings', () => {
  expect(sectionSource).toContain(
    "'checkin_setting.expire_unused_quota': false",
  );
  expect(sectionSource).toContain(
    "field={'checkin_setting.expire_unused_quota'}",
  );
  expect(
    sectionSource.match(
      /handleFieldChange\(\s*'checkin_setting\.expire_unused_quota',?\s*\)/s,
    ),
  ).not.toBeNull();
  expect(sectionSource).toContain("t('Expire unused check-in rewards daily')");
  expect(operationSource).toContain(
    "'checkin_setting.expire_unused_quota': false",
  );
});

test('shows the classic expiration notice only for todays remaining reward', () => {
  expect(profileSource).toContain('record.checkin_date === todayString');
  expect(profileSource).toContain('todayRecord.expires_at > 0');
  expect(profileSource).toContain('todayRecord.quota_remaining > 0');
  expect(profileSource).toContain(
    "t('Unused check-in reward expires at midnight')",
  );
});
