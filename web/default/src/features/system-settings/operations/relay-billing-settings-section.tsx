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
import { AlertCircle, AlertTriangle } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import { getRelayBillingSettings, updateRelayBillingSettings } from '../api'
import {
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import type { RelayBillingSettings } from '../types'
import {
  DEFAULT_RELAY_BILLING_SETTINGS,
  highPriceWarnings,
  hydrateRelayBillingSettings,
  validateRelayBillingSettings,
} from './relay-billing-settings-state'

function prettyJson(value: unknown) {
  return JSON.stringify(value, null, 2)
}

function parseObject(text: string): Record<string, unknown> {
  const value = JSON.parse(text) as unknown
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('JSON must be an object')
  }
  return value as Record<string, unknown>
}

export function RelayBillingSettingsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState(DEFAULT_RELAY_BILLING_SETTINGS)
  const [jsonErrors, setJsonErrors] = useState<Record<string, string>>({})

  const query = useQuery({
    queryKey: ['relay-billing-settings'],
    queryFn: async () => {
      const response = await getRelayBillingSettings()
      if (!response.success || !response.data) {
        throw new Error(
          response.message || 'Failed to load relay billing settings'
        )
      }
      return hydrateRelayBillingSettings(response.data)
    },
  })

  useEffect(() => {
    if (query.data) {
      setDraft(query.data)
      setJsonErrors({})
    }
  }, [query.data])

  const updateMutation = useMutation({
    mutationFn: async (settings: RelayBillingSettings) => {
      const response = await updateRelayBillingSettings(settings)
      if (!response.success || !response.data) {
        throw new Error(
          response.message || 'Failed to save relay billing settings'
        )
      }
      return hydrateRelayBillingSettings(response.data)
    },
    onSuccess: (settings) => {
      setDraft(settings)
      queryClient.setQueryData(['relay-billing-settings'], settings)
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Relay billing settings saved'))
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const errors = useMemo(() => {
    const fieldErrors = validateRelayBillingSettings(draft)
    return [...fieldErrors, ...Object.keys(jsonErrors)]
  }, [draft, jsonErrors])
  const warnings = useMemo(() => highPriceWarnings(draft), [draft])

  const reset = () => {
    const settings = query.data ?? DEFAULT_RELAY_BILLING_SETTINGS
    setDraft(hydrateRelayBillingSettings(settings))
    setJsonErrors({})
  }

  const updateTool = (patch: Partial<RelayBillingSettings['tool_pricing']>) =>
    setDraft((current) => ({
      ...current,
      tool_pricing: { ...current.tool_pricing, ...patch },
    }))

  const updateDiagnostic = (
    patch: Partial<RelayBillingSettings['upstream_diagnostic']>
  ) =>
    setDraft((current) => ({
      ...current,
      upstream_diagnostic: { ...current.upstream_diagnostic, ...patch },
    }))

  const updateJson = (
    field: 'prices' | 'image_price_matrix' | 'image_model_mode_overrides',
    text: string
  ) => {
    try {
      const parsed = parseObject(text)
      updateTool({ [field]: parsed } as Partial<
        RelayBillingSettings['tool_pricing']
      >)
      setJsonErrors((current) => {
        const next = { ...current }
        delete next[field]
        return next
      })
    } catch (error) {
      setJsonErrors((current) => ({
        ...current,
        [field]: error instanceof Error ? error.message : 'Invalid JSON',
      }))
    }
  }

  const save = () => {
    if (errors.length) {
      toast.error(t('Please fix relay billing settings before saving'))
      return
    }
    if (
      warnings.length &&
      !window.confirm(t('Some prices are unusually high. Save anyway?'))
    ) {
      return
    }
    updateMutation.mutate(draft)
  }

  const loading = query.isLoading || updateMutation.isPending

  return (
    <SettingsSection title={t('Relay Billing & Diagnostics')}>
      <SettingsForm
        onSubmit={(event) => {
          event.preventDefault()
          save()
        }}
      >
        <SettingsPageFormActions
          onSave={save}
          onReset={reset}
          isSaving={loading}
          isSaveDisabled={loading || errors.length > 0}
          saveLabel='Save relay billing settings'
        />

        {query.isError ? (
          <Alert variant='destructive'>
            <AlertCircle />
            <AlertTitle>
              {t('Failed to load relay billing settings')}
            </AlertTitle>
            <AlertDescription>{query.error.message}</AlertDescription>
          </Alert>
        ) : null}
        {warnings.length ? (
          <Alert>
            <AlertTriangle />
            <AlertTitle>{t('Review high prices')}</AlertTitle>
            <AlertDescription>
              {t('High prices will be saved as entered after confirmation.')}
            </AlertDescription>
          </Alert>
        ) : null}

        <SettingsControlGroup>
          <SettingsSwitchField
            checked={draft.alpha_search.enabled}
            onCheckedChange={(enabled) =>
              setDraft((current) => ({ ...current, alpha_search: { enabled } }))
            }
            label={t('Enable Alpha Search')}
            description={t(
              'Allow the /v1/alpha/search endpoint to route requests.'
            )}
            disabled={loading}
          />
          <SettingsSwitchField
            checked={draft.tool_pricing.enabled}
            onCheckedChange={(enabled) => updateTool({ enabled })}
            label={t('Enable tool pricing')}
            description={t(
              'Charge configured tool calls in addition to model usage.'
            )}
            disabled={loading}
          />
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor='relay-multiplier-mode'>
                {t('Tool multiplier mode')}
              </Label>
              <select
                id='relay-multiplier-mode'
                className='border-input bg-background h-8 w-full rounded-lg border px-2 text-sm'
                value={draft.tool_pricing.multiplier_mode}
                onChange={(event) =>
                  updateTool({
                    multiplier_mode: event.target.value as
                      | 'group_only'
                      | 'full',
                  })
                }
              >
                <option value='group_only'>{t('Group only')}</option>
                <option value='full'>{t('Full multiplier')}</option>
              </select>
            </div>
            <div className='space-y-2'>
              <Label htmlFor='relay-image-mode'>
                {t('Image pricing mode')}
              </Label>
              <select
                id='relay-image-mode'
                className='border-input bg-background h-8 w-full rounded-lg border px-2 text-sm'
                value={draft.tool_pricing.image_pricing_mode}
                onChange={(event) =>
                  updateTool({
                    image_pricing_mode: event.target.value as 'matrix' | 'flat',
                  })
                }
              >
                <option value='matrix'>{t('Quality and size matrix')}</option>
                <option value='flat'>{t('Flat per-1K price')}</option>
              </select>
            </div>
          </div>
          {(
            [
              [
                'prices',
                draft.tool_pricing.prices,
                t('Tool prices ($/1K calls)'),
              ],
              [
                'image_price_matrix',
                draft.tool_pricing.image_price_matrix,
                t('Image price matrix'),
              ],
              [
                'image_model_mode_overrides',
                draft.tool_pricing.image_model_mode_overrides,
                t('Image model mode overrides'),
              ],
            ] as const
          ).map(([field, value, label]) => (
            <div className='space-y-2' key={field}>
              <Label htmlFor={`relay-${field}`}>{label}</Label>
              <Textarea
                id={`relay-${field}`}
                value={prettyJson(value)}
                onChange={(event) => updateJson(field, event.target.value)}
                className='min-h-24 font-mono text-xs'
                spellCheck={false}
                aria-invalid={Boolean(jsonErrors[field])}
              />
              {jsonErrors[field] ? (
                <p className='text-destructive text-xs'>{jsonErrors[field]}</p>
              ) : null}
            </div>
          ))}
        </SettingsControlGroup>

        <SettingsControlGroup>
          <SettingsSwitchField
            checked={draft.upstream_diagnostic.enabled}
            onCheckedChange={(enabled) => updateDiagnostic({ enabled })}
            label={t('Record upstream diagnostics')}
            description={t(
              'Keep a bounded original upstream response for failed requests.'
            )}
            disabled={loading}
          />
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor='relay-capture-mode'>{t('Capture mode')}</Label>
              <select
                id='relay-capture-mode'
                className='border-input bg-background h-8 w-full rounded-lg border px-2 text-sm'
                value={draft.upstream_diagnostic.capture_mode}
                onChange={(event) =>
                  updateDiagnostic({
                    capture_mode: event.target.value as
                      | 'fallback_only'
                      | 'all_errors',
                  })
                }
              >
                <option value='fallback_only'>{t('Fallback only')}</option>
                <option value='all_errors'>{t('All errors')}</option>
              </select>
            </div>
            <div className='space-y-2'>
              <Label htmlFor='relay-preview-chars'>
                {t('Preview characters')}
              </Label>
              <Input
                id='relay-preview-chars'
                type='number'
                min={256}
                max={8192}
                value={draft.upstream_diagnostic.preview_chars}
                onChange={(event) =>
                  updateDiagnostic({
                    preview_chars: Number(event.target.value),
                  })
                }
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='relay-dedupe-window'>
                {t('Dedupe window (seconds)')}
              </Label>
              <Input
                id='relay-dedupe-window'
                type='number'
                min={0}
                max={3600}
                value={draft.upstream_diagnostic.dedupe_window_seconds}
                onChange={(event) =>
                  updateDiagnostic({
                    dedupe_window_seconds: Number(event.target.value),
                  })
                }
              />
            </div>
          </div>
          <SettingsSwitchField
            checked={draft.upstream_diagnostic.sync_to_usage_log}
            onCheckedChange={(sync_to_usage_log) =>
              updateDiagnostic({ sync_to_usage_log })
            }
            label={t('Sync diagnostics to failed usage logs')}
            disabled={loading}
          />
        </SettingsControlGroup>
      </SettingsForm>
    </SettingsSection>
  )
}
