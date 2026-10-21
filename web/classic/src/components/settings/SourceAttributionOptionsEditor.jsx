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
import {
  Button,
  Input,
  InputNumber,
  Space,
  Switch,
  Table,
  Typography,
} from '@douyinfe/semi-ui';
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const createOption = (index) => ({
  id: `option_${index + 1}`,
  name: '',
  target_user_id: 0,
  requires_description: false,
  sort_order: index + 1,
});

const resequence = (options) =>
  options.map((option, index) => ({ ...option, sort_order: index + 1 }));

const SourceAttributionOptionsEditor = ({ options, onChange, disabled }) => {
  const { t } = useTranslation();

  const updateOption = (index, patch) => {
    onChange(
      options.map((option, optionIndex) =>
        optionIndex === index ? { ...option, ...patch } : option,
      ),
    );
  };

  const moveOption = (index, direction) => {
    const targetIndex = index + direction;
    if (targetIndex < 0 || targetIndex >= options.length) return;
    const next = [...options];
    [next[index], next[targetIndex]] = [next[targetIndex], next[index]];
    onChange(resequence(next));
  };

  const columns = [
    {
      title: t('选项 ID'),
      dataIndex: 'id',
      render: (_, record, index) => (
        <Input
          value={record.id}
          disabled={disabled}
          onChange={(value) => updateOption(index, { id: value })}
        />
      ),
    },
    {
      title: t('显示名称'),
      dataIndex: 'name',
      render: (_, record, index) => (
        <Input
          value={record.name}
          disabled={disabled}
          onChange={(value) => updateOption(index, { name: value })}
        />
      ),
    },
    {
      title: t('目标用户 ID'),
      dataIndex: 'target_user_id',
      render: (_, record, index) => (
        <InputNumber
          min={1}
          value={record.target_user_id}
          disabled={disabled}
          onChange={(value) =>
            updateOption(index, { target_user_id: Number(value) || 0 })
          }
        />
      ),
    },
    {
      title: t('要求说明'),
      dataIndex: 'requires_description',
      width: 100,
      render: (_, record, index) => (
        <Switch
          checked={!!record.requires_description}
          disabled={disabled}
          onChange={(checked) =>
            updateOption(index, { requires_description: !!checked })
          }
        />
      ),
    },
    {
      title: t('排序'),
      dataIndex: 'sort_order',
      width: 110,
      render: (_, record, index) => (
        <InputNumber
          value={record.sort_order}
          disabled={disabled}
          onChange={(value) =>
            updateOption(index, { sort_order: Number(value) || 0 })
          }
        />
      ),
    },
    {
      title: t('操作'),
      width: 140,
      render: (_, record, index) => (
        <Space>
          <Button
            icon={<ArrowUp size={14} />}
            size='small'
            disabled={disabled || index === 0}
            onClick={() => moveOption(index, -1)}
            aria-label={t('上移')}
          />
          <Button
            icon={<ArrowDown size={14} />}
            size='small'
            disabled={disabled || index === options.length - 1}
            onClick={() => moveOption(index, 1)}
            aria-label={t('下移')}
          />
          <Button
            icon={<Trash2 size={14} />}
            size='small'
            type='danger'
            disabled={disabled}
            onClick={() =>
              onChange(
                resequence(
                  options.filter((_, optionIndex) => optionIndex !== index),
                ),
              )
            }
            aria-label={t('删除')}
          />
        </Space>
      ),
    },
  ];

  return (
    <Space vertical align='start' style={{ width: '100%' }}>
      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
        <Typography.Text type='tertiary'>
          {t('可视化维护来源归属选项，保存时仍会写入专用接口。')}
        </Typography.Text>
        <Button
          icon={<Plus size={14} />}
          disabled={disabled}
          onClick={() => onChange([...options, createOption(options.length)])}
        >
          {t('新增选项')}
        </Button>
      </Space>
      <Table
        rowKey={(record, index) => `${record.id || 'option'}-${index}`}
        columns={columns}
        dataSource={options}
        pagination={false}
        empty={t('暂无来源归属选项')}
      />
    </Space>
  );
};

export default SourceAttributionOptionsEditor;
