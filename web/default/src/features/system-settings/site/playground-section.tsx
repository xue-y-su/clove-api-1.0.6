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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { updateSystemOption } from '../api'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import {
  buildPlaygroundOptionUpdates,
  type PlaygroundSettingsDefaults,
  type PlaygroundSettingsValues,
} from './playground-settings'

type PlaygroundSectionProps = {
  defaultValues: PlaygroundSettingsDefaults
}

const toValues = (
  defaults: PlaygroundSettingsDefaults
): PlaygroundSettingsValues => ({
  ...defaults,
  apiKey: '',
  clearAPIKey: false,
})

export function PlaygroundSection(props: PlaygroundSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [values, setValues] = useState<PlaygroundSettingsValues>(() =>
    toValues(props.defaultValues)
  )
  const serializedDefaults = JSON.stringify(props.defaultValues)
  const defaults = useMemo(
    () => props.defaultValues,
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [serializedDefaults]
  )

  useEffect(() => {
    setValues(toValues(defaults))
  }, [defaults])

  const updateMutation = useMutation({
    mutationFn: async (nextValues: PlaygroundSettingsValues) => {
      const updates = buildPlaygroundOptionUpdates(defaults, nextValues)
      for (const update of updates) {
        const response = await updateSystemOption(update)
        if (!response.success) {
          throw new Error(response.message || 'Failed to update setting')
        }
      }
      return updates.length
    },
    onSuccess: (updatedCount) => {
      if (updatedCount === 0) {
        toast.info(t('No changes to save'))
        return
      }
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Setting updated successfully'))
      setValues((current) => ({
        ...current,
        apiKey: '',
        apiKeyConfigured: current.clearAPIKey
          ? false
          : current.apiKeyConfigured || Boolean(current.apiKey.trim()),
        clearAPIKey: false,
      }))
    },
    onError: (error: Error) => {
      toast.error(t(error.message || 'Failed to update setting'))
    },
  })

  const setValue = <K extends keyof PlaygroundSettingsValues>(
    key: K,
    value: PlaygroundSettingsValues[K]
  ) => {
    setValues((current) => ({ ...current, [key]: value }))
  }

  const handleSave = () => updateMutation.mutate(values)
  const handleReset = () => setValues(toValues(defaults))

  return (
    <SettingsSection title={t('Playground')}>
      <SettingsForm
        onSubmit={(event) => {
          event.preventDefault()
          handleSave()
        }}
      >
        <SettingsPageFormActions
          onSave={handleSave}
          onReset={handleReset}
          isSaving={updateMutation.isPending}
          saveLabel={t('Save playground settings')}
        />

        <SettingsSwitchField
          checked={values.enabled}
          onCheckedChange={(checked) => setValue('enabled', checked)}
          label={t('Enable Tavily fallback search')}
          description={t(
            'Used only when the selected routed model cannot perform native web search.'
          )}
        />

        <SettingsFormGrid>
          <SettingsFormGridItem>
            <div className='grid gap-2'>
              <div className='flex items-center justify-between gap-2'>
                <Label htmlFor='playground-tavily-key'>
                  {t('Tavily API Key')}
                </Label>
                <span className='text-muted-foreground text-xs'>
                  {values.apiKeyConfigured
                    ? t('Configured')
                    : t('Not configured')}
                </span>
              </div>
              <Input
                id='playground-tavily-key'
                type='password'
                autoComplete='new-password'
                maxLength={4096}
                value={values.apiKey}
                placeholder={
                  values.apiKeyConfigured
                    ? t('Enter a new key to replace the configured key')
                    : t('Enter Tavily API Key')
                }
                disabled={values.clearAPIKey}
                onChange={(event) => setValue('apiKey', event.target.value)}
              />
              {values.apiKeyConfigured ? (
                <label className='flex items-center gap-2 text-xs'>
                  <Checkbox
                    checked={values.clearAPIKey}
                    onCheckedChange={(checked) => {
                      const shouldClear = checked === true
                      setValues((current) => ({
                        ...current,
                        apiKey: shouldClear ? '' : current.apiKey,
                        clearAPIKey: shouldClear,
                        enabled: shouldClear ? false : current.enabled,
                      }))
                    }}
                  />
                  {t('Clear the configured key')}
                </label>
              ) : null}
            </div>
          </SettingsFormGridItem>

          <SettingsFormGridItem>
            <div className='grid gap-2'>
              <Label>{t('Default search depth')}</Label>
              <Select
                items={[
                  { value: 'basic', label: t('Basic') },
                  { value: 'advanced', label: t('Advanced') },
                ]}
                value={values.defaultDepth}
                onValueChange={(value) =>
                  setValue('defaultDepth', value as 'basic' | 'advanced')
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='basic'>{t('Basic')}</SelectItem>
                    <SelectItem value='advanced'>{t('Advanced')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
          </SettingsFormGridItem>

          <SettingsFormGridItem>
            <div className='grid gap-2'>
              <Label htmlFor='playground-search-results'>
                {t('Maximum search results')}
              </Label>
              <Input
                id='playground-search-results'
                type='number'
                min={1}
                max={10}
                value={values.maxResults}
                onChange={(event) =>
                  setValue('maxResults', Number(event.target.value))
                }
              />
            </div>
          </SettingsFormGridItem>

          <SettingsFormGridItem>
            <div className='grid gap-2'>
              <Label htmlFor='playground-search-context'>
                {t('Maximum search context length')}
              </Label>
              <Input
                id='playground-search-context'
                type='number'
                min={500}
                max={20000}
                step={100}
                value={values.maxContextLength}
                onChange={(event) =>
                  setValue('maxContextLength', Number(event.target.value))
                }
              />
            </div>
          </SettingsFormGridItem>

          <SettingsFormGridItem span='full'>
            <div className='grid gap-2'>
              <Label htmlFor='playground-native-models'>
                {t('Native web search model patterns')}
              </Label>
              <Textarea
                id='playground-native-models'
                rows={3}
                maxLength={20000}
                value={values.nativeModels}
                onChange={(event) =>
                  setValue('nativeModels', event.target.value)
                }
              />
            </div>
          </SettingsFormGridItem>
        </SettingsFormGrid>
      </SettingsForm>
    </SettingsSection>
  )
}
