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
import { Modal, Input, Toast } from '@douyinfe/semi-ui';
import { API, showError } from '../../helpers';
import { useTranslation } from 'react-i18next';

export default function ClaimUserRedemptionModal({
  visible,
  onClose,
  onClaimed,
  expectedRewardType = 'subscription',
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [code, setCode] = useState('');

  useEffect(() => {
    if (visible) setCode('');
  }, [visible]);

  async function submit() {
    const trimmed = String(code || '').trim();
    if (!trimmed) return;
    setBusy(true);
    try {
      const response = await API.post('/api/user-redemption/claim', {
        code: trimmed,
        expected_reward_type: expectedRewardType,
      });
      if (!response.data?.success) {
        showError(response.data?.message || t('claim failed'));
        return;
      }
      Toast.success(t('Redeemed'));
      onClose?.();
      onClaimed?.();
    } catch (e) {
      showError(e?.message || t('claim failed'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title={t('Redeem Code')}
      visible={visible}
      onCancel={onClose}
      onOk={() => void submit()}
      confirmLoading={busy}
      okText={t('Redeem')}
      cancelText={t('Cancel')}
    >
      <Input
        value={code}
        onChange={(v) => setCode(v)}
        placeholder={t('Enter redemption code')}
      />
    </Modal>
  );
}
