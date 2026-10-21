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
import { Modal, TextArea } from '@douyinfe/semi-ui';

const EnableDisableUserModal = ({
  visible,
  onCancel,
  onConfirm,
  user,
  action,
  t,
}) => {
  const isDisable = action === 'disable';
  const [banReasonPublic, setBanReasonPublic] = useState('');
  const [banReasonInternal, setBanReasonInternal] = useState('');

  useEffect(() => {
    if (!visible) {
      setBanReasonPublic('');
      setBanReasonInternal('');
    }
  }, [visible, user?.id, action]);

  return (
    <Modal
      title={isDisable ? t('确定要禁用此用户吗？') : t('确定要启用此用户吗？')}
      visible={visible}
      onCancel={onCancel}
      onOk={() =>
        onConfirm({
          ban_reason_public: banReasonPublic.trim(),
          ban_reason_internal: banReasonInternal.trim(),
        })
      }
      type='warning'
    >
      {isDisable ? t('此操作将禁用用户账户') : t('此操作将启用用户账户')}
      {isDisable && (
        <div className='mt-4 space-y-3'>
          <TextArea
            value={banReasonPublic}
            onChange={setBanReasonPublic}
            placeholder={t('用户可见封禁理由（登录或访问接口时显示）')}
            autosize={{ minRows: 3, maxRows: 4 }}
          />
          <TextArea
            value={banReasonInternal}
            onChange={setBanReasonInternal}
            placeholder={t('内部处理备注（仅管理员和审计可见）')}
            autosize={{ minRows: 3, maxRows: 4 }}
          />
        </div>
      )}
    </Modal>
  );
};

export default EnableDisableUserModal;
