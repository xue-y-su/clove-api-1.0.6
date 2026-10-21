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
import {
  Button,
  Card,
  Input,
  Space,
  Table,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, timestamp2string } from '../../helpers';

const pageSize = 20;

const ComplianceConfirmations = ({ cardTitle }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [confirmations, setConfirmations] = useState([]);
  const [filters, setFilters] = useState({ username: '', version: '', ip: '' });
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);

  const loadConfirmations = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/compliance/confirmations', {
        params: {
          p: page,
          page_size: pageSize,
          username: filters.username || undefined,
          version: filters.version || undefined,
          ip: filters.ip || undefined,
        },
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setConfirmations(data?.items || []);
      setTotal(data?.total || 0);
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const updateFilter = (key, value) => {
    setPage(1);
    setFilters((current) => ({ ...current, [key]: value }));
  };

  useEffect(() => {
    loadConfirmations();
  }, [filters.username, filters.version, filters.ip, page]);

  const columns = [
    { title: t('用户'), dataIndex: 'username' },
    { title: t('版本'), dataIndex: 'version' },
    { title: 'IP', dataIndex: 'ip' },
    {
      title: t('确认时间'),
      dataIndex: 'confirmed_at',
      render: (value) => (value ? timestamp2string(value) : '-'),
    },
    {
      title: t('用户代理'),
      dataIndex: 'user_agent',
      render: (value) => (
        <Typography.Text
          ellipsis={{ showTooltip: true }}
          style={{ maxWidth: 320 }}
        >
          {value || '-'}
        </Typography.Text>
      ),
    },
  ];

  return (
    <Card title={cardTitle || t('合规确认记录')} style={{ width: '100%' }}>
      <Space style={{ marginBottom: 12 }} wrap>
        <Input
          placeholder={t('用户')}
          value={filters.username}
          onChange={(value) => updateFilter('username', value)}
        />
        <Input
          placeholder={t('版本')}
          value={filters.version}
          onChange={(value) => updateFilter('version', value)}
        />
        <Input
          placeholder='IP'
          value={filters.ip}
          onChange={(value) => updateFilter('ip', value)}
        />
        <Button onClick={loadConfirmations} loading={loading}>
          {t('刷新')}
        </Button>
      </Space>
      <Table
        rowKey='id'
        size='small'
        columns={columns}
        dataSource={confirmations}
        loading={loading}
        pagination={{
          currentPage: page,
          pageSize,
          total,
          onPageChange: setPage,
        }}
      />
    </Card>
  );
};

export default ComplianceConfirmations;
