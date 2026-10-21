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

import {
  buildProfileIdentityPatch,
  formatDisplayNameCooldownRemaining,
  isDisplayNameCoolingDown,
  uploadAvatarToImageHost,
} from './profile-identity.js';

describe('classic profile identity', () => {
  test('trims profile fields before submission', () => {
    expect(
      buildProfileIdentityPatch(
        {
          displayName: 'Old User',
          avatarURL: 'https://img.scdn.io/old.png',
        },
        {
          displayName: '  Snow User  ',
          avatarURL: '  https://img.scdn.io/avatar.png  ',
        },
      ),
    ).toEqual({
      display_name: 'Snow User',
      avatar_url: 'https://img.scdn.io/avatar.png',
    });
  });

  test('omits an unchanged nickname during avatar-only edits', () => {
    expect(
      buildProfileIdentityPatch(
        {
          displayName: 'Snow User',
          avatarURL: 'https://img.scdn.io/old.png',
        },
        {
          displayName: '  Snow User  ',
          avatarURL: 'https://img.scdn.io/new.png',
        },
      ),
    ).toEqual({ avatar_url: 'https://img.scdn.io/new.png' });
  });

  test('treats the server timestamp as an exclusive cooldown boundary', () => {
    expect(isDisplayNameCoolingDown(200, 199)).toBe(true);
    expect(isDisplayNameCoolingDown(200, 200)).toBe(false);
    expect(isDisplayNameCoolingDown(undefined, 199)).toBe(false);
  });

  test('formats a seven-day cooldown as an explicit wait', () => {
    expect(formatDisplayNameCooldownRemaining(604800, 0, 'en')).toBe(
      'in 7 days',
    );
  });

  test('uploads the selected image directly to the configured image host', async () => {
    const file = new File(['image'], 'avatar.png', { type: 'image/png' });
    let submittedImage = null;
    const fetcher = async (_input, init) => {
      submittedImage = init.body.get('image');
      return new Response(
        JSON.stringify({
          success: true,
          data: { url: 'https://img.scdn.io/avatar.png' },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      );
    };

    const url = await uploadAvatarToImageHost(
      file,
      {
        enabled: true,
        endpoint: 'https://img.scdn.io/api/v1.php',
        allowed_domains: ['img.scdn.io'],
      },
      fetcher,
    );

    expect(submittedImage).toBeInstanceOf(File);
    expect(submittedImage.name).toBe('avatar.png');
    expect(submittedImage.type).toBe('image/png');
    expect(url).toBe('https://img.scdn.io/avatar.png');
  });

  test('classifies avatar host network failures', async () => {
    const file = new File(['image'], 'avatar.png', { type: 'image/png' });
    const fetcher = async () => {
      throw new TypeError('Failed to fetch');
    };

    await expect(
      uploadAvatarToImageHost(
        file,
        {
          enabled: true,
          endpoint: 'https://img.scdn.io/api/v1.php',
          allowed_domains: ['img.scdn.io'],
        },
        fetcher,
      ),
    ).rejects.toThrow('无法连接头像图床，请检查网络或稍后重试');
  });

  test('does not start an avatar upload after cancellation', async () => {
    const file = new File(['image'], 'avatar.png', { type: 'image/png' });
    const controller = new AbortController();
    controller.abort();
    let called = false;
    const fetcher = async () => {
      called = true;
      return new Response('{}', { status: 200 });
    };

    await expect(
      uploadAvatarToImageHost(
        file,
        {
          enabled: true,
          endpoint: 'https://img.scdn.io/api/v1.php',
          allowed_domains: ['img.scdn.io'],
        },
        fetcher,
        { signal: controller.signal },
      ),
    ).rejects.toThrow('头像上传已取消');
    expect(called).toBe(false);
  });

  test('translates avatar upload recovery states in every locale', async () => {
    const keys = [
      '头像上传超时，请重试',
      '头像上传已取消',
      '无法连接头像图床，请检查网络或稍后重试',
      '取消上传',
      '正在上传头像',
    ];
    for (const localeFile of [
      'en.json',
      'fr.json',
      'ja.json',
      'ru.json',
      'vi.json',
      'zh-CN.json',
      'zh-TW.json',
      'zh.json',
    ]) {
      const locale = await Bun.file(
        new URL(`../../../i18n/locales/${localeFile}`, import.meta.url),
      ).json();
      for (const key of keys) {
        expect(locale.translation[key]).toBeTruthy();
        if (!localeFile.startsWith('zh')) {
          expect(locale.translation[key]).not.toBe(key);
        }
      }
    }
  });

  test('owns the profile editor from the top header without a second card', async () => {
    const pageSource = await Bun.file(
      new URL('../PersonalSetting.jsx', import.meta.url),
    ).text();
    const headerSource = await Bun.file(
      new URL('./components/UserInfoHeader.jsx', import.meta.url),
    ).text();
    const editorSource = await Bun.file(
      new URL('./cards/ProfileIdentity.jsx', import.meta.url),
    ).text();

    expect(pageSource).not.toContain('<ProfileIdentity');
    expect(headerSource).toContain(
      "import ProfileIdentity from '../cards/ProfileIdentity'",
    );
    expect(headerSource).toContain('<ProfileIdentity');
    expect(headerSource).toContain("aria-label={t('编辑个人资料')}");
    expect(headerSource).toContain('getDisplayName()');
    expect(headerSource).toContain("className='relative h-40 sm:h-32'");
    expect(headerSource).toContain(
      "className='text-2xl font-bold leading-tight whitespace-normal break-words [overflow-wrap:anywhere] sm:text-3xl'",
    );
    expect(headerSource).not.toContain('text-3xl font-bold truncate');
    expect(headerSource).toContain(
      'const usernameHandle = `@${getUsername()}`;',
    );
    expect(headerSource).toContain('const profileMetaItems = [');
    expect(headerSource).toContain('usernameHandle,');
    expect(headerSource).toContain(
      "userState?.user?.group_name || userState?.user?.group || t('默认'),",
    );
    expect(headerSource).toContain('inviterLabel,');
    expect(headerSource).toContain('.filter(Boolean);');
    expect(headerSource).not.toContain("{t('用户名')}:");
    expect(headerSource).not.toContain("t('邀请人')}: {inviterLabel");
    expect(headerSource).not.toContain("t('无邀请人')");
    expect(headerSource).not.toContain('info.email');
    expect(headerSource).not.toContain('info.role_label');
    expect(headerSource).not.toContain('info.aff_code');
    expect(headerSource).not.toContain("t('邀请来源')");
    expect(editorSource).toContain('<Modal');
    expect(editorSource).toContain("type='file'");
    expect(editorSource).toContain('new AbortController()');
    expect(editorSource).toContain("setSavePhase('uploading')");
    expect(editorSource).toContain("t('取消上传')");
    expect(editorSource).not.toContain('<Card');
    expect(editorSource).not.toContain("t('头像 URL')");
  });

  test('nickname cooldown leaves avatar controls enabled', async () => {
    const source = await Bun.file(
      new URL('./cards/ProfileIdentity.jsx', import.meta.url),
    ).text();

    expect(source).toContain('disabled={saving || coolingDown}');
    expect(source).toContain(
      'disabled={saving || !profile.avatar_upload?.enabled}',
    );
    expect(source).toContain('disabled={saving}');
  });

  test('checks nickname cooldown before uploading a selected avatar', async () => {
    const source = await Bun.file(
      new URL('./cards/ProfileIdentity.jsx', import.meta.url),
    ).text();
    const submitSource = source.slice(
      source.indexOf('const saveProfile = async () =>'),
      source.indexOf(
        '\n  const cooldownNotice',
        source.indexOf('const saveProfile = async () =>'),
      ),
    );

    expect(submitSource.indexOf('if (coolingDown')).toBeGreaterThanOrEqual(0);
    expect(submitSource.indexOf('if (coolingDown')).toBeLessThan(
      submitSource.indexOf('uploadAvatarToImageHost('),
    );
  });
});
