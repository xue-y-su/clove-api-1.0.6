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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Input,
  Modal,
  Pagination,
  Table,
  Tabs,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Check, Copy, Eye, Search, Snowflake, Sun, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { copy, renderQuota, showError, showSuccess } from '../../helpers';
import {
  freezeInvitationRewardPayoutRequest,
  getInvitationRewardPayoutRequestDetail,
  listInvitationRewardPayoutRequests,
  reviewInvitationRewardPayoutRequest,
  unfreezeInvitationRewardPayoutRequest,
} from './api';

const PAGE_SIZE = 20;
export default function AdminInvitationRewards() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [view, setView] = useState('pending');
  const [keywordInput, setKeywordInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [reviewDialog, setReviewDialog] = useState(null);
  const [reviewReason, setReviewReason] = useState('');
  const [detailDialog, setDetailDialog] = useState(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [mutationDialog, setMutationDialog] = useState(null);
  const [mutationReason, setMutationReason] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const loadItems = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listInvitationRewardPayoutRequests({
        page,
        page_size: PAGE_SIZE,
        status: view,
        keyword: keyword || undefined,
      });
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (error) {
      setItems([]);
      setTotal(0);
      showError(error.message || t('Request failed'));
    } finally {
      setLoading(false);
    }
  }, [keyword, page, t, view]);

  useEffect(() => {
    loadItems();
  }, [loadItems]);

  const columns = useMemo(
    () =>
      rewardColumns(t, openReviewDialog, openDetailDialog, openMutationDialog),
    [t],
  );

  function search() {
    setPage(1);
    setKeyword(keywordInput.trim());
  }

  function openReviewDialog(item, approve) {
    setReviewDialog({ item, approve });
    setReviewReason('');
  }

  async function openDetailDialog(item) {
    setDetailDialog({ item, detail: null, error: false });
    setDetailLoading(true);
    try {
      const detail = await getInvitationRewardPayoutRequestDetail(item.id);
      setDetailDialog({ item, detail, error: false });
    } catch (error) {
      setDetailDialog({ item, detail: null, error: true });
      showError(error.message || t('Request failed'));
    } finally {
      setDetailLoading(false);
    }
  }

  function openMutationDialog(item, mode) {
    setMutationDialog({ item, mode });
    setMutationReason('');
  }

  async function submitReview() {
    if (!reviewDialog) return;
    const reason = reviewReason.trim();
    if (!reviewDialog.approve && !reason) {
      showError(t('A rejection reason is required'));
      return;
    }
    setSubmitting(true);
    try {
      await reviewInvitationRewardPayoutRequest(reviewDialog.item.id, {
        approve: reviewDialog.approve,
        review_reason: reason,
      });
      showSuccess(t('Payout request reviewed'));
      setReviewDialog(null);
      await loadItems();
    } catch (error) {
      showError(error.message || t('Request failed'));
    } finally {
      setSubmitting(false);
    }
  }

  async function submitMutation() {
    if (!mutationDialog) return;
    const reason = mutationReason.trim();
    if (!reason) {
      showError(t('A reason is required'));
      return;
    }
    setSubmitting(true);
    try {
      if (mutationDialog.mode === 'freeze') {
        await freezeInvitationRewardPayoutRequest(
          mutationDialog.item.id,
          reason,
        );
        showSuccess(t('Payout request frozen'));
      } else {
        await unfreezeInvitationRewardPayoutRequest(
          mutationDialog.item.id,
          reason,
        );
        showSuccess(t('Payout request unfrozen'));
      }
      setMutationDialog(null);
      await loadItems();
    } catch (error) {
      showError(error.message || t('Request failed'));
    } finally {
      setSubmitting(false);
    }
  }

  const actions = {
    review: openReviewDialog,
    detail: openDetailDialog,
    mutate: openMutationDialog,
  };

  return (
    <Card className='!rounded-lg border border-[var(--semi-color-border)] shadow-sm'>
      <div className='mb-4'>
        <Typography.Title heading={4}>
          {t('Invitation reward approvals')}
        </Typography.Title>
        <Typography.Text type='tertiary'>
          {t(
            'Review aggregate payout requests before invitation rewards are credited.',
          )}
        </Typography.Text>
      </div>

      <div className='mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <Tabs
          type='button'
          activeKey={view}
          onChange={(value) => {
            setView(value);
            setPage(1);
          }}
          tabList={[
            { itemKey: 'pending', tab: t('Pending review') },
            { itemKey: 'history', tab: t('Approval history') },
          ]}
        />
        <Input
          value={keywordInput}
          onChange={setKeywordInput}
          onEnterPress={search}
          placeholder={t('Search users or codes')}
          suffix={
            <Button
              theme='borderless'
              icon={<Search size={15} />}
              aria-label={t('Search')}
              onClick={search}
            />
          }
          className='w-full sm:w-72'
        />
      </div>

      <div className='space-y-3 lg:hidden'>
        {items.map((item) => (
          <RewardCard key={item.id} item={item} actions={actions} t={t} />
        ))}
        {!loading && items.length === 0 ? (
          <div className='py-12 text-center text-sm text-[var(--semi-color-text-2)]'>
            {t('No payout requests found')}
          </div>
        ) : null}
      </div>

      <div className='hidden lg:block'>
        <Table
          columns={columns}
          dataSource={items}
          rowKey='id'
          loading={loading}
          pagination={false}
          empty={t('No payout requests found')}
          size='small'
        />
      </div>

      <div className='mt-4 flex max-w-full justify-end overflow-hidden'>
        <div className='hidden sm:block'>
          <Pagination
            currentPage={page}
            pageSize={PAGE_SIZE}
            total={total}
            onPageChange={setPage}
            showTotal
          />
        </div>
        <div className='max-w-full sm:hidden'>
          <Pagination
            currentPage={page}
            pageSize={PAGE_SIZE}
            total={total}
            onPageChange={setPage}
            showTotal={false}
          />
        </div>
      </div>

      <Modal
        title={
          reviewDialog?.approve
            ? t('Approve payout request')
            : t('Reject payout request')
        }
        visible={Boolean(reviewDialog)}
        onCancel={() => setReviewDialog(null)}
        onOk={submitReview}
        confirmLoading={submitting}
        maskClosable={false}
        centered
      >
        <TextArea
          value={reviewReason}
          onChange={setReviewReason}
          maxCount={500}
          autosize={{ minRows: 3, maxRows: 6 }}
          placeholder={
            reviewDialog?.approve
              ? t('Optional review note')
              : t('Enter a rejection reason')
          }
        />
      </Modal>

      <MutationModal
        dialog={mutationDialog}
        reason={mutationReason}
        submitting={submitting}
        onReasonChange={setMutationReason}
        onCancel={() => setMutationDialog(null)}
        onSubmit={submitMutation}
        t={t}
      />

      <Modal
        title={t('Payout request details')}
        visible={Boolean(detailDialog)}
        onCancel={() => setDetailDialog(null)}
        footer={null}
        width={760}
        centered
      >
        {detailLoading ? (
          <div className='py-10 text-center'>{t('Loading')}</div>
        ) : null}
        {detailDialog?.error ? (
          <div className='py-10 text-center text-red-500'>
            {t('Unable to load payout request details')}
          </div>
        ) : null}
        {detailDialog?.detail ? (
          <RewardDetail detail={detailDialog.detail} t={t} />
        ) : null}
      </Modal>
    </Card>
  );
}

function rewardColumns(t, onReview, onDetail, onMutation) {
  const actions = { review: onReview, detail: onDetail, mutate: onMutation };
  return [
    {
      title: t('Applicant'),
      render: (_, item) => (
        <div>
          <div>{item.display_name || item.username}</div>
          <Typography.Text type='tertiary' size='small'>
            {t('@{{username}} · #{{userId}}', {
              username: item.username,
              userId: item.user_id,
            })}
          </Typography.Text>
        </div>
      ),
    },
    {
      title: t('Amount'),
      dataIndex: 'amount',
      render: (value) => renderQuota(value),
    },
    { title: t('Reward count'), dataIndex: 'item_count' },
    {
      title: t('Operation ID'),
      dataIndex: 'operation_id',
      render: (value) => (
        <div className='max-w-48 break-all font-mono text-xs' title={value}>
          {value}
        </div>
      ),
    },
    {
      title: t('Status'),
      dataIndex: 'status',
      render: (value) => <RequestStatus value={value} t={t} />,
    },
    { title: t('Submitted'), dataIndex: 'created_at', render: formatTimestamp },
    { title: t('Reviewed'), dataIndex: 'reviewed_at', render: formatTimestamp },
    {
      title: t('Approval reason'),
      render: (_, item) => (
        <div className='max-w-48 break-words'>
          <div>{item.reason || '-'}</div>
          {item.reviewer_username ? (
            <Typography.Text type='tertiary' size='small'>
              {t('Reviewer')}: {item.reviewer_username}
            </Typography.Text>
          ) : null}
        </div>
      ),
    },
    {
      title: t('Actions'),
      render: (_, item) => renderActions(item, actions, t, true),
    },
  ];
}

function RewardCard({ item, actions, t }) {
  return (
    <article className='space-y-3 rounded-md border border-[var(--semi-color-border)] p-3'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='break-words font-medium'>
            {item.display_name || item.username}
          </div>
          <div className='break-all text-xs text-[var(--semi-color-text-2)]'>
            {t('@{{username}} · #{{userId}}', {
              username: item.username,
              userId: item.user_id,
            })}
          </div>
        </div>
        <RequestStatus value={item.status} t={t} />
      </div>
      <div className='grid grid-cols-2 gap-3 text-sm'>
        <DetailTerm label={t('Amount')} value={renderQuota(item.amount)} />
        <DetailTerm label={t('Reward count')} value={String(item.item_count)} />
        <DetailTerm
          label={t('Submitted')}
          value={formatTimestamp(item.created_at)}
        />
        <DetailTerm
          label={t('Reviewed')}
          value={formatTimestamp(item.reviewed_at)}
        />
      </div>
      <div className='break-all font-mono text-xs text-[var(--semi-color-text-2)]'>
        {item.operation_id}
      </div>
      {item.reason ? (
        <p className='break-words text-sm'>{item.reason}</p>
      ) : null}
      {renderActions(item, actions, t, false)}
    </article>
  );
}

function renderActions(item, actions, t, compact) {
  const buttonProps = compact ? { size: 'small' } : {};
  return (
    <div className='flex flex-wrap justify-end gap-1'>
      <Button
        theme='borderless'
        icon={<Eye size={15} />}
        onClick={() => actions.detail(item)}
        {...buttonProps}
      >
        {t('Details')}
      </Button>
      {item.status === 'pending' ? (
        <>
          <Button
            theme='borderless'
            icon={<Check size={15} />}
            onClick={() => actions.review(item, true)}
            {...buttonProps}
          >
            {t('Approve')}
          </Button>
          <Button
            theme='borderless'
            type='danger'
            icon={<X size={15} />}
            onClick={() => actions.review(item, false)}
            {...buttonProps}
          >
            {t('Reject')}
          </Button>
        </>
      ) : null}
      {item.status === 'paid' ? (
        <Button
          theme='borderless'
          type='danger'
          icon={<Snowflake size={15} />}
          onClick={() => actions.mutate(item, 'freeze')}
          {...buttonProps}
        >
          {t('Freeze')}
        </Button>
      ) : null}
      {item.status === 'frozen' ? (
        <Button
          theme='borderless'
          icon={<Sun size={15} />}
          onClick={() => actions.mutate(item, 'unfreeze')}
          {...buttonProps}
        >
          {t('Unfreeze')}
        </Button>
      ) : null}
    </div>
  );
}

function MutationModal({
  dialog,
  reason,
  submitting,
  onReasonChange,
  onCancel,
  onSubmit,
  t,
}) {
  const freeze = dialog?.mode === 'freeze';
  return (
    <Modal
      title={freeze ? t('Freeze payout request') : t('Unfreeze payout request')}
      visible={Boolean(dialog)}
      onCancel={onCancel}
      onOk={onSubmit}
      confirmLoading={submitting}
      okButtonProps={{ type: freeze ? 'danger' : 'primary' }}
      maskClosable={false}
      centered
    >
      <Typography.Paragraph type='tertiary'>
        {freeze
          ? t(
              'The full approved amount will be deducted. The user balance may become negative.',
            )
          : t('The exact frozen amount will be restored to the user balance.')}
      </Typography.Paragraph>
      <TextArea
        value={reason}
        onChange={onReasonChange}
        maxCount={500}
        autosize={{ minRows: 3, maxRows: 6 }}
        placeholder={t('Enter a reason for this action')}
      />
    </Modal>
  );
}

function RewardDetail({ detail, t }) {
  const request = detail.request;
  return (
    <div className='space-y-5'>
      <div className='grid gap-3 sm:grid-cols-2'>
        <DetailTerm
          label={t('Applicant')}
          value={`${request.display_name || request.username} (@${request.username}, #${request.user_id})`}
        />
        <DetailTerm
          label={t('Status')}
          value={t(requestStatusLabel(request.status))}
        />
        <DetailTerm
          label={t('Original request amount')}
          value={renderQuota(request.amount)}
        />
        <DetailTerm
          label={t('Request reversed amount')}
          value={renderQuota(request.reversed_amount)}
        />
        <DetailTerm
          label={t('Current net payout')}
          value={renderQuota(request.net_amount)}
        />
        <DetailTerm
          label={t('Reviewer')}
          value={request.reviewer_username || '-'}
        />
        <DetailTerm
          label={t('Reviewed')}
          value={formatTimestamp(request.reviewed_at)}
        />
        <DetailTerm
          label={t('Approval reason')}
          value={request.reason || '-'}
        />
        <DetailTerm
          label={t('Payout operation ID')}
          value={request.operation_id}
          mono
        />
      </div>
      <DetailSection
        title={t('Reward sources')}
        empty={detail.rewards.length === 0}
        t={t}
      >
        {detail.rewards.map((reward) => (
          <div
            key={reward.reward_id}
            className='grid gap-2 border-b border-[var(--semi-color-border)] py-3 last:border-0 sm:grid-cols-2'
          >
            <DetailTerm
              label={t('Source account')}
              value={`${reward.source_display_name || reward.source_username} (@${reward.source_username}, #${reward.source_user_id})`}
            />
            <DetailTerm
              label={t('Trigger type')}
              value={t(rewardTriggerTypeLabel(reward.trigger_type))}
            />
            <DetailTerm
              label={t('Trigger ID')}
              value={
                <div className='flex min-w-0 items-start gap-1'>
                  <span className='min-w-0 flex-1 break-all font-mono text-xs'>
                    {reward.trigger_id || '-'}
                  </span>
                  {reward.trigger_id ? (
                    <Button
                      theme='borderless'
                      size='small'
                      icon={<Copy size={14} />}
                      aria-label={t('Copy trigger ID')}
                      onClick={() => copy(reward.trigger_id)}
                    />
                  ) : null}
                </div>
              }
            />
            <DetailTerm
              label={t('Eligible quota')}
              value={renderQuota(reward.eligible_quota)}
            />
            <DetailTerm
              label={t('Commission rate')}
              value={formatBasisPoints(reward.rate_basis_points)}
            />
            <DetailTerm
              label={t('Gross reward')}
              value={renderQuota(reward.amount)}
            />
            <DetailTerm
              label={t('Reversed amount')}
              value={renderQuota(reward.reversed_amount)}
            />
            <DetailTerm
              label={t('Net reward')}
              value={renderQuota(reward.net_amount)}
            />
            <DetailTerm
              label={t('Reversal status')}
              value={t(rewardReversalStatusLabel(reward.reversal_state))}
            />
            <DetailTerm
              label={t('Registration IP')}
              value={reward.registration_ip || '-'}
              mono
            />
            <DetailTerm
              label={t('Registered')}
              value={formatTimestamp(reward.registered_at)}
            />
            <DetailTerm
              label={t('Invitation code')}
              value={reward.invite_code || '-'}
              mono
            />
            <DetailTerm
              label={t('Status')}
              value={t(requestStatusLabel(reward.status))}
            />
          </div>
        ))}
      </DetailSection>
      <DetailSection
        title={t('Freeze history')}
        empty={detail.actions.length === 0}
        t={t}
      >
        {detail.actions.map((action) => (
          <div
            key={action.id}
            className='grid gap-2 border-b border-[var(--semi-color-border)] py-3 last:border-0 sm:grid-cols-2'
          >
            <DetailTerm
              label={t('Action')}
              value={action.action === 'freeze' ? t('Frozen') : t('Unfrozen')}
            />
            <DetailTerm label={t('Revision')} value={String(action.revision)} />
            <DetailTerm
              label={t('Operator')}
              value={`${action.actor_display_name || action.actor_username} (@${action.actor_username})`}
            />
            <DetailTerm
              label={t('Time')}
              value={formatTimestamp(action.created_at)}
            />
            <DetailTerm label={t('Reason')} value={action.reason} />
            <DetailTerm
              label={t('Wallet operation ID')}
              value={action.wallet_operation_id}
              mono
            />
          </div>
        ))}
      </DetailSection>
    </div>
  );
}

function DetailSection({ title, empty, children, t }) {
  return (
    <section>
      <Typography.Title heading={6}>{title}</Typography.Title>
      <div className='mt-2 rounded-md border border-[var(--semi-color-border)] px-3'>
        {empty ? (
          <div className='py-6 text-center text-sm text-[var(--semi-color-text-2)]'>
            {t('No records')}
          </div>
        ) : (
          children
        )}
      </div>
    </section>
  );
}

function DetailTerm({ label, value, mono = false }) {
  return (
    <div className='min-w-0'>
      <div className='text-xs text-[var(--semi-color-text-2)]'>{label}</div>
      <div
        className={`mt-1 break-words text-sm ${mono ? 'break-all font-mono text-xs' : ''}`}
      >
        {value}
      </div>
    </div>
  );
}

function RequestStatus({ value, t }) {
  const color =
    value === 'paid'
      ? 'green'
      : value === 'rejected'
        ? 'red'
        : value === 'frozen'
          ? 'blue'
          : 'grey';
  return <Tag color={color}>{t(requestStatusLabel(value))}</Tag>;
}

function requestStatusLabel(value) {
  if (value === 'paid') return 'Paid';
  if (value === 'rejected') return 'Rejected';
  if (value === 'frozen') return 'Frozen';
  if (value === 'cancelled') return 'Cancelled';
  return 'Pending review';
}

function rewardTriggerTypeLabel(value) {
  if (value === 'registration_inviter') return 'Inviter registration reward';
  if (value === 'cash_topup') return 'Cash top-up';
  if (value === 'cash_subscription') return 'Cash subscription purchase';
  if (value === 'admin_balance_redemption') {
    return 'Administrator balance code';
  }
  if (value === 'admin_subscription_redemption') {
    return 'Administrator subscription code';
  }
  return value || '-';
}

function rewardReversalStatusLabel(value) {
  if (value === 'none') return 'Not reversed';
  if (value === 'partial') return 'Partially reversed';
  if (value === 'full') return 'Fully reversed';
  return value || '-';
}

function formatBasisPoints(value) {
  const basisPoints = Number(value);
  if (!Number.isFinite(basisPoints)) return '-';
  return `${(basisPoints / 100).toLocaleString(undefined, {
    maximumFractionDigits: 2,
  })}%`;
}

function formatTimestamp(timestamp) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString() : '-';
}
