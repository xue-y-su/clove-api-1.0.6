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
  ButtonGroup,
  Modal,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

import { renderQuota } from '../../helpers';
import {
  deleteAdminUserRedemption,
  disableAdminUserRedemption,
  enableAdminUserRedemption,
  terminateAdminUserRedemption,
} from './api.js';

export default function AdminUserRedemptionGovernanceModal({
  target,
  action,
  onClose,
  onCompleted,
}) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');
  const [settlement, setSettlement] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const live =
    target?.status === 'active' ||
    target?.status === 'disabled' ||
    target?.status === 'expired_pending';
  const settlementRequired =
    action === 'terminate' || (action === 'delete' && live);
  const isSubscription = target?.reward_type === 'subscription';
  const normalizedReason = reason.trim();

  useEffect(() => {
    setReason('');
    setSettlement('');
    setError('');
  }, [action, target?.id]);

  async function submit() {
    if (!target || !action || !normalizedReason) return;
    if (settlementRequired && !settlement) return;
    setBusy(true);
    setError('');
    try {
      if (action === 'disable') {
        await disableAdminUserRedemption(target.id, normalizedReason);
      } else if (action === 'enable') {
        await enableAdminUserRedemption(target.id, normalizedReason);
      } else if (action === 'terminate') {
        await terminateAdminUserRedemption(
          target.id,
          settlement,
          normalizedReason,
        );
      } else if (action === 'delete') {
        await deleteAdminUserRedemption(
          target.id,
          settlementRequired ? settlement : '',
          normalizedReason,
        );
      }
      onClose?.();
      await onCompleted?.();
    } catch (caught) {
      setError(caught?.message || t('Request failed'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title={getTitle(action, t)}
      visible={Boolean(target && action)}
      onCancel={busy ? undefined : onClose}
      width={520}
      footer={
        <div className='flex justify-end gap-2'>
          <Button disabled={busy} onClick={onClose}>
            {t('Cancel')}
          </Button>
          <Button
            theme='solid'
            type={action === 'delete' ? 'danger' : 'primary'}
            loading={busy}
            disabled={
              !normalizedReason ||
              normalizedReason.length > 500 ||
              (settlementRequired && !settlement)
            }
            onClick={() => void submit()}
          >
            {t('Confirm')}
          </Button>
        </div>
      }
    >
      <div className='space-y-4'>
        {target ? (
          <div className='rounded-md border border-[var(--semi-color-border)] p-3'>
            <div className='font-mono'>{target.code}</div>
            <Typography.Text type='tertiary' size='small'>
              {t('Remaining')}:{' '}
              {isSubscription
                ? `${Math.max(0, target.max_claims - target.claim_count)} ${t('units')}`
                : renderQuota(target.remaining_quota)}
            </Typography.Text>
          </div>
        ) : null}
        {settlementRequired ? (
          <div className='space-y-2'>
            <Typography.Text strong>{t('Settlement')}</Typography.Text>
            <ButtonGroup className='w-full'>
              <Button
                className='flex-1'
                theme={settlement === 'refund' ? 'solid' : 'light'}
                type={settlement === 'refund' ? 'primary' : 'tertiary'}
                onClick={() => setSettlement('refund')}
              >
                {isSubscription
                  ? t('Refund remaining units')
                  : t('Refund remaining balance')}
              </Button>
              <Button
                className='flex-1'
                theme={settlement === 'forfeit' ? 'solid' : 'light'}
                type={settlement === 'forfeit' ? 'danger' : 'tertiary'}
                onClick={() => setSettlement('forfeit')}
              >
                {isSubscription
                  ? t('Forfeit remaining units')
                  : t('Forfeit remaining balance')}
              </Button>
            </ButtonGroup>
          </div>
        ) : null}
        <div className='space-y-2'>
          <Typography.Text strong>{t('Reason')}</Typography.Text>
          <TextArea
            value={reason}
            maxCount={500}
            autosize={{ minRows: 3, maxRows: 6 }}
            placeholder={t('Enter a reason for this action')}
            onChange={setReason}
          />
        </div>
        {error ? (
          <Banner type='danger' description={error} closeIcon={null} />
        ) : null}
      </div>
    </Modal>
  );
}

function getTitle(action, t) {
  if (action === 'disable') return t('禁用');
  if (action === 'enable') return t('启用');
  if (action === 'terminate') return t('终止');
  if (action === 'delete') return t('删除');
  return t('管理');
}
