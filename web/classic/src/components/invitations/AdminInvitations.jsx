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
  Select,
  Table,
  Tabs,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Check, Copy, Plus, Search, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { copy, showError, showSuccess } from '../../helpers';
import {
  issueInvitationCode,
  listInvitationApplications,
  listInvitationCodes,
  listInvitationUses,
  reviewInvitationApplication,
  revokeInvitationCode,
} from './api';
import { getInvitationStatusTranslationKey } from './invitation-state';

const PAGE_SIZE = 20;

export default function AdminInvitations() {
  const { t } = useTranslation();
  const [tab, setTab] = useState('applications');
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');
  const [keywordInput, setKeywordInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [dialog, setDialog] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [form, setForm] = useState(defaultIssueForm());

  const loadItems = useCallback(async () => {
    setLoading(true);
    const params = {
      page,
      page_size: PAGE_SIZE,
      status: status || undefined,
      keyword: keyword || undefined,
    };
    try {
      let result;
      if (tab === 'applications') {
        result = await listInvitationApplications(params);
      } else if (tab === 'codes') {
        result = await listInvitationCodes(params);
      } else {
        result = await listInvitationUses(params);
      }
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (error) {
      setItems([]);
      setTotal(0);
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [keyword, page, status, t, tab]);

  useEffect(() => {
    loadItems();
  }, [loadItems]);

  const columns = useMemo(() => {
    if (tab === 'applications') {
      return applicationColumns(t, openReviewDialog);
    }
    if (tab === 'codes') {
      return codeColumns(t, openRevokeDialog);
    }
    return useColumns(t);
  }, [tab, t]);

  function changeTab(value) {
    setTab(value);
    setPage(1);
    setStatus('');
    setItems([]);
  }

  function submitSearch() {
    setPage(1);
    setKeyword(keywordInput.trim());
  }

  function openReviewDialog(item, approve) {
    setForm(defaultIssueForm());
    setDialog({ type: 'review', item, approve });
  }

  function openIssueDialog() {
    setForm({ ...defaultIssueForm(), targetUserId: '' });
    setDialog({ type: 'issue' });
  }

  function openRevokeDialog(item) {
    setForm({ ...defaultIssueForm(), reason: '' });
    setDialog({ type: 'revoke', item });
  }

  async function submitDialog() {
    if (!dialog) return;
    setSubmitting(true);
    try {
      if (dialog.type === 'review') {
        if (!dialog.approve && !form.reason.trim()) {
          showError(t('驳回时必须填写原因'));
          return;
        }
        await reviewInvitationApplication(
          dialog.item.id,
          issuePayload(form, {
            approve: dialog.approve,
            review_reason: form.reason.trim(),
          }),
        );
        showSuccess(t('申请已处理'));
      } else if (dialog.type === 'issue') {
        if (!Number(form.targetUserId) || !form.reason.trim()) {
          showError(t('请填写目标用户和发放原因'));
          return;
        }
        await issueInvitationCode(
          issuePayload(form, {
            target_user_id: Number(form.targetUserId),
            reason: form.reason.trim(),
          }),
        );
        showSuccess(t('邀请码已发放'));
      } else {
        if (!form.reason.trim()) {
          showError(t('请填写撤销原因'));
          return;
        }
        await revokeInvitationCode(dialog.item.id, form.reason.trim());
        showSuccess(t('邀请码已撤销'));
      }
      setDialog(null);
      await loadItems();
    } catch (error) {
      showError(error.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Card className='!rounded-lg border border-[var(--semi-color-border)] shadow-sm'>
      <div className='mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
        <div>
          <Typography.Title heading={4}>{t('邀请码管理')}</Typography.Title>
          <Typography.Text type='tertiary'>
            {t('审批申请、直接发放、撤销邀请码并查看注册记录。')}
          </Typography.Text>
        </div>
        <Button
          theme='solid'
          type='primary'
          icon={<Plus size={16} />}
          onClick={openIssueDialog}
        >
          {t('直接发放')}
        </Button>
      </div>

      <Tabs
        type='button'
        activeKey={tab}
        onChange={changeTab}
        tabList={[
          { tab: t('申请'), itemKey: 'applications' },
          { tab: t('邀请码'), itemKey: 'codes' },
          { tab: t('注册记录'), itemKey: 'uses' },
        ]}
      />

      <div className='my-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end'>
        <Select
          value={status}
          onChange={(value) => {
            setStatus(value || '');
            setPage(1);
          }}
          optionList={[
            { label: t('全部状态'), value: '' },
            ...statusOptions(tab).map((value) => ({
              label: t(getInvitationStatusTranslationKey(value)),
              value,
            })),
          ]}
          className='w-full sm:w-40'
        />
        <Input
          value={keywordInput}
          onChange={setKeywordInput}
          onEnterPress={submitSearch}
          placeholder={t('搜索用户或邀请码')}
          suffix={
            <Button
              theme='borderless'
              icon={<Search size={15} />}
              aria-label={t('搜索')}
              onClick={submitSearch}
            />
          }
          className='w-full sm:w-72'
        />
      </div>

      <div className='overflow-x-auto'>
        <Table
          columns={columns}
          dataSource={items}
          rowKey='id'
          loading={loading}
          pagination={false}
          empty={t('暂无邀请码记录')}
          size='small'
        />
      </div>
      <div className='mt-4 flex justify-end'>
        <Pagination
          currentPage={page}
          pageSize={PAGE_SIZE}
          total={total}
          onPageChange={setPage}
          showTotal
        />
      </div>

      <InvitationActionModal
        dialog={dialog}
        form={form}
        setForm={setForm}
        submitting={submitting}
        onCancel={() => setDialog(null)}
        onSubmit={submitDialog}
        t={t}
      />
    </Card>
  );
}

function InvitationActionModal({
  dialog,
  form,
  setForm,
  submitting,
  onCancel,
  onSubmit,
  t,
}) {
  if (!dialog) return null;
  const approving = dialog.type === 'review' && dialog.approve;
  const needsIssueFields = dialog.type === 'issue' || approving;
  let title = t('撤销邀请码');
  if (dialog.type === 'issue') title = t('直接发放邀请码');
  if (dialog.type === 'review') {
    title = dialog.approve ? t('批准申请') : t('驳回申请');
  }
  return (
    <Modal
      title={title}
      visible
      onCancel={onCancel}
      onOk={onSubmit}
      confirmLoading={submitting}
      maskClosable={false}
      centered
    >
      <div className='space-y-4'>
        {dialog.type === 'issue' ? (
          <LabeledField label={t('目标用户 ID')}>
            <Input
              type='number'
              min={1}
              value={form.targetUserId}
              onChange={(value) => setForm({ ...form, targetUserId: value })}
            />
          </LabeledField>
        ) : null}
        {needsIssueFields ? (
          <>
            <div className='grid grid-cols-2 gap-3'>
              <LabeledField label={t('类型')}>
                <Select
                  value={form.kind}
                  onChange={(value) =>
                    setForm({
                      ...form,
                      kind: value,
                      maxUses: value === 'single' ? 1 : 10,
                    })
                  }
                  optionList={[
                    { label: t('一次性'), value: 'single' },
                    { label: t('多次使用'), value: 'multi' },
                  ]}
                  className='w-full'
                />
              </LabeledField>
              <LabeledField label={t('最大使用次数')}>
                <Input
                  type='number'
                  min={form.kind === 'single' ? 1 : 2}
                  max={100}
                  disabled={form.kind === 'single'}
                  value={form.maxUses}
                  onChange={(value) => setForm({ ...form, maxUses: value })}
                />
              </LabeledField>
            </div>
            <LabeledField label={t('过期时间')}>
              <Input
                type='datetime-local'
                value={form.expiresAt}
                onChange={(value) => setForm({ ...form, expiresAt: value })}
              />
            </LabeledField>
          </>
        ) : null}
        <LabeledField
          label={
            dialog.type === 'review' && !dialog.approve
              ? t('驳回原因')
              : t('说明')
          }
        >
          <TextArea
            value={form.reason}
            maxCount={500}
            autosize={{ minRows: 3, maxRows: 6 }}
            onChange={(value) => setForm({ ...form, reason: value })}
          />
        </LabeledField>
      </div>
    </Modal>
  );
}

function LabeledField({ label, children }) {
  return (
    <label className='block space-y-1.5'>
      <Typography.Text strong>{label}</Typography.Text>
      {children}
    </label>
  );
}

function applicationColumns(t, openReview) {
  return [
    { title: t('申请人'), dataIndex: 'applicant_username' },
    {
      title: t('理由'),
      dataIndex: 'reason',
      render: (value) => <span className='line-clamp-2 max-w-72'>{value}</span>,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => <StatusTag value={value} t={t} />,
    },
    { title: t('提交时间'), dataIndex: 'created_at', render: formatTimestamp },
    {
      title: t('操作'),
      render: (_, item) => (
        <div className='flex gap-1'>
          <Button
            theme='borderless'
            icon={<Check size={15} />}
            disabled={item.status !== 'pending'}
            onClick={() => openReview(item, true)}
          >
            {t('批准')}
          </Button>
          <Button
            theme='borderless'
            type='danger'
            icon={<X size={15} />}
            disabled={item.status !== 'pending'}
            onClick={() => openReview(item, false)}
          >
            {t('驳回')}
          </Button>
        </div>
      ),
    },
  ];
}

function codeColumns(t, openRevoke) {
  return [
    { title: t('所有者'), dataIndex: 'owner_username' },
    {
      title: t('邀请码'),
      dataIndex: 'code',
      render: (value) => (
        <div className='flex items-center gap-1 font-mono'>
          <span className='max-w-48 truncate'>{value}</span>
          <Button
            theme='borderless'
            icon={<Copy size={14} />}
            aria-label={t('复制邀请码')}
            onClick={async () => {
              if (await copy(value)) showSuccess(t('邀请码已复制'));
            }}
          />
        </div>
      ),
    },
    {
      title: t('使用次数'),
      render: (_, item) => `${item.used_count}/${item.max_uses}`,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => <StatusTag value={value} t={t} />,
    },
    { title: t('过期时间'), dataIndex: 'expires_at', render: formatTimestamp },
    {
      title: t('操作'),
      render: (_, item) => (
        <Button
          type='danger'
          disabled={item.status !== 'active'}
          onClick={() => openRevoke(item)}
        >
          {t('撤销')}
        </Button>
      ),
    },
  ];
}

function useColumns(t) {
  return [
    { title: t('邀请人'), dataIndex: 'inviter_username' },
    { title: t('注册用户'), dataIndex: 'registered_username' },
    { title: t('邀请码'), dataIndex: 'invite_code' },
    { title: t('来源'), dataIndex: 'source' },
    { title: t('IP 地址'), dataIndex: 'registration_ip' },
    {
      title: t('注册时间'),
      dataIndex: 'registered_at',
      render: formatTimestamp,
    },
    {
      title: t('奖励状态'),
      render: (_, item) =>
        `${item.inviter_reward_status} / ${item.invitee_reward_status}`,
    },
  ];
}

function StatusTag({ value, t }) {
  const color =
    value === 'active' || value === 'approved'
      ? 'green'
      : value === 'rejected' || value === 'revoked'
        ? 'red'
        : 'grey';
  return <Tag color={color}>{t(getInvitationStatusTranslationKey(value))}</Tag>;
}

function statusOptions(tab) {
  if (tab === 'applications')
    return ['pending', 'approved', 'rejected', 'withdrawn'];
  if (tab === 'codes') return ['active', 'exhausted', 'expired', 'revoked'];
  return ['pending', 'completed', 'skipped', 'failed'];
}

function defaultIssueForm() {
  return {
    targetUserId: '',
    reason: '',
    kind: 'single',
    maxUses: 1,
    expiresAt: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 16),
  };
}

function issuePayload(form, extra) {
  return {
    kind: form.kind,
    max_uses: form.kind === 'single' ? 1 : Number(form.maxUses),
    expires_at: Math.floor(new Date(form.expiresAt).getTime() / 1000),
    ...extra,
  };
}

function formatTimestamp(timestamp) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString() : '-';
}
