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
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  getComplianceSettings,
  updateComplianceSettings,
  type ComplianceSettings,
  type ComplianceStatementItem,
} from '@/features/compliance/api'

import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { ComplianceStatementItemsEditor } from './compliance-statement-items-editor'

const complianceSettingsSchema = z
  .object({
    statement_enabled: z.boolean(),
    statement_version: z.string(),
    statement_content: z.string(),
    statement_items_json: z.string(),
    statement_display_mode: z.string(),
    token_api_enforcement_enabled: z.boolean(),
    show_version_to_user: z.boolean(),
    show_content_hash_to_user: z.boolean(),
    content_hash: z.string(),
  })
  .superRefine((value, context) => {
    let items: ComplianceStatementItem[] = []
    try {
      const parsed = JSON.parse(value.statement_items_json || '[]') as unknown
      if (!Array.isArray(parsed)) {
        context.addIssue({
          code: 'custom',
          path: ['statement_items_json'],
          message: 'Compliance terms must be a JSON array',
        })
        return
      }
      items = parsed as ComplianceStatementItem[]
    } catch {
      context.addIssue({
        code: 'custom',
        path: ['statement_items_json'],
        message: 'Compliance terms must be valid JSON',
      })
      return
    }
    if (!value.statement_enabled || items.length > 0) return
    if (!value.statement_version.trim()) {
      context.addIssue({
        code: 'custom',
        path: ['statement_version'],
        message: 'Version is required when compliance statement is enabled',
      })
    }
    if (!value.statement_content.trim()) {
      context.addIssue({
        code: 'custom',
        path: ['statement_content'],
        message: 'Content is required when compliance statement is enabled',
      })
    }
  })

const emptySettings: ComplianceSettings = {
  statement_enabled: false,
  statement_version: '',
  statement_content: '',
  statement_items: [],
  statement_display_mode: 'single',
  token_api_enforcement_enabled: false,
  show_version_to_user: false,
  show_content_hash_to_user: false,
  content_hash: '',
}

type ComplianceSettingsFormValues = z.infer<typeof complianceSettingsSchema>

function settingsToFormValues(
  settings: ComplianceSettings
): ComplianceSettingsFormValues {
  return {
    statement_enabled: settings.statement_enabled,
    statement_version: settings.statement_version,
    statement_content: settings.statement_content,
    statement_items_json: JSON.stringify(
      settings.statement_items ?? [],
      null,
      2
    ),
    statement_display_mode: settings.statement_display_mode || 'single',
    token_api_enforcement_enabled: settings.token_api_enforcement_enabled,
    show_version_to_user: Boolean(settings.show_version_to_user),
    show_content_hash_to_user: Boolean(settings.show_content_hash_to_user),
    content_hash: settings.content_hash,
  }
}

function parseStatementItemsOrThrow(json: string): ComplianceStatementItem[] {
  const parsed = JSON.parse(json || '[]') as unknown
  if (!Array.isArray(parsed)) {
    throw new Error('Compliance terms must be a JSON array')
  }
  return parsed as ComplianceStatementItem[]
}

export function ComplianceSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
  const [statementItems, setStatementItems] = useState<
    ComplianceStatementItem[]
  >([])
  const settingsQuery = useQuery({
    queryKey: ['compliance-settings'],
    queryFn: getComplianceSettings,
  })
  const updateMutation = useMutation({
    mutationFn: updateComplianceSettings,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to update settings'))
        return
      }
      queryClient.invalidateQueries({ queryKey: ['compliance-settings'] })
      queryClient.invalidateQueries({ queryKey: ['compliance-statement'] })
      toast.success(t('Setting updated successfully'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to update settings'))
    },
  })
  const settings = settingsQuery.data?.data ?? emptySettings
  const form = useForm<ComplianceSettingsFormValues>({
    resolver: zodResolver(complianceSettingsSchema),
    defaultValues: settingsToFormValues(settings),
  })

  useEffect(() => {
    form.reset(settingsToFormValues(settings))
    setStatementItems(settings.statement_items ?? [])
  }, [form, settings])

  const handleVisualItemsChange = (items: ComplianceStatementItem[]) => {
    setStatementItems(items)
    form.setValue('statement_items_json', JSON.stringify(items, null, 2), {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const handleItemsJsonChange = (value: string) => {
    form.setValue('statement_items_json', value, {
      shouldDirty: true,
      shouldValidate: true,
    })
    try {
      const parsed = JSON.parse(value || '[]') as unknown
      if (Array.isArray(parsed)) {
        setStatementItems(parsed as ComplianceStatementItem[])
      }
    } catch {
      // Keep the last valid visual state while the JSON editor is invalid.
    }
  }

  const onSubmit = async (values: ComplianceSettingsFormValues) => {
    let parsedStatementItems: ComplianceStatementItem[]
    try {
      parsedStatementItems = parseStatementItemsOrThrow(
        values.statement_items_json
      )
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('Compliance terms must be valid JSON')
      form.setError('statement_items_json', {
        type: 'manual',
        message,
      })
      toast.error(t(message))
      return
    }
    await updateMutation.mutateAsync({
      statement_enabled: values.statement_enabled,
      statement_version: values.statement_version.trim(),
      statement_content: values.statement_content,
      statement_items: parsedStatementItems,
      statement_display_mode: values.statement_display_mode,
      token_api_enforcement_enabled: values.token_api_enforcement_enabled,
      show_version_to_user: values.show_version_to_user,
      show_content_hash_to_user: values.show_content_hash_to_user,
      content_hash: values.content_hash,
    })
  }

  return (
    <SettingsSection title={t('Compliance Statement')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateMutation.isPending}
            saveLabel='Save compliance settings'
          />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='statement_enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div className='space-y-1'>
                    <FormLabel>{t('Enable compliance statement')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Require console users to confirm the compliance statement.'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='token_api_enforcement_enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div className='space-y-1'>
                    <FormLabel>
                      {t('Enforce confirmation for Token API')}
                    </FormLabel>
                    <FormDescription>
                      {t('Reject API token calls until the user confirms.')}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='show_version_to_user'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div className='space-y-1'>
                    <FormLabel>{t('Show version to users')}</FormLabel>
                    <FormDescription>
                      {t(
                        'When disabled, regular user compliance dialogs and statement APIs will not show the version.'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='show_content_hash_to_user'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div className='space-y-1'>
                    <FormLabel>{t('Show content hash to users')}</FormLabel>
                    <FormDescription>
                      {t(
                        'When disabled, regular user compliance dialogs and statement APIs will not show the content hash.'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='statement_version'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Version')}</FormLabel>
                  <FormControl>
                    <Input placeholder='2026-07' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='statement_display_mode'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Display mode')}</FormLabel>
                  <FormControl>
                    <Select
                      items={[
                        { value: 'single', label: t('Single dialog') },
                        { value: 'multiple', label: t('Multiple dialogs') },
                      ]}
                      value={field.value}
                      onValueChange={field.onChange}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          <SelectItem value='single'>
                            {t('Single dialog')}
                          </SelectItem>
                          <SelectItem value='multiple'>
                            {t('Multiple dialogs')}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Control whether pending terms are shown together or one by one in order.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='content_hash'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Content hash')}</FormLabel>
                  <FormControl>
                    <Input readOnly {...field} />
                  </FormControl>
                </FormItem>
              )}
            />
            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='statement_content'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Statement content')}</FormLabel>
                    <FormControl>
                      <Textarea rows={10} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Compatible with the legacy single statement. This content is used when no term configuration is provided.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGridItem>
            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='statement_items_json'
                render={({ field }) => (
                  <Tabs
                    value={editMode}
                    onValueChange={(value) =>
                      setEditMode(value as 'visual' | 'json')
                    }
                  >
                    <TabsList className='grid w-full grid-cols-2'>
                      <TabsTrigger value='visual'>
                        {t('Visual editor')}
                      </TabsTrigger>
                      <TabsTrigger value='json'>{t('JSON Editor')}</TabsTrigger>
                    </TabsList>
                    <TabsContent value='visual' className='mt-4'>
                      <FormItem>
                        <FormControl>
                          <ComplianceStatementItemsEditor
                            items={statementItems}
                            onChange={handleVisualItemsChange}
                            disabled={settingsQuery.isLoading}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    </TabsContent>
                    <TabsContent value='json' className='mt-4'>
                      <FormItem>
                        <FormLabel>{t('Terms JSON')}</FormLabel>
                        <FormControl>
                          <Textarea
                            rows={12}
                            className='font-mono text-xs'
                            value={field.value}
                            onChange={(event) =>
                              handleItemsJsonChange(event.target.value)
                            }
                            onBlur={field.onBlur}
                            name={field.name}
                            ref={field.ref}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Configure terms as an array. Fields include item_key, title, source_type, version, content, sort_order, confirmation_mode, confirmation_phrase, and segments. After the JSON parses successfully, it is synced to the visual editor.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    </TabsContent>
                  </Tabs>
                )}
              />
            </SettingsFormGridItem>
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
