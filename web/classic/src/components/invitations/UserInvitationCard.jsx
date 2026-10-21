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
import React, {
  useCallback,
  useEffect,
  useMemo,
  useContext,
  useRef,
  useState,
} from 'react';
import {
  Banner,
  Button,
  Card,
  Input,
  Spin,
  Table,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { Copy, Gift, QrCode, Send, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  copy,
  getLogo,
  getSystemName,
  renderQuota,
  showError,
  showSuccess,
} from '../../helpers';
import { StatusContext } from '../../context/Status';
import { UserContext } from '../../context/User';
import {
  applyForInvitation,
  getInvitationRewardSummary,
  getInvitationSelf,
  revokeOwnInvitationCode,
  submitInvitationRewardPayout,
  withdrawInvitationApplication,
} from './api';
import {
  buildInvitationRegistrationUrl,
  getInvitationErrorMessage,
  getInvitationReapplyRefreshDelay,
  getInvitationRemainingUses,
  getInvitationStatusTranslationKey,
  getInvitationViewState,
} from './invitation-state';
import InvitationQRModal from './InvitationQRModal';
import InvitationApplicationModal from './InvitationApplicationModal';
import InvitationRevokeModal from './InvitationRevokeModal';

export default function UserInvitationCard({ complianceConfirmed = true }) {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const [invitation, setInvitation] = useState(null);
  const [rewardSummary, setRewardSummary] = useState(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [qrVisible, setQrVisible] = useState(false);
  const [applicationModalVisible, setApplicationModalVisible] = useState(false);
  const [revokeModalVisible, setRevokeModalVisible] = useState(false);
  const refreshedReapplyAt = useRef(0);

  const loadInvitation = useCallback(async () => {
    setLoading(true);
    setFailed(false);
    try {
      setInvitation(await getInvitationSelf());
    } catch (error) {
      setFailed(true);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadRewardSummary = useCallback(async () => {
    try {
      setRewardSummary(await getInvitationRewardSummary());
    } catch (error) {
      setRewardSummary(null);
    }
  }, []);

  useEffect(() => {
    loadInvitation();
    loadRewardSummary();
  }, [loadInvitation, loadRewardSummary]);

  const reapplyAt = invitation?.reapply_at || 0;
  const canApply = invitation?.can_apply || false;
  useEffect(() => {
    const delay = getInvitationReapplyRefreshDelay(reapplyAt, canApply);
    if (delay === null) {
      refreshedReapplyAt.current = 0;
      return;
    }
    if (refreshedReapplyAt.current === reapplyAt) return;

    const timer = window.setTimeout(() => {
      if (Date.now() >= reapplyAt * 1000) {
        refreshedReapplyAt.current = reapplyAt;
      }
      void loadInvitation();
    }, delay);
    return () => window.clearTimeout(timer);
  }, [canApply, loadInvitation, reapplyAt]);

  const view = getInvitationViewState(invitation, loading, failed);
  const registrationUrl = useMemo(() => {
    if (!invitation?.code?.code) return '';
    return buildInvitationRegistrationUrl(
      window.location.origin,
      invitation.code.code,
    );
  }, [invitation?.code?.code]);
  const status = statusState?.status || {};
  const systemName = status.system_name || getSystemName();
  const logo = status.logo || getLogo();
  const displayName =
    userState?.user?.display_name?.trim() || userState?.user?.username || '';

  async function submitApplication(reason) {
    const normalized = reason.trim();
    if (normalized.length < 5) return;
    setSubmitting(true);
    try {
      await applyForInvitation(normalized);
      setApplicationModalVisible(false);
      showSuccess(t('邀请码申请已提交'));
      await loadInvitation();
    } catch (error) {
      showError(getInvitationErrorMessage(error, t('请求失败')));
    } finally {
      setSubmitting(false);
    }
  }

  async function revokeInvitationCode(reason) {
    const codeId = invitation?.code?.id;
    if (!codeId) return;
    setSubmitting(true);
    try {
      await revokeOwnInvitationCode(codeId, reason.trim());
      setRevokeModalVisible(false);
      showSuccess(t('邀请码已撤销'));
      await loadInvitation();
    } catch (error) {
      showError(getInvitationErrorMessage(error, t('请求失败')));
    } finally {
      setSubmitting(false);
    }
  }

  async function withdrawApplication() {
    if (!invitation?.application?.id) return;
    setSubmitting(true);
    try {
      await withdrawInvitationApplication(invitation.application.id);
      showSuccess(t('邀请码申请已撤回'));
      await loadInvitation();
    } catch (error) {
      showError(getInvitationErrorMessage(error, t('请求失败')));
    } finally {
      setSubmitting(false);
    }
  }

  async function submitRewardPayout() {
    setSubmitting(true);
    try {
      await submitInvitationRewardPayout();
      showSuccess(t('提现申请已提交'));
      await loadRewardSummary();
    } catch (error) {
      showError(getInvitationErrorMessage(error, t('请求失败')));
    } finally {
      setSubmitting(false);
    }
  }

  async function copyValue(value, message) {
    if (await copy(value)) showSuccess(message);
  }

  return (
    <>
      <Card className='!self-start !rounded-lg border border-[var(--semi-color-border)] shadow-sm'>
        <div className='mb-4 flex flex-wrap items-start justify-between gap-3'>
          <div className='flex items-start gap-3'>
            <div className='flex size-9 items-center justify-center rounded-md bg-[var(--semi-color-primary-light-default)] text-[var(--semi-color-primary)]'>
              <Gift size={18} />
            </div>
            <div>
              <Typography.Title heading={5}>{t('我的邀请码')}</Typography.Title>
              <Typography.Text type='tertiary' size='small'>
                {t('申请后由管理员审批，批准后才能分享邀请码。')}
              </Typography.Text>
            </div>
          </div>
          {invitation?.code ? (
            <Tag color={invitation.code.status === 'active' ? 'green' : 'grey'}>
              {t(getInvitationStatusTranslationKey(invitation.code.status))}
            </Tag>
          ) : null}
        </div>

        {view === 'loading' ? (
          <div className='flex h-36 items-center justify-center'>
            <Spin />
          </div>
        ) : null}
        {view === 'error' ? (
          <Banner
            type='danger'
            description={t('邀请码信息加载失败，请重试。')}
            closeIcon={null}
            action={<Button onClick={loadInvitation}>{t('重试')}</Button>}
          />
        ) : null}
        {view === 'active' ? (
          <ActiveInvitation
            invitation={invitation}
            registrationUrl={registrationUrl}
            onCopy={copyValue}
            onShowQR={() => setQrVisible(true)}
            onRevoke={() => setRevokeModalVisible(true)}
            t={t}
          />
        ) : null}
        {view === 'pending' ? (
          <Banner
            type='info'
            title={t('申请待审批')}
            description={invitation.application.reason}
            closeIcon={null}
            action={
              <Button
                theme='borderless'
                type='danger'
                icon={<X size={15} />}
                loading={submitting}
                onClick={withdrawApplication}
              >
                {t('撤回申请')}
              </Button>
            }
          />
        ) : null}
        {view === 'rejected' ? (
          <Banner
            type='danger'
            title={t('申请未通过')}
            description={
              <div>
                <div>
                  {invitation.application.review_reason || t('未填写原因')}
                </div>
                {invitation.reapply_at ? (
                  <div className='mt-1 text-xs'>
                    {t('可再次申请时间')}：
                    {formatTimestamp(invitation.reapply_at)}
                  </div>
                ) : null}
              </div>
            }
            closeIcon={null}
          />
        ) : null}
        {view === 'terminal' ? (
          <Banner
            type='warning'
            title={t('Invitation code status: {{status}}', {
              status: t(
                getInvitationStatusTranslationKey(invitation.code.status),
              ),
            })}
            description={t('需要新邀请码时可重新提交申请。')}
            closeIcon={null}
          />
        ) : null}

        {!loading &&
        !failed &&
        view !== 'active' &&
        view !== 'pending' &&
        invitation?.can_apply ? (
          <div className='mt-4 flex justify-end'>
            <Button
              theme='solid'
              type='primary'
              icon={<Send size={15} />}
              loading={submitting}
              onClick={() => setApplicationModalVisible(true)}
            >
              {t('申请邀请码')}
            </Button>
          </div>
        ) : null}

        {rewardSummary?.cash_commission_visible ? (
          <Typography.Text
            type='success'
            size='small'
            className='mt-4 block font-medium'
          >
            {t('Top-up cashback {{rate}}%', {
              rate: formatCommissionRate(
                rewardSummary?.cash_commission_rate || '0',
              ),
            })}
          </Typography.Text>
        ) : null}

        <div className='mt-5 grid max-w-2xl grid-cols-2 gap-2 border-t border-[var(--semi-color-border)] pt-4 text-center sm:grid-cols-6'>
          <RewardMetric
            label={t('邀请人数')}
            value={String(rewardSummary?.invited_count || 0)}
          />
          <RewardMetric
            label={t('累计奖励')}
            value={renderQuota(rewardSummary?.total_earned || 0)}
          />
          <RewardMetric
            label={t('可申请')}
            value={renderQuota(rewardSummary?.available || 0)}
          />
          <RewardMetric
            label={t('审核中')}
            value={renderQuota(rewardSummary?.pending_payout || 0)}
          />
          <RewardMetric
            label={t('已到账')}
            value={renderQuota(rewardSummary?.paid || 0)}
          />
          <RewardMetric
            label={t('Frozen')}
            value={renderQuota(rewardSummary?.frozen || 0)}
          />
        </div>
        <div className='mt-3 flex justify-end'>
          <Button
            disabled={
              !complianceConfirmed || (rewardSummary?.available || 0) <= 0
            }
            loading={submitting}
            onClick={submitRewardPayout}
          >
            {t('提交提现审核')}
          </Button>
        </div>
      </Card>

      <InvitationQRModal
        visible={qrVisible}
        onCancel={() => setQrVisible(false)}
        value={registrationUrl}
        code={invitation?.code?.code || ''}
        systemName={systemName}
        logo={logo}
        displayName={displayName}
        title={status.invitation_poster_title || ''}
        subtitle={status.invitation_poster_subtitle || ''}
      />
      <InvitationApplicationModal
        visible={applicationModalVisible}
        submitting={submitting}
        onCancel={() => setApplicationModalVisible(false)}
        onSubmit={submitApplication}
      />
      <InvitationRevokeModal
        visible={revokeModalVisible}
        title={t('Revoke invitation code?')}
        submitting={submitting}
        onCancel={() => setRevokeModalVisible(false)}
        onConfirm={revokeInvitationCode}
      />
    </>
  );
}

function ActiveInvitation({
  invitation,
  registrationUrl,
  onCopy,
  onShowQR,
  onRevoke,
  t,
}) {
  const remaining = getInvitationRemainingUses(invitation.code);
  const columns = [
    {
      title: t('注册用户'),
      dataIndex: 'registered_user_id',
      render: (value) => `#${value}`,
    },
    {
      title: t('注册时间'),
      dataIndex: 'registered_at',
      render: formatTimestamp,
    },
    { title: t('来源'), dataIndex: 'source', render: (value) => value || '-' },
  ];
  return (
    <div className='space-y-4'>
      <Input
        value={invitation.code.code}
        readonly
        className='font-mono'
        suffix={
          <div className='flex items-center'>
            <Tooltip content={t('复制邀请码')}>
              <Button
                theme='borderless'
                icon={<Copy size={15} />}
                aria-label={t('复制邀请码')}
                onClick={() => onCopy(invitation.code.code, t('邀请码已复制'))}
              />
            </Tooltip>
          </div>
        }
      />
      <div className='space-y-1'>
        <Typography.Text type='tertiary' size='small'>
          {t('Invitation link')}
        </Typography.Text>
        <Input
          value={registrationUrl}
          readonly
          suffix={
            <div className='flex items-center'>
              <Tooltip content={t('复制注册链接')}>
                <Button
                  theme='borderless'
                  icon={<Copy size={15} />}
                  aria-label={t('复制注册链接')}
                  onClick={() => onCopy(registrationUrl, t('注册链接已复制'))}
                />
              </Tooltip>
              <Tooltip content={t('生成二维码')}>
                <Button
                  theme='borderless'
                  icon={<QrCode size={15} />}
                  aria-label={t('生成二维码')}
                  onClick={onShowQR}
                />
              </Tooltip>
            </div>
          }
        />
      </div>
      <div className='flex justify-end'>
        <Button
          theme='borderless'
          type='danger'
          icon={<X size={15} />}
          onClick={onRevoke}
        >
          {t('Revoke invitation code')}
        </Button>
      </div>
      <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
        <RewardMetric
          label={t('类型')}
          value={t(
            invitation.code.kind === 'multi' ? 'Multiple use' : 'Single use',
          )}
        />
        <RewardMetric
          label={t('已使用')}
          value={String(invitation.code.used_count)}
        />
        <RewardMetric label={t('剩余')} value={String(remaining)} />
        <RewardMetric
          label={t('过期时间')}
          value={formatTimestamp(invitation.code.expires_at)}
        />
      </div>
      <Table
        columns={columns}
        dataSource={invitation.uses || []}
        rowKey='id'
        pagination={false}
        size='small'
        empty={t('暂无使用记录')}
      />
    </div>
  );
}

function RewardMetric({ label, value }) {
  return (
    <div className='min-w-0 rounded-md bg-[var(--semi-color-fill-0)] px-2 py-2'>
      <Typography.Text type='tertiary' size='small'>
        {label}
      </Typography.Text>
      <div className='mt-1 truncate font-semibold tabular-nums'>{value}</div>
    </div>
  );
}

function formatTimestamp(timestamp) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString() : '-';
}

function formatCommissionRate(rate) {
  const normalized = String(rate).trim();
  if (!normalized.includes('.')) return normalized;
  return normalized.replace(/0+$/, '').replace(/\.$/, '');
}
