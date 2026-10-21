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
import { Button, Card, Form, Space, Tabs } from '@douyinfe/semi-ui';
import { ShieldCheck } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import ComplianceConfirmations from '../compliance/ComplianceConfirmations';
import ComplianceStatementItemsEditor from './ComplianceStatementItemsEditor';

const emptySettings = {
  statement_enabled: false,
  statement_version: '',
  statement_content: '',
  statement_items: [],
  statement_display_mode: 'single',
  token_api_enforcement_enabled: false,
  show_version_to_user: false,
  show_content_hash_to_user: false,
  content_hash: '',
};

const ComplianceSetting = () => {
  const { t } = useTranslation();
  const [settings, setSettings] = useState(emptySettings);
  const [editMode, setEditMode] = useState('visual');
  const [statementItems, setStatementItems] = useState([]);
  const [statementItemsJson, setStatementItemsJson] = useState('[]');
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(false);

  const loadSettings = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/compliance/settings');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const nextSettings = { ...emptySettings, ...(data || {}) };
      setSettings(nextSettings);
      setStatementItems(nextSettings.statement_items || []);
      setStatementItemsJson(
        JSON.stringify(nextSettings.statement_items || [], null, 2),
      );
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const updateField = (key, value) => {
    setSettings((current) => ({ ...current, [key]: value }));
  };

  const parseStatementItems = () => {
    try {
      const parsed = JSON.parse(statementItemsJson || '[]');
      if (!Array.isArray(parsed)) {
        showError(t('条款配置必须是 JSON 数组'));
        return null;
      }
      return parsed;
    } catch (error) {
      showError(t('条款配置必须是有效 JSON'));
      return null;
    }
  };

  const handleVisualItemsChange = (items) => {
    setStatementItems(items);
    setStatementItemsJson(JSON.stringify(items, null, 2));
  };

  const handleItemsJsonChange = (value) => {
    setStatementItemsJson(value);
    try {
      const parsed = JSON.parse(value || '[]');
      if (Array.isArray(parsed)) {
        setStatementItems(parsed);
      }
    } catch (error) {
      // 保留上一次有效的可视化数据，保存时再拦截非法 JSON。
    }
  };

  const saveSettings = async () => {
    const statementItems = parseStatementItems();
    if (statementItems === null) {
      return;
    }
    if (settings.statement_enabled && statementItems.length === 0) {
      if (!settings.statement_version.trim()) {
        showError(t('启用合规声明时版本不能为空'));
        return;
      }
      if (!settings.statement_content.trim()) {
        showError(t('启用合规声明时内容不能为空'));
        return;
      }
    }

    setSaving(true);
    try {
      const res = await API.put('/api/compliance/settings', {
        ...settings,
        statement_version: settings.statement_version.trim(),
        statement_items: statementItems,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const nextSettings = data || settings;
      setSettings(nextSettings);
      setStatementItems(nextSettings.statement_items || statementItems);
      setStatementItemsJson(
        JSON.stringify(nextSettings.statement_items || statementItems, null, 2),
      );
      showSuccess(t('合规设置已保存'));
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    loadSettings();
  }, []);

  return (
    <Space vertical align='start' style={{ width: '100%' }}>
      <Card
        title={
          <Space>
            <ShieldCheck size={18} />
            {t('合规声明')}
          </Space>
        }
        style={{ width: '100%' }}
        loading={loading}
      >
        <Form labelPosition='left' labelWidth={180}>
          <Form.Switch
            field='statement_enabled'
            label={t('启用合规声明')}
            checked={settings.statement_enabled}
            onChange={(value) => updateField('statement_enabled', value)}
          />
          <Form.Switch
            field='token_api_enforcement_enabled'
            label={t('Token API 强制确认')}
            checked={settings.token_api_enforcement_enabled}
            onChange={(value) =>
              updateField('token_api_enforcement_enabled', value)
            }
          />
          <Form.Switch
            field='show_version_to_user'
            label={t('向用户展示版本号')}
            checked={settings.show_version_to_user}
            onChange={(value) => updateField('show_version_to_user', value)}
          />
          <Form.Switch
            field='show_content_hash_to_user'
            label={t('向用户展示内容哈希')}
            checked={settings.show_content_hash_to_user}
            onChange={(value) =>
              updateField('show_content_hash_to_user', value)
            }
          />
          <Form.Input
            field='statement_version'
            label={t('版本')}
            value={settings.statement_version}
            onChange={(value) => updateField('statement_version', value)}
          />
          <Form.Select
            field='statement_display_mode'
            label={t('显示模式')}
            value={settings.statement_display_mode || 'single'}
            optionList={[
              { label: t('单弹窗'), value: 'single' },
              { label: t('多弹窗'), value: 'multiple' },
            ]}
            onChange={(value) => updateField('statement_display_mode', value)}
          />
          <Form.Input
            field='content_hash'
            label={t('内容哈希')}
            value={settings.content_hash}
            readonly
          />
          <Form.TextArea
            field='statement_content'
            label={t('声明内容')}
            value={settings.statement_content}
            autosize={{ minRows: 8, maxRows: 16 }}
            onChange={(value) => updateField('statement_content', value)}
          />
          <Tabs activeKey={editMode} onChange={setEditMode} type='button'>
            <Tabs.TabPane tab={t('可视化编辑')} itemKey='visual'>
              <ComplianceStatementItemsEditor
                items={statementItems}
                onChange={handleVisualItemsChange}
                disabled={loading}
              />
            </Tabs.TabPane>
            <Tabs.TabPane tab={t('JSON 编辑')} itemKey='json'>
              <Form.TextArea
                field='statement_items'
                label={t('条款配置 JSON')}
                value={statementItemsJson}
                autosize={{ minRows: 10, maxRows: 18 }}
                style={{ fontFamily: 'JetBrains Mono, Consolas' }}
                onChange={handleItemsJsonChange}
                helpText={t(
                  '使用数组配置条款，字段包含 item_key、title、source_type、version、content、sort_order、confirmation_mode、confirmation_phrase 和 segments。JSON 解析成功后会同步到可视化编辑。',
                )}
              />
            </Tabs.TabPane>
          </Tabs>
          <Button
            type='primary'
            theme='solid'
            loading={saving}
            onClick={saveSettings}
          >
            {t('保存合规设置')}
          </Button>
        </Form>
      </Card>

      <ComplianceConfirmations />
    </Space>
  );
};

export default ComplianceSetting;
