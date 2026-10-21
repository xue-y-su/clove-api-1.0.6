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
import { IconAlertTriangle, IconRefresh } from '@douyinfe/semi-icons';
import { Button, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

import { extractGateRateLimit, extractGateRequestId } from './gate-load-error';

const GateLoadError = ({ gate, error, onRetry }) => {
  const { t } = useTranslation();
  const requestId = extractGateRequestId(error);
  const rateLimit = extractGateRateLimit(error);
  const [retryAfterSeconds, setRetryAfterSeconds] = useState(
    rateLimit?.retryAfterSeconds ?? 0,
  );
  const gateName =
    gate === 'compliance' ? t('Compliance Statement') : t('Source Attribution');

  useEffect(() => {
    setRetryAfterSeconds(rateLimit?.retryAfterSeconds ?? 0);
  }, [error, rateLimit?.retryAfterSeconds]);

  useEffect(() => {
    if (!rateLimit || retryAfterSeconds <= 0) return undefined;
    const timer = window.setTimeout(() => {
      setRetryAfterSeconds((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [rateLimit, retryAfterSeconds]);

  return (
    <div
      role='alert'
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 2000,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
        background: 'var(--semi-color-bg-0)',
      }}
    >
      <div
        style={{
          display: 'flex',
          maxWidth: 480,
          flexDirection: 'column',
          alignItems: 'center',
          gap: 12,
          textAlign: 'center',
        }}
      >
        <IconAlertTriangle
          size='extra-large'
          style={{ color: 'var(--semi-color-danger)' }}
        />
        <Typography.Title heading={5}>
          {rateLimit ? t('请求过于频繁') : t('加载失败')}
        </Typography.Title>
        <Typography.Text type='tertiary'>
          {rateLimit
            ? t('{{seconds}} 秒后可重新查询。', {
                seconds: retryAfterSeconds,
              })
            : gateName}
        </Typography.Text>
        {requestId ? (
          <Typography.Text
            type='tertiary'
            size='small'
            style={{
              maxWidth: '100%',
              fontFamily: 'monospace',
              wordBreak: 'break-all',
            }}
          >
            {t('Request ID')}: {requestId}
          </Typography.Text>
        ) : null}
        <Button
          theme='solid'
          type='primary'
          icon={<IconRefresh />}
          onClick={onRetry}
          disabled={Boolean(rateLimit && retryAfterSeconds > 0)}
        >
          {t('重试')}
        </Button>
      </div>
    </div>
  );
};

export default GateLoadError;
