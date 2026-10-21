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
import {
  CircleAlert,
  ImagePlus,
  LoaderCircle,
  Plus,
  RefreshCw,
  Save,
  Trash2,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { uploadAnnouncementMedia } from '@/features/announcements/api'
import { isSafeAnnouncementImageURL } from '@/features/announcements/global-announcement'
import dayjs from '@/lib/dayjs'

import { getAnnouncementSettings, updateAnnouncementSettings } from '../api'
import {
  SettingsControlGroup,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { formatAnnouncementRelativeTime } from './announcement-relative-time'

type Announcement = {
  id: number
  title?: string
  content: string
  publishDate: string
  type: 'default' | 'ongoing' | 'success' | 'warning' | 'error'
  extra?: string
  imageUrl?: string
  popup: boolean
  published: boolean
}

type AnnouncementsSectionProps = {
  enabled: boolean
  data: string
}

type AnnouncementFormValues = {
  title?: string
  content: string
  publishDate: string
  type: 'default' | 'ongoing' | 'success' | 'warning' | 'error'
  extra?: string
  imageUrl?: string
  popup: boolean
  published: boolean
}

const ANNOUNCEMENT_FORM_ID = 'announcement-form'

const typeOptions = [
  {
    value: 'default',
    label: 'Default',
    color: 'bg-gray-500',
    badgeVariant: 'neutral' as const,
  },
  {
    value: 'ongoing',
    label: 'Ongoing',
    color: 'bg-blue-500',
    badgeVariant: 'info' as const,
  },
  {
    value: 'success',
    label: 'Success',
    color: 'bg-green-500',
    badgeVariant: 'success' as const,
  },
  {
    value: 'warning',
    label: 'Warning',
    color: 'bg-orange-500',
    badgeVariant: 'warning' as const,
  },
  {
    value: 'error',
    label: 'Error',
    color: 'bg-red-500',
    badgeVariant: 'danger' as const,
  },
]

export function AnnouncementsSection({
  enabled,
  data,
}: AnnouncementsSectionProps) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()
  const announcementSchema = useMemo(
    () =>
      z.object({
        title: z
          .string()
          .trim()
          .max(200, t('Announcement title must be less than 200 characters'))
          .optional(),
        content: z
          .string()
          .min(1, t('Content is required'))
          .refine(
            (value) => new TextEncoder().encode(value).byteLength <= 64 * 1024,
            t('Announcement content must not exceed 64 KiB')
          ),
        publishDate: z.string().min(1, t('Publish date is required')),
        type: z.enum(['default', 'ongoing', 'success', 'warning', 'error']),
        popup: z.boolean(),
        published: z.boolean(),
        extra: z
          .string()
          .max(100, t('Extra must be less than 100 characters'))
          .optional(),
        imageUrl: z
          .string()
          .trim()
          .max(2000, t('Image URL must be less than 2000 characters'))
          .refine(
            (value) => value === '' || isSafeAnnouncementImageURL(value),
            t('Image URL must be a public HTTPS URL')
          )
          .optional(),
      }),
    [t]
  )
  const [announcements, setAnnouncements] = useState<Announcement[]>([])
  const [isEnabled, setIsEnabled] = useState(enabled)
  const [imageMaxMiB, setImageMaxMiB] = useState(5)
  const [hasChanges, setHasChanges] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editingAnnouncement, setEditingAnnouncement] =
    useState<Announcement | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<'single' | 'batch'>('single')
  const [isUploadingImage, setIsUploadingImage] = useState(false)
  const [imageUploadError, setImageUploadError] = useState('')
  const imageInputRef = useRef<HTMLInputElement>(null)

  const announcementSettingsQuery = useQuery({
    queryKey: ['announcement-settings'],
    queryFn: async () => {
      const response = await getAnnouncementSettings()
      if (!response.success || !response.data) {
        throw new Error(
          response.message || 'Failed to load announcement settings'
        )
      }
      return response.data
    },
  })

  const announcementSettingsMutation = useMutation({
    mutationFn: async () => {
      const response = await updateAnnouncementSettings({
        enabled: isEnabled,
        image_max_bytes: imageMaxMiB * 1024 * 1024,
      })
      if (!response.success || !response.data) {
        throw new Error(
          response.message || 'Failed to update announcement settings'
        )
      }
      return response.data
    },
    onSuccess: (settings) => {
      setIsEnabled(settings.enabled)
      setImageMaxMiB(Math.round(settings.image_max_bytes / (1024 * 1024)))
      queryClient.setQueryData(['announcement-settings'], settings)
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Announcement settings saved'))
    },
    onError: (error: Error) => {
      toast.error(t(error.message || 'Failed to update announcement settings'))
    },
  })

  const form = useForm<AnnouncementFormValues>({
    resolver: zodResolver(announcementSchema),
    defaultValues: {
      title: '',
      content: '',
      publishDate: new Date().toISOString(),
      type: 'default',
      extra: '',
      imageUrl: '',
      popup: false,
      published: true,
    },
  })

  useEffect(() => {
    try {
      const parsed = JSON.parse(data || '[]')
      if (Array.isArray(parsed)) {
        setAnnouncements(
          parsed.map((item, idx) => ({
            id: item.id || idx + 1,
            title: item.title,
            content: item.content,
            publishDate: item.publishDate,
            type: item.type || 'default',
            extra: item.extra,
            imageUrl: item.imageUrl,
            popup: item.popup === true,
            published: item.published !== false,
          }))
        )
      }
    } catch {
      setAnnouncements([])
    }
  }, [data])

  useEffect(() => {
    const settings = announcementSettingsQuery.data
    setIsEnabled(settings?.enabled ?? enabled)
    setImageMaxMiB(
      settings ? Math.round(settings.image_max_bytes / (1024 * 1024)) : 5
    )
  }, [announcementSettingsQuery.data, enabled])

  const handleAdd = () => {
    setEditingAnnouncement(null)
    setImageUploadError('')
    form.reset({
      title: '',
      content: '',
      publishDate: new Date().toISOString(),
      type: 'default',
      extra: '',
      imageUrl: '',
      popup: false,
      published: true,
    })
    setShowDialog(true)
  }

  const handleEdit = (announcement: Announcement) => {
    setEditingAnnouncement(announcement)
    setImageUploadError('')
    form.reset({
      title: announcement.title || '',
      content: announcement.content,
      publishDate: announcement.publishDate,
      type: announcement.type,
      extra: announcement.extra || '',
      imageUrl: announcement.imageUrl || '',
      popup: announcement.popup,
      published: announcement.published,
    })
    setShowDialog(true)
  }

  const handleDelete = (announcement: Announcement) => {
    setEditingAnnouncement(announcement)
    setDeleteTarget('single')
    setShowDeleteDialog(true)
  }

  const handleBatchDelete = () => {
    if (selectedIds.length === 0) {
      toast.error(t('Please select items to delete'))
      return
    }
    setDeleteTarget('batch')
    setShowDeleteDialog(true)
  }

  const confirmDelete = () => {
    if (deleteTarget === 'single' && editingAnnouncement) {
      setAnnouncements((prev) =>
        prev.filter((item) => item.id !== editingAnnouncement.id)
      )
      setHasChanges(true)
      toast.success(t('Announcement deleted. Click "Save Settings" to apply.'))
    } else if (deleteTarget === 'batch') {
      setAnnouncements((prev) =>
        prev.filter((item) => !selectedIds.includes(item.id))
      )
      setSelectedIds([])
      setHasChanges(true)
      toast.success(
        t('{{count}} announcements deleted. Click "Save Settings" to apply.', {
          count: selectedIds.length,
        })
      )
    }
    setShowDeleteDialog(false)
    setEditingAnnouncement(null)
  }

  const handleSubmitForm = (values: AnnouncementFormValues) => {
    if (isUploadingImage) return

    const normalizedValues = {
      title: values.title?.trim() || undefined,
      content: values.content,
      publishDate: values.publishDate,
      type: values.type,
      extra: values.extra?.trim() || undefined,
      imageUrl: values.imageUrl?.trim() || undefined,
      popup: values.popup,
      published: values.published,
    }
    if (editingAnnouncement) {
      setAnnouncements((prev) =>
        prev.map((item) =>
          item.id === editingAnnouncement.id
            ? { ...item, ...normalizedValues }
            : item
        )
      )
      toast.success(t('Announcement updated. Click "Save Settings" to apply.'))
    } else {
      const newId = Math.max(...announcements.map((item) => item.id), 0) + 1
      setAnnouncements((prev) => [...prev, { id: newId, ...normalizedValues }])
      toast.success(t('Announcement added. Click "Save Settings" to apply.'))
    }
    setHasChanges(true)
    setShowDialog(false)
  }

  const handleImageUpload = async (file: File) => {
    const supportedTypes = new Set([
      'image/png',
      'image/jpeg',
      'image/gif',
      'image/webp',
    ])
    if (!supportedTypes.has(file.type)) {
      const message = t('Announcement image must be PNG, JPEG, GIF, or WebP')
      setImageUploadError(message)
      toast.error(message)
      return
    }
    if (file.size > imageMaxMiB * 1024 * 1024) {
      const message = t('Announcement image exceeds the configured size limit')
      setImageUploadError(message)
      toast.error(message)
      return
    }

    setIsUploadingImage(true)
    setImageUploadError('')
    try {
      const imageURL = await uploadAnnouncementMedia(file)
      form.setValue('imageUrl', imageURL, {
        shouldDirty: true,
        shouldValidate: true,
      })
    } catch (error) {
      const message =
        error instanceof Error && error.message
          ? t(error.message)
          : t('Upload announcement image failed')
      setImageUploadError(message)
      toast.error(message)
    } finally {
      setIsUploadingImage(false)
    }
  }

  const handleSaveAll = async () => {
    try {
      await updateOption.mutateAsync({
        key: 'console_setting.announcements',
        value: JSON.stringify(announcements),
      })
      setHasChanges(false)
      toast.success(t('Announcements saved successfully'))
    } catch {
      toast.error(t('Failed to save announcements'))
    }
  }

  const handleSavePolicy = () => {
    announcementSettingsMutation.mutate()
  }

  const toggleSelectAll = (checked: boolean) => {
    setSelectedIds(checked ? announcements.map((item) => item.id) : [])
  }

  const toggleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((prev) =>
      checked ? [...prev, id] : prev.filter((item) => item !== id)
    )
  }

  const sortedAnnouncements = useMemo(() => {
    return [...announcements].sort((a, b) => {
      return (
        new Date(b.publishDate).getTime() - new Date(a.publishDate).getTime()
      )
    })
  }, [announcements])

  let announcementSubmitLabel = t('Add')
  if (isUploadingImage) {
    announcementSubmitLabel = t('Uploading...')
  } else if (editingAnnouncement) {
    announcementSubmitLabel = t('Update')
  }

  return (
    <SettingsSection title={t('Announcements')}>
      <div className='space-y-4'>
        <SettingsControlGroup>
          <div className='grid gap-3 lg:grid-cols-2'>
            <SettingsSwitchField
              checked={isEnabled}
              onCheckedChange={setIsEnabled}
              label={t('Enable announcements')}
              description={t(
                'Allow published announcements to appear in the site.'
              )}
              disabled={
                announcementSettingsQuery.isLoading ||
                announcementSettingsQuery.isError
              }
              className='py-0'
            />
            <div className='grid max-w-sm gap-2'>
              <Label htmlFor='announcement-image-max-mib'>
                {t('Maximum announcement image size (MiB)')}
              </Label>
              <Input
                id='announcement-image-max-mib'
                name='announcement-image-max-mib'
                type='number'
                inputMode='numeric'
                autoComplete='off'
                min={1}
                max={5}
                step={1}
                value={imageMaxMiB}
                onChange={(event) =>
                  setImageMaxMiB(
                    Math.max(1, Math.min(5, Number(event.target.value) || 1))
                  )
                }
                disabled={
                  announcementSettingsQuery.isLoading ||
                  announcementSettingsQuery.isError
                }
              />
            </div>
            <div className='flex items-end justify-start lg:justify-end'>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={handleSavePolicy}
                disabled={
                  announcementSettingsQuery.isLoading ||
                  announcementSettingsQuery.isError ||
                  announcementSettingsMutation.isPending
                }
              >
                <Save className='mr-2 h-4 w-4' />
                {announcementSettingsMutation.isPending
                  ? t('Saving...')
                  : t('Save announcement settings')}
              </Button>
            </div>
          </div>
          {announcementSettingsQuery.isError ? (
            <div
              className='border-destructive/30 bg-destructive/5 text-destructive mt-3 flex flex-wrap items-center justify-between gap-3 rounded-md border p-3 text-sm'
              role='alert'
            >
              <span className='flex min-w-0 items-center gap-2'>
                <CircleAlert className='h-4 w-4 shrink-0' aria-hidden='true' />
                <span className='break-words'>
                  {t('Failed to load announcement settings')}
                </span>
              </span>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() => void announcementSettingsQuery.refetch()}
              >
                <RefreshCw className='mr-2 h-4 w-4' aria-hidden='true' />
                {t('Retry')}
              </Button>
            </div>
          ) : null}
        </SettingsControlGroup>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button onClick={handleAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add Announcement')}
            </Button>
            <Button
              onClick={handleBatchDelete}
              size='sm'
              variant='destructive'
              disabled={selectedIds.length === 0}
            >
              <Trash2 className='mr-2 h-4 w-4' />
              {t('Delete (')}
              {selectedIds.length})
            </Button>
            <Button
              onClick={handleSaveAll}
              size='sm'
              variant='secondary'
              disabled={!hasChanges || updateOption.isPending}
            >
              <Save className='mr-2 h-4 w-4' />
              {updateOption.isPending ? t('Saving...') : t('Save Settings')}
            </Button>
          </div>
        </div>

        <StaticDataTable
          data={sortedAnnouncements}
          getRowKey={(announcement) => announcement.id}
          emptyContent={t(
            'No announcements yet. Click "Add Announcement" to create one.'
          )}
          columns={[
            {
              id: 'select',
              header: (
                <Checkbox
                  checked={
                    selectedIds.length === announcements.length &&
                    announcements.length > 0
                  }
                  onCheckedChange={toggleSelectAll}
                />
              ),
              className: 'w-12',
              cell: (announcement) => (
                <Checkbox
                  checked={selectedIds.includes(announcement.id)}
                  onCheckedChange={(checked) =>
                    toggleSelectOne(announcement.id, checked as boolean)
                  }
                />
              ),
            },
            {
              id: 'content',
              header: t('Content'),
              cellClassName: 'max-w-xs',
              cell: (announcement) => (
                <div className='min-w-0'>
                  {announcement.title ? (
                    <div className='truncate text-sm font-medium'>
                      {announcement.title}
                    </div>
                  ) : null}
                  <div className='text-muted-foreground truncate text-xs'>
                    {announcement.content}
                  </div>
                </div>
              ),
            },
            {
              id: 'publish-date',
              header: t('Publish Date'),
              cell: (announcement) => (
                <div className='flex flex-col gap-1'>
                  <span className='text-sm font-medium'>
                    {formatAnnouncementRelativeTime(
                      announcement.publishDate,
                      i18n.resolvedLanguage || i18n.language
                    )}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    {dayjs(announcement.publishDate).format(
                      'YYYY-MM-DD HH:mm:ss'
                    )}
                  </span>
                </div>
              ),
            },
            {
              id: 'type',
              header: t('Type'),
              cell: (announcement) => (
                <StatusBadge
                  label={t(
                    typeOptions.find(
                      (option) => option.value === announcement.type
                    )?.label ?? 'Default'
                  )}
                  variant={
                    typeOptions.find((opt) => opt.value === announcement.type)
                      ?.badgeVariant ?? 'neutral'
                  }
                  copyable={false}
                />
              ),
            },
            {
              id: 'published',
              header: t('Published'),
              cell: (announcement) => (
                <StatusBadge
                  label={announcement.published ? t('Published') : t('Draft')}
                  variant={announcement.published ? 'success' : 'neutral'}
                  copyable={false}
                />
              ),
            },
            {
              id: 'popup',
              header: t('Popup'),
              cell: (announcement) =>
                announcement.popup ? t('Enabled') : t('Disabled'),
            },
            {
              id: 'extra',
              header: t('Extra'),
              cellClassName: 'text-muted-foreground max-w-xs truncate',
              cell: (announcement) => announcement.extra || '-',
            },
            {
              id: 'actions',
              header: t('Actions'),
              cell: (announcement) => (
                <StaticRowActions
                  editLabel={t('Edit')}
                  deleteLabel={t('Delete')}
                  menuLabel={t('Open menu')}
                  onEdit={() => handleEdit(announcement)}
                  onDelete={() => handleDelete(announcement)}
                />
              ),
            },
          ]}
        />
      </div>

      <Dialog
        open={showDialog}
        onOpenChange={(nextOpen) => {
          if (nextOpen || !isUploadingImage) setShowDialog(nextOpen)
        }}
        title={
          editingAnnouncement ? t('Edit Announcement') : t('Add Announcement')
        }
        description={t(
          'Create or update system announcements for the dashboard'
        )}
        contentClassName='max-w-2xl'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={() => setShowDialog(false)}
              disabled={isUploadingImage}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='submit'
              form={ANNOUNCEMENT_FORM_ID}
              disabled={isUploadingImage}
            >
              {announcementSubmitLabel}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={ANNOUNCEMENT_FORM_ID}
            onSubmit={form.handleSubmit(handleSubmitForm)}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='title'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Announcement title')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Optional concise announcement title')}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='content'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Content')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t(
                        'Enter announcement content (supports Markdown/HTML)'
                      )}
                      rows={4}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Supports Markdown and HTML. Each announcement is limited to 64 KiB.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='published'
              render={({ field }) => (
                <FormItem className='flex flex-row items-start gap-3 space-y-0 rounded-md border p-3'>
                  <FormControl>
                    <Checkbox
                      checked={field.value}
                      onCheckedChange={(checked) =>
                        field.onChange(checked === true)
                      }
                    />
                  </FormControl>
                  <div className='space-y-1 leading-none'>
                    <FormLabel>{t('Publish this announcement')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Unpublished announcements remain visible only to administrators.'
                      )}
                    </FormDescription>
                  </div>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='popup'
              render={({ field }) => (
                <FormItem className='flex flex-row items-start gap-3 space-y-0 rounded-md border p-3'>
                  <FormControl>
                    <Checkbox
                      checked={field.value}
                      onCheckedChange={(checked) =>
                        field.onChange(checked === true)
                      }
                    />
                  </FormControl>
                  <div className='space-y-1 leading-none'>
                    <FormLabel>
                      {t('Show this announcement as a popup')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Signed-in users must confirm this announcement once.'
                      )}
                    </FormDescription>
                  </div>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='imageUrl'
              render={({ field }) => {
                const imageURL =
                  field.value && isSafeAnnouncementImageURL(field.value)
                    ? field.value
                    : ''

                return (
                  <FormItem>
                    <FormLabel>{t('Announcement image (optional)')}</FormLabel>
                    <FormControl>
                      <div className='flex items-start gap-3'>
                        <input
                          ref={imageInputRef}
                          type='file'
                          accept='image/png,image/jpeg,image/gif,image/webp'
                          className='sr-only'
                          aria-label={t('Add announcement image')}
                          onChange={(event) => {
                            const file = event.target.files?.[0]
                            event.target.value = ''
                            if (file) void handleImageUpload(file)
                          }}
                        />
                        {imageURL ? (
                          <div className='bg-muted/20 relative h-28 w-28 shrink-0 overflow-hidden rounded-md border'>
                            <img
                              src={imageURL}
                              alt={t('Announcement image')}
                              width={112}
                              height={112}
                              className='h-full w-full object-contain'
                              loading='eager'
                              referrerPolicy='no-referrer'
                            />
                            <div className='absolute top-1 right-1 flex gap-1'>
                              <Button
                                type='button'
                                size='icon-sm'
                                variant='secondary'
                                title={t('Replace announcement image')}
                                aria-label={t('Replace announcement image')}
                                disabled={isUploadingImage}
                                onClick={() => imageInputRef.current?.click()}
                              >
                                <RefreshCw className='h-4 w-4' />
                              </Button>
                              <Button
                                type='button'
                                size='icon-sm'
                                variant='destructive'
                                title={t('Remove announcement image')}
                                aria-label={t('Remove announcement image')}
                                disabled={isUploadingImage}
                                onClick={() => {
                                  field.onChange('')
                                  setImageUploadError('')
                                }}
                              >
                                <Trash2 className='h-4 w-4' />
                              </Button>
                            </div>
                          </div>
                        ) : (
                          <button
                            type='button'
                            className='border-input bg-muted/20 hover:bg-muted/50 text-muted-foreground flex aspect-square h-28 w-28 shrink-0 flex-col items-center justify-center gap-2 rounded-md border border-dashed text-xs transition-colors disabled:cursor-not-allowed disabled:opacity-60'
                            title={t('Add announcement image')}
                            aria-label={t('Add announcement image')}
                            disabled={isUploadingImage}
                            onClick={() => imageInputRef.current?.click()}
                          >
                            {isUploadingImage ? (
                              <LoaderCircle className='h-6 w-6 animate-spin motion-reduce:animate-none' />
                            ) : (
                              <ImagePlus className='h-6 w-6' />
                            )}
                            <span>
                              {isUploadingImage
                                ? t('Uploading...')
                                : t('Add image')}
                            </span>
                          </button>
                        )}
                      </div>
                    </FormControl>
                    <FormDescription>
                      {t('PNG, JPEG, GIF, or WebP. Maximum {{size}} MiB.', {
                        size: imageMaxMiB,
                      })}
                    </FormDescription>
                    {imageUploadError ? (
                      <p className='text-destructive text-sm' role='alert'>
                        {imageUploadError}
                      </p>
                    ) : null}
                    <FormMessage />
                  </FormItem>
                )
              }}
            />
            <FormField
              control={form.control}
              name='publishDate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Publish Date')}</FormLabel>
                  <FormControl>
                    <DateTimePicker
                      value={field.value ? new Date(field.value) : undefined}
                      onChange={(date) =>
                        field.onChange(date ? date.toISOString() : '')
                      }
                      placeholder={t('Select publish date')}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Date and time when this announcement should be displayed'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Type')}</FormLabel>
                  <Select
                    items={typeOptions.map((option) => ({
                      value: option.value,
                      label: (
                        <div className='flex items-center gap-2'>
                          <div
                            className={`h-3 w-3 rounded-full ${option.color}`}
                          />
                          {t(option.label)}
                        </div>
                      ),
                    }))}
                    onValueChange={field.onChange}
                    value={field.value}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue
                          placeholder={t('Select announcement type')}
                        />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {typeOptions.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            <div className='flex items-center gap-2'>
                              <div
                                className={`h-3 w-3 rounded-full ${option.color}`}
                              />
                              {t(option.label)}
                            </div>
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='extra'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Extra Notes (Optional)')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Additional information')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional supplementary information (max 100 characters)'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
      </Dialog>

      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Are you sure?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget === 'single'
                ? t('This announcement will be removed from the list.')
                : t('{{count}} announcements will be removed from the list.', {
                    count: selectedIds.length,
                  })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction variant='destructive' onClick={confirmDelete}>
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
