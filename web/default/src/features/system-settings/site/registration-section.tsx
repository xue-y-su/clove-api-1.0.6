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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  countPosterCharacters,
  limitPosterCharacters,
} from '@/features/invitations/lib/poster-preferences'
import {
  getRegistrationSettings,
  updateRegistrationSettings,
} from '@/features/source-attribution/api'

import {
  SettingsForm,
  SettingsFormGridItem,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

const INVITATION_POSTER_SITE_NAME_TOKEN = '{\u7ad9\u70b9\u540d\u79f0}'

export function RegistrationSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const settingsQuery = useQuery({
    queryKey: ['registration-settings'],
    queryFn: getRegistrationSettings,
  })
  const [affRequired, setAffRequired] = useState(false)
  const [invitationApplicationsEnabled, setInvitationApplicationsEnabled] =
    useState(true)
  const [
    invitationReapplyCooldownSeconds,
    setInvitationReapplyCooldownSeconds,
  ] = useState(24 * 60 * 60)
  const [invitationPosterTitle, setInvitationPosterTitle] = useState('')
  const [invitationPosterSubtitle, setInvitationPosterSubtitle] = useState('')
  const updateMutation = useMutation({
    mutationFn: updateRegistrationSettings,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to update settings'))
        return
      }
      queryClient.invalidateQueries({ queryKey: ['registration-settings'] })
      queryClient.invalidateQueries({ queryKey: ['status'] })
      toast.success(t('Setting updated successfully'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to update settings'))
    },
  })

  useEffect(() => {
    const settings = settingsQuery.data?.data
    setAffRequired(settings?.aff_required ?? false)
    setInvitationApplicationsEnabled(
      settings?.invitation_applications_enabled ?? true
    )
    setInvitationReapplyCooldownSeconds(
      settings?.invitation_reapply_cooldown_seconds ?? 24 * 60 * 60
    )
    setInvitationPosterTitle(settings?.invitation_poster_title ?? '')
    setInvitationPosterSubtitle(settings?.invitation_poster_subtitle ?? '')
  }, [settingsQuery.data?.data])

  const handleSave = () => {
    const settings = settingsQuery.data?.data
    if (!settings) return
    updateMutation.mutate({
      aff_required: affRequired,
      invite_limit_unlimited: settings?.invite_limit_unlimited ?? true,
      invite_limit_count: settings?.invite_limit_count ?? 0,
      invitation_applications_enabled: invitationApplicationsEnabled,
      invitation_reapply_cooldown_seconds: invitationReapplyCooldownSeconds,
      invitation_poster_title: invitationPosterTitle,
      invitation_poster_subtitle: invitationPosterSubtitle,
    })
  }

  return (
    <SettingsSection title={t('Invitation Registration')}>
      <SettingsForm
        onSubmit={(event) => {
          event.preventDefault()
          handleSave()
        }}
      >
        <SettingsPageFormActions
          onSave={handleSave}
          isSaving={updateMutation.isPending}
          isSaveDisabled={!settingsQuery.data?.data}
          saveLabel='Save registration settings'
        />
        <SettingsSwitchField
          checked={affRequired}
          onCheckedChange={setAffRequired}
          label={t('Require approved invitation code for registration')}
          description={t(
            'When enabled, registration requires an active invitation code issued through the application or administrator issuance workflow.'
          )}
          disabled={!settingsQuery.data?.data || settingsQuery.isLoading}
        />
        <SettingsSwitchField
          checked={invitationApplicationsEnabled}
          onCheckedChange={setInvitationApplicationsEnabled}
          label={t('Allow users to apply for invitation codes')}
          description={t(
            'When disabled, users cannot submit new invitation applications. Administrators can still issue and manage codes.'
          )}
          disabled={!settingsQuery.data?.data || settingsQuery.isLoading}
        />
        <SettingsFormGridItem span='full' className='grid max-w-md gap-2'>
          <Label htmlFor='invitation-reapply-cooldown-seconds'>
            {t('Invitation reapply cooldown (seconds)')}
          </Label>
          <Input
            id='invitation-reapply-cooldown-seconds'
            type='number'
            min={0}
            max={30 * 24 * 60 * 60}
            step={60}
            value={invitationReapplyCooldownSeconds}
            onChange={(event) => {
              const value = Number(event.target.value)
              setInvitationReapplyCooldownSeconds(
                Number.isFinite(value)
                  ? Math.max(0, Math.min(30 * 24 * 60 * 60, Math.trunc(value)))
                  : 0
              )
            }}
            disabled={!settingsQuery.data?.data || settingsQuery.isLoading}
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'How long a rejected application must wait before the user can apply again. Set 0 for no cooldown.'
            )}
          </p>
        </SettingsFormGridItem>
        <SettingsFormGridItem span='full' className='grid max-w-xl gap-2'>
          <div className='flex items-center justify-between gap-3'>
            <Label htmlFor='invitation-poster-title'>
              {t('Invitation poster title')}
            </Label>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {countPosterCharacters(invitationPosterTitle)}/80
            </span>
          </div>
          <Input
            id='invitation-poster-title'
            value={invitationPosterTitle}
            onChange={(event) =>
              setInvitationPosterTitle(
                limitPosterCharacters(event.target.value, 80)
              )
            }
            placeholder={t('Invite you to join "{{siteName}}"', {
              siteName: INVITATION_POSTER_SITE_NAME_TOKEN,
            })}
            disabled={!settingsQuery.data?.data || settingsQuery.isLoading}
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'Shown on invitation posters. Use {\u7ad9\u70b9\u540d\u79f0} to insert the current site name.'
            )}
          </p>
        </SettingsFormGridItem>
        <SettingsFormGridItem span='full' className='grid max-w-xl gap-2'>
          <div className='flex items-center justify-between gap-3'>
            <Label htmlFor='invitation-poster-subtitle'>
              {t('Invitation poster subtitle')}
            </Label>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {countPosterCharacters(invitationPosterSubtitle)}/160
            </span>
          </div>
          <Textarea
            id='invitation-poster-subtitle'
            value={invitationPosterSubtitle}
            onChange={(event) =>
              setInvitationPosterSubtitle(
                limitPosterCharacters(event.target.value, 160)
              )
            }
            rows={3}
            placeholder={t(
              'Scan to register and fill in the invitation code automatically'
            )}
            disabled={!settingsQuery.data?.data || settingsQuery.isLoading}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Leave blank to use the localized default copy.')}
          </p>
        </SettingsFormGridItem>
      </SettingsForm>
    </SettingsSection>
  )
}
