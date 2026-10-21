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
  Select,
  Space,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const sourceTypeOptions = [
  { value: 'manual', label: '手写声明' },
  { value: 'user_agreement', label: '用户协议' },
  { value: 'privacy_policy', label: '隐私政策' },
];

const confirmationModeOptions = [
  { value: 'checkbox', label: '普通勾选' },
  { value: 'phrase', label: '输入确认短语' },
  { value: 'segments', label: '分段复制或输入' },
];

const createItem = (index) => ({
  item_key: `statement_${index + 1}`,
  title: '',
  source_type: 'manual',
  version: '',
  content: '',
  sort_order: index + 1,
  confirmation_mode: 'checkbox',
  confirmation_phrase: '',
  segments: [],
});

const resequence = (items) =>
  items.map((item, index) => ({ ...item, sort_order: index + 1 }));

const segmentText = (segments) => (segments || []).join('\n');

const parseSegments = (value) =>
  String(value || '')
    .split('\n')
    .map((segment) => segment.trim())
    .filter(Boolean);

const ComplianceStatementItemsEditor = ({ items, onChange, disabled }) => {
  const { t } = useTranslation();

  const updateItem = (index, patch) => {
    onChange(
      items.map((item, itemIndex) =>
        itemIndex === index ? { ...item, ...patch } : item,
      ),
    );
  };

  const moveItem = (index, direction) => {
    const targetIndex = index + direction;
    if (targetIndex < 0 || targetIndex >= items.length) return;
    const next = [...items];
    [next[index], next[targetIndex]] = [next[targetIndex], next[index]];
    onChange(resequence(next));
  };

  return (
    <Space vertical align='start' style={{ width: '100%' }}>
      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
        <Typography.Text type='tertiary'>
          {t('可视化维护合规条款，JSON 编辑仍会保留全部高级字段。')}
        </Typography.Text>
        <Button
          icon={<Plus size={14} />}
          disabled={disabled}
          onClick={() => onChange([...items, createItem(items.length)])}
        >
          {t('新增条款')}
        </Button>
      </Space>
      {items.length === 0 ? (
        <div
          style={{
            width: '100%',
            padding: 24,
            textAlign: 'center',
            color: 'var(--semi-color-text-2)',
            border: '1px dashed var(--semi-color-border)',
            borderRadius: 8,
          }}
        >
          {t('暂无条款配置，系统会使用上方旧版单声明内容。')}
        </div>
      ) : (
        items.map((item, index) => {
          const sourceType = item.source_type || 'manual';
          const confirmationMode = item.confirmation_mode || 'checkbox';
          const sourceContentDisabled = sourceType !== 'manual';
          return (
            <div
              key={`${item.item_key || 'statement'}-${index}`}
              style={{
                width: '100%',
                border: '1px solid var(--semi-color-border)',
                borderRadius: 8,
                padding: 16,
                background: 'var(--semi-color-fill-0)',
              }}
            >
              <Space
                style={{
                  width: '100%',
                  justifyContent: 'space-between',
                  marginBottom: 12,
                }}
              >
                <Typography.Text strong>
                  {t('条款')} {index + 1}
                </Typography.Text>
                <Space>
                  <Button
                    icon={<ArrowUp size={14} />}
                    size='small'
                    disabled={disabled || index === 0}
                    onClick={() => moveItem(index, -1)}
                    aria-label={t('上移')}
                  />
                  <Button
                    icon={<ArrowDown size={14} />}
                    size='small'
                    disabled={disabled || index === items.length - 1}
                    onClick={() => moveItem(index, 1)}
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
                          items.filter((_, itemIndex) => itemIndex !== index),
                        ),
                      )
                    }
                    aria-label={t('删除')}
                  />
                </Space>
              </Space>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                  gap: 12,
                  marginBottom: 12,
                }}
              >
                <label>
                  <Typography.Text>{t('条款标识')}</Typography.Text>
                  <Input
                    value={item.item_key}
                    disabled={disabled}
                    onChange={(value) => updateItem(index, { item_key: value })}
                  />
                </label>
                <label>
                  <Typography.Text>{t('标题')}</Typography.Text>
                  <Input
                    value={item.title}
                    disabled={disabled}
                    onChange={(value) => updateItem(index, { title: value })}
                  />
                </label>
                <label>
                  <Typography.Text>{t('来源类型')}</Typography.Text>
                  <Select
                    value={sourceType}
                    optionList={sourceTypeOptions.map((option) => ({
                      ...option,
                      label: t(option.label),
                    }))}
                    disabled={disabled}
                    style={{ width: '100%' }}
                    onChange={(value) =>
                      updateItem(index, {
                        source_type: value,
                        content: value === 'manual' ? item.content : '',
                      })
                    }
                  />
                </label>
                <label>
                  <Typography.Text>{t('版本号')}</Typography.Text>
                  <Input
                    value={item.version}
                    disabled={disabled}
                    onChange={(value) => updateItem(index, { version: value })}
                  />
                </label>
                <label>
                  <Typography.Text>{t('确认方式')}</Typography.Text>
                  <Select
                    value={confirmationMode}
                    optionList={confirmationModeOptions.map((option) => ({
                      ...option,
                      label: t(option.label),
                    }))}
                    disabled={disabled}
                    style={{ width: '100%' }}
                    onChange={(value) =>
                      updateItem(index, {
                        confirmation_mode: value,
                        confirmation_phrase:
                          value === 'phrase'
                            ? item.confirmation_phrase || ''
                            : '',
                        segments:
                          value === 'segments' ? item.segments || [] : [],
                      })
                    }
                  />
                </label>
                <label>
                  <Typography.Text>{t('排序')}</Typography.Text>
                  <InputNumber
                    value={item.sort_order}
                    disabled={disabled}
                    style={{ width: '100%' }}
                    onChange={(value) =>
                      updateItem(index, { sort_order: Number(value) || 0 })
                    }
                  />
                </label>
              </div>
              <div style={{ marginBottom: 12 }}>
                <Typography.Text>{t('正文')}</Typography.Text>
                <TextArea
                  value={item.content}
                  disabled={disabled || sourceContentDisabled}
                  autosize={{ minRows: 4, maxRows: 8 }}
                  placeholder={
                    sourceContentDisabled
                      ? t(
                          '内容来自当前用户协议或隐私政策，保存时以后端快照为准。',
                        )
                      : ''
                  }
                  onChange={(value) => updateItem(index, { content: value })}
                />
              </div>
              {confirmationMode === 'phrase' && (
                <div style={{ marginBottom: 12 }}>
                  <Typography.Text>{t('确认短语')}</Typography.Text>
                  <Input
                    value={item.confirmation_phrase || ''}
                    disabled={disabled}
                    onChange={(value) =>
                      updateItem(index, { confirmation_phrase: value })
                    }
                  />
                </div>
              )}
              {confirmationMode === 'segments' && (
                <div style={{ marginBottom: 12 }}>
                  <Typography.Text>{t('分段内容')}</Typography.Text>
                  <TextArea
                    value={segmentText(item.segments)}
                    disabled={disabled}
                    autosize={{ minRows: 4, maxRows: 8 }}
                    placeholder={t('每行一个需要用户复制或输入确认的分段')}
                    onChange={(value) =>
                      updateItem(index, { segments: parseSegments(value) })
                    }
                  />
                </div>
              )}
            </div>
          );
        })
      )}
    </Space>
  );
};

export default ComplianceStatementItemsEditor;
