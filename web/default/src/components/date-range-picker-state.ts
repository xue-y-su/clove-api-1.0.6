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
import dayjs from '@/lib/dayjs'

export type DateRangeValue = { from?: Date; to?: Date }

export function normalizeDateRange(value: DateRangeValue): DateRangeValue {
  return {
    from: value.from ? dayjs(value.from).startOf('day').toDate() : undefined,
    to: value.to ? dayjs(value.to).endOf('day').toDate() : undefined,
  }
}

export function formatDateRangeLabel(
  value: DateRangeValue,
  placeholder: string
): string {
  if (!value.from && !value.to) return placeholder
  const from = value.from ? dayjs(value.from).format('YYYY-MM-DD') : '-'
  const to = value.to ? dayjs(value.to).format('YYYY-MM-DD') : '-'
  return `${from} ~ ${to}`
}

export function getDateRangePreset(
  preset: 'today' | '7d' | 'month',
  now = new Date()
): DateRangeValue {
  const current = dayjs(now)
  if (preset === 'today') {
    return {
      from: current.startOf('day').toDate(),
      to: current.endOf('day').toDate(),
    }
  }
  if (preset === '7d') {
    return {
      from: current.subtract(6, 'day').startOf('day').toDate(),
      to: current.endOf('day').toDate(),
    }
  }
  return {
    from: current.startOf('month').toDate(),
    to: current.endOf('month').toDate(),
  }
}
