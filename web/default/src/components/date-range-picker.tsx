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
import { CalendarDays, X } from 'lucide-react'
import { enUS, fr, ja, ru, vi, zhCN, zhTW } from 'react-day-picker/locale'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/lib/utils'

import {
  formatDateRangeLabel,
  getDateRangePreset,
  normalizeDateRange,
  type DateRangeValue,
} from './date-range-picker-state'

export type { DateRangeValue } from './date-range-picker-state'

const calendarLocales = {
  en: enUS,
  zh: zhCN,
  zhCN,
  zhTW,
  fr,
  ru,
  ja,
  vi,
} as const

export type DateRangePickerProps = {
  value: DateRangeValue
  onChange: (value: DateRangeValue) => void
  placeholder?: string
  className?: string
}

export function DateRangePicker({
  value,
  onChange,
  placeholder,
  className,
}: DateRangePickerProps) {
  const { t, i18n } = useTranslation()
  const placeholderText = placeholder ?? t('Date Range')
  const calendarLocale =
    calendarLocales[i18n.language as keyof typeof calendarLocales] ?? enUS

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            className={cn(
              'w-full min-w-0 justify-start gap-2 px-2.5 font-normal tabular-nums',
              !value.from && !value.to && 'text-muted-foreground',
              className
            )}
          />
        }
      >
        <CalendarDays className='size-4 shrink-0 opacity-60' />
        <span className='truncate'>
          {formatDateRangeLabel(value, placeholderText)}
        </span>
      </PopoverTrigger>
      <PopoverContent
        align='end'
        className='w-auto max-w-[calc(100vw-2rem)] overflow-hidden p-0'
      >
        <Calendar
          mode='range'
          selected={value.from ? { from: value.from, to: value.to } : undefined}
          onSelect={(nextValue) =>
            onChange(normalizeDateRange(nextValue ?? {}))
          }
          locale={calendarLocale}
          captionLayout='dropdown'
        />
        <div className='flex flex-wrap gap-1.5 border-t p-2'>
          <Button
            type='button'
            variant='secondary'
            size='sm'
            className='h-7 flex-1 px-2 text-xs'
            onClick={() => onChange(getDateRangePreset('today'))}
          >
            {t('Today')}
          </Button>
          <Button
            type='button'
            variant='secondary'
            size='sm'
            className='h-7 flex-1 px-2 text-xs'
            onClick={() => onChange(getDateRangePreset('7d'))}
          >
            {t('7 Days')}
          </Button>
          <Button
            type='button'
            variant='secondary'
            size='sm'
            className='h-7 flex-1 px-2 text-xs'
            onClick={() => onChange(getDateRangePreset('month'))}
          >
            {t('This month')}
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            aria-label={t('Clear')}
            title={t('Clear')}
            onClick={() => onChange({})}
          >
            <X aria-hidden='true' />
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
