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
import { ArrowDown, ArrowUp, ChevronRight, RotateCcw, X } from 'lucide-react'
import { useMemo, type ComponentProps, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'

type AutoGroupOrderEditorProps = Omit<ComponentProps<'div'>, 'onChange'> & {
  value: string[]
  mode: 'inherit' | 'custom'
  options: ApiKeyGroupOption[]
  globalOptions: ApiKeyGroupOption[]
  maxCount: number
  onChange: (value: { groups: string[]; mode: 'inherit' | 'custom' }) => void
}

export function AutoGroupOrderEditor({
  value,
  mode,
  options,
  globalOptions,
  maxCount: configuredMaxCount,
  onChange,
  className,
  ...props
}: AutoGroupOrderEditorProps) {
  const { t } = useTranslation()
  const maxCount =
    Number.isInteger(configuredMaxCount) && configuredMaxCount > 0
      ? configuredMaxCount
      : 5
  const isInheriting = mode === 'inherit'
  const atLimit = value.length >= maxCount
  const optionMap = useMemo(
    () => new Map(options.map((option) => [option.value, option])),
    [options]
  )
  const candidates = useMemo(
    () =>
      options.filter(
        (option) => option.value !== 'auto' && !value.includes(option.value)
      ),
    [options, value]
  )

  const move = (index: number, direction: -1 | 1) => {
    const target = index + direction
    if (target < 0 || target >= value.length) return
    const next = [...value]
    ;[next[index], next[target]] = [next[target], next[index]]
    onChange({ groups: next, mode: 'custom' })
  }

  let orderContent: ReactNode
  if (isInheriting && globalOptions.length === 0) {
    orderContent = (
      <p className='text-muted-foreground rounded-md border px-3 py-5 text-center text-xs'>
        {t('No available groups in the global Auto order.')}
      </p>
    )
  } else if (isInheriting) {
    orderContent = (
      <ol
        className='flex max-h-28 flex-wrap content-start gap-1.5 overflow-y-auto rounded-md border p-2'
        aria-label={t('Inherit global Auto order')}
      >
        {globalOptions.map((option, index) => (
          <li key={option.value} className='flex min-w-0 items-center gap-1'>
            {index > 0 && (
              <ChevronRight className='text-muted-foreground size-3.5 shrink-0' />
            )}
            <span
              title={option.desc}
              className='bg-muted/40 flex min-w-0 items-center gap-1.5 rounded border px-2 py-1'
            >
              <span className='bg-primary/10 text-primary flex size-4 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold tabular-nums'>
                {index + 1}
              </span>
              <span className='max-w-40 truncate text-xs font-medium'>
                {option.label}
              </span>
            </span>
          </li>
        ))}
      </ol>
    )
  } else if (value.length === 0) {
    orderContent = (
      <p className='text-destructive rounded-md border px-3 py-5 text-center text-xs'>
        {t(
          'No valid custom Auto groups remain. Add a group or restore global Auto.'
        )}
      </p>
    )
  } else {
    orderContent = (
      <ol className='space-y-2'>
        {value.map((group, index) => {
          const option = optionMap.get(group)
          return (
            <li
              key={group}
              className='bg-background flex min-h-11 items-center gap-2 rounded-md border px-2 py-1.5'
            >
              <span className='bg-primary/10 text-primary flex size-5 shrink-0 items-center justify-center rounded-full text-xs font-semibold tabular-nums'>
                {index + 1}
              </span>
              <span className='min-w-0 flex-1'>
                <span className='block truncate text-sm font-medium'>
                  {option?.label || group}
                </span>
                {option?.desc && (
                  <span className='text-muted-foreground block truncate text-xs'>
                    {option.desc}
                  </span>
                )}
              </span>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                disabled={index === 0}
                title={t('Move {{group}} up', { group })}
                aria-label={t('Move {{group}} up', { group })}
                onClick={() => move(index, -1)}
              >
                <ArrowUp className='size-4' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                disabled={index === value.length - 1}
                title={t('Move {{group}} down', { group })}
                aria-label={t('Move {{group}} down', { group })}
                onClick={() => move(index, 1)}
              >
                <ArrowDown className='size-4' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                title={t('Remove {{group}}', { group })}
                aria-label={t('Remove {{group}}', { group })}
                onClick={() =>
                  onChange({
                    groups: value.filter((item) => item !== group),
                    mode: 'custom',
                  })
                }
              >
                <X className='size-4' />
              </Button>
            </li>
          )
        })}
      </ol>
    )
  }

  return (
    <div
      {...props}
      role='group'
      aria-label={props['aria-label'] || t('Auto group order')}
      className={cn('space-y-3', className)}
    >
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <p className='text-muted-foreground text-xs' aria-live='polite'>
          {isInheriting
            ? t('Using the complete global Auto order ({{count}} groups)', {
                count: globalOptions.length,
              })
            : t('{{count}} / {{max}} groups selected', {
                count: value.length,
                max: maxCount,
              })}
        </p>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={isInheriting}
          onClick={() => onChange({ groups: [], mode: 'inherit' })}
        >
          <RotateCcw className='size-4' />
          {t('Restore global Auto')}
        </Button>
      </div>

      <ApiKeyGroupCombobox
        options={candidates}
        value={undefined}
        onValueChange={(group) => {
          if (atLimit || value.includes(group)) return
          onChange({ groups: [...value, group], mode: 'custom' })
        }}
        placeholder={
          atLimit
            ? t('Maximum {{max}} groups selected', { max: maxCount })
            : t('Add Auto group')
        }
        disabled={atLimit || candidates.length === 0}
        compact
      />

      {orderContent}
    </div>
  )
}
