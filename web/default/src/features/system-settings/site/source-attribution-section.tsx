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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  getSourceAttributionSettings,
  updateSourceAttributionSettings,
  type SourceAttributionAdminOption,
} from '@/features/source-attribution/api'

import {
  SettingsForm,
  SettingsFormGridItem,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { SourceAttributionOptionsEditor } from './source-attribution-options-editor'

const defaultOptionsJson = '[]'

function formatOptions(options: SourceAttributionAdminOption[] | undefined) {
  return JSON.stringify(options ?? [], null, 2)
}

function parseOptions(json: string): SourceAttributionAdminOption[] | null {
  try {
    const parsed = JSON.parse(json || defaultOptionsJson) as unknown
    if (!Array.isArray(parsed)) return null
    return parsed as SourceAttributionAdminOption[]
  } catch {
    return null
  }
}

export function SourceAttributionSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const settingsQuery = useQuery({
    queryKey: ['source-attribution-settings'],
    queryFn: getSourceAttributionSettings,
  })
  const [enabled, setEnabled] = useState(false)
  const [tokenApiEnforcementEnabled, setTokenApiEnforcementEnabled] =
    useState(false)
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
  const [options, setOptions] = useState<SourceAttributionAdminOption[]>([])
  const [optionsJson, setOptionsJson] = useState(defaultOptionsJson)
  const updateMutation = useMutation({
    mutationFn: updateSourceAttributionSettings,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to update settings'))
        return
      }
      queryClient.invalidateQueries({
        queryKey: ['source-attribution-settings'],
      })
      queryClient.invalidateQueries({
        queryKey: ['source-attribution-status'],
      })
      toast.success(t('Setting updated successfully'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to update settings'))
    },
  })

  useEffect(() => {
    const settings = settingsQuery.data?.data
    if (!settings) return
    setEnabled(settings.enabled)
    setTokenApiEnforcementEnabled(settings.token_api_enforcement_enabled)
    setOptions(settings.options ?? [])
    setOptionsJson(formatOptions(settings.options))
  }, [settingsQuery.data?.data])

  const handleVisualOptionsChange = (
    nextOptions: SourceAttributionAdminOption[]
  ) => {
    setOptions(nextOptions)
    setOptionsJson(formatOptions(nextOptions))
  }

  const handleJsonChange = (value: string) => {
    setOptionsJson(value)
    const parsed = parseOptions(value)
    if (parsed) {
      setOptions(parsed)
    }
  }

  const handleSave = () => {
    const parsedOptions = parseOptions(optionsJson)
    if (!parsedOptions) {
      toast.error(t('Source attribution options must be a JSON array'))
      return
    }
    updateMutation.mutate({
      enabled,
      token_api_enforcement_enabled: tokenApiEnforcementEnabled,
      options: parsedOptions,
    })
  }

  return (
    <SettingsSection title={t('Source Attribution')}>
      <SettingsForm
        onSubmit={(event) => {
          event.preventDefault()
          handleSave()
        }}
      >
        <SettingsPageFormActions
          onSave={handleSave}
          isSaving={updateMutation.isPending}
          saveLabel='Save source attribution settings'
        />
        <SettingsSwitchField
          checked={enabled}
          onCheckedChange={setEnabled}
          label={t('Enable source attribution')}
          description={t(
            'Users without an inviter must select a source attribution before continuing.'
          )}
          disabled={settingsQuery.isLoading}
        />
        <SettingsSwitchField
          checked={tokenApiEnforcementEnabled}
          onCheckedChange={setTokenApiEnforcementEnabled}
          label={t('Enforce source attribution for Token API')}
          description={t(
            'Reject API token calls until the user selects a source attribution.'
          )}
          disabled={settingsQuery.isLoading}
        />
        <SettingsFormGridItem span='full'>
          <Tabs
            value={editMode}
            onValueChange={(value) => setEditMode(value as 'visual' | 'json')}
          >
            <TabsList className='grid w-full grid-cols-2'>
              <TabsTrigger value='visual'>{t('Visual editor')}</TabsTrigger>
              <TabsTrigger value='json'>{t('JSON Editor')}</TabsTrigger>
            </TabsList>
            <TabsContent value='visual' className='mt-4'>
              <SourceAttributionOptionsEditor
                options={options}
                onChange={handleVisualOptionsChange}
                disabled={settingsQuery.isLoading}
              />
            </TabsContent>
            <TabsContent value='json' className='mt-4 grid gap-2'>
              <Label htmlFor='source-attribution-options-json'>
                {t('Attribution options JSON')}
              </Label>
              <Textarea
                id='source-attribution-options-json'
                rows={12}
                className='font-mono text-xs'
                value={optionsJson}
                onChange={(event) => handleJsonChange(event.target.value)}
                disabled={settingsQuery.isLoading}
              />
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Each option includes option ID, name, target user ID, whether source description is required, and sort order. After the JSON parses successfully, it is synced to the visual editor.'
                )}
              </p>
            </TabsContent>
          </Tabs>
        </SettingsFormGridItem>
      </SettingsForm>
    </SettingsSection>
  )
}
