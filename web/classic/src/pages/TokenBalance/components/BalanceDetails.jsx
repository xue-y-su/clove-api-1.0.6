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

import { IconInfoCircle } from '@douyinfe/semi-icons';
import { Tooltip } from '@douyinfe/semi-ui';
import dayjs from 'dayjs';
import PropTypes from 'prop-types';
import React from 'react';

import { formatBalanceQuota, getTokenCallableQuota } from '../balance';

const preferenceKeys = {
  subscription_first: '订阅优先',
  wallet_first: '账户余额优先',
  subscription_only: '仅限订阅',
  wallet_only: '仅限账户余额',
};

const restrictionKeys = {
  subscription_quota: '受订阅额度限制',
  wallet_quota: '受账户余额限制',
  funding_quota: '受可用资金限制',
  key_unavailable: 'API 密钥当前不可用',
};

function DetailRow({ label, children }) {
  return (
    <div className='grid min-w-0 grid-cols-[minmax(7rem,0.7fr)_minmax(0,1.3fr)] gap-3 border-b border-semi-color-border py-3 last:border-b-0'>
      <dt className='text-sm text-semi-color-text-2'>{label}</dt>
      <dd className='min-w-0 break-words text-right text-sm font-medium text-semi-color-text-0'>
        {children}
      </dd>
    </div>
  );
}

DetailRow.propTypes = {
  label: PropTypes.string.isRequired,
  children: PropTypes.node.isRequired,
};

const BalanceDetails = ({ data, t }) => {
  const unlimited = t('无限制');
  const format = (amount) =>
    formatBalanceQuota(amount, data.display, unlimited);
  const tokenCallableQuota = getTokenCallableQuota(data.balance);
  const groupValue =
    data.group.mode === 'auto'
      ? `${t('自动分组')}：${data.group.effective_groups?.join(', ') || '-'}`
      : data.group.name || data.group.effective_groups?.[0] || '-';
  const modelValue = data.model_limits.enabled
    ? t('仅限 {{count}} 个模型', { count: data.model_limits.count })
    : t('无模型限制');
  const models = data.model_limits.models || [];

  return (
    <div className='grid grid-cols-1 divide-y divide-semi-color-border sm:grid-cols-2 sm:divide-x sm:divide-y-0'>
      <section className='min-w-0 px-4 py-4 sm:px-5'>
        <h2 className='text-sm font-semibold text-semi-color-text-0'>
          {t('额度构成')}
        </h2>
        <dl className='mt-2'>
          {data.balance.token_unlimited ? (
            <DetailRow label={t('密钥限额')}>{unlimited}</DetailRow>
          ) : (
            <DetailRow label={t('密钥剩余额度')}>
              {format({
                unlimited: false,
                value: data.balance.token_remaining,
              })}
            </DetailRow>
          )}
          {data.balance.wallet ? (
            <DetailRow label={t('钱包可消费余额')}>
              {format(data.balance.wallet.available)}
            </DetailRow>
          ) : null}
          <DetailRow label={t('密钥可调用额度')}>
            {format(tokenCallableQuota)}
          </DetailRow>
          {data.balance.subscription ? (
            <DetailRow label={t('订阅剩余额度')}>
              {format(data.balance.subscription.available)}
            </DetailRow>
          ) : null}
          {data.balance.restriction_reason ? (
            <DetailRow label={t('限制原因')}>
              <span className='text-semi-color-danger'>
                {t(
                  restrictionKeys[data.balance.restriction_reason] ||
                    '受限额度',
                )}
              </span>
            </DetailRow>
          ) : null}
        </dl>
      </section>

      <section className='min-w-0 px-4 py-4 sm:px-5'>
        <h2 className='text-sm font-semibold text-semi-color-text-0'>
          {t('密钥配置')}
        </h2>
        <dl className='mt-2'>
          <DetailRow label={t('计费来源')}>
            {t(
              preferenceKeys[data.billing_preference] ||
                data.billing_preference,
            )}
          </DetailRow>
          <DetailRow label={t('分组限制')}>{groupValue}</DetailRow>
          <DetailRow label={t('模型限制')}>
            <span className='inline-flex max-w-full items-center justify-end gap-1.5'>
              <span>{modelValue}</span>
              {data.model_limits.enabled && models.length > 0 ? (
                <Tooltip
                  content={
                    <span className='max-w-72 break-words'>
                      {models.join(', ')}
                    </span>
                  }
                  position='top'
                  showArrow
                >
                  <button
                    type='button'
                    className='inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded text-semi-color-text-2 focus-visible:outline focus-visible:outline-2 focus-visible:outline-semi-color-focus-border'
                    aria-label={t('查看模型限制')}
                  >
                    <IconInfoCircle size='small' />
                  </button>
                </Tooltip>
              ) : null}
            </span>
          </DetailRow>
          <DetailRow label={t('单次请求上限')}>
            {format(data.balance.single_request_limit)}
          </DetailRow>
          <DetailRow label={t('过期时间')}>
            {data.expires_at > 0
              ? dayjs.unix(data.expires_at).format('YYYY-MM-DD HH:mm:ss')
              : t('永不过期')}
          </DetailRow>
        </dl>
      </section>
    </div>
  );
};

BalanceDetails.propTypes = {
  data: PropTypes.object.isRequired,
  t: PropTypes.func.isRequired,
};

export default BalanceDetails;
