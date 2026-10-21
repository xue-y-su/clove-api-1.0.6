/*
Copyright (C) 2023-2026 QuantumNous

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
import { expect, test } from 'bun:test'

const apiSource = await Bun.file(new URL('./api.ts', import.meta.url)).text()
const announcementSource = await Bun.file(
  new URL('./content/announcements-section.tsx', import.meta.url)
).text()
const securityRegistrySource = await Bun.file(
  new URL('./security/section-registry.tsx', import.meta.url)
).text()
const securityControlsSource = await Bun.file(
  new URL('./security/feature-controls-section.tsx', import.meta.url)
).text()
const playgroundSource = await Bun.file(
  new URL('./site/playground-section.tsx', import.meta.url)
).text()
const updateCheckerSource = await Bun.file(
  new URL('./maintenance/update-checker-section.tsx', import.meta.url)
).text()
const emailTemplatesSource = await Bun.file(
  new URL('./integrations/email-templates-section.tsx', import.meta.url)
).text()
const emailSettingsSource = await Bun.file(
  new URL('./integrations/email-settings-section.tsx', import.meta.url)
).text()

test('uses dedicated atomic APIs for announcement and security policies', () => {
  expect(apiSource).toContain("'/api/announcement/settings'")
  expect(apiSource).toContain("'/api/security/settings'")
  expect(apiSource).toContain('updateAnnouncementSettings')
  expect(apiSource).toContain('updateFeatureSecuritySettings')
})

test('keeps announcement delivery policy beside the composer', () => {
  expect(announcementSource).toContain('getAnnouncementSettings')
  expect(announcementSource).toContain('updateAnnouncementSettings')
  expect(announcementSource).not.toContain('popupEnabled')
  expect(announcementSource).toContain('imageMaxMiB')
  expect(announcementSource).not.toContain(
    "key: 'console_setting.announcements_enabled'"
  )
})

test('exposes custom key, user governance, and IP audit controls together', () => {
  expect(securityRegistrySource).toContain('FeatureControlsSection')
  expect(securityRegistrySource).toContain("id: 'feature-controls'")
  expect(securityControlsSource).toContain(
    "saveLabel={t('Save security settings')}"
  )
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
    expect(securityControlsSource).toContain(field)
  }
})

test('does not restore removed playground image policy controls', () => {
  for (const field of [
    'PlaygroundImageModelsAllowlist',
    'PlaygroundImageModelsDenylist',
    'PlaygroundImageModelsDisplayOrder',
    'PlaygroundImageModelsExtraAllowlist',
  ]) {
    expect(playgroundSource).not.toContain(field)
  }
})

test('keeps maintenance informational without an update network action', () => {
  expect(updateCheckerSource).not.toContain('api.github.com')
  expect(updateCheckerSource).not.toContain('handleCheckUpdates')
  expect(updateCheckerSource).not.toContain("t('Check for updates')")
  expect(updateCheckerSource).toContain("t('Current version')")
  expect(updateCheckerSource).toContain("t('Uptime since')")
  expect(updateCheckerSource).toContain(
    "const version = currentVersion || t('Unknown')"
  )
})

test('starts SMTP verification templates with Chinese content', () => {
  for (const source of [emailTemplatesSource, emailSettingsSource]) {
    expect(source).toContain('您的邮箱验证码是')
    expect(source).toContain('{{expiration_minutes}} 分钟')
    expect(source).not.toContain('<p>Your verification code is')
  }
  expect(emailTemplatesSource).not.toContain('catch (e: any)')
})
