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
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import type {
  ComplianceConfirmationMode,
  ComplianceSourceType,
  ComplianceStatementItem,
} from '@/features/compliance/api'

type ComplianceStatementItemsEditorProps = {
  items: ComplianceStatementItem[]
  onChange: (items: ComplianceStatementItem[]) => void
  disabled?: boolean
}

const sourceTypes: Array<{
  value: ComplianceSourceType
  label: string
}> = [
  { value: 'manual', label: 'Manual statement' },
  { value: 'user_agreement', label: 'User Agreement' },
  { value: 'privacy_policy', label: 'Privacy Policy' },
]

const confirmationModes: Array<{
  value: ComplianceConfirmationMode
  label: string
}> = [
  { value: 'checkbox', label: 'Checkbox confirmation' },
  { value: 'phrase', label: 'Enter confirmation phrase' },
  { value: 'segments', label: 'Segment copy or input' },
]

function createStatementItem(index: number): ComplianceStatementItem {
  return {
    item_key: `statement_${index + 1}`,
    title: '',
    source_type: 'manual',
    version: '',
    content: '',
    sort_order: index + 1,
    confirmation_mode: 'checkbox',
    confirmation_phrase: '',
    segments: [],
  }
}

function resequence(items: ComplianceStatementItem[]) {
  return items.map((item, index) => ({
    ...item,
    sort_order: index + 1,
  }))
}

function toSegmentText(segments?: string[]) {
  return (segments ?? []).join('\n')
}

function fromSegmentText(value: string) {
  return value
    .split('\n')
    .map((segment) => segment.trim())
    .filter(Boolean)
}

export function ComplianceStatementItemsEditor({
  items,
  onChange,
  disabled,
}: ComplianceStatementItemsEditorProps) {
  const { t } = useTranslation()

  const updateItem = (
    index: number,
    patch: Partial<ComplianceStatementItem>
  ) => {
    const next = items.map((item, itemIndex) =>
      itemIndex === index ? { ...item, ...patch } : item
    )
    onChange(next)
  }

  const moveItem = (index: number, direction: -1 | 1) => {
    const targetIndex = index + direction
    if (targetIndex < 0 || targetIndex >= items.length) return
    const next = [...items]
    const current = next[index]
    next[index] = next[targetIndex]
    next[targetIndex] = current
    onChange(resequence(next))
  }

  const addItem = () => {
    onChange([...items, createStatementItem(items.length)])
  }

  const removeItem = (index: number) => {
    onChange(resequence(items.filter((_, itemIndex) => itemIndex !== index)))
  }

  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Manage compliance terms visually. JSON editing still preserves all advanced fields.'
          )}
        </p>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={addItem}
          disabled={disabled}
        >
          <Plus />
          {t('Add term')}
        </Button>
      </div>
      {items.length === 0 ? (
        <div className='text-muted-foreground rounded-md border border-dashed p-6 text-center text-sm'>
          {t(
            'No terms configured yet. The system will use the legacy single statement above.'
          )}
        </div>
      ) : (
        items.map((item, index) => {
          const sourceType = item.source_type || 'manual'
          const confirmationMode = item.confirmation_mode || 'checkbox'
          const contentDisabled = sourceType !== 'manual'
          return (
            <div
              key={`${item.item_key || 'statement'}-${item.sort_order}`}
              className='bg-muted/20 space-y-3 rounded-lg border p-3'
            >
              <div className='flex items-center justify-between gap-3'>
                <div className='text-sm font-medium'>
                  {t('Term')} {index + 1}
                </div>
                <div className='flex gap-1'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    onClick={() => moveItem(index, -1)}
                    disabled={disabled || index === 0}
                    aria-label={t('Move up')}
                    title={t('Move up')}
                  >
                    <ArrowUp />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    onClick={() => moveItem(index, 1)}
                    disabled={disabled || index === items.length - 1}
                    aria-label={t('Move down')}
                    title={t('Move down')}
                  >
                    <ArrowDown />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    onClick={() => removeItem(index)}
                    disabled={disabled}
                    aria-label={t('Delete')}
                    title={t('Delete')}
                  >
                    <Trash2 />
                  </Button>
                </div>
              </div>
              <div className='grid gap-3 md:grid-cols-2'>
                <label className='grid gap-1 text-sm'>
                  <span>{t('Term ID')}</span>
                  <Input
                    value={item.item_key}
                    onChange={(event) =>
                      updateItem(index, { item_key: event.target.value })
                    }
                    disabled={disabled}
                  />
                </label>
                <label className='grid gap-1 text-sm'>
                  <span>{t('Title')}</span>
                  <Input
                    value={item.title}
                    onChange={(event) =>
                      updateItem(index, { title: event.target.value })
                    }
                    disabled={disabled}
                  />
                </label>
                <label className='grid gap-1 text-sm'>
                  <span>{t('Source type')}</span>
                  <NativeSelect
                    value={sourceType}
                    onChange={(event) => {
                      const nextSource = event.target
                        .value as ComplianceSourceType
                      updateItem(index, {
                        source_type: nextSource,
                        content: nextSource === 'manual' ? item.content : '',
                      })
                    }}
                    disabled={disabled}
                    className='w-full'
                  >
                    {sourceTypes.map((source) => (
                      <NativeSelectOption
                        key={source.value}
                        value={source.value}
                      >
                        {t(source.label)}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </label>
                <label className='grid gap-1 text-sm'>
                  <span>{t('Version number')}</span>
                  <Input
                    value={item.version}
                    onChange={(event) =>
                      updateItem(index, { version: event.target.value })
                    }
                    disabled={disabled}
                  />
                </label>
                <label className='grid gap-1 text-sm'>
                  <span>{t('Confirmation method')}</span>
                  <NativeSelect
                    value={confirmationMode}
                    onChange={(event) => {
                      const nextMode = event.target
                        .value as ComplianceConfirmationMode
                      updateItem(index, {
                        confirmation_mode: nextMode,
                        confirmation_phrase:
                          nextMode === 'phrase'
                            ? item.confirmation_phrase || ''
                            : '',
                        segments:
                          nextMode === 'segments' ? (item.segments ?? []) : [],
                      })
                    }}
                    disabled={disabled}
                    className='w-full'
                  >
                    {confirmationModes.map((mode) => (
                      <NativeSelectOption key={mode.value} value={mode.value}>
                        {t(mode.label)}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </label>
                <label className='grid gap-1 text-sm'>
                  <span>{t('Sort order')}</span>
                  <Input
                    type='number'
                    value={item.sort_order}
                    onChange={(event) =>
                      updateItem(index, {
                        sort_order: Number(event.target.value) || 0,
                      })
                    }
                    disabled={disabled}
                  />
                </label>
              </div>
              <label className='grid gap-1 text-sm'>
                <span>{t('Body')}</span>
                <Textarea
                  rows={5}
                  value={item.content}
                  onChange={(event) =>
                    updateItem(index, { content: event.target.value })
                  }
                  disabled={disabled || contentDisabled}
                  placeholder={
                    contentDisabled
                      ? t(
                          'Content comes from the current user agreement or privacy policy. Backend snapshots are used when saving.'
                        )
                      : undefined
                  }
                />
              </label>
              {confirmationMode === 'phrase' ? (
                <label className='grid gap-1 text-sm'>
                  <span>{t('Confirmation phrase')}</span>
                  <Input
                    value={item.confirmation_phrase ?? ''}
                    onChange={(event) =>
                      updateItem(index, {
                        confirmation_phrase: event.target.value,
                      })
                    }
                    disabled={disabled}
                  />
                </label>
              ) : null}
              {confirmationMode === 'segments' ? (
                <label className='grid gap-1 text-sm'>
                  <span>{t('Segment content')}</span>
                  <Textarea
                    rows={5}
                    value={toSegmentText(item.segments)}
                    onChange={(event) =>
                      updateItem(index, {
                        segments: fromSegmentText(event.target.value),
                      })
                    }
                    disabled={disabled}
                    placeholder={t(
                      'One segment per line for users to copy or type for confirmation'
                    )}
                  />
                </label>
              ) : null}
            </div>
          )
        })
      )}
    </div>
  )
}
