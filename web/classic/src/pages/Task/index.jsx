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
import { Tabs, TabPane } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import ComplianceConfirmations from '../../components/compliance/ComplianceConfirmations';
import SourceAttributionConfirmations from '../../components/source-attribution/SourceAttributionConfirmations';
import TaskLogsTable from '../../components/table/task-logs';
import { isAdmin } from '../../helpers';

const normalizeTaskTab = (tab) => {
  if (isAdmin() && (tab === 'compliance' || tab === 'source-attribution')) {
    return tab;
  }
  return 'task';
};

const Task = () => {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = normalizeTaskTab(searchParams.get('tab'));

  const handleTabChange = (tab) => {
    const next = new URLSearchParams(searchParams);
    if (tab === 'task') {
      next.delete('tab');
    } else {
      next.set('tab', tab);
    }
    setSearchParams(next, { replace: true });
  };

  return (
    <div className='mt-[60px] px-2'>
      <Tabs type='line' activeKey={activeTab} onChange={handleTabChange}>
        <TabPane tab={t('任务日志')} itemKey='task'>
          <TaskLogsTable />
        </TabPane>
        {isAdmin() && (
          <TabPane tab={t('Compliance Confirmations')} itemKey='compliance'>
            <ComplianceConfirmations
              cardTitle={t('Compliance Confirmations')}
            />
          </TabPane>
        )}
        {isAdmin() && (
          <TabPane
            tab={t('Source Attribution Confirmations')}
            itemKey='source-attribution'
          >
            <SourceAttributionConfirmations
              cardTitle={t('Source Attribution Confirmations')}
            />
          </TabPane>
        )}
      </Tabs>
    </div>
  );
};

export default Task;
