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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import { Button, Form, Modal, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, setUserData, showError, showSuccess } from '../../helpers';
import { UserContext } from '../../context/User';

const SourceAttributionGate = ({
  queryEnabled = true,
  displayEnabled = true,
  retryToken = 0,
  onResolutionChange,
}) => {
  const { t } = useTranslation();
  const [, userDispatch] = useContext(UserContext);
  const [status, setStatus] = useState(null);
  const [optionId, setOptionId] = useState('');
  const [sourceDescription, setSourceDescription] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(null);
  const [confirming, setConfirming] = useState(false);

  const loadStatus = async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const sourceRes = await API.get('/api/source-attribution/status', {
        disableDuplicate: true,
        skipErrorHandler: true,
        timeout: 10000,
      });
      if (sourceRes.data?.success) {
        setStatus(sourceRes.data.data);
      } else {
        throw new Error(
          sourceRes.data?.message || 'Failed to load source attribution',
        );
      }
    } catch (error) {
      setLoadError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!queryEnabled) {
      setStatus(null);
      setLoadError(null);
      onResolutionChange?.('loading');
      return;
    }
    loadStatus();
  }, [queryEnabled, retryToken]);

  const options = useMemo(() => {
    const items = status?.options || [];
    return [...items].sort((a, b) => {
      if (a.sort_order === b.sort_order) {
        return String(a.name).localeCompare(String(b.name));
      }
      return a.sort_order - b.sort_order;
    });
  }, [status?.options]);

  const selectedOption = options.find((item) => item.id === optionId);
  const open = Boolean(status?.enabled && !status.confirmed);
  const resolution =
    !queryEnabled || loading
      ? 'loading'
      : loadError || !status
        ? 'error'
        : open
          ? 'required'
          : 'satisfied';
  const confirmDisabled =
    confirming ||
    loading ||
    !optionId ||
    (selectedOption?.requires_description && !sourceDescription.trim());

  useEffect(() => {
    if (!open) {
      setOptionId('');
      setSourceDescription('');
    }
  }, [open]);

  useEffect(() => {
    onResolutionChange?.(resolution, loadError);
  }, [loadError, onResolutionChange, resolution]);

  const refreshUser = async () => {
    const res = await API.get('/api/user/self');
    if (res.data?.success) {
      userDispatch({ type: 'login', payload: res.data.data });
      setUserData(res.data.data);
    }
  };

  const confirmAttribution = async () => {
    if (confirmDisabled) return;
    setConfirming(true);
    try {
      const res = await API.post('/api/source-attribution/confirm', {
        option_id: optionId,
        source_description: sourceDescription.trim(),
      });
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('来源归属已确认'));
      await refreshUser();
      await loadStatus();
    } catch (error) {
      showError(error);
    } finally {
      setConfirming(false);
    }
  };

  return (
    <>
      <Modal
        visible={displayEnabled && open}
        title={t('来源归属')}
        closable={false}
        maskClosable={false}
        footer={
          <Button
            theme='solid'
            type='primary'
            loading={confirming}
            disabled={confirmDisabled}
            onClick={confirmAttribution}
          >
            {t('确认')}
          </Button>
        }
      >
        <Spin spinning={loading}>
          <Form labelPosition='top'>
            <Form.Select
              field='option_id'
              label={t('归属选项')}
              value={optionId}
              placeholder={t('请选择归属选项')}
              optionList={options.map((item) => ({
                label: item.name,
                value: item.id,
              }))}
              onChange={(value) => {
                setOptionId(value || '');
                setSourceDescription('');
              }}
              style={{ width: '100%' }}
            />
            {selectedOption?.requires_description && (
              <Form.TextArea
                field='source_description'
                label={t('来源说明')}
                value={sourceDescription}
                placeholder={t('请输入来源说明')}
                maxCount={200}
                autosize={{ minRows: 3, maxRows: 5 }}
                onChange={setSourceDescription}
              />
            )}
            {options.length === 0 && (
              <div className='text-sm text-gray-500'>
                {t('尚未配置归属选项，请联系管理员。')}
              </div>
            )}
          </Form>
        </Spin>
      </Modal>
    </>
  );
};

export default SourceAttributionGate;
