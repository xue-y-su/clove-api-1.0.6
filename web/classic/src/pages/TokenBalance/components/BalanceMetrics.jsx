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

import PropTypes from 'prop-types';
import React from 'react';

import { formatBalanceQuota, getBalanceMetricValues } from '../balance';

function Metric({ color, label, value, description, danger }) {
  return (
    <div className='min-w-0 px-4 py-4 sm:px-5'>
      <div className='flex items-center gap-2 text-xs font-medium text-semi-color-text-2'>
        <span
          className='size-2 shrink-0 rounded-full'
          style={{ background: color }}
        />
        <span className='min-w-0 break-words'>{label}</span>
      </div>
      <div
        className='mt-1.5 break-all text-xl font-semibold tabular-nums sm:text-2xl'
        style={{ color: danger ? 'var(--semi-color-danger)' : undefined }}
        title={value}
      >
        {value}
      </div>
      <div className='mt-1 text-xs text-semi-color-text-3'>{description}</div>
    </div>
  );
}

Metric.propTypes = {
  color: PropTypes.string.isRequired,
  label: PropTypes.string.isRequired,
  value: PropTypes.string.isRequired,
  description: PropTypes.string.isRequired,
  danger: PropTypes.bool,
};

const BalanceMetrics = ({ balance, display, t }) => {
  const metrics = getBalanceMetricValues(balance);
  const unlimited = t('无限制');
  const formatValue = (value) =>
    formatBalanceQuota({ unlimited: false, value }, display, unlimited);
  const isRestricted = metrics.third.kind === 'restricted';

  return (
    <div className='grid grid-cols-1 divide-y divide-semi-color-border border-b border-semi-color-border sm:grid-cols-3 sm:divide-x sm:divide-y-0'>
      <Metric
        color='var(--semi-color-primary)'
        label={t('密钥可调用额度')}
        value={formatBalanceQuota(metrics.available, display, unlimited)}
        description={t('此 API 密钥在钱包、订阅和密钥限制后的当前可调用额度')}
      />
      <Metric
        color='var(--semi-color-text-3)'
        label={t('已使用额度')}
        value={formatValue(metrics.used)}
        description={t('此 API 密钥累计消费的额度')}
      />
      <Metric
        color={
          isRestricted ? 'var(--semi-color-danger)' : 'var(--semi-color-text-3)'
        }
        label={isRestricted ? t('不可用额度') : t('密钥限额')}
        value={isRestricted ? formatValue(metrics.third.value) : unlimited}
        description={
          isRestricted ? t('受当前可用资金限制') : t('未设置独立额度上限')
        }
        danger={isRestricted && metrics.third.value > 0}
      />
    </div>
  );
};

BalanceMetrics.propTypes = {
  balance: PropTypes.object.isRequired,
  display: PropTypes.object.isRequired,
  t: PropTypes.func.isRequired,
};

export default BalanceMetrics;
