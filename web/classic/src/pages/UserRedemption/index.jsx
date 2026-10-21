/*
Copyright (C) 2025 QuantumNous

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
import { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Input,
  Pagination,
  Select,
  Typography,
} from '@douyinfe/semi-ui';
import { RotateCcw, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import AdminUserRedemptionGovernanceModal from '../../components/user-redemptions/AdminUserRedemptionGovernanceModal.jsx';
import AdminBulkSettlementModal from '../../components/user-redemptions/AdminBulkSettlementModal.jsx';
import UserRedemptionDetailModal from '../../components/user-redemptions/UserRedemptionDetailModal.jsx';
import UserRedemptionsTable from '../../components/user-redemptions/UserRedemptionsTable.jsx';
import { listUserRedemptions } from '../../components/user-redemptions/api.js';
import { showError } from '../../helpers';

const { Title, Text } = Typography;
const PAGE_SIZE = 20;

const EMPTY_FILTERS = {
  creatorId: '',
  creatorUsername: '',
  rewardType: '',
  status: '',
  kind: '',
  deleted: 'all',
  createdFrom: '',
  createdTo: '',
  expiresFrom: '',
  expiresTo: '',
};

const STATUS_OPTIONS = [
  { value: 'active', label: 'Active' },
  { value: 'disabled', label: 'Disabled' },
  { value: 'exhausted', label: 'Exhausted' },
  { value: 'expired_pending', label: 'Pending settlement' },
  { value: 'refund_pending', label: 'Refund pending' },
  { value: 'refunded', label: 'Refunded' },
  { value: 'forfeited', label: 'Forfeited' },
  { value: 'deleted', label: 'Deleted' },
];

function dateToUnix(value, endOfDay = false) {
  if (!value) return undefined;
  const timestamp = new Date(
    `${value}${endOfDay ? 'T23:59:59' : 'T00:00:00'}`,
  ).getTime();
  return Number.isFinite(timestamp) ? Math.floor(timestamp / 1000) : undefined;
}

function buildParams(filters, page) {
  const creatorId = Number.parseInt(filters.creatorId, 10);
  return {
    page,
    page_size: PAGE_SIZE,
    creator_user_id: Number.isFinite(creatorId) ? creatorId : undefined,
    creator_username: filters.creatorUsername.trim() || undefined,
    reward_type: filters.rewardType || undefined,
    status: filters.status || undefined,
    kind: filters.kind || undefined,
    deleted: filters.deleted,
    created_from: dateToUnix(filters.createdFrom),
    created_to: dateToUnix(filters.createdTo, true),
    expires_from: dateToUnix(filters.expiresFrom),
    expires_to: dateToUnix(filters.expiresTo, true),
  };
}

export default function UserRedemptionPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [draftFilters, setDraftFilters] = useState(EMPTY_FILTERS);
  const [filters, setFilters] = useState(EMPTY_FILTERS);
  const [selectedId, setSelectedId] = useState(null);
  const [governanceTarget, setGovernanceTarget] = useState(null);
  const [governanceAction, setGovernanceAction] = useState(null);
  const [selectedIds, setSelectedIds] = useState([]);
  const [bulkSettlementVisible, setBulkSettlementVisible] = useState(false);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listUserRedemptions(
        buildParams(filters, page),
        true,
      );
      setItems(result.items || []);
      setTotal(result.total || 0);
      setSelectedIds((current) =>
        current.filter((id) => result.items?.some((item) => item.id === id)),
      );
      setError('');
    } catch (caught) {
      const message = caught?.message || t('load failed');
      setError(message);
      showError(message);
    } finally {
      setLoading(false);
    }
  }, [filters, page, t]);

  useEffect(() => {
    void reload();
  }, [reload]);

  function openGovernance(id, action) {
    const target = items.find((item) => item.id === id);
    if (!target) return;
    setGovernanceTarget(target);
    setGovernanceAction(action);
  }

  function closeGovernance() {
    setGovernanceTarget(null);
    setGovernanceAction(null);
  }

  return (
    <div className='space-y-4 p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <Title heading={3} style={{ margin: 0 }}>
            {t('User Redemption Codes')}
          </Title>
          <Text type='tertiary'>
            {t(
              'Audit and govern wallet-funded redemption codes created by users.',
            )}
          </Text>
        </div>
        <div className='flex flex-col items-end gap-2'>
          <Text type='tertiary'>
            {t('Total')}: {total}
          </Text>
          <Button
            theme='outline'
            disabled={selectedIds.length === 0}
            onClick={() => setBulkSettlementVisible(true)}
          >
            {t('Batch settlement')} ({selectedIds.length})
          </Button>
        </div>
      </div>

      <section className='space-y-3 border-y border-[var(--semi-color-border)] py-4'>
        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-6'>
          <Input
            value={draftFilters.creatorUsername}
            placeholder={t('Creator username')}
            onChange={(value) =>
              setDraftFilters((current) => ({
                ...current,
                creatorUsername: value,
              }))
            }
          />
          <Input
            value={draftFilters.creatorId}
            type='number'
            placeholder={t('Creator ID')}
            onChange={(value) =>
              setDraftFilters((current) => ({ ...current, creatorId: value }))
            }
          />
          <Select
            value={draftFilters.rewardType}
            placeholder={t('All reward types')}
            onChange={(value) =>
              setDraftFilters((current) => ({
                ...current,
                rewardType: value,
              }))
            }
            style={{ width: '100%' }}
          >
            <Select.Option value=''>{t('All reward types')}</Select.Option>
            <Select.Option value='balance'>{t('Balance')}</Select.Option>
            <Select.Option value='subscription'>
              {t('Subscription')}
            </Select.Option>
          </Select>
          <Select
            value={draftFilters.kind}
            placeholder={t('All modes')}
            onChange={(value) =>
              setDraftFilters((current) => ({ ...current, kind: value }))
            }
            style={{ width: '100%' }}
          >
            <Select.Option value=''>{t('All modes')}</Select.Option>
            <Select.Option value='single'>{t('Single')}</Select.Option>
            <Select.Option value='random'>{t('Red packet')}</Select.Option>
            <Select.Option value='fixed'>{t('Fixed units')}</Select.Option>
          </Select>
          <Select
            value={draftFilters.status}
            placeholder={t('All statuses')}
            onChange={(value) =>
              setDraftFilters((current) => ({ ...current, status: value }))
            }
            style={{ width: '100%' }}
          >
            <Select.Option value=''>{t('All statuses')}</Select.Option>
            {STATUS_OPTIONS.map((status) => (
              <Select.Option key={status.value} value={status.value}>
                {t(status.label)}
              </Select.Option>
            ))}
          </Select>
          <Select
            value={draftFilters.deleted}
            onChange={(value) =>
              setDraftFilters((current) => ({ ...current, deleted: value }))
            }
            style={{ width: '100%' }}
          >
            <Select.Option value='all'>{t('All records')}</Select.Option>
            <Select.Option value='false'>{t('Not deleted')}</Select.Option>
            <Select.Option value='true'>{t('Deleted')}</Select.Option>
          </Select>
        </div>

        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
          <DateFilter
            label={t('Created from')}
            value={draftFilters.createdFrom}
            onChange={(value) =>
              setDraftFilters((current) => ({ ...current, createdFrom: value }))
            }
          />
          <DateFilter
            label={t('Created to')}
            value={draftFilters.createdTo}
            onChange={(value) =>
              setDraftFilters((current) => ({ ...current, createdTo: value }))
            }
          />
          <DateFilter
            label={t('Expires from')}
            value={draftFilters.expiresFrom}
            onChange={(value) =>
              setDraftFilters((current) => ({ ...current, expiresFrom: value }))
            }
          />
          <DateFilter
            label={t('Expires to')}
            value={draftFilters.expiresTo}
            onChange={(value) =>
              setDraftFilters((current) => ({ ...current, expiresTo: value }))
            }
          />
        </div>

        <div className='flex justify-end gap-2'>
          <Button
            icon={<RotateCcw size={15} />}
            onClick={() => {
              setDraftFilters(EMPTY_FILTERS);
              setFilters(EMPTY_FILTERS);
              setPage(1);
            }}
          >
            {t('Reset')}
          </Button>
          <Button
            theme='solid'
            type='primary'
            icon={<Search size={15} />}
            onClick={() => {
              setFilters(draftFilters);
              setPage(1);
            }}
          >
            {t('Search')}
          </Button>
        </div>
      </section>

      {error ? <Text type='danger'>{error}</Text> : null}
      <UserRedemptionsTable
        items={items}
        loading={loading}
        showCreator
        onView={setSelectedId}
        onDisable={(id) => openGovernance(id, 'disable')}
        onEnable={(id) => openGovernance(id, 'enable')}
        onTerminate={(id) => openGovernance(id, 'terminate')}
        onDelete={(id) => openGovernance(id, 'delete')}
        selectedIds={selectedIds}
        onSelectedIdsChange={setSelectedIds}
      />
      {total > PAGE_SIZE ? (
        <div className='flex justify-end'>
          <Pagination
            currentPage={page}
            pageSize={PAGE_SIZE}
            total={total}
            onPageChange={setPage}
            showTotal
          />
        </div>
      ) : null}

      <UserRedemptionDetailModal
        id={selectedId}
        admin
        onClose={() => setSelectedId(null)}
      />
      <AdminUserRedemptionGovernanceModal
        target={governanceTarget}
        action={governanceAction}
        onClose={closeGovernance}
        onCompleted={reload}
      />
      <AdminBulkSettlementModal
        visible={bulkSettlementVisible}
        ids={selectedIds}
        onClose={() => setBulkSettlementVisible(false)}
        onCompleted={async () => {
          setSelectedIds([]);
          await reload();
        }}
      />
    </div>
  );
}

function DateFilter({ label, value, onChange }) {
  return (
    <label className='space-y-1 text-xs text-[var(--semi-color-text-2)]'>
      <span>{label}</span>
      <Input type='date' value={value} onChange={onChange} />
    </label>
  );
}
