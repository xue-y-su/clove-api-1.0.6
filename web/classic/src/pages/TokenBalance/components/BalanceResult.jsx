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

import { Banner, Card, Tag } from '@douyinfe/semi-ui';
import PropTypes from 'prop-types';
import React from 'react';

import { getBalanceStatusExplanationKey } from '../balance';
import BalanceDetails from './BalanceDetails';
import BalanceMetrics from './BalanceMetrics';
import BalanceProgress from './BalanceProgress';

const statusKeys = {
  active: '正常',
  disabled: '已禁用',
  expired: '已过期',
  exhausted: '额度已耗尽',
  user_disabled: '所属用户已禁用',
};

const groupModeKeys = {
  fixed: '密钥指定',
  user: '用户分组',
  auto: '自动分组',
};

const BalanceResult = ({ data, t }) => {
  const active = data.status === 'active';
  const groupTitle =
    data.group.mode === 'auto'
      ? t('自动分组')
      : data.group.name || data.group.effective_groups?.[0] || '-';
  const statusExplanationKey = getBalanceStatusExplanationKey(data.status);

  return (
    <Card
      className='overflow-hidden !rounded-lg'
      bodyStyle={{ padding: 0 }}
      aria-live='polite'
    >
      <header className='flex min-w-0 flex-wrap items-center gap-2 border-b border-semi-color-border px-4 py-4 sm:px-5'>
        <span
          className='min-w-0 truncate font-mono text-xs text-semi-color-text-2'
          title={data.masked_key}
        >
          {data.masked_key}
        </span>
        <Tag color='blue' className='max-w-full break-all'>
          {groupTitle}
        </Tag>
        <Tag color='grey'>{t(groupModeKeys[data.group.mode])}</Tag>
        <Tag color={active ? 'green' : 'red'} className='ml-auto'>
          {t(statusKeys[data.status])}
        </Tag>
      </header>

      <BalanceMetrics balance={data.balance} display={data.display} t={t} />
      <BalanceProgress balance={data.balance} display={data.display} t={t} />
      <BalanceDetails data={data} t={t} />

      {statusExplanationKey ? (
        <div className='border-t border-semi-color-border p-4 sm:p-5'>
          <Banner
            type='danger'
            title={t(statusKeys[data.status])}
            description={t(statusExplanationKey)}
            closeIcon={null}
            className='!rounded-lg'
          />
        </div>
      ) : null}
    </Card>
  );
};

BalanceResult.propTypes = {
  data: PropTypes.object.isRequired,
  t: PropTypes.func.isRequired,
};

export default BalanceResult;
