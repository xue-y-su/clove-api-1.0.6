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

import React, { useEffect, useState } from 'react';
import {
  Button,
  Card,
  Divider,
  Form,
  InputNumber,
  Space,
  Spin,
  Switch,
  Typography,
} from '@douyinfe/semi-ui';
import { Gift, KeyRound, ShieldCheck, UserRound } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { API, showError, showSuccess } from '../../helpers';

const defaultSettings = {
  api_key: { allow_custom_keys: true },
  user_governance: {
    avatar_upload_enabled: true,
    avatar_upload_endpoint: 'https://img.scdn.io/api/v1.php',
    avatar_allowed_domains: ['img.scdn.io', 'img.cdn1.vip'],
    display_name_max_length: 40,
    display_name_cooldown_seconds: 7 * 24 * 60 * 60,
  },
  ip_security: {
    enabled: true,
    retention_days: 90,
    projection_interval_seconds: 60,
    projection_batch_size: 500,
    shared_registration_user_threshold: 3,
    shared_api_user_threshold: 5,
    unauthorized_threshold: 10,
  },
  user_redemption: {
    enabled: true,
    single_enabled: true,
    random_enabled: true,
    subscription_enabled: true,
    max_active_codes_per_user: 0,
    max_claims_per_code: 100,
    max_expiry_hours: 720,
  },
};

const FeatureSecuritySetting = () => {
  const { t } = useTranslation();
  const [settings, setSettings] = useState(defaultSettings);
  const [avatarDomains, setAvatarDomains] = useState(
    defaultSettings.user_governance.avatar_allowed_domains.join('\n'),
  );
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const loadSettings = async () => {
    setLoading(true);
    setLoaded(false);
    try {
      const response = await API.get('/api/security/settings');
      if (!response.data?.success || !response.data?.data) {
        showError(
          response.data?.message || t('Failed to load security settings'),
        );
        return;
      }
      setSettings(response.data.data);
      setAvatarDomains(
        (response.data.data.user_governance.avatar_allowed_domains || []).join(
          '\n',
        ),
      );
      setLoaded(true);
    } catch (error) {
      showError(error?.message || t('Failed to load security settings'));
    } finally {
      setLoading(false);
    }
  };

  const updateAPIKey = (allowCustomKeys) => {
    setSettings((current) => ({
      ...current,
      api_key: { allow_custom_keys: allowCustomKeys },
    }));
  };

  const updateUserGovernance = (key, value) => {
    setSettings((current) => ({
      ...current,
      user_governance: { ...current.user_governance, [key]: value },
    }));
  };

  const updateIPSecurity = (key, value) => {
    setSettings((current) => ({
      ...current,
      ip_security: { ...current.ip_security, [key]: value },
    }));
  };

  const updateUserRedemption = (key, value) => {
    setSettings((current) => ({
      ...current,
      user_redemption: { ...current.user_redemption, [key]: value },
    }));
  };

  const saveSettings = async () => {
    if (!loaded) return;
    setSaving(true);
    try {
      const payload = {
        ...settings,
        user_governance: {
          ...settings.user_governance,
          avatar_allowed_domains: avatarDomains
            .split(/[\n,]/)
            .map((domain) => domain.trim())
            .filter(Boolean),
        },
      };
      const response = await API.put('/api/security/settings', payload);
      if (!response.data?.success || !response.data?.data) {
        showError(response.data?.message || t('Failed to update settings'));
        return;
      }
      setSettings(response.data.data);
      setAvatarDomains(
        (response.data.data.user_governance.avatar_allowed_domains || []).join(
          '\n',
        ),
      );
      showSuccess(t('Security settings saved'));
    } catch (error) {
      showError(error?.message || t('Failed to update settings'));
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    loadSettings();
  }, []);

  return (
    <Spin spinning={loading} size='large'>
      <Card
        title={
          <Space>
            <ShieldCheck size={18} />
            {t('Feature Controls')}
          </Space>
        }
      >
        <Form labelPosition='left' labelWidth={240}>
          <Form.Section text={t('Custom API Keys')}>
            <div className='mb-3 flex items-start gap-2 text-semi-color-text-2'>
              <KeyRound size={16} className='mt-0.5 shrink-0' />
              <Typography.Text type='tertiary'>
                {t(
                  'Control whether users may choose a custom value when creating an API Key.',
                )}
              </Typography.Text>
            </div>
            <Form.Slot label={t('Allow custom API Key creation')}>
              <Switch
                aria-label={t('Allow custom API Key creation')}
                checked={settings.api_key.allow_custom_keys}
                onChange={updateAPIKey}
                disabled={!loaded}
              />
            </Form.Slot>
          </Form.Section>

          <Divider margin='20px' />

          <Form.Section text={t('User redemption codes')}>
            <div className='mb-3 flex items-start gap-2 text-semi-color-text-2'>
              <Gift size={16} className='mt-0.5 shrink-0' />
              <Typography.Text type='tertiary'>
                {t(
                  'Control which user-funded redemption codes can be created and limit their lifetime and recipients.',
                )}
              </Typography.Text>
            </div>
            <Form.Slot label={t('Allow users to create redemption codes')}>
              <Switch
                aria-label={t('Allow users to create redemption codes')}
                checked={settings.user_redemption.enabled}
                onChange={(checked) => updateUserRedemption('enabled', checked)}
                disabled={!loaded}
              />
            </Form.Slot>
            <Form.Slot label={t('Allow single-recipient codes')}>
              <Switch
                aria-label={t('Allow single-recipient codes')}
                checked={settings.user_redemption.single_enabled}
                onChange={(checked) =>
                  updateUserRedemption('single_enabled', checked)
                }
                disabled={!loaded}
              />
            </Form.Slot>
            <Form.Slot label={t('Allow red packets')}>
              <Switch
                aria-label={t('Allow red packets')}
                checked={settings.user_redemption.random_enabled}
                onChange={(checked) =>
                  updateUserRedemption('random_enabled', checked)
                }
                disabled={!loaded}
              />
            </Form.Slot>
            <Form.Slot label={t('Allow subscription codes')}>
              <Switch
                aria-label={t('Allow subscription codes')}
                checked={settings.user_redemption.subscription_enabled}
                onChange={(checked) =>
                  updateUserRedemption('subscription_enabled', checked)
                }
                disabled={!loaded}
              />
            </Form.Slot>
            <Form.Slot label={t('Maximum active codes per user')}>
              <InputNumber
                value={settings.user_redemption.max_active_codes_per_user}
                min={0}
                max={1000}
                onChange={(value) =>
                  updateUserRedemption(
                    'max_active_codes_per_user',
                    Number(value) || 0,
                  )
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
            <Form.Slot label={t('Maximum red packet recipients')}>
              <InputNumber
                value={settings.user_redemption.max_claims_per_code}
                min={2}
                max={100}
                onChange={(value) =>
                  updateUserRedemption(
                    'max_claims_per_code',
                    Number(value) || 2,
                  )
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
            <Form.Slot label={t('Maximum validity (hours)')}>
              <InputNumber
                value={settings.user_redemption.max_expiry_hours}
                min={1}
                max={720}
                onChange={(value) =>
                  updateUserRedemption('max_expiry_hours', Number(value) || 1)
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
          </Form.Section>

          <Divider margin='20px' />

          <Form.Section text={t('User profile policy')}>
            <div className='mb-3 flex items-start gap-2 text-semi-color-text-2'>
              <UserRound size={16} className='mt-0.5 shrink-0' />
              <Typography.Text type='tertiary'>
                {t('Configure avatar uploads and display name changes.')}
              </Typography.Text>
            </div>
            <Form.Slot label={t('Enable avatar uploads')}>
              <Switch
                aria-label={t('Enable avatar uploads')}
                checked={settings.user_governance.avatar_upload_enabled}
                onChange={(checked) =>
                  updateUserGovernance('avatar_upload_enabled', checked)
                }
                disabled={!loaded}
              />
            </Form.Slot>
            <Form.Input
              field='avatar_upload_endpoint'
              label={t('Avatar upload endpoint')}
              value={settings.user_governance.avatar_upload_endpoint}
              onChange={(value) =>
                updateUserGovernance('avatar_upload_endpoint', value)
              }
              disabled={
                !loaded || !settings.user_governance.avatar_upload_enabled
              }
            />
            <Form.TextArea
              field='avatar_allowed_domains'
              label={t('Allowed avatar domains')}
              extraText={t('One domain per line')}
              value={avatarDomains}
              onChange={setAvatarDomains}
              rows={3}
              disabled={
                !loaded || !settings.user_governance.avatar_upload_enabled
              }
            />
            <Form.Slot label={t('Maximum display name length')}>
              <InputNumber
                value={settings.user_governance.display_name_max_length}
                min={1}
                max={100}
                onChange={(value) =>
                  updateUserGovernance(
                    'display_name_max_length',
                    Number(value) || 1,
                  )
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
            <Form.Slot label={t('Display name cooldown (seconds)')}>
              <InputNumber
                value={settings.user_governance.display_name_cooldown_seconds}
                min={0}
                max={365 * 24 * 60 * 60}
                step={60}
                onChange={(value) =>
                  updateUserGovernance(
                    'display_name_cooldown_seconds',
                    Number(value) || 0,
                  )
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
          </Form.Section>

          <Divider margin='20px' />

          <Form.Section text={t('IP Security Audit')}>
            <Form.Slot label={t('Enable IP security event collection')}>
              <Switch
                aria-label={t('Enable IP security event collection')}
                checked={settings.ip_security.enabled}
                onChange={(checked) => updateIPSecurity('enabled', checked)}
                disabled={!loaded}
              />
            </Form.Slot>
            <Form.Slot label={t('Retention days')}>
              <InputNumber
                value={settings.ip_security.retention_days}
                min={0}
                max={3650}
                onChange={(value) =>
                  updateIPSecurity('retention_days', Number(value) || 0)
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
            <Form.Slot label={t('Projection interval (seconds)')}>
              <InputNumber
                value={settings.ip_security.projection_interval_seconds}
                min={5}
                max={3600}
                onChange={(value) =>
                  updateIPSecurity(
                    'projection_interval_seconds',
                    Number(value) || 5,
                  )
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
            <Form.Slot label={t('Projection batch size')}>
              <InputNumber
                value={settings.ip_security.projection_batch_size}
                min={10}
                max={5000}
                onChange={(value) =>
                  updateIPSecurity('projection_batch_size', Number(value) || 10)
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
            <Form.Slot label={t('Shared registration user threshold')}>
              <InputNumber
                value={settings.ip_security.shared_registration_user_threshold}
                min={1}
                max={100000}
                onChange={(value) =>
                  updateIPSecurity(
                    'shared_registration_user_threshold',
                    Number(value) || 1,
                  )
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
            <Form.Slot label={t('Shared API user threshold')}>
              <InputNumber
                value={settings.ip_security.shared_api_user_threshold}
                min={1}
                max={100000}
                onChange={(value) =>
                  updateIPSecurity(
                    'shared_api_user_threshold',
                    Number(value) || 1,
                  )
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
            <Form.Slot label={t('Unauthorized request threshold')}>
              <InputNumber
                value={settings.ip_security.unauthorized_threshold}
                min={1}
                max={100000}
                onChange={(value) =>
                  updateIPSecurity('unauthorized_threshold', Number(value) || 1)
                }
                disabled={!loaded}
                style={{ width: '100%' }}
              />
            </Form.Slot>
          </Form.Section>

          <Button
            type='primary'
            theme='solid'
            loading={saving}
            disabled={!loaded}
            onClick={saveSettings}
          >
            {t('Save security settings')}
          </Button>
        </Form>
      </Card>
    </Spin>
  );
};

export default FeatureSecuritySetting;
