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
import React from 'react';
import { Button, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { Ban, CircleCheck, Copy, Eye, OctagonX, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { copy, renderQuota, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  canSettleUserRedemption,
  getUserRedemptionKindTranslationKey,
  getUserRedemptionRewardTranslationKey,
  getUserRedemptionStatusTranslationKey,
  shouldShowDeletedBadge,
} from './user-redemption-state.js';

const { Text } = Typography;

function formatTs(timestamp) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString() : '-';
}

function RedemptionActions({
  row,
  busy,
  compact,
  onCancel,
  onView,
  onDisable,
  onEnable,
  onTerminate,
  onDelete,
}) {
  const { t } = useTranslation();

  return (
    <div className='flex flex-wrap gap-1'>
      <Button
        size='small'
        theme={compact ? 'borderless' : undefined}
        icon={<Copy size={14} />}
        aria-label={t('Copy')}
        onClick={async () => {
          if (await copy(row.code)) {
            showSuccess(t('Copied to clipboard'));
          } else {
            showError(t('Failed to copy to clipboard'));
          }
        }}
      >
        {compact ? null : t('Copy')}
      </Button>
      <Button
        size='small'
        theme={compact ? 'borderless' : undefined}
        icon={<Eye size={14} />}
        aria-label={t('View')}
        onClick={() => onView?.(row.id)}
      >
        {compact ? null : t('View')}
      </Button>
      {row.status === 'active' && onCancel ? (
        <Button
          size='small'
          type='danger'
          theme='borderless'
          icon={compact ? <Ban size={14} /> : undefined}
          aria-label={t('Cancel')}
          disabled={busy}
          onClick={() => onCancel(row.id)}
        >
          {compact ? null : t('Cancel')}
        </Button>
      ) : null}
      {row.status === 'active' && onDisable ? (
        <Button
          size='small'
          theme={compact ? 'borderless' : undefined}
          icon={compact ? <Ban size={14} /> : undefined}
          aria-label={t('Disable')}
          onClick={() => onDisable(row.id)}
        >
          {compact ? null : t('Disable')}
        </Button>
      ) : null}
      {row.status === 'disabled' && onEnable ? (
        <Button
          size='small'
          theme={compact ? 'borderless' : undefined}
          icon={compact ? <CircleCheck size={14} /> : undefined}
          aria-label={t('Enable')}
          onClick={() => onEnable(row.id)}
        >
          {compact ? null : t('Enable')}
        </Button>
      ) : null}
      {(row.status === 'active' ||
        row.status === 'disabled' ||
        row.status === 'expired_pending') &&
      onTerminate ? (
        <Button
          size='small'
          type='warning'
          theme={compact ? 'borderless' : undefined}
          icon={compact ? <OctagonX size={14} /> : undefined}
          aria-label={t('Terminate')}
          onClick={() => onTerminate(row.id)}
        >
          {compact ? null : t('Terminate')}
        </Button>
      ) : null}
      {!row.deleted && onDelete ? (
        <Button
          size='small'
          type='danger'
          theme={compact ? 'borderless' : undefined}
          icon={compact ? <Trash2 size={14} /> : undefined}
          aria-label={t('Delete')}
          onClick={() => onDelete(row.id)}
        >
          {compact ? null : t('Delete')}
        </Button>
      ) : null}
    </div>
  );
}

export default function UserRedemptionsTable({
  items = [],
  loading,
  busy,
  showCreator,
  onCancel,
  onView,
  onDisable,
  onEnable,
  onTerminate,
  onDelete,
  selectedIds = [],
  onSelectedIdsChange,
}) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  function toggleSelection(row, checked) {
    if (!onSelectedIdsChange || !canSettleUserRedemption(row)) return;
    if (checked) {
      onSelectedIdsChange([...new Set([...selectedIds, row.id])]);
      return;
    }
    onSelectedIdsChange(selectedIds.filter((id) => id !== row.id));
  }

  if (isMobile) {
    if (loading) {
      return <div className='text-sm text-gray-500'>{t('Loading...')}</div>;
    }
    if (!items.length) {
      return <div className='text-sm text-gray-500'>{t('No codes yet.')}</div>;
    }

    return (
      <div className='divide-y rounded-md border'>
        {items.map((row) => (
          <article key={row.id} className='space-y-3 p-3'>
            <div className='flex items-start gap-2'>
              {onSelectedIdsChange ? (
                <input
                  type='checkbox'
                  className='mt-1 h-4 w-4 shrink-0'
                  checked={selectedIds.includes(row.id)}
                  disabled={!canSettleUserRedemption(row)}
                  aria-label={t('Select redemption code {{code}}', {
                    code: row.code,
                  })}
                  onChange={(event) =>
                    toggleSelection(row, event.target.checked)
                  }
                />
              ) : null}
              <div>
                <div className='break-all font-mono text-xs leading-5'>
                  {row.code}
                </div>
                <div className='mt-1 flex flex-wrap items-center gap-1'>
                  <Tag>
                    {t(getUserRedemptionRewardTranslationKey(row.reward_type))}
                  </Tag>
                  <Tag>{t(getUserRedemptionKindTranslationKey(row.kind))}</Tag>
                  <Tag>
                    {t(getUserRedemptionStatusTranslationKey(row.status))}
                  </Tag>
                  {shouldShowDeletedBadge(row.status, row.deleted) ? (
                    <Tag color='red'>{t('Deleted')}</Tag>
                  ) : null}
                </div>
              </div>
            </div>

            {row.reward_type === 'subscription' && row.plan_title ? (
              <div className='text-xs'>
                <Text type='tertiary' size='small'>
                  {t('Plan')}
                </Text>
                <div>{row.plan_title}</div>
              </div>
            ) : null}

            {showCreator ? (
              <div className='text-xs'>
                <Text type='tertiary' size='small'>
                  {t('Creator ID')}
                </Text>
                <div>
                  #{row.creator_user_id}
                  {row.creator_username ? ` · ${row.creator_username}` : ''}
                </div>
              </div>
            ) : null}

            <dl className='grid grid-cols-2 gap-x-4 gap-y-2 text-xs'>
              <div>
                <dt className='text-gray-500'>{t('Quota')}</dt>
                <dd className='mt-0.5 tabular-nums'>
                  {row.reward_type === 'subscription' ? (
                    <>
                      {row.claim_count}/{row.max_claims} {t('units')}
                      <div className='text-gray-500'>
                        {t('Per unit')}: {renderQuota(row.unit_paid_quota || 0)}
                      </div>
                    </>
                  ) : (
                    <>
                      {renderQuota(row.claimed_quota)}/
                      {renderQuota(row.total_quota)}
                      <div className='text-gray-500'>
                        {t('Remaining')}: {renderQuota(row.remaining_quota)}
                      </div>
                    </>
                  )}
                </dd>
              </div>
              <div>
                <dt className='text-gray-500'>{t('Claims')}</dt>
                <dd className='mt-0.5 tabular-nums'>
                  {row.claim_count}/{row.max_claims}
                </dd>
              </div>
              <div className='col-span-2'>
                <dt className='text-gray-500'>{t('Expires')}</dt>
                <dd className='mt-0.5 tabular-nums'>
                  {formatTs(row.expires_at)}
                </dd>
              </div>
            </dl>

            <div className='flex justify-end border-t pt-2'>
              <RedemptionActions
                row={row}
                busy={busy}
                compact
                onCancel={onCancel}
                onView={onView}
                onDisable={onDisable}
                onEnable={onEnable}
                onTerminate={onTerminate}
                onDelete={onDelete}
              />
            </div>
          </article>
        ))}
      </div>
    );
  }

  const columns = [];
  if (showCreator) {
    columns.push({
      title: t('Creator'),
      dataIndex: 'creator_user_id',
      render: (_, row) => (
        <div>
          <div>{row.creator_username || t('Unknown user')}</div>
          <Text type='tertiary' size='small'>
            {t('Creator ID')}: {row.creator_user_id}
          </Text>
        </div>
      ),
    });
  }
  columns.push(
    {
      title: t('Code'),
      dataIndex: 'code',
      render: (text, row) => (
        <Button theme='borderless' onClick={() => onView?.(row.id)}>
          <Text code>{text}</Text>
        </Button>
      ),
    },
    {
      title: t('Kind'),
      dataIndex: 'kind',
      render: (kind) => t(getUserRedemptionKindTranslationKey(kind)),
    },
    {
      title: t('Reward type'),
      dataIndex: 'reward_type',
      render: (rewardType, row) => (
        <div>
          <div>{t(getUserRedemptionRewardTranslationKey(rewardType))}</div>
          {row.plan_title ? (
            <Text type='tertiary' size='small'>
              {row.plan_title}
            </Text>
          ) : null}
        </div>
      ),
    },
    {
      title: t('Status'),
      dataIndex: 'status',
      render: (status, row) => (
        <div className='flex flex-wrap gap-1'>
          <Tag>{t(getUserRedemptionStatusTranslationKey(status))}</Tag>
          {shouldShowDeletedBadge(status, row.deleted) ? (
            <Tag color='red'>{t('Deleted')}</Tag>
          ) : null}
        </div>
      ),
    },
    {
      title: t('Quota'),
      render: (_, row) => (
        <div>
          {row.reward_type === 'subscription' ? (
            <>
              <div>
                {row.claim_count}/{row.max_claims} {t('units')}
              </div>
              <Text type='tertiary' size='small'>
                {t('Per unit')}: {renderQuota(row.unit_paid_quota || 0)}
              </Text>
            </>
          ) : (
            <>
              <div>
                {renderQuota(row.claimed_quota)}/{renderQuota(row.total_quota)}
              </div>
              <Text type='tertiary' size='small'>
                {t('Remaining')}: {renderQuota(row.remaining_quota)}
              </Text>
            </>
          )}
        </div>
      ),
    },
    {
      title: t('Claims'),
      render: (_, row) => `${row.claim_count}/${row.max_claims}`,
    },
    {
      title: t('Expires'),
      dataIndex: 'expires_at',
      render: formatTs,
    },
    {
      title: t('Actions'),
      render: (_, row) => (
        <RedemptionActions
          row={row}
          busy={busy}
          onCancel={onCancel}
          onView={onView}
          onDisable={onDisable}
          onEnable={onEnable}
          onTerminate={onTerminate}
          onDelete={onDelete}
        />
      ),
    },
  );

  const rowSelection = onSelectedIdsChange
    ? {
        selectedRowKeys: selectedIds,
        onChange: (keys) => onSelectedIdsChange(keys.map(Number)),
        getCheckboxProps: (row) => ({
          disabled: !canSettleUserRedemption(row),
        }),
      }
    : undefined;

  return (
    <Table
      rowKey='id'
      columns={columns}
      dataSource={items}
      loading={loading}
      pagination={false}
      rowSelection={rowSelection}
      empty={<div>{t('No codes yet.')}</div>}
    />
  );
}
