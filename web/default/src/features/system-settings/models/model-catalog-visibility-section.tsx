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
import { useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import {
  AlertTriangle,
  Globe2,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { MODEL_CATALOG_VISIBILITY_REFRESH_KEY } from '@/features/pricing/constants'

import { getModelCatalogVisibility, updateModelCatalogVisibility } from '../api'
import { SettingsSection } from '../components/settings-section'
import type { ModelCatalogAdminGroup, ModelCatalogAdminState } from '../types'
import {
  buildCatalogVisibilityDraft,
  clearCatalogRule,
  filterCatalogGroups,
  getCatalogRule,
  getEffectiveUserGroupNames,
  getRestrictedCatalogGroupCount,
  getVisibleUserGroupCount,
  isCatalogRuleExplicit,
  setCatalogDefaultMode,
  setCatalogVisibilityEnabled,
  setCatalogRuleMode,
  setCatalogRuleUserGroup,
  toVisibilityUpdate,
  type ModelCatalogVisibilityDraft,
} from './model-catalog-visibility-state'

export function ModelCatalogVisibilitySection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<ModelCatalogVisibilityDraft | null>(null)
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      for (let attempt = 0; attempt < 3; attempt += 1) {
        try {
          const response = await getModelCatalogVisibility()
          if (!response.success) throw new Error(response.message)
          setDraft(buildCatalogVisibilityDraft(response.data))
          return
        } catch {
          if (attempt < 2) {
            await new Promise((resolve) =>
              setTimeout(resolve, 100 * (attempt + 1))
            )
          }
        }
      }
      setError(t('Failed to load catalog visibility'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    void load()
  }, [load])

  const visibleGroups = useMemo(
    () => (draft ? filterCatalogGroups(draft.groups, query) : []),
    [draft, query]
  )

  const setGroupMode = (catalogGroup: string, mode: 'all' | 'selected') => {
    setDraft((current) =>
      current ? setCatalogRuleMode(current, catalogGroup, mode) : current
    )
  }

  const setUserGroup = (
    catalogGroup: string,
    userGroup: string,
    checked: boolean
  ) => {
    setDraft((current) =>
      current
        ? setCatalogRuleUserGroup(current, catalogGroup, userGroup, checked)
        : current
    )
  }

  const clearRule = (catalogGroup: string) => {
    setDraft((current) =>
      current ? clearCatalogRule(current, catalogGroup) : current
    )
  }

  const save = async () => {
    if (!draft) return
    setSaving(true)
    setError('')
    try {
      const response = await updateModelCatalogVisibility(
        toVisibilityUpdate(draft)
      )
      if (!response.success) throw new Error(response.message)
      const refreshed = await getModelCatalogVisibility()
      if (!refreshed.success) throw new Error(refreshed.message)
      setDraft(buildCatalogVisibilityDraft(refreshed.data))
      await queryClient.invalidateQueries({ queryKey: ['pricing'] })
      try {
        window.localStorage.setItem(
          MODEL_CATALOG_VISIBILITY_REFRESH_KEY,
          String(Date.now())
        )
      } catch {
        // The current tab was already refreshed; storage is only cross-tab sync.
      }
      toast.success(t('Setting saved'))
    } catch (requestError) {
      if (isAxiosError(requestError) && requestError.response?.status === 409) {
        setError(t('Catalog visibility changed; refresh before saving'))
      } else {
        setError(t('Failed to save catalog visibility'))
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsSection title={t('Model Catalog Group Visibility')}>
      {loading ? (
        <div className='flex min-h-32 items-center justify-center'>
          <Spinner />
        </div>
      ) : null}
      {error ? (
        <Alert variant='destructive'>
          <AlertTitle>{error}</AlertTitle>
          <AlertDescription>
            <Button type='button' variant='outline' size='sm' onClick={load}>
              <RefreshCw aria-hidden='true' />
              {t('Refresh')}
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}
      {draft ? (
        <div className='min-w-0 space-y-4 pb-16 md:pb-0'>
          {draft.configError ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Invalid stored catalog visibility')}</AlertTitle>
            </Alert>
          ) : null}

          <div className='grid grid-cols-1 gap-2 sm:grid-cols-3'>
            <SummaryMetric
              label={t('Catalog groups')}
              value={draft.summary.group_count}
            />
            <SummaryMetric
              label={t('Models')}
              value={draft.summary.model_count}
            />
            <SummaryMetric
              label={t('Restricted groups')}
              value={getRestrictedCatalogGroupCount(draft)}
            />
          </div>

          <div className='flex flex-col gap-3 rounded-md border p-4 sm:flex-row sm:items-center sm:justify-between'>
            <div className='min-w-0'>
              <div className='font-medium'>
                {t('Enable custom model catalog visibility rules')}
              </div>
              <div className='text-muted-foreground mt-1 text-sm'>
                {t(
                  'When enabled, configure which user groups can see each model group.'
                )}
              </div>
            </div>
            <div className='flex shrink-0 items-center gap-3'>
              <Badge variant={draft.enabled ? 'default' : 'secondary'}>
                {draft.enabled
                  ? t('Use custom visibility rules')
                  : t('Use platform original rules')}
              </Badge>
              <Switch
                checked={draft.enabled}
                onCheckedChange={(enabled) =>
                  setDraft((current) =>
                    current
                      ? setCatalogVisibilityEnabled(current, enabled)
                      : current
                  )
                }
                aria-label={t('Enable custom model catalog visibility rules')}
              />
            </div>
          </div>

          {!draft.enabled ? (
            <Alert>
              <Globe2 aria-hidden='true' />
              <AlertTitle>{t('Using original model catalog rules')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Custom visibility rules are disabled. The model catalog uses its original display rules. Saved custom rules are retained and will apply again when enabled.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}

          <div className='space-y-4' aria-disabled={!draft.enabled}>
            <div className='space-y-3 border-y py-4'>
              <div>
                <div className='font-medium'>
                  {t('Default rule for model groups without a custom row')}
                </div>
                <p className='text-muted-foreground mt-1 text-sm'>
                  {t(
                    'This also controls newly added model groups until a custom row is saved.'
                  )}
                </p>
              </div>
              <div className='grid gap-2 sm:grid-cols-2'>
                <Button
                  type='button'
                  variant={draft.defaultMode === 'all' ? 'default' : 'outline'}
                  disabled={!draft.enabled}
                  onClick={() =>
                    setDraft((current) =>
                      current ? setCatalogDefaultMode(current, 'all') : current
                    )
                  }
                >
                  <Globe2 aria-hidden='true' />
                  {t('Visible to all user groups by default')}
                </Button>
                <Button
                  type='button'
                  variant={
                    draft.defaultMode === 'hidden' ? 'default' : 'outline'
                  }
                  disabled={!draft.enabled}
                  onClick={() =>
                    setDraft((current) =>
                      current
                        ? setCatalogDefaultMode(current, 'hidden')
                        : current
                    )
                  }
                >
                  <AlertTriangle aria-hidden='true' />
                  {t('Hidden from all user groups by default')}
                </Button>
              </div>
            </div>

            <div className='space-y-3'>
              <div>
                <div className='font-medium'>
                  {t('Model groups visible to each user group')}
                </div>
                <p className='text-muted-foreground mt-1 text-sm'>
                  {t(
                    'Rows are user groups and columns are model groups. Check the model groups that each user group can see in the catalog.'
                  )}
                </p>
              </div>
              <div className='relative max-w-md'>
                <Search className='text-muted-foreground absolute top-2.5 left-3 size-4' />
                <Input
                  className='pl-9'
                  value={query}
                  disabled={!draft.enabled}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t('Search catalog groups')}
                />
              </div>

              <CatalogVisibilityMatrix
                groups={visibleGroups}
                draft={draft}
                disabled={!draft.enabled}
                onModeChange={setGroupMode}
                onUserGroupChange={setUserGroup}
                onClearRule={clearRule}
              />
            </div>
          </div>

          <div className='bg-background fixed inset-x-0 bottom-0 z-10 border-t p-3 md:static md:border-0 md:p-0'>
            <Button type='button' disabled={saving} onClick={save}>
              {saving ? <Spinner /> : <Save aria-hidden='true' />}
              {t('Save visibility')}
            </Button>
          </div>
        </div>
      ) : null}
    </SettingsSection>
  )
}

type SummaryMetricProps = {
  label: string
  value: number
}

function SummaryMetric(props: SummaryMetricProps) {
  return (
    <div className='rounded-md border p-3'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='text-lg font-semibold'>{props.value}</div>
    </div>
  )
}

type CatalogVisibilityMatrixProps = {
  groups: ModelCatalogAdminGroup[]
  draft: ModelCatalogVisibilityDraft
  disabled: boolean
  onModeChange: (catalogGroup: string, mode: 'all' | 'selected') => void
  onUserGroupChange: (
    catalogGroup: string,
    userGroup: string,
    checked: boolean
  ) => void
  onClearRule: (catalogGroup: string) => void
}

function CatalogVisibilityMatrix(props: CatalogVisibilityMatrixProps) {
  const { t } = useTranslation()
  const userGroups = props.draft.userGroups.filter((group) => group.exists)

  return (
    <div
      className={`overflow-x-auto rounded-md border ${
        props.disabled ? 'opacity-60' : ''
      }`}
    >
      <table className='w-full min-w-max border-collapse text-sm'>
        <caption className='sr-only'>
          {t('Model groups visible to each user group')}
        </caption>
        <thead>
          <tr className='bg-muted/40 border-b'>
            <th
              scope='col'
              className='bg-background sticky left-0 z-20 min-w-56 border-r p-3 text-left font-medium'
            >
              {t('User group')}
            </th>
            {props.groups.map((group) => {
              const display = getGroupDisplay(group)
              const rule = getCatalogRule(props.draft, group.name)
              const explicit = isCatalogRuleExplicit(
                props.draft,
                group.name
              )
              return (
                <th
                  key={group.name}
                  scope='col'
                  className='max-w-52 min-w-44 border-r p-3 text-center align-top font-medium whitespace-normal'
                >
                  <span className='block break-words'>{display.label}</span>
                  {display.identifier ? (
                    <span className='text-muted-foreground mt-1 block text-xs font-normal break-all'>
                      {display.identifier}
                    </span>
                  ) : null}
                  <div className='mt-2 flex flex-wrap items-center justify-center gap-1'>
                    <Badge variant={explicit ? 'secondary' : 'outline'}>
                      {explicit ? t('Custom rule') : t('Default rule')}
                    </Badge>
                    <Badge
                      variant={rule.mode === 'all' ? 'default' : 'secondary'}
                    >
                      {rule.mode === 'all'
                        ? t('All user groups')
                        : t('{{count}} user groups', {
                            count: getVisibleUserGroupCount(
                              props.draft,
                              group.name
                            ),
                          })}
                    </Badge>
                  </div>
                  <div className='mt-2 flex justify-center gap-1'>
                    <Button
                      type='button'
                      size='icon-sm'
                      variant='ghost'
                      disabled={props.disabled || !group.exists}
                      title={t('All current and future user groups')}
                      aria-label={t('All current and future user groups')}
                      onClick={() => props.onModeChange(group.name, 'all')}
                    >
                      <Globe2 aria-hidden='true' />
                    </Button>
                    {explicit ? (
                      <Button
                        type='button'
                        size='icon-sm'
                        variant='ghost'
                        disabled={props.disabled}
                        title={t('Inherit default rule')}
                        aria-label={t('Inherit default rule')}
                        onClick={() => props.onClearRule(group.name)}
                      >
                        <RotateCcw aria-hidden='true' />
                      </Button>
                    ) : null}
                  </div>
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {userGroups.length > 0 && props.groups.length > 0 ? (
            userGroups.map((userGroup) => (
              <tr
                key={userGroup.name}
                className='hover:bg-muted/20 border-b last:border-b-0'
              >
                <th
                  scope='row'
                  className='bg-background sticky left-0 z-10 min-w-56 border-r p-3 text-left font-normal'
                >
                  <GroupLabel group={userGroup} />
                </th>
                {props.groups.map((group) => {
                  const catalogDisplay = getGroupDisplay(group)
                  const userDisplay = getGroupDisplay(userGroup)
                  const checked = getEffectiveUserGroupNames(
                    props.draft,
                    group.name
                  ).includes(userGroup.name)
                  return (
                    <td
                      key={group.name}
                      className='min-w-44 border-r p-3 text-center align-middle'
                    >
                      <Checkbox
                        checked={checked}
                        disabled={props.disabled || !group.exists}
                        aria-label={t(
                          'Visible to {{userGroup}} for {{catalogGroup}}',
                          {
                            userGroup: userDisplay.label,
                            catalogGroup: catalogDisplay.label,
                          }
                        )}
                        onCheckedChange={(nextChecked) =>
                          props.onUserGroupChange(
                            group.name,
                            userGroup.name,
                            nextChecked === true
                          )
                        }
                      />
                    </td>
                  )
                })}
              </tr>
            ))
          ) : (
            <tr>
              <td
                colSpan={Math.max(2, props.groups.length + 1)}
                className='text-muted-foreground p-6 text-center'
              >
                {t('No catalog groups match')}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

function GroupLabel({ group }: { group: ModelCatalogAdminGroup }) {
  const display = getGroupDisplay(group)
  return (
    <>
      <span className='font-medium break-words'>{display.label}</span>
      {display.identifier ? (
        <span className='text-muted-foreground mt-1 block text-xs break-all'>
          {display.identifier}
        </span>
      ) : null}
    </>
  )
}

function getGroupDisplay(group: ModelCatalogAdminGroup) {
  const description = group.description.trim()
  if (!description || description === group.name) {
    return { label: group.name, identifier: '' }
  }
  return { label: description, identifier: group.name }
}

export type { ModelCatalogAdminState }
