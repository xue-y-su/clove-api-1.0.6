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
import { readFileSync } from 'node:fs';

import {
  buildRegistrationSettingsPayload,
  countPosterCharacters,
  limitPosterCharacters,
  normalizeRegistrationSettings,
} from './registration-settings';

describe('classic registration settings', () => {
  test('renders loaded registration booleans with controlled switches', () => {
    const source = readFileSync(
      new URL('./RegistrationSetting.jsx', import.meta.url),
      'utf8',
    );

    expect(source).toContain('checked={affRequired}');
    expect(source).toContain('checked={invitationApplicationsEnabled}');
    expect(source).not.toContain('<Form.Switch');
    expect(source).toContain('statusDispatch');
    expect(source).toContain(
      'invitation_poster_title: invitationPosterTitle.trim()',
    );
  });

  test('keeps zero as a finite invitation limit', () => {
    expect(
      normalizeRegistrationSettings({
        aff_required: true,
        invite_limit_unlimited: false,
        invite_limit_count: 0,
        invitation_applications_enabled: false,
        invitation_reapply_cooldown_seconds: 1800,
      }),
    ).toEqual({
      affRequired: true,
      inviteLimitUnlimited: false,
      inviteLimitCount: 0,
      invitationApplicationsEnabled: false,
      invitationReapplyCooldownSeconds: 1800,
      invitationPosterTitle: '',
      invitationPosterSubtitle: '',
    });
  });

  test('uses unlimited only when the explicit switch is enabled', () => {
    expect(
      buildRegistrationSettingsPayload({
        affRequired: false,
        inviteLimitUnlimited: true,
        inviteLimitCount: 12,
        invitationApplicationsEnabled: true,
        invitationReapplyCooldownSeconds: 7200,
        invitationPosterTitle: '',
        invitationPosterSubtitle: '',
      }),
    ).toEqual({
      aff_required: false,
      invite_limit_unlimited: true,
      invite_limit_count: 12,
      invitation_applications_enabled: true,
      invitation_reapply_cooldown_seconds: 7200,
      invitation_poster_title: '',
      invitation_poster_subtitle: '',
    });
  });

  test('round-trips poster copy and limits it by Unicode code point', () => {
    const normalized = normalizeRegistrationSettings({
      invitation_poster_title: '  加入 {站点名称}  ',
      invitation_poster_subtitle: '  扫码注册  ',
    });
    expect(normalized.invitationPosterTitle).toBe('加入 {站点名称}');
    expect(normalized.invitationPosterSubtitle).toBe('扫码注册');
    expect(buildRegistrationSettingsPayload(normalized)).toMatchObject({
      invitation_poster_title: '加入 {站点名称}',
      invitation_poster_subtitle: '扫码注册',
    });
    expect(countPosterCharacters('邀请😜')).toBe(3);
    expect(limitPosterCharacters('邀请😜加入', 4)).toBe('邀请😜加');
  });
});
