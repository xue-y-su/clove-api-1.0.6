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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircle, KeyRound, ShieldCheck, UserRound } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import {
  getFeatureSecuritySettings,
  updateFeatureSecuritySettings,
} from '../api'
import {
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import type { FeatureSecuritySettings } from '../types'
import { UserRedemptionControls } from './user-redemption-controls'

const DEFAULT_SETTINGS: FeatureSecuritySettings = {
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
    shared_registration_user_threshold: 2,
    shared_api_user_threshold: 2,
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
}

function domainList(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

export function FeatureControlsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState(DEFAULT_SETTINGS)
  const [avatarDomains, setAvatarDomains] = useState(
    DEFAULT_SETTINGS.user_governance.avatar_allowed_domains.join('\n')
  )

  const settingsQuery = useQuery({
    queryKey: ['feature-security-settings'],
    queryFn: async () => {
      const response = await getFeatureSecuritySettings()
      if (!response.success || !response.data) {
        throw new Error(response.message || 'Failed to load security settings')
      }
      return response.data
    },
  })

  useEffect(() => {
    if (!settingsQuery.data) return
    setDraft(settingsQuery.data)
    setAvatarDomains(
      settingsQuery.data.user_governance.avatar_allowed_domains.join('\n')
    )
  }, [settingsQuery.data])

  const updateMutation = useMutation({
    mutationFn: async (settings: FeatureSecuritySettings) => {
      const response = await updateFeatureSecuritySettings(settings)
      if (!response.success || !response.data) {
        throw new Error(response.message || 'Failed to update settings')
      }
      return response.data
    },
    onSuccess: (settings) => {
      setDraft(settings)
      setAvatarDomains(
        settings.user_governance.avatar_allowed_domains.join('\n')
      )
      queryClient.setQueryData(['feature-security-settings'], settings)
      queryClient.invalidateQueries({ queryKey: ['status'] })
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Security settings saved'))
    },
    onError: (error: Error) => {
      toast.error(t(error.message || 'Failed to update settings'))
    },
  })

  const updateAPIKey = (allowCustomKeys: boolean) => {
    setDraft((current) => ({
      ...current,
      api_key: { allow_custom_keys: allowCustomKeys },
    }))
  }

  const updateUserGovernance = <
    K extends keyof FeatureSecuritySettings['user_governance'],
  >(
    key: K,
    value: FeatureSecuritySettings['user_governance'][K]
  ) => {
    setDraft((current) => ({
      ...current,
      user_governance: { ...current.user_governance, [key]: value },
    }))
  }

  const updateIPSecurity = <
    K extends keyof FeatureSecuritySettings['ip_security'],
  >(
    key: K,
    value: FeatureSecuritySettings['ip_security'][K]
  ) => {
    setDraft((current) => ({
      ...current,
      ip_security: { ...current.ip_security, [key]: value },
    }))
  }

  const updateUserRedemption = (
    value: FeatureSecuritySettings['user_redemption']
  ) => {
    setDraft((current) => ({ ...current, user_redemption: value }))
  }

  const resetDraft = () => {
    const settings = settingsQuery.data ?? DEFAULT_SETTINGS
    setDraft(settings)
    setAvatarDomains(settings.user_governance.avatar_allowed_domains.join('\n'))
  }

  const saveDraft = () => {
    updateMutation.mutate({
      ...draft,
      user_governance: {
        ...draft.user_governance,
        avatar_allowed_domains: domainList(avatarDomains),
      },
    })
  }

  const disabled = settingsQuery.isLoading || !settingsQuery.data

  return (
    <SettingsSection title={t('Feature Controls')}>
      <SettingsForm
        onSubmit={(event) => {
          event.preventDefault()
          saveDraft()
        }}
      >
        <SettingsPageFormActions
          onSave={saveDraft}
          onReset={resetDraft}
          isSaving={updateMutation.isPending}
          isSaveDisabled={disabled}
          saveLabel={t('Save security settings')}
        />

        {settingsQuery.isError ? (
          <Alert variant='destructive'>
            <AlertCircle />
            <AlertTitle>{t('Failed to load security settings')}</AlertTitle>
            <AlertDescription>
              {t(
                settingsQuery.error instanceof Error
                  ? settingsQuery.error.message
                  : 'Please try again.'
              )}
            </AlertDescription>
          </Alert>
        ) : null}

        <SettingsControlGroup>
          <div className='flex items-start gap-2'>
            <KeyRound className='mt-0.5 size-4 shrink-0' />
            <div className='min-w-0'>
              <h3 className='text-sm font-medium'>{t('Custom API Keys')}</h3>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Control whether users may choose a custom value when creating an API Key.'
                )}
              </p>
            </div>
          </div>
          <SettingsControlChildren>
            <SettingsSwitchField
              checked={draft.api_key.allow_custom_keys}
              onCheckedChange={updateAPIKey}
              label={t('Allow custom API Key creation')}
              disabled={disabled}
            />
          </SettingsControlChildren>
        </SettingsControlGroup>

        <UserRedemptionControls
          value={draft.user_redemption}
          disabled={disabled}
          onChange={updateUserRedemption}
        />

        <SettingsControlGroup>
          <div className='flex items-start gap-2'>
            <UserRound className='mt-0.5 size-4 shrink-0' />
            <div className='min-w-0'>
              <h3 className='text-sm font-medium'>
                {t('User profile policy')}
              </h3>
              <p className='text-muted-foreground text-xs'>
                {t('Configure avatar uploads and display name changes.')}
              </p>
            </div>
          </div>
          <SettingsControlChildren className='space-y-4'>
            <SettingsSwitchField
              checked={draft.user_governance.avatar_upload_enabled}
              onCheckedChange={(checked) =>
                updateUserGovernance('avatar_upload_enabled', checked)
              }
              label={t('Enable avatar uploads')}
              disabled={disabled}
            />
            <SettingsFormGrid>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='avatar-upload-endpoint'>
                    {t('Avatar upload endpoint')}
                  </Label>
                  <Input
                    id='avatar-upload-endpoint'
                    type='url'
                    value={draft.user_governance.avatar_upload_endpoint}
                    onChange={(event) =>
                      updateUserGovernance(
                        'avatar_upload_endpoint',
                        event.target.value
                      )
                    }
                    disabled={
                      disabled || !draft.user_governance.avatar_upload_enabled
                    }
                  />
                </div>
              </SettingsFormGridItem>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='avatar-allowed-domains'>
                    {t('Allowed avatar domains')}
                  </Label>
                  <Textarea
                    id='avatar-allowed-domains'
                    rows={3}
                    value={avatarDomains}
                    onChange={(event) => setAvatarDomains(event.target.value)}
                    placeholder={t('One domain per line')}
                    disabled={
                      disabled || !draft.user_governance.avatar_upload_enabled
                    }
                  />
                </div>
              </SettingsFormGridItem>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='display-name-max-length'>
                    {t('Maximum display name length')}
                  </Label>
                  <Input
                    id='display-name-max-length'
                    type='number'
                    min={1}
                    max={100}
                    value={draft.user_governance.display_name_max_length}
                    onChange={(event) =>
                      updateUserGovernance(
                        'display_name_max_length',
                        Number(event.target.value)
                      )
                    }
                    disabled={disabled}
                  />
                </div>
              </SettingsFormGridItem>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='display-name-cooldown'>
                    {t('Display name cooldown (seconds)')}
                  </Label>
                  <Input
                    id='display-name-cooldown'
                    type='number'
                    min={0}
                    max={365 * 24 * 60 * 60}
                    step={60}
                    value={draft.user_governance.display_name_cooldown_seconds}
                    onChange={(event) =>
                      updateUserGovernance(
                        'display_name_cooldown_seconds',
                        Number(event.target.value)
                      )
                    }
                    disabled={disabled}
                  />
                </div>
              </SettingsFormGridItem>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='ip-shared-registration-threshold'>
                    {t('Shared registration user threshold')}
                  </Label>
                  <Input
                    id='ip-shared-registration-threshold'
                    type='number'
                    min={1}
                    max={100000}
                    value={draft.ip_security.shared_registration_user_threshold}
                    onChange={(event) =>
                      updateIPSecurity(
                        'shared_registration_user_threshold',
                        Number(event.target.value)
                      )
                    }
                    disabled={disabled}
                  />
                </div>
              </SettingsFormGridItem>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='ip-shared-api-threshold'>
                    {t('Shared API user threshold')}
                  </Label>
                  <Input
                    id='ip-shared-api-threshold'
                    type='number'
                    min={1}
                    max={100000}
                    value={draft.ip_security.shared_api_user_threshold}
                    onChange={(event) =>
                      updateIPSecurity(
                        'shared_api_user_threshold',
                        Number(event.target.value)
                      )
                    }
                    disabled={disabled}
                  />
                </div>
              </SettingsFormGridItem>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='ip-unauthorized-threshold'>
                    {t('Unauthorized request threshold')}
                  </Label>
                  <Input
                    id='ip-unauthorized-threshold'
                    type='number'
                    min={1}
                    max={100000}
                    value={draft.ip_security.unauthorized_threshold}
                    onChange={(event) =>
                      updateIPSecurity(
                        'unauthorized_threshold',
                        Number(event.target.value)
                      )
                    }
                    disabled={disabled}
                  />
                </div>
              </SettingsFormGridItem>
            </SettingsFormGrid>
          </SettingsControlChildren>
        </SettingsControlGroup>

        <SettingsControlGroup>
          <div className='flex items-start gap-2'>
            <ShieldCheck className='mt-0.5 size-4 shrink-0' />
            <div className='min-w-0'>
              <h3 className='text-sm font-medium'>{t('IP Security Audit')}</h3>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Configure event collection, retention, and projection worker limits.'
                )}
              </p>
            </div>
          </div>
          <SettingsControlChildren className='space-y-4'>
            <SettingsSwitchField
              checked={draft.ip_security.enabled}
              onCheckedChange={(checked) =>
                updateIPSecurity('enabled', checked)
              }
              label={t('Enable IP security event collection')}
              disabled={disabled}
            />
            <SettingsFormGrid>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='ip-retention-days'>
                    {t('Retention days')}
                  </Label>
                  <Input
                    id='ip-retention-days'
                    type='number'
                    min={0}
                    max={3650}
                    value={draft.ip_security.retention_days}
                    onChange={(event) =>
                      updateIPSecurity(
                        'retention_days',
                        Number(event.target.value)
                      )
                    }
                    disabled={disabled}
                  />
                </div>
              </SettingsFormGridItem>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='ip-projection-interval'>
                    {t('Projection interval (seconds)')}
                  </Label>
                  <Input
                    id='ip-projection-interval'
                    type='number'
                    min={5}
                    max={3600}
                    value={draft.ip_security.projection_interval_seconds}
                    onChange={(event) =>
                      updateIPSecurity(
                        'projection_interval_seconds',
                        Number(event.target.value)
                      )
                    }
                    disabled={disabled}
                  />
                </div>
              </SettingsFormGridItem>
              <SettingsFormGridItem>
                <div className='grid gap-2'>
                  <Label htmlFor='ip-projection-batch'>
                    {t('Projection batch size')}
                  </Label>
                  <Input
                    id='ip-projection-batch'
                    type='number'
                    min={10}
                    max={5000}
                    value={draft.ip_security.projection_batch_size}
                    onChange={(event) =>
                      updateIPSecurity(
                        'projection_batch_size',
                        Number(event.target.value)
                      )
                    }
                    disabled={disabled}
                  />
                </div>
              </SettingsFormGridItem>
            </SettingsFormGrid>
          </SettingsControlChildren>
        </SettingsControlGroup>
      </SettingsForm>
    </SettingsSection>
  )
}
