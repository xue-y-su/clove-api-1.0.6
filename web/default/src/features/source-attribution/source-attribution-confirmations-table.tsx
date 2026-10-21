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
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { getSourceAttributionConfirmations } from './api'

const pageSize = 20

function formatTime(timestamp: number) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

export function SourceAttributionConfirmationsTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState({
    username: '',
    option_id: '',
    target_user_id: '',
    ip: '',
  })
  const confirmationsQuery = useQuery({
    queryKey: ['source-attribution-confirmations', filters, page],
    queryFn: () =>
      getSourceAttributionConfirmations({
        p: page,
        page_size: pageSize,
        username: filters.username || undefined,
        option_id: filters.option_id || undefined,
        target_user_id: Number(filters.target_user_id) || undefined,
        ip: filters.ip || undefined,
      }),
  })
  const confirmations = confirmationsQuery.data?.data?.items ?? []
  const total = confirmationsQuery.data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const updateFilter = (key: keyof typeof filters, value: string) => {
    setPage(1)
    setFilters((current) => ({ ...current, [key]: value }))
  }

  return (
    <div className='flex h-full min-h-0 flex-col gap-4'>
      <div className='grid shrink-0 gap-3 md:grid-cols-4'>
        <Input
          value={filters.username}
          onChange={(event) => updateFilter('username', event.target.value)}
          placeholder={t('User')}
        />
        <Input
          value={filters.option_id}
          onChange={(event) => updateFilter('option_id', event.target.value)}
          placeholder={t('Option ID')}
        />
        <Input
          type='number'
          min={1}
          value={filters.target_user_id}
          onChange={(event) =>
            updateFilter('target_user_id', event.target.value)
          }
          placeholder={t('Target user ID')}
        />
        <Input
          value={filters.ip}
          onChange={(event) => updateFilter('ip', event.target.value)}
          placeholder='IP'
        />
      </div>
      <div className='min-h-0 flex-1 overflow-y-auto rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Attribution option')}</TableHead>
              <TableHead>{t('Target user')}</TableHead>
              <TableHead>{t('Target user ID')}</TableHead>
              <TableHead>{t('Source description')}</TableHead>
              <TableHead>IP</TableHead>
              <TableHead>{t('Confirmation time')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {confirmations.map((item) => (
              <TableRow key={item.id}>
                <TableCell>{item.username}</TableCell>
                <TableCell>{item.option_name || item.option_id}</TableCell>
                <TableCell>{item.target_username || '-'}</TableCell>
                <TableCell>{item.target_user_id || '-'}</TableCell>
                <TableCell className='max-w-64 truncate'>
                  {item.source_description || '-'}
                </TableCell>
                <TableCell>{item.ip || '-'}</TableCell>
                <TableCell>{formatTime(item.confirmed_at)}</TableCell>
              </TableRow>
            ))}
            {confirmations.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-muted-foreground h-20 text-center'
                >
                  {confirmationsQuery.isLoading
                    ? t('Loading...')
                    : t('No data')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className='flex shrink-0 items-center justify-between gap-3'>
        <div className='text-muted-foreground text-sm'>
          {t('Total')}: {total}
        </div>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            onClick={() => confirmationsQuery.refetch()}
            disabled={confirmationsQuery.isFetching}
          >
            {t('Refresh records')}
          </Button>
          <Button
            variant='outline'
            onClick={() => setPage((current) => Math.max(1, current - 1))}
            disabled={page <= 1 || confirmationsQuery.isFetching}
          >
            {t('Previous page')}
          </Button>
          <span className='text-muted-foreground text-sm'>
            {page} / {totalPages}
          </span>
          <Button
            variant='outline'
            onClick={() =>
              setPage((current) => Math.min(totalPages, current + 1))
            }
            disabled={page >= totalPages || confirmationsQuery.isFetching}
          >
            {t('Next page')}
          </Button>
        </div>
      </div>
    </div>
  )
}
