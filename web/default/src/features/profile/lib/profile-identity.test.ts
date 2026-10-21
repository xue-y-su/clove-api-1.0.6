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

import {
  buildProfilePatch,
  formatDisplayNameCooldownDate,
  formatDisplayNameCooldownRemaining,
  isDisplayNameCoolingDown,
  uploadAvatarToImageHost,
} from './profile-identity'

describe('profile identity patch', () => {
  test('merges refreshed display fields without discarding session state', async () => {
    const profileIdentity = (await import('./profile-identity')) as Record<
      string,
      unknown
    >
    const mergeAuthUserProfile = profileIdentity.mergeAuthUserProfile as
      | ((
          authUser: Record<string, unknown>,
          profile: Record<string, unknown>
        ) => Record<string, unknown>)
      | undefined

    expect(mergeAuthUserProfile).toBeFunction()
    if (!mergeAuthUserProfile) return

    expect(
      mergeAuthUserProfile(
        {
          id: 1,
          username: 'clove',
          display_name: 'Old',
          avatar_url: 'https://img.example/old.png',
          group: 'default',
          role: 100,
          permissions: { sidebar_settings: true },
          access_token: 'session-only',
        },
        {
          id: 1,
          username: 'clove',
          display_name: 'New',
          avatar_url: 'https://img.example/new.png',
          group: 'vip',
          role: 100,
        }
      )
    ).toEqual({
      id: 1,
      username: 'clove',
      display_name: 'New',
      avatar_url: 'https://img.example/new.png',
      group: 'vip',
      role: 100,
      permissions: { sidebar_settings: true },
      access_token: 'session-only',
    })
  })

  test('trims editable fields', () => {
    expect(
      buildProfilePatch(
        { display_name: 'Old', avatar_url: 'https://img.example/old.png' },
        { display_name: '  New  ', avatar_url: '  ' }
      )
    ).toEqual({ display_name: 'New', avatar_url: '' })
  })

  test('omits unchanged profile fields', () => {
    expect(
      buildProfilePatch(
        { display_name: 'Old', avatar_url: 'https://img.example/old.png' },
        {
          display_name: '  Old  ',
          avatar_url: '  https://img.example/old.png  ',
        }
      )
    ).toEqual({})
  })

  test('does not initialize an imported empty display name during avatar edits', () => {
    expect(
      buildProfilePatch(
        {
          username: 'legacy-user',
          display_name: '',
          avatar_url: 'https://img.example/old.png',
        },
        {
          display_name: 'legacy-user',
          avatar_url: 'https://img.example/new.png',
        }
      )
    ).toEqual({ avatar_url: 'https://img.example/new.png' })
  })

  test('reports an active nickname cooldown from the server timestamp', () => {
    expect(
      isDisplayNameCoolingDown({ display_name_next_change_at: 200 }, 100)
    ).toBe(true)
    expect(
      isDisplayNameCoolingDown({ display_name_next_change_at: 200 }, 200)
    ).toBe(false)
    expect(
      buildProfilePatch(
        {
          display_name: 'Old',
          avatar_url: '',
          display_name_next_change_at: 200,
        },
        { display_name: 'New', avatar_url: '' },
        100
      )
    ).toEqual({ display_name: 'New' })
  })

  test('formats a seven-day cooldown as an explicit wait', () => {
    expect(formatDisplayNameCooldownRemaining(604800, 0, 'en')).toBe(
      'in 7 days'
    )
  })

  test('does not round a small server clock lead into an eighth day', () => {
    expect(formatDisplayNameCooldownRemaining(604801, 0, 'en')).toBe(
      'in 7 days'
    )
  })

  test('formats cooldown values with the internal simplified Chinese locale', () => {
    expect(formatDisplayNameCooldownRemaining(604800, 0, 'zhCN')).not.toBe('')
    expect(formatDisplayNameCooldownDate(604800, 'zhCN')).not.toBe('')
  })

  test('uploads the selected image directly and accepts only an allowed URL', async () => {
    const file = new File(['image'], 'avatar.png', { type: 'image/png' })
    let submittedImage: FormDataEntryValue | null = null
    const fetcher = (async (_input: RequestInfo | URL, init?: RequestInit) => {
      if (!init?.body) throw new Error('missing upload body')
      submittedImage = (init.body as FormData).get('image')
      return new Response(
        JSON.stringify({
          success: true,
          data: { url: 'https://img.scdn.io/avatar.png' },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } }
      )
    }) as typeof fetch

    const url = await uploadAvatarToImageHost(
      file,
      {
        enabled: true,
        endpoint: 'https://img.scdn.io/api/v1.php',
        allowed_domains: ['img.scdn.io'],
      },
      fetcher
    )

    expect(submittedImage).toBeInstanceOf(File)
    expect((submittedImage as File).name).toBe(file.name)
    expect((submittedImage as File).type).toBe(file.type)
    expect(url).toBe('https://img.scdn.io/avatar.png')
  })

  test('classifies avatar host network failures', async () => {
    const file = new File(['image'], 'avatar.png', { type: 'image/png' })
    const fetcher = (async () => {
      throw new TypeError('Failed to fetch')
    }) as typeof fetch

    await expect(
      uploadAvatarToImageHost(
        file,
        {
          enabled: true,
          endpoint: 'https://img.scdn.io/api/v1.php',
          allowed_domains: ['img.scdn.io'],
        },
        fetcher
      )
    ).rejects.toThrow('Unable to reach avatar host')
  })

  test('does not start an avatar upload after cancellation', async () => {
    const file = new File(['image'], 'avatar.png', { type: 'image/png' })
    const controller = new AbortController()
    controller.abort()
    let called = false
    const fetcher = (async () => {
      called = true
      return new Response('{}', { status: 200 })
    }) as typeof fetch

    await expect(
      uploadAvatarToImageHost(
        file,
        {
          enabled: true,
          endpoint: 'https://img.scdn.io/api/v1.php',
          allowed_domains: ['img.scdn.io'],
        },
        fetcher,
        { signal: controller.signal }
      )
    ).rejects.toThrow('Avatar upload cancelled')
    expect(called).toBe(false)
  })

  test('translates avatar upload recovery states in every locale', async () => {
    const keys = [
      'Avatar upload timed out',
      'Avatar upload cancelled',
      'Unable to reach avatar host',
      'Cancel upload',
      'Uploading avatar',
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

  test('owns the profile editor from the top header without a second card', async () => {
    const pageSource = await Bun.file(
      new URL('../index.tsx', import.meta.url)
    ).text()
    const headerSource = await Bun.file(
      new URL('../components/profile-header.tsx', import.meta.url)
    ).text()
    const editorSource = await Bun.file(
      new URL('../components/profile-identity-card.tsx', import.meta.url)
    ).text()

    expect(pageSource).not.toContain('ProfileIdentityCard')
    expect(pageSource).toContain('onProfileUpdate={refreshProfile}')
    expect(headerSource).toContain('ProfileIdentityDialog')
    expect(headerSource).toContain("aria-label={t('Edit profile')}")
    expect(headerSource).toContain('const profileMetaItems = [')
    expect(headerSource).toContain('`@${profile.username}`,')
    expect(headerSource).toContain(
      "profile.group_name || profile.group || t('Default')"
    )
    expect(headerSource).toContain('buildInviterLabel(profile),')
    expect(headerSource).toContain('.filter(Boolean)')
    expect(headerSource).not.toContain("${t('Username')}:")
    expect(headerSource).not.toContain("${t('User Group')}:")
    expect(headerSource).not.toContain("${t('Inviter')}:")
    expect(headerSource).not.toContain("t('No Inviter')")
    expect(headerSource).not.toContain('profile.email')
    expect(headerSource).not.toContain('profile_subtitle_item')
    expect(headerSource).not.toContain('inviterInfo.role_label')
    expect(headerSource).not.toContain('inviterInfo.aff_code')
    expect(editorSource).toContain('<Dialog')
    expect(editorSource).toContain("type='file'")
    expect(editorSource).toContain('new AbortController()')
    expect(editorSource).toContain("setSavePhase('uploading')")
    expect(editorSource).toContain("t('Cancel upload')")
    expect(editorSource).not.toContain('<TitledCard')
    expect(editorSource).not.toContain("t('Avatar URL')")
  })

  test('nickname cooldown leaves avatar controls enabled', async () => {
    const source = await Bun.file(
      new URL('../components/profile-identity-card.tsx', import.meta.url)
    ).text()

    expect(source).toContain('disabled={saving || coolingDown}')
    expect(source).toContain(
      'disabled={saving || !currentProfile.avatar_upload?.enabled}'
    )
    expect(source).toContain('disabled={saving}')
  })

  test('checks nickname cooldown before uploading a selected avatar', async () => {
    const source = await Bun.file(
      new URL('../components/profile-identity-card.tsx', import.meta.url)
    ).text()
    const submitSource = source.slice(
      source.indexOf('async function handleSubmit()'),
      source.indexOf(
        '\n  return (',
        source.indexOf('async function handleSubmit()')
      )
    )

    expect(submitSource.indexOf('if (coolingDown')).toBeGreaterThanOrEqual(0)
    expect(submitSource.indexOf('if (coolingDown')).toBeLessThan(
      submitSource.indexOf('uploadAvatarToImageHost(')
    )
  })
})
