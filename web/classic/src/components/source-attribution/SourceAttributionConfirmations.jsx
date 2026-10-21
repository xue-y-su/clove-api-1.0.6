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
  InputNumber,
  Space,
  Table,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, timestamp2string } from '../../helpers';

const pageSize = 20;

const SourceAttributionConfirmations = ({ cardTitle }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [filters, setFilters] = useState({
    username: '',
    option_id: '',
    target_user_id: '',
    ip: '',
  });
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);

  const loadConfirmations = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/source-attribution/confirmations', {
        params: {
          p: page,
          page_size: pageSize,
          username: filters.username || undefined,
          option_id: filters.option_id || undefined,
          target_user_id: Number(filters.target_user_id) || undefined,
          ip: filters.ip || undefined,
        },
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setItems(data?.items || []);
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
  }, [
    filters.username,
    filters.option_id,
    filters.target_user_id,
    filters.ip,
    page,
  ]);

  const columns = [
    { title: t('用户'), dataIndex: 'username' },
    {
      title: t('归属选项'),
      dataIndex: 'option_name',
      render: (value, record) => value || record.option_id || '-',
    },
    {
      title: t('邀请人'),
      dataIndex: 'target_username',
      render: (value) => value || '-',
    },
    {
      title: t('目标用户 ID'),
      dataIndex: 'target_user_id',
      render: (value) => value || '-',
    },
    {
      title: t('来源说明'),
      dataIndex: 'source_description',
      render: (value) => (
        <Typography.Text
          ellipsis={{ showTooltip: true }}
          style={{ maxWidth: 260 }}
        >
          {value || '-'}
        </Typography.Text>
      ),
    },
    { title: 'IP', dataIndex: 'ip', render: (value) => value || '-' },
    {
      title: t('确认时间'),
      dataIndex: 'confirmed_at',
      render: (value) => (value ? timestamp2string(value) : '-'),
    },
  ];

  return (
    <Card title={cardTitle || t('来源归属确认记录')} style={{ width: '100%' }}>
      <Space style={{ marginBottom: 12 }} wrap>
        <Input
          placeholder={t('用户')}
          value={filters.username}
          onChange={(value) => updateFilter('username', value)}
        />
        <Input
          placeholder={t('选项 ID')}
          value={filters.option_id}
          onChange={(value) => updateFilter('option_id', value)}
        />
        <InputNumber
          min={1}
          placeholder={t('目标用户 ID')}
          value={filters.target_user_id || undefined}
          onChange={(value) =>
            updateFilter('target_user_id', Number(value) || '')
          }
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
        dataSource={items}
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

export default SourceAttributionConfirmations;
