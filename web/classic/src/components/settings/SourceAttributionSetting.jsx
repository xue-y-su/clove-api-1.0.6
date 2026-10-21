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
import { Button, Card, Form, Space, Switch, Tabs } from '@douyinfe/semi-ui';
import { GitBranch } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import SourceAttributionOptionsEditor from './SourceAttributionOptionsEditor';

const SourceAttributionSetting = () => {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState(false);
  const [tokenApiEnforcementEnabled, setTokenApiEnforcementEnabled] =
    useState(false);
  const [editMode, setEditMode] = useState('visual');
  const [options, setOptions] = useState([]);
  const [optionsJson, setOptionsJson] = useState('[]');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadSettings = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/source-attribution/settings');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setEnabled(Boolean(data?.enabled));
      setTokenApiEnforcementEnabled(
        Boolean(data?.token_api_enforcement_enabled),
      );
      const nextOptions = data?.options || [];
      setOptions(nextOptions);
      setOptionsJson(JSON.stringify(nextOptions, null, 2));
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const parseOptions = () => {
    try {
      const parsed = JSON.parse(optionsJson || '[]');
      if (!Array.isArray(parsed)) {
        showError(t('选项配置必须是 JSON 数组'));
        return null;
      }
      return parsed;
    } catch (error) {
      showError(t('选项配置必须是有效 JSON'));
      return null;
    }
  };

  const handleVisualOptionsChange = (nextOptions) => {
    setOptions(nextOptions);
    setOptionsJson(JSON.stringify(nextOptions, null, 2));
  };

  const handleJsonChange = (value) => {
    setOptionsJson(value);
    try {
      const parsed = JSON.parse(value || '[]');
      if (Array.isArray(parsed)) {
        setOptions(parsed);
      }
    } catch (error) {
      // 保留上一次有效的可视化数据，保存时再拦截非法 JSON。
    }
  };

  const saveSettings = async () => {
    const options = parseOptions();
    if (!options) return;
    setSaving(true);
    try {
      const res = await API.put('/api/source-attribution/settings', {
        enabled,
        token_api_enforcement_enabled: tokenApiEnforcementEnabled,
        options,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const nextOptions = data?.options || options;
      setOptions(nextOptions);
      setOptionsJson(JSON.stringify(nextOptions, null, 2));
      showSuccess(t('来源归属设置已保存'));
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
    <Card
      title={
        <Space>
          <GitBranch size={18} />
          {t('来源归属')}
        </Space>
      }
      loading={loading}
    >
      <Form labelPosition='left' labelWidth={220}>
        <Form.Slot label={t('启用来源归属')}>
          <Switch
            aria-label={t('启用来源归属')}
            checked={enabled}
            onChange={setEnabled}
          />
        </Form.Slot>
        <Form.Slot label={t('Token API 强制来源归属')}>
          <Switch
            aria-label={t('Token API 强制来源归属')}
            checked={tokenApiEnforcementEnabled}
            onChange={setTokenApiEnforcementEnabled}
          />
        </Form.Slot>
        <Tabs activeKey={editMode} onChange={setEditMode} type='button'>
          <Tabs.TabPane tab={t('可视化编辑')} itemKey='visual'>
            <SourceAttributionOptionsEditor
              options={options}
              onChange={handleVisualOptionsChange}
              disabled={loading}
            />
          </Tabs.TabPane>
          <Tabs.TabPane tab={t('JSON 编辑')} itemKey='json'>
            <Form.TextArea
              field='options'
              label={t('归属选项 JSON')}
              value={optionsJson}
              autosize={{ minRows: 10, maxRows: 18 }}
              style={{ fontFamily: 'JetBrains Mono, Consolas' }}
              onChange={handleJsonChange}
              helpText={t(
                '每个选项包含：选项 ID、名称、目标用户 ID、是否要求填写来源说明和排序值。JSON 解析成功后会同步到可视化编辑。',
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
          {t('保存来源归属设置')}
        </Button>
      </Form>
    </Card>
  );
};

export default SourceAttributionSetting;
