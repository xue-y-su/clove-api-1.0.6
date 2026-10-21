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
import { describe, expect, test } from 'bun:test'

const source = await Bun.file(
  new URL('./registration-section.tsx', import.meta.url)
).text()

describe('invitation registration settings workflow', () => {
  test('exposes separate registration and invitation-application policies', () => {
    expect(source).toContain(
      "t('Require approved invitation code for registration')"
    )
    expect(source).toContain("t('Allow users to apply for invitation codes')")
    expect(source).toContain(
      'invitation_applications_enabled: invitationApplicationsEnabled'
    )
    expect(source).toContain(
      'invitation_reapply_cooldown_seconds: invitationReapplyCooldownSeconds'
    )
    expect(source).toContain("type='number'")
    expect(source).not.toContain("t('Do not limit invitations by default')")
    expect(source).not.toContain("t('Default invitation limit')")
  })

  test('preserves legacy limit values when saving the current policy', () => {
    expect(source).toContain(
      'invite_limit_unlimited: settings?.invite_limit_unlimited ?? true'
    )
    expect(source).toContain(
      'invite_limit_count: settings?.invite_limit_count ?? 0'
    )
    expect(source).toContain('isSaveDisabled={!settingsQuery.data?.data}')
    expect(source).toContain('if (!settings) return')
  })

  test('round-trips invitation poster copy with Unicode-aware limits', () => {
    expect(source).toContain('invitation_poster_title: invitationPosterTitle')
    expect(source).toContain(
      'invitation_poster_subtitle: invitationPosterSubtitle'
    )
    expect(source).toContain('limitPosterCharacters(event.target.value, 80)')
    expect(source).toContain('limitPosterCharacters(event.target.value, 160)')
    expect(source).toContain("t('Invitation poster title')")
    expect(source).toContain("t('Invitation poster subtitle')")
    expect(source).toContain("'{\\u7ad9\\u70b9\\u540d\\u79f0}'")
  })

  test('translates the approved-code policy in every locale', async () => {
    const keys = [
      'Require approved invitation code for registration',
      'When enabled, registration requires an active invitation code issued through the application or administrator issuance workflow.',
      'Allow users to apply for invitation codes',
      'When disabled, users cannot submit new invitation applications. Administrators can still issue and manage codes.',
      'Invitation reapply cooldown (seconds)',
      'How long a rejected application must wait before the user can apply again. Set 0 for no cooldown.',
      'Invitation poster title',
      'Invitation poster subtitle',
      'Shown on invitation posters. Use {站点名称} to insert the current site name.',
      'Leave blank to use the localized default copy.',
    ]
    for (const localeFile of [
      'en.json',
      'fr.json',
      'ja.json',
      'ru.json',
      'vi.json',
      'zh-TW.json',
      'zh.json',
    ]) {
      const locale = (await Bun.file(
        new URL(`../../../i18n/locales/${localeFile}`, import.meta.url)
      ).json()) as { translation: Record<string, string> }
      for (const key of keys) {
        expect(locale.translation[key]).toBeTruthy()
        if (localeFile !== 'en.json') {
          expect(locale.translation[key]).not.toBe(key)
        }
      }
    }
  })
})
