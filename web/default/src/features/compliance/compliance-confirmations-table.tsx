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

import { getComplianceConfirmations } from './api'

const pageSize = 20

function formatTime(timestamp: number) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

export function ComplianceConfirmationsTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState({ username: '', version: '', ip: '' })
  const confirmationsQuery = useQuery({
    queryKey: ['compliance-confirmations', filters, page],
    queryFn: () =>
      getComplianceConfirmations({
        p: page,
        page_size: pageSize,
        username: filters.username || undefined,
        version: filters.version || undefined,
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
    <div className='flex min-h-0 flex-col gap-4'>
      <div className='grid gap-3 md:grid-cols-3'>
        <Input
          value={filters.username}
          onChange={(event) => updateFilter('username', event.target.value)}
          placeholder={t('User')}
        />
        <Input
          value={filters.version}
          onChange={(event) => updateFilter('version', event.target.value)}
          placeholder={t('Version number')}
        />
        <Input
          value={filters.ip}
          onChange={(event) => updateFilter('ip', event.target.value)}
          placeholder='IP'
        />
      </div>
      <div className='rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Version number')}</TableHead>
              <TableHead>{t('IP')}</TableHead>
              <TableHead>{t('Confirmation time')}</TableHead>
              <TableHead>{t('User agent')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {confirmations.map((item) => (
              <TableRow key={item.id}>
                <TableCell>{item.username}</TableCell>
                <TableCell>{item.version}</TableCell>
                <TableCell>{item.ip || '-'}</TableCell>
                <TableCell>{formatTime(item.confirmed_at)}</TableCell>
                <TableCell className='max-w-80 truncate'>
                  {item.user_agent || '-'}
                </TableCell>
              </TableRow>
            ))}
            {confirmations.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={5}
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
      <div className='flex items-center justify-between gap-3'>
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
