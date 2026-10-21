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
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { SourceAttributionAdminOption } from '@/features/source-attribution/api'

type SourceAttributionOptionsEditorProps = {
  options: SourceAttributionAdminOption[]
  onChange: (options: SourceAttributionAdminOption[]) => void
  disabled?: boolean
}

function createOption(index: number): SourceAttributionAdminOption {
  return {
    id: `option_${index + 1}`,
    name: '',
    target_user_id: 0,
    requires_description: false,
    sort_order: index + 1,
  }
}

function resequence(options: SourceAttributionAdminOption[]) {
  return options.map((option, index) => ({
    ...option,
    sort_order: index + 1,
  }))
}

export function SourceAttributionOptionsEditor({
  options,
  onChange,
  disabled,
}: SourceAttributionOptionsEditorProps) {
  const { t } = useTranslation()

  const updateOption = (
    index: number,
    patch: Partial<SourceAttributionAdminOption>
  ) => {
    const next = options.map((option, optionIndex) =>
      optionIndex === index ? { ...option, ...patch } : option
    )
    onChange(next)
  }

  const moveOption = (index: number, direction: -1 | 1) => {
    const targetIndex = index + direction
    if (targetIndex < 0 || targetIndex >= options.length) return
    const next = [...options]
    const current = next[index]
    next[index] = next[targetIndex]
    next[targetIndex] = current
    onChange(resequence(next))
  }

  const addOption = () => {
    onChange([...options, createOption(options.length)])
  }

  const removeOption = (index: number) => {
    onChange(
      resequence(options.filter((_, optionIndex) => optionIndex !== index))
    )
  }

  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Manage source attribution options visually. Saving still writes through the dedicated API.'
          )}
        </p>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={addOption}
          disabled={disabled}
        >
          <Plus />
          {t('Add option')}
        </Button>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='min-w-32'>{t('Option ID')}</TableHead>
            <TableHead className='min-w-32'>{t('Display name')}</TableHead>
            <TableHead className='min-w-36'>{t('Target user ID')}</TableHead>
            <TableHead className='w-28'>{t('Description required')}</TableHead>
            <TableHead className='w-24'>{t('Sort order')}</TableHead>
            <TableHead className='w-32 text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {options.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={6}
                className='text-muted-foreground py-6 text-center'
              >
                {t('No source attribution options yet')}
              </TableCell>
            </TableRow>
          ) : (
            options.map((option, index) => (
              <TableRow key={`${option.id || 'option'}-${option.sort_order}`}>
                <TableCell>
                  <Input
                    value={option.id}
                    onChange={(event) =>
                      updateOption(index, { id: event.target.value })
                    }
                    disabled={disabled}
                  />
                </TableCell>
                <TableCell>
                  <Input
                    value={option.name}
                    onChange={(event) =>
                      updateOption(index, { name: event.target.value })
                    }
                    disabled={disabled}
                  />
                </TableCell>
                <TableCell>
                  <Input
                    type='number'
                    min={1}
                    value={option.target_user_id}
                    onChange={(event) =>
                      updateOption(index, {
                        target_user_id: Number(event.target.value) || 0,
                      })
                    }
                    disabled={disabled}
                  />
                </TableCell>
                <TableCell>
                  <Checkbox
                    checked={option.requires_description}
                    onCheckedChange={(checked) =>
                      updateOption(index, { requires_description: !!checked })
                    }
                    disabled={disabled}
                    aria-label={t('Require source description')}
                  />
                </TableCell>
                <TableCell>
                  <Input
                    type='number'
                    value={option.sort_order}
                    onChange={(event) =>
                      updateOption(index, {
                        sort_order: Number(event.target.value) || 0,
                      })
                    }
                    disabled={disabled}
                  />
                </TableCell>
                <TableCell>
                  <div className='flex justify-end gap-1'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      onClick={() => moveOption(index, -1)}
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
                      onClick={() => moveOption(index, 1)}
                      disabled={disabled || index === options.length - 1}
                      aria-label={t('Move down')}
                      title={t('Move down')}
                    >
                      <ArrowDown />
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      onClick={() => removeOption(index)}
                      disabled={disabled}
                      aria-label={t('Delete')}
                      title={t('Delete')}
                    >
                      <Trash2 />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  )
}
