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
import { Button, Modal, TextArea, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

export default function InvitationRevokeModal({
  visible,
  title,
  submitting,
  onCancel,
  onConfirm,
}) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');
  const normalized = reason.trim();

  useEffect(() => {
    if (visible) setReason('');
  }, [visible]);

  return (
    <Modal
      title={title}
      visible={visible}
      onCancel={onCancel}
      width={500}
      footer={
        <div className='flex justify-end gap-2'>
          <Button disabled={submitting} onClick={onCancel}>
            {t('Cancel')}
          </Button>
          <Button
            theme='solid'
            type='danger'
            loading={submitting}
            disabled={!normalized || normalized.length > 500}
            onClick={() => onConfirm?.(normalized)}
          >
            {t('Revoke invitation code')}
          </Button>
        </div>
      }
    >
      <div className='space-y-3'>
        <Typography.Text type='tertiary'>
          {t(
            'This code will stop accepting new registrations. Existing registrations remain unchanged.',
          )}
        </Typography.Text>
        <Typography.Text strong>{t('Reason')}</Typography.Text>
        <TextArea
          value={reason}
          maxCount={500}
          autosize={{ minRows: 3, maxRows: 6 }}
          placeholder={t('Enter a reason for this action')}
          onChange={setReason}
        />
      </div>
    </Modal>
  );
}
