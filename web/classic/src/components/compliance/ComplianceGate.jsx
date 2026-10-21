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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Divider,
  Input,
  Modal,
  Spin,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import MarkdownRenderer from '../common/markdown/MarkdownRenderer';
import { API, showError, showSuccess } from '../../helpers';

const ComplianceGate = ({
  queryEnabled = true,
  displayEnabled = true,
  retryToken = 0,
  onResolutionChange,
}) => {
  const { t } = useTranslation();
  const [statement, setStatement] = useState(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(null);
  const [confirming, setConfirming] = useState(false);
  const [confirmationTexts, setConfirmationTexts] = useState({});
  const [segmentTexts, setSegmentTexts] = useState({});

  const loadStatement = async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const res = await API.get('/api/compliance/statement', {
        disableDuplicate: true,
        skipErrorHandler: true,
        timeout: 10000,
      });
      if (res.data?.success) {
        setStatement(res.data.data);
      } else {
        throw new Error(
          res.data?.message || 'Failed to load compliance statement',
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
      setStatement(null);
      setLoadError(null);
      onResolutionChange?.('loading');
      return;
    }
    loadStatement();
  }, [queryEnabled, retryToken]);

  const pendingSnapshots = useMemo(() => {
    return (statement?.snapshots || [])
      .filter((snapshot) => !snapshot.confirmed)
      .sort((a, b) => {
        if (a.sort_order === b.sort_order) {
          return String(a.item_key).localeCompare(String(b.item_key));
        }
        return a.sort_order - b.sort_order;
      });
  }, [statement?.snapshots]);

  const activeSnapshots = useMemo(() => {
    if (pendingSnapshots.length === 0) return [];
    if (statement?.display_mode === 'multiple') return [pendingSnapshots[0]];
    return pendingSnapshots;
  }, [pendingSnapshots, statement?.display_mode]);

  const open = Boolean(statement?.enabled && !statement?.confirmed);
  const resolution =
    !queryEnabled || loading
      ? 'loading'
      : loadError || !statement
        ? 'error'
        : open
          ? 'required'
          : 'satisfied';
  const legacyMode = open && activeSnapshots.length === 0;

  useEffect(() => {
    onResolutionChange?.(resolution, loadError);
  }, [loadError, onResolutionChange, resolution]);

  useEffect(() => {
    setConfirmationTexts({});
    setSegmentTexts({});
  }, [
    statement?.content,
    activeSnapshots
      .map((item) => `${item.id || item.item_key}:${item.confirmation_mode}`)
      .join(','),
  ]);

  const updateSegmentText = (snapshotId, index, value) => {
    setSegmentTexts((current) => {
      const next = { ...current };
      const values = [...(next[snapshotId] || [])];
      values[index] = value;
      next[snapshotId] = values;
      return next;
    });
  };

  const isSnapshotReady = (snapshot) => {
    if (snapshot.confirmation_mode === 'phrase') {
      return (
        (confirmationTexts[snapshot.id] || '').trim() ===
        (snapshot.confirmation_phrase || '').trim()
      );
    }
    if (snapshot.confirmation_mode === 'segments') {
      const expected = snapshot.segments || [];
      const values = segmentTexts[snapshot.id] || [];
      return (
        expected.length > 0 &&
        expected.every((segment, index) => {
          return (values[index] || '').trim() === String(segment).trim();
        })
      );
    }
    return true;
  };

  const confirmDisabled =
    confirming ||
    loading ||
    activeSnapshots.some((snapshot) => !isSnapshotReady(snapshot));

  const buildConfirmationPayload = (snapshot) => {
    if (!snapshot?.id) return {};
    const payload = { snapshot_id: snapshot.id };
    if (snapshot.confirmation_mode === 'phrase') {
      payload.confirmation_text = confirmationTexts[snapshot.id] || '';
    }
    if (snapshot.confirmation_mode === 'segments') {
      payload.segments = segmentTexts[snapshot.id] || [];
    }
    return payload;
  };

  const confirmStatement = async () => {
    setConfirming(true);
    try {
      if (legacyMode) {
        const res = await API.post('/api/compliance/confirm');
        const { success, message } = res.data;
        if (!success) {
          showError(message);
          return;
        }
      } else {
        for (const snapshot of activeSnapshots) {
          const res = await API.post(
            '/api/compliance/confirm',
            buildConfirmationPayload(snapshot),
          );
          const { success, message } = res.data;
          if (!success) {
            showError(message);
            return;
          }
        }
      }
      showSuccess(t('合规声明已确认'));
      window.dispatchEvent(new Event('compliance-confirmed'));
      await loadStatement();
    } catch (error) {
      showError(error);
    } finally {
      setConfirming(false);
    }
  };

  const renderConfirmationControl = (snapshot) => {
    if (snapshot.confirmation_mode === 'phrase') {
      return (
        <Input
          label={t('确认短语')}
          placeholder={snapshot.confirmation_phrase || t('请输入确认短语')}
          value={confirmationTexts[snapshot.id] || ''}
          onChange={(value) =>
            setConfirmationTexts((current) => ({
              ...current,
              [snapshot.id]: value,
            }))
          }
        />
      );
    }
    if (snapshot.confirmation_mode === 'segments') {
      return (
        <div className='space-y-3'>
          <Typography.Text strong>{t('分段确认')}</Typography.Text>
          {(snapshot.segments || []).map((segment, index) => (
            <div key={`${snapshot.id}-${index}`} className='space-y-1'>
              <Typography.Paragraph copyable={{ content: segment }}>
                {segment}
              </Typography.Paragraph>
              <TextArea
                placeholder={t('复制或输入本段内容')}
                value={(segmentTexts[snapshot.id] || [])[index] || ''}
                autosize={{ minRows: 2, maxRows: 4 }}
                onChange={(value) =>
                  updateSegmentText(snapshot.id, index, value)
                }
              />
            </div>
          ))}
        </div>
      );
    }
    return null;
  };

  const renderMeta = (version, contentHash) => {
    const parts = [];
    if (version) parts.push(`${t('版本号')}: ${version}`);
    if (contentHash) parts.push(`${t('内容哈希')}: ${contentHash}`);
    if (parts.length === 0) return null;
    return (
      <Typography.Text type='tertiary' size='small'>
        {parts.join(' / ')}
      </Typography.Text>
    );
  };

  const renderSnapshot = (snapshot, index) => (
    <div key={snapshot.id || snapshot.item_key}>
      {index > 0 && <Divider />}
      <Typography.Title heading={5}>
        {snapshot.title || snapshot.item_key}
      </Typography.Title>
      {snapshot.version ? (
        <Typography.Text type='tertiary' size='small'>
          {t('版本号')}: {snapshot.version}
        </Typography.Text>
      ) : null}
      <div style={{ maxHeight: '45vh', overflow: 'auto' }}>
        <MarkdownRenderer content={snapshot.content || ''} />
      </div>
      {renderConfirmationControl(snapshot)}
      {renderMeta(null, snapshot.content_hash)}
    </div>
  );

  return (
    <>
      <Modal
        visible={displayEnabled && open}
        title={t('合规声明')}
        closable={false}
        maskClosable={false}
        footer={
          <Button
            theme='solid'
            type='primary'
            loading={confirming}
            disabled={confirmDisabled}
            onClick={confirmStatement}
          >
            {t('我已阅读并同意')}
          </Button>
        }
      >
        <Spin spinning={loading}>
          {legacyMode ? (
            <>
              <div style={{ maxHeight: '55vh', overflow: 'auto' }}>
                <MarkdownRenderer content={statement?.content || ''} />
              </div>
              {renderMeta(statement?.version, statement?.content_hash)}
            </>
          ) : (
            <div className='space-y-4'>
              {activeSnapshots.map((snapshot, index) =>
                renderSnapshot(snapshot, index),
              )}
            </div>
          )}
        </Spin>
      </Modal>
    </>
  );
};

export default ComplianceGate;
