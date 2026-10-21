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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ChangeEvent } from 'react'
import type { Control, Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { formatQuota } from '@/lib/format'

import {
  getInvitationIncentiveSettings,
  updateInvitationIncentiveSettings,
} from '../api'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import type { InvitationIncentiveSettings } from '../types'

const percentageSchema = z
  .string()
  .trim()
  .regex(
    /^\d+(?:\.\d{1,2})?$/,
    'Enter a non-negative percentage with at most two decimal places'
  )

const schema = z.object({
  enabled: z.boolean(),
  inviter_registration_reward_enabled: z.boolean(),
  invitee_registration_gift_enabled: z.boolean(),
  cash_commission_enabled: z.boolean(),
  cash_commission_rate: percentageSchema,
  admin_balance_redemption_commission_enabled: z.boolean(),
  admin_balance_redemption_commission_rate: percentageSchema,
  admin_subscription_redemption_commission_enabled: z.boolean(),
  admin_subscription_redemption_commission_rate: percentageSchema,
  QuotaForInviter: z.coerce.number().int().min(0),
  QuotaForInvitee: z.coerce.number().int().min(0),
})

type FormValues = z.infer<typeof schema>

const defaultPolicy: InvitationIncentiveSettings = {
  enabled: true,
  inviter_registration_reward_enabled: true,
  invitee_registration_gift_enabled: true,
  cash_commission_enabled: false,
  cash_commission_rate: '0.00',
  admin_balance_redemption_commission_enabled: false,
  admin_balance_redemption_commission_rate: '0.00',
  admin_subscription_redemption_commission_enabled: false,
  admin_subscription_redemption_commission_rate: '0.00',
}

const policyFields = new Set<keyof InvitationIncentiveSettings>([
  'enabled',
  'inviter_registration_reward_enabled',
  'invitee_registration_gift_enabled',
  'cash_commission_enabled',
  'cash_commission_rate',
  'admin_balance_redemption_commission_enabled',
  'admin_balance_redemption_commission_rate',
  'admin_subscription_redemption_commission_enabled',
  'admin_subscription_redemption_commission_rate',
])

type InvitationIncentiveSectionProps = {
  inviterRewardQuota: number
  inviteeGiftQuota: number
  complianceConfirmed: boolean
}

export function InvitationIncentiveSection(
  props: InvitationIncentiveSectionProps
) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()
  const policyQuery = useQuery({
    queryKey: ['invitation', 'incentive-settings'],
    queryFn: async () => {
      const response = await getInvitationIncentiveSettings()
      if (!response.success) throw new Error(response.message)
      return response.data
    },
  })
  const policyMutation = useMutation({
    mutationFn: async (settings: InvitationIncentiveSettings) => {
      const response = await updateInvitationIncentiveSettings(settings)
      if (!response.success) throw new Error(response.message)
      return response.data
    },
    onSuccess: (settings) => {
      queryClient.setQueryData(['invitation', 'incentive-settings'], settings)
      toast.success(t('Invitation incentive settings updated'))
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    },
  })

  const defaults: FormValues = {
    ...(policyQuery.data ?? defaultPolicy),
    QuotaForInviter: props.inviterRewardQuota,
    QuotaForInvitee: props.inviteeGiftQuota,
  }
  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<FormValues>({
      resolver: zodResolver(schema) as Resolver<
        FormValues,
        unknown,
        FormValues
      >,
      defaultValues: defaults,
      onSubmit: async (values, changedFields) => {
        const policyChanged = Object.keys(changedFields).some((key) =>
          policyFields.has(key as keyof InvitationIncentiveSettings)
        )
        if (policyChanged) {
          await policyMutation.mutateAsync(toPolicy(values))
        }
        for (const key of ['QuotaForInviter', 'QuotaForInvitee'] as const) {
          if (key in changedFields) {
            await updateOption.mutateAsync({ key, value: values[key] })
          }
        }
      },
    })

  const enabled = form.watch('enabled')
  const inviterRewardEnabled = form.watch('inviter_registration_reward_enabled')
  const inviteeGiftEnabled = form.watch('invitee_registration_gift_enabled')
  const cashCommissionEnabled = form.watch('cash_commission_enabled')
  const balanceCommissionEnabled = form.watch(
    'admin_balance_redemption_commission_enabled'
  )
  const subscriptionCommissionEnabled = form.watch(
    'admin_subscription_redemption_commission_enabled'
  )
  const isSaving =
    policyQuery.isLoading ||
    policyMutation.isPending ||
    updateOption.isPending ||
    isSubmitting
  const childrenDisabled = !enabled || isSaving

  const handleQuotaChange =
    (onChange: (value: number | '') => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.currentTarget.valueAsNumber
      onChange(Number.isNaN(value) ? '' : value)
    }

  return (
    <SettingsSection title={t('Invitation incentives')}>
      <FormNavigationGuard when={isDirty} />
      {!props.complianceConfirmed ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'Non-zero invitation rewards require compliance confirmation in Payment Gateway settings.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}
      {policyQuery.isError ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t('Unable to load invitation incentive settings')}
          </AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={isSaving}
            isSaveDisabled={!isDirty || policyQuery.isError}
          />
          <FormDirtyIndicator isDirty={isDirty} />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem className='border-b pb-5'>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable invitation incentives')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Keep invitation relationships while controlling whether new rewards are created.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={isSaving}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <SettingsFormGrid className='gap-y-5'>
            <FormField
              control={form.control}
              name='inviter_registration_reward_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Inviter registration reward')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Create a fixed reward when an invited user registers'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={childrenDisabled}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <FormField
              control={form.control}
              name='QuotaForInviter'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Inviter reward amount')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      value={field.value ?? ''}
                      onChange={handleQuotaChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                      disabled={childrenDisabled || !inviterRewardEnabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {formatQuota(field.value || 0)}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='invitee_registration_gift_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Invitee registration gift')}</FormLabel>
                    <FormDescription>
                      {t('Credit a fixed gift to the newly registered user')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={childrenDisabled}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <FormField
              control={form.control}
              name='QuotaForInvitee'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invitee gift amount')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      value={field.value ?? ''}
                      onChange={handleQuotaChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                      disabled={childrenDisabled || !inviteeGiftEnabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {formatQuota(field.value || 0)}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='cash_commission_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Cash payment commission')}</FormLabel>
                    <FormDescription>
                      {t('Reward eligible cash top-ups and cash subscriptions')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={childrenDisabled}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <CommissionRateField
              control={form.control}
              name='cash_commission_rate'
              label={t('Cash commission rate')}
              disabled={childrenDisabled || !cashCommissionEnabled}
            />

            <FormField
              control={form.control}
              name='admin_balance_redemption_commission_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('Administrator balance-code commission')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Reward claims from eligible administrator balance codes'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={childrenDisabled}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <CommissionRateField
              control={form.control}
              name='admin_balance_redemption_commission_rate'
              label={t('Balance-code commission rate')}
              disabled={childrenDisabled || !balanceCommissionEnabled}
            />

            <FormField
              control={form.control}
              name='admin_subscription_redemption_commission_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('Administrator subscription-code commission')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Reward claims from eligible administrator subscription codes'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={childrenDisabled}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <CommissionRateField
              control={form.control}
              name='admin_subscription_redemption_commission_rate'
              label={t('Subscription-code commission rate')}
              disabled={childrenDisabled || !subscriptionCommissionEnabled}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

function CommissionRateField(props: {
  control: Control<FormValues>
  name:
    | 'cash_commission_rate'
    | 'admin_balance_redemption_commission_rate'
    | 'admin_subscription_redemption_commission_rate'
  label: string
  disabled: boolean
}) {
  const { t } = useTranslation()
  return (
    <FormField
      control={props.control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{props.label}</FormLabel>
          <FormControl>
            <Input
              type='number'
              step='0.01'
              min={0}
              inputMode='decimal'
              {...field}
              disabled={props.disabled}
            />
          </FormControl>
          <FormDescription>
            {t(
              'Percentage of the eligible amount; values above 100% are allowed'
            )}
          </FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function toPolicy(values: FormValues): InvitationIncentiveSettings {
  return {
    enabled: values.enabled,
    inviter_registration_reward_enabled:
      values.inviter_registration_reward_enabled,
    invitee_registration_gift_enabled: values.invitee_registration_gift_enabled,
    cash_commission_enabled: values.cash_commission_enabled,
    cash_commission_rate: values.cash_commission_rate,
    admin_balance_redemption_commission_enabled:
      values.admin_balance_redemption_commission_enabled,
    admin_balance_redemption_commission_rate:
      values.admin_balance_redemption_commission_rate,
    admin_subscription_redemption_commission_enabled:
      values.admin_subscription_redemption_commission_enabled,
    admin_subscription_redemption_commission_rate:
      values.admin_subscription_redemption_commission_rate,
  }
}
