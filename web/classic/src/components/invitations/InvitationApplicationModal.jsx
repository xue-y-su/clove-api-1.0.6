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
import { Send } from 'lucide-react';
import { useTranslation } from 'react-i18next';

export default function InvitationApplicationModal({
  visible,
  submitting,
  onCancel,
  onSubmit,
}) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');
  const normalized = reason.trim();

  useEffect(() => {
    if (visible) setReason('');
  }, [visible]);

  return (
    <Modal
      title={t('Apply for invitation code')}
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
            type='primary'
            icon={<Send size={15} />}
            loading={submitting}
            disabled={normalized.length < 5 || normalized.length > 500}
            onClick={() => onSubmit?.(normalized)}
          >
            {t('Submit application')}
          </Button>
        </div>
      }
    >
      <div className='space-y-2'>
        <Typography.Text strong>{t('Application reason')}</Typography.Text>
        <TextArea
          value={reason}
          maxCount={500}
          autosize={{ minRows: 4, maxRows: 7 }}
          placeholder={t('Describe who you plan to invite and why.')}
          onChange={setReason}
        />
      </div>
    </Modal>
  );
}
