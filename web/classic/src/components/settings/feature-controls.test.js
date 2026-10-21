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

const settingsPageSource = await Bun.file(
  new URL('../../pages/Setting/index.jsx', import.meta.url),
).text();
const securitySource = await Bun.file(
  new URL('./FeatureSecuritySetting.jsx', import.meta.url),
).text();
const announcementSource = await Bun.file(
  new URL(
    '../../pages/Setting/Dashboard/SettingsAnnouncements.jsx',
    import.meta.url,
  ),
).text();
const playgroundSource = await Bun.file(
  new URL('./PlaygroundSetting.jsx', import.meta.url),
).text();
const maintenanceSource = await Bun.file(
  new URL('./OtherSetting.jsx', import.meta.url),
).text();
const emailTemplatesSource = await Bun.file(
  new URL(
    '../../pages/Setting/Operation/SettingsEmailTemplates.jsx',
    import.meta.url,
  ),
).text();
const emailSettingsSource = await Bun.file(
  new URL('./SystemSetting.jsx', import.meta.url),
).text();

test('adds one root-only production feature controls tab', () => {
  expect(settingsPageSource).toContain('FeatureSecuritySetting');
  expect(settingsPageSource).toContain("itemKey: 'feature-security'");
});

test('saves all security policies through one dedicated endpoint', () => {
  expect(securitySource).toContain("API.get('/api/security/settings')");
  expect(securitySource).toContain("API.put('/api/security/settings'");
  for (const field of [
    'allow_custom_keys',
    'avatar_upload_enabled',
    'avatar_upload_endpoint',
    'avatar_allowed_domains',
    'display_name_max_length',
    'display_name_cooldown_seconds',
    'retention_days',
    'projection_interval_seconds',
    'projection_batch_size',
    'shared_registration_user_threshold',
    'shared_api_user_threshold',
    'unauthorized_threshold',
    'user_redemption',
    'single_enabled',
    'random_enabled',
    'max_active_codes_per_user',
    'max_claims_per_code',
    'max_expiry_hours',
  ]) {
    expect(securitySource).toContain(field);
  }
});

test('renders server-backed booleans with controlled switches', () => {
  expect(securitySource).toContain(
    'import {\n  Button,\n  Card,\n  Divider,\n  Form,\n  InputNumber,\n  Space,\n  Spin,\n  Switch,',
  );
  expect(securitySource).toContain(
    'checked={settings.api_key.allow_custom_keys}',
  );
  expect(securitySource).not.toContain('<Form.Switch');
});

test('uses the dedicated announcement settings endpoint', () => {
  expect(announcementSource).toContain("API.get('/api/announcement/settings')");
  expect(announcementSource).toContain("API.put('/api/announcement/settings'");
  expect(announcementSource).not.toContain('popupEnabled');
  expect(announcementSource).toContain('imageMaxMiB');
  expect(announcementSource).toContain('Maximum {{size}} MiB');
  expect(announcementSource).not.toContain('最大 5 MiB');
  expect(announcementSource).not.toContain(
    "key: 'console_setting.announcements_enabled'",
  );
});

test('does not restore removed playground image policy controls', () => {
  for (const field of [
    'PlaygroundImageModelsAllowlist',
    'PlaygroundImageModelsDenylist',
    'PlaygroundImageModelsDisplayOrder',
    'PlaygroundImageModelsExtraAllowlist',
  ]) {
    expect(playgroundSource).not.toContain(field);
  }
});

test('keeps maintenance informational without an update network action', () => {
  expect(maintenanceSource).not.toContain('api.github.com');
  expect(maintenanceSource).not.toContain('checkUpdate');
  expect(maintenanceSource).not.toContain("t('检查更新')");
  expect(maintenanceSource).toContain("t('当前版本')");
  expect(maintenanceSource).toContain("t('启动时间')");
});

test('starts SMTP verification templates with Chinese content', () => {
  for (const source of [emailTemplatesSource, emailSettingsSource]) {
    expect(source).toContain('您的邮箱验证码是');
    expect(source).toContain('{{expiration_minutes}} 分钟');
    expect(source).not.toContain('<p>Your verification code is');
  }
});
