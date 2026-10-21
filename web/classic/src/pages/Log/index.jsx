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
import { TabPane, Tabs } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import UsageLogsTable from '../../components/table/usage-logs';
import IPAuditPanel from '../../components/table/usage-logs/IPAuditPanel';
import {
  normalizeLogTab,
  readUsageLogDeepLink,
} from '../../components/table/usage-logs/ip-audit-state';
import { isAdmin } from '../../helpers';

const Log = () => {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const admin = isAdmin();
  const activeTab = normalizeLogTab(searchParams.get('tab'), admin);
  const usageLogDeepLink = readUsageLogDeepLink(searchParams);

  const handleTabChange = (tab) => {
    const next = new URLSearchParams(searchParams);
    if (tab === 'ip-audit' && admin) {
      next.set('tab', 'ip-audit');
    } else {
      next.delete('tab');
      for (const key of [
        'range',
        'start_timestamp',
        'end_timestamp',
        'p',
        'page_size',
      ]) {
        next.delete(key);
      }
    }
    setSearchParams(next, { replace: true });
  };

  return (
    <div className='mt-[60px] px-2'>
      <Tabs type='line' activeKey={activeTab} onChange={handleTabChange}>
        <TabPane tab={t('使用日志')} itemKey='usage'>
          {activeTab === 'usage' ? (
            <UsageLogsTable initialQuery={usageLogDeepLink} />
          ) : null}
        </TabPane>
        {admin ? (
          <TabPane tab={t('IP 审计')} itemKey='ip-audit'>
            {activeTab === 'ip-audit' ? (
              <IPAuditPanel
                searchParams={searchParams}
                setSearchParams={setSearchParams}
                t={t}
              />
            ) : null}
          </TabPane>
        ) : null}
      </Tabs>
    </div>
  );
};

export default Log;
