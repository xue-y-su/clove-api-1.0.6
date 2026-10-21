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
import { Gift } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import {
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsFormGrid,
  SettingsFormGridItem,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import type { FeatureSecuritySettings } from '../types'

type UserRedemptionSettings = FeatureSecuritySettings['user_redemption']

interface UserRedemptionControlsProps {
  value: UserRedemptionSettings
  disabled: boolean
  onChange: (value: UserRedemptionSettings) => void
}

export function UserRedemptionControls(props: UserRedemptionControlsProps) {
  const { t } = useTranslation()

  const update = <K extends keyof UserRedemptionSettings>(
    key: K,
    value: UserRedemptionSettings[K]
  ) => {
    props.onChange({ ...props.value, [key]: value })
  }

  return (
    <SettingsControlGroup>
      <div className='flex items-start gap-2'>
        <Gift className='mt-0.5 size-4 shrink-0' />
        <div className='min-w-0'>
          <h3 className='text-sm font-medium'>{t('User redemption codes')}</h3>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Control which user-funded redemption codes can be created and limit their lifetime and recipients.'
            )}
          </p>
        </div>
      </div>
      <SettingsControlChildren className='space-y-4'>
        <SettingsSwitchField
          checked={props.value.enabled}
          onCheckedChange={(checked) => update('enabled', checked)}
          label={t('Allow users to create redemption codes')}
          description={t(
            'Existing codes remain claimable and manageable when new creation is disabled.'
          )}
          disabled={props.disabled}
        />
        <div className='grid gap-3 sm:grid-cols-3'>
          <SettingsSwitchField
            checked={props.value.single_enabled}
            onCheckedChange={(checked) => update('single_enabled', checked)}
            label={t('Allow single-recipient codes')}
            disabled={props.disabled}
          />
          <SettingsSwitchField
            checked={props.value.random_enabled}
            onCheckedChange={(checked) => update('random_enabled', checked)}
            label={t('Allow red packets')}
            disabled={props.disabled}
          />
          <SettingsSwitchField
            checked={props.value.subscription_enabled}
            onCheckedChange={(checked) =>
              update('subscription_enabled', checked)
            }
            label={t('Allow subscription codes')}
            disabled={props.disabled}
          />
        </div>
        <SettingsFormGrid>
          <SettingsFormGridItem>
            <div className='grid gap-2'>
              <Label htmlFor='user-redemption-active-limit'>
                {t('Maximum active codes per user')}
              </Label>
              <Input
                id='user-redemption-active-limit'
                type='number'
                min={0}
                max={1000}
                value={props.value.max_active_codes_per_user}
                onChange={(event) =>
                  update(
                    'max_active_codes_per_user',
                    Number(event.target.value)
                  )
                }
                disabled={props.disabled}
              />
              <p className='text-muted-foreground text-xs'>
                {t('Set 0 for no active-code limit.')}
              </p>
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <div className='grid gap-2'>
              <Label htmlFor='user-redemption-claim-limit'>
                {t('Maximum red packet recipients')}
              </Label>
              <Input
                id='user-redemption-claim-limit'
                type='number'
                min={2}
                max={100}
                value={props.value.max_claims_per_code}
                onChange={(event) =>
                  update('max_claims_per_code', Number(event.target.value))
                }
                disabled={props.disabled}
              />
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <div className='grid gap-2'>
              <Label htmlFor='user-redemption-expiry-limit'>
                {t('Maximum validity (hours)')}
              </Label>
              <Input
                id='user-redemption-expiry-limit'
                type='number'
                min={1}
                max={720}
                value={props.value.max_expiry_hours}
                onChange={(event) =>
                  update('max_expiry_hours', Number(event.target.value))
                }
                disabled={props.disabled}
              />
            </div>
          </SettingsFormGridItem>
        </SettingsFormGrid>
      </SettingsControlChildren>
    </SettingsControlGroup>
  )
}
