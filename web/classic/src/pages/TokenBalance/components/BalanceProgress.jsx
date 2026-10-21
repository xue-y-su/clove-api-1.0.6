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

import {
  formatBalanceQuota,
  getBalanceProgressSegments,
  getTokenCallableQuota,
} from '../balance';

const restrictionKeys = {
  subscription_quota: '受订阅额度限制',
  wallet_quota: '受账户余额限制',
  funding_quota: '受可用资金限制',
  key_unavailable: 'API 密钥当前不可用',
};

const BalanceProgress = ({ balance, display, t }) => {
  const segments = getBalanceProgressSegments(balance);
  const callable = getTokenCallableQuota(balance);
  const unlimited = t('无限制');
  const format = (amount) => formatBalanceQuota(amount, display, unlimited);
  const formatValue = (value) => format({ unlimited: false, value });
  let rightLabel = `${t('不可用额度')} ${formatValue(0)}`;
  if (balance.token_unlimited) {
    rightLabel = `${t('已使用额度')} ${formatValue(balance.token_used)}`;
  } else if (balance.restricted > 0) {
    rightLabel = `${formatValue(balance.restricted)} ${t(
      restrictionKeys[balance.restriction_reason] || '受限额度',
    )}`;
  }

  return (
    <section className='border-b border-semi-color-border px-4 py-4 sm:px-5'>
      <div
        className='flex h-2.5 w-full overflow-hidden rounded-full bg-semi-color-fill-0'
        role='img'
        aria-label={t('额度使用进度')}
      >
        {segments.available > 0 ? (
          <span
            className='h-full'
            style={{
              width: `${segments.available}%`,
              background: 'var(--semi-color-primary)',
            }}
          />
        ) : null}
        {segments.used > 0 ? (
          <span
            className='h-full border-l border-white/80'
            style={{
              width: `${segments.used}%`,
              background: 'var(--semi-color-text-3)',
            }}
          />
        ) : null}
        {segments.restricted > 0 ? (
          <span
            className='h-full border-l border-white/80'
            style={{
              width: `${segments.restricted}%`,
              background: 'var(--semi-color-danger)',
            }}
          />
        ) : null}
      </div>
      <div className='mt-2 flex flex-col justify-between gap-1 text-xs text-semi-color-text-2 sm:flex-row'>
        <span>
          {balance.token_unlimited
            ? `${t('密钥可调用额度')} ${format(callable)}`
            : `${t('密钥总限额')} ${formatValue(balance.token_total)}`}
        </span>
        <span
          style={{
            color:
              balance.restricted > 0 ? 'var(--semi-color-danger)' : undefined,
          }}
        >
          {rightLabel}
        </span>
      </div>
    </section>
  );
};

BalanceProgress.propTypes = {
  balance: PropTypes.object.isRequired,
  display: PropTypes.object.isRequired,
  t: PropTypes.func.isRequired,
};

export default BalanceProgress;
