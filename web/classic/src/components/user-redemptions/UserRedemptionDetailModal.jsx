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
import React, { useEffect, useState } from 'react';
import {
  Banner,
  Button,
  Input,
  Modal,
  Spin,
  Table,
  Typography,
} from '@douyinfe/semi-ui';
import { Copy } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { copy, renderQuota, showError, showSuccess } from '../../helpers';
import { getUserRedemption } from './api.js';
import {
  getUserRedemptionRewardTranslationKey,
  getUserRedemptionStatusTranslationKey,
} from './user-redemption-state.js';

function formatTimestamp(timestamp) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString() : '-';
}

export default function UserRedemptionDetailModal({
  id,
  admin = false,
  onClose,
}) {
  const { t } = useTranslation();
  const [detail, setDetail] = useState(null);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!id) {
      setDetail(null);
      return;
    }
    let active = true;
    setLoading(true);
    setFailed(false);
    getUserRedemption(id, admin)
      .then((data) => {
        if (active) setDetail(data);
      })
      .catch(() => {
        if (active) setFailed(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [admin, id]);

  async function copyCode() {
    if (!detail?.code?.code) return;
    if (await copy(detail.code.code)) {
      showSuccess(t('Copied to clipboard'));
    } else {
      showError(t('Failed to copy to clipboard'));
    }
  }

  const claimColumns = [
    {
      title: t('Recipient'),
      render: (_, claim) => (
        <div>
          <div>
            {claim.claimant_username ||
              claim.claimant_display ||
              t('Unknown user')}
          </div>
          {claim.claimant_user_id ? (
            <Typography.Text type='tertiary' size='small'>
              #{claim.claimant_user_id}
            </Typography.Text>
          ) : null}
        </div>
      ),
    },
    {
      title: t('Quota'),
      render: (_, claim) =>
        claim.reward_type === 'subscription'
          ? `${t('Subscription')} #${claim.user_subscription_id || '-'}`
          : renderQuota(claim.quota),
    },
    {
      title: t('Claimed at'),
      dataIndex: 'claimed_at',
      render: formatTimestamp,
    },
  ];

  return (
    <Modal
      title={t('Redemption Code Details')}
      visible={Boolean(id)}
      onCancel={onClose}
      footer={null}
      width={760}
    >
      {loading ? (
        <div className='flex min-h-40 items-center justify-center'>
          <Spin />
        </div>
      ) : null}
      {failed ? (
        <Banner
          type='danger'
          description={t('Unable to load redemption code')}
          closeIcon={null}
        />
      ) : null}
      {detail ? (
        <div className='space-y-5'>
          <Input
            value={detail.code.code}
            readonly
            className='font-mono'
            suffix={
              <Button
                theme='borderless'
                icon={<Copy size={16} />}
                aria-label={t('Copy redemption code')}
                onClick={() => void copyCode()}
              />
            }
          />
          <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
            <Metric
              label={t('Reward type')}
              value={t(
                getUserRedemptionRewardTranslationKey(detail.code.reward_type),
              )}
            />
            <Metric
              label={t('Status')}
              value={t(
                getUserRedemptionStatusTranslationKey(detail.code.status),
              )}
            />
            <Metric
              label={t('Total')}
              value={
                detail.code.reward_type === 'subscription'
                  ? `${detail.code.max_claims} ${t('units')}`
                  : renderQuota(detail.code.total_quota)
              }
            />
            <Metric
              label={t('Claimed')}
              value={
                detail.code.reward_type === 'subscription'
                  ? `${detail.code.claim_count} ${t('units')}`
                  : renderQuota(detail.code.claimed_quota)
              }
            />
            <Metric
              label={t('Remaining')}
              value={
                detail.code.reward_type === 'subscription'
                  ? `${Math.max(
                      0,
                      detail.code.max_claims - detail.code.claim_count,
                    )} ${t('units')}`
                  : renderQuota(detail.code.remaining_quota)
              }
            />
            <Metric
              label={t('Refunded')}
              value={renderQuota(detail.code.refunded_quota)}
            />
            <Metric
              label={t('Forfeited')}
              value={renderQuota(detail.code.forfeited_quota)}
            />
            <Metric
              label={t('Claims')}
              value={`${detail.code.claim_count}/${detail.code.max_claims}`}
            />
            <Metric
              label={t('Expires')}
              value={formatTimestamp(detail.code.expires_at)}
            />
          </div>
          {detail.code.reward_type === 'subscription' ? (
            <div className='grid gap-2 sm:grid-cols-2'>
              <AuditLine
                label={t('Plan')}
                value={detail.code.plan_title || '-'}
              />
              <AuditLine
                label={t('Per unit')}
                value={renderQuota(detail.code.unit_paid_quota || 0)}
              />
            </div>
          ) : null}
          {admin ? (
            <div className='grid gap-2 sm:grid-cols-2'>
              <AuditLine
                label={t('Creator')}
                value={`${detail.code.creator_username || '-'} (#${detail.code.creator_user_id})`}
              />
              <AuditLine
                label={t('Created')}
                value={formatTimestamp(detail.code.created_at)}
              />
              <AuditLine
                label={t('Disabled reason')}
                value={detail.code.disabled_reason || '-'}
              />
              <AuditLine
                label={t('Terminal reason')}
                value={detail.code.terminal_reason || '-'}
              />
              <AuditLine
                label={t('Delete reason')}
                value={detail.code.delete_reason || '-'}
              />
              <AuditLine
                label={t('Deleted')}
                value={
                  detail.code.deleted
                    ? formatTimestamp(detail.code.deleted_at)
                    : '-'
                }
              />
            </div>
          ) : null}
          <div>
            <Typography.Title heading={6} className='mb-2'>
              {t('Claim history')}
            </Typography.Title>
            <Table
              rowKey='id'
              columns={claimColumns}
              dataSource={detail.claims || []}
              pagination={false}
              size='small'
              empty={t('No claims yet.')}
            />
          </div>
        </div>
      ) : null}
    </Modal>
  );
}

function Metric({ label, value }) {
  return (
    <div className='rounded-md border border-[var(--semi-color-border)] p-2'>
      <Typography.Text type='tertiary' size='small'>
        {label}
      </Typography.Text>
      <div className='mt-1 truncate font-semibold tabular-nums'>{value}</div>
    </div>
  );
}

function AuditLine({ label, value }) {
  return (
    <div className='flex justify-between gap-3 border-b border-[var(--semi-color-border)] py-2 text-sm'>
      <Typography.Text type='tertiary'>{label}</Typography.Text>
      <span className='break-words text-right'>{value}</span>
    </div>
  );
}
