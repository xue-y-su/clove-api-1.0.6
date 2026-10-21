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

import React from 'react';
import { DatePicker, RadioGroup } from '@douyinfe/semi-ui';

const THIRTY_DAYS_IN_SECONDS = 30 * 24 * 60 * 60;

const rangeOptions = [
  { value: '7d', labelKey: '近 7 天' },
  { value: '30d', labelKey: '近 30 天' },
  { value: '90d', labelKey: '近 90 天' },
  { value: 'all', labelKey: '全部历史' },
  { value: 'custom', labelKey: '自定义' },
];

const viewOptions = [
  { value: 'risk', labelKey: '风险优先' },
  { value: 'shared_api', labelKey: '共享 API IP' },
  { value: 'shared_registration', labelKey: '共享注册 IP' },
  { value: 'authentication', labelKey: '认证活动' },
  { value: 'all', labelKey: '全部 IP 活动' },
];

const toUnixTimestamp = (value) => {
  const timestamp = value instanceof Date ? value.getTime() : Date.parse(value);
  return Number.isFinite(timestamp) ? Math.floor(timestamp / 1000) : 0;
};

const IPAuditRangeControls = ({ query, onChange, disabled = false, t }) => {
  const handleViewChange = (event) => {
    onChange({ view: event.target.value });
  };

  const handleRangeChange = (event) => {
    const range = event.target.value;
    if (range !== 'custom') {
      onChange({ range, startTimestamp: 0, endTimestamp: 0 });
      return;
    }
    const endTimestamp = Math.floor(Date.now() / 1000);
    onChange({
      range,
      startTimestamp: endTimestamp - THIRTY_DAYS_IN_SECONDS,
      endTimestamp,
    });
  };

  const handleCustomRangeChange = (value) => {
    if (!Array.isArray(value) || value.length !== 2) {
      return;
    }
    const startTimestamp = toUnixTimestamp(value[0]);
    const endTimestamp = toUnixTimestamp(value[1]);
    if (!startTimestamp || !endTimestamp || startTimestamp > endTimestamp) {
      return;
    }
    onChange({ range: 'custom', startTimestamp, endTimestamp });
  };

  return (
    <div className='flex min-w-0 flex-col gap-2'>
      <RadioGroup
        type='button'
        buttonSize='small'
        value={query.view}
        disabled={disabled}
        options={viewOptions.map((option) => ({
          value: option.value,
          label: t(option.labelKey),
        }))}
        onChange={handleViewChange}
        aria-label={t('IP 审计视图')}
        className='max-w-full overflow-x-auto'
      />
      <div className='flex min-w-0 flex-col gap-2 md:flex-row md:items-center md:justify-between'>
        <RadioGroup
          type='button'
          buttonSize='small'
          value={query.range}
          disabled={disabled}
          options={rangeOptions.map((option) => ({
            value: option.value,
            label: t(option.labelKey),
          }))}
          onChange={handleRangeChange}
          aria-label={t('审计时间范围')}
          className='max-w-full overflow-x-auto'
        />
        {query.range === 'custom' ? (
          <DatePicker
            type='dateTimeRange'
            value={[
              new Date(query.startTimestamp * 1000),
              new Date(query.endTimestamp * 1000),
            ]}
            onChange={handleCustomRangeChange}
            disabled={disabled}
            showClear={false}
            needConfirm
            size='small'
            className='w-full md:w-auto'
            aria-label={t('审计时间范围')}
            placeholder={[t('开始时间'), t('结束时间')]}
          />
        ) : null}
      </div>
    </div>
  );
};

export default IPAuditRangeControls;
