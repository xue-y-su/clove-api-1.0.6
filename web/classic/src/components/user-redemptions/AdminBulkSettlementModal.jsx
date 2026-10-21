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

import { batchSettleAdminUserRedemptions } from './api.js';

export default function AdminBulkSettlementModal({
  visible,
  ids,
  onClose,
  onCompleted,
}) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');
  const [disposition, setDisposition] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [results, setResults] = useState([]);
  const normalizedReason = reason.trim();

  useEffect(() => {
    if (!visible) {
      setReason('');
      setDisposition('');
      setError('');
      setResults([]);
    }
  }, [visible]);

  async function submit() {
    if (!ids.length || !normalizedReason || !disposition) return;
    setBusy(true);
    setError('');
    try {
      const response = await batchSettleAdminUserRedemptions({
        ids,
        disposition,
        reason: normalizedReason,
      });
      setResults(response);
      await onCompleted?.();
    } catch (caught) {
      setError(caught?.message || t('Request failed'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title={t('Batch settlement')}
      visible={visible}
      onCancel={busy ? undefined : onClose}
      width={540}
      footer={
        <div className='flex justify-end gap-2'>
          <Button disabled={busy} onClick={onClose}>
            {results.length ? t('Close') : t('Cancel')}
          </Button>
          <Button
            theme='solid'
            type='primary'
            loading={busy}
            disabled={
              results.length > 0 ||
              !ids.length ||
              !normalizedReason ||
              normalizedReason.length > 500 ||
              !disposition
            }
            onClick={() => void submit()}
          >
            {t('Confirm')}
          </Button>
        </div>
      }
    >
      <div className='space-y-4'>
        <Typography.Text type='tertiary'>
          {t('{{count}} eligible codes selected.', { count: ids.length })}
        </Typography.Text>
        <div className='space-y-2'>
          <Typography.Text strong>{t('Settlement')}</Typography.Text>
          <ButtonGroup className='w-full'>
            <Button
              className='flex-1'
              theme={disposition === 'refund' ? 'solid' : 'light'}
              type={disposition === 'refund' ? 'primary' : 'tertiary'}
              onClick={() => setDisposition('refund')}
            >
              {t('Refund remaining balance')}
            </Button>
            <Button
              className='flex-1'
              theme={disposition === 'forfeit' ? 'solid' : 'light'}
              type={disposition === 'forfeit' ? 'danger' : 'tertiary'}
              onClick={() => setDisposition('forfeit')}
            >
              {t('Forfeit remaining balance')}
            </Button>
          </ButtonGroup>
        </div>
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
        {results.length ? (
          <div className='space-y-2'>
            <Typography.Text strong>
              {t('Batch settlement results')}
            </Typography.Text>
            <div className='divide-y rounded-md border border-[var(--semi-color-border)]'>
              {results.map((result) => (
                <div
                  key={result.id}
                  className='flex items-center justify-between gap-3 px-3 py-2 text-sm'
                >
                  <span className='font-mono text-xs'>#{result.id}</span>
                  <Typography.Text type={result.success ? 'success' : 'danger'}>
                    {result.success
                      ? t('Settled')
                      : getResultMessage(result.code, t)}
                  </Typography.Text>
                </div>
              ))}
            </div>
          </div>
        ) : null}
        {error ? (
          <Banner type='danger' description={error} closeIcon={null} />
        ) : null}
      </div>
    </Modal>
  );
}

function getResultMessage(code, t) {
  if (code === 'not_found') return t('Redemption code was not found');
  if (code === 'inactive') {
    return t('Redemption code is not eligible for settlement');
  }
  if (code === 'conflict') {
    return t('Redemption code has already been settled differently');
  }
  if (code === 'invalid') return t('Settlement request is invalid');
  return t('Unable to settle this redemption code');
}
