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

import React, { useContext, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Space,
  Switch,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { UserPlus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { StatusContext } from '../../context/Status';
import {
  buildRegistrationSettingsPayload,
  countPosterCharacters,
  INVITATION_POSTER_SUBTITLE_MAX_CHARACTERS,
  INVITATION_POSTER_TITLE_MAX_CHARACTERS,
  limitPosterCharacters,
  MAX_INVITATION_REAPPLY_COOLDOWN_SECONDS,
  normalizeRegistrationSettings,
} from './registration-settings';

const RegistrationSetting = () => {
  const { t } = useTranslation();
  const [statusState, statusDispatch] = useContext(StatusContext);
  const [affRequired, setAffRequired] = useState(false);
  const [inviteLimitUnlimited, setInviteLimitUnlimited] = useState(true);
  const [inviteLimitCount, setInviteLimitCount] = useState(0);
  const [invitationApplicationsEnabled, setInvitationApplicationsEnabled] =
    useState(true);
  const [
    invitationReapplyCooldownSeconds,
    setInvitationReapplyCooldownSeconds,
  ] = useState(24 * 60 * 60);
  const [invitationPosterTitle, setInvitationPosterTitle] = useState('');
  const [invitationPosterSubtitle, setInvitationPosterSubtitle] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [settingsLoaded, setSettingsLoaded] = useState(false);

  const loadSettings = async () => {
    setLoading(true);
    setSettingsLoaded(false);
    try {
      const res = await API.get('/api/registration/settings');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const settings = normalizeRegistrationSettings(data);
      setAffRequired(settings.affRequired);
      setInviteLimitUnlimited(settings.inviteLimitUnlimited);
      setInviteLimitCount(settings.inviteLimitCount);
      setInvitationApplicationsEnabled(settings.invitationApplicationsEnabled);
      setInvitationReapplyCooldownSeconds(
        settings.invitationReapplyCooldownSeconds,
      );
      setInvitationPosterTitle(settings.invitationPosterTitle);
      setInvitationPosterSubtitle(settings.invitationPosterSubtitle);
      setSettingsLoaded(true);
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const saveSettings = async () => {
    if (!settingsLoaded) return;
    setSaving(true);
    try {
      const res = await API.put(
        '/api/registration/settings',
        buildRegistrationSettingsPayload({
          affRequired,
          inviteLimitUnlimited,
          inviteLimitCount,
          invitationApplicationsEnabled,
          invitationReapplyCooldownSeconds,
          invitationPosterTitle,
          invitationPosterSubtitle,
        }),
      );
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const nextStatus = {
        ...(statusState?.status || {}),
        invitation_poster_title: invitationPosterTitle.trim(),
        invitation_poster_subtitle: invitationPosterSubtitle.trim(),
      };
      statusDispatch({ type: 'set', payload: nextStatus });
      try {
        localStorage.setItem('status', JSON.stringify(nextStatus));
      } catch {
        // The in-memory status remains current when storage is unavailable.
      }
      showSuccess(t('注册设置已保存'));
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    loadSettings();
  }, []);

  return (
    <Card
      title={
        <Space>
          <UserPlus size={18} />
          {t('邀请注册')}
        </Space>
      }
      loading={loading}
    >
      <Form labelPosition='left' labelWidth={220}>
        <Form.Slot label={t('注册必须填写邀请码')}>
          <Switch
            aria-label={t('注册必须填写邀请码')}
            checked={affRequired}
            onChange={setAffRequired}
            disabled={!settingsLoaded}
          />
        </Form.Slot>
        <Form.Slot
          label={t('Allow users to apply for invitation codes')}
          extraText={t(
            'When disabled, users cannot submit new invitation applications. Administrators can still issue and manage codes.',
          )}
        >
          <Switch
            aria-label={t('Allow users to apply for invitation codes')}
            checked={invitationApplicationsEnabled}
            onChange={setInvitationApplicationsEnabled}
            disabled={!settingsLoaded}
          />
        </Form.Slot>
        <Form.Slot
          label={t('Invitation reapply cooldown (seconds)')}
          extraText={t(
            'How long a rejected application must wait before the user can apply again. Set 0 for no cooldown.',
          )}
        >
          <InputNumber
            value={invitationReapplyCooldownSeconds}
            min={0}
            max={MAX_INVITATION_REAPPLY_COOLDOWN_SECONDS}
            step={60}
            onChange={(value) =>
              setInvitationReapplyCooldownSeconds(Number(value) || 0)
            }
            disabled={!settingsLoaded}
            style={{ width: '100%' }}
          />
        </Form.Slot>
        <Form.Slot
          label={t('Invitation poster title')}
          extraText={t(
            'Shown on invitation posters. Use {站点名称} to insert the current site name.',
          )}
        >
          <div className='space-y-1'>
            <Input
              value={invitationPosterTitle}
              onChange={(value) =>
                setInvitationPosterTitle(
                  limitPosterCharacters(
                    value,
                    INVITATION_POSTER_TITLE_MAX_CHARACTERS,
                  ),
                )
              }
              placeholder={t('Invite you to join "{{siteName}}"', {
                siteName: '{站点名称}',
              })}
              disabled={!settingsLoaded}
            />
            <Typography.Text type='tertiary' size='small'>
              {countPosterCharacters(invitationPosterTitle)}/
              {INVITATION_POSTER_TITLE_MAX_CHARACTERS}
            </Typography.Text>
          </div>
        </Form.Slot>
        <Form.Slot
          label={t('Invitation poster subtitle')}
          extraText={t('Leave blank to use the localized default copy.')}
        >
          <div className='space-y-1'>
            <TextArea
              value={invitationPosterSubtitle}
              onChange={(value) =>
                setInvitationPosterSubtitle(
                  limitPosterCharacters(
                    value,
                    INVITATION_POSTER_SUBTITLE_MAX_CHARACTERS,
                  ),
                )
              }
              rows={3}
              placeholder={t(
                'Scan to register and fill in the invitation code automatically',
              )}
              disabled={!settingsLoaded}
            />
            <Typography.Text type='tertiary' size='small'>
              {countPosterCharacters(invitationPosterSubtitle)}/
              {INVITATION_POSTER_SUBTITLE_MAX_CHARACTERS}
            </Typography.Text>
          </div>
        </Form.Slot>
        <Button
          type='primary'
          theme='solid'
          loading={saving}
          disabled={!settingsLoaded}
          onClick={saveSettings}
        >
          {t('保存注册设置')}
        </Button>
      </Form>
    </Card>
  );
};

export default RegistrationSetting;
