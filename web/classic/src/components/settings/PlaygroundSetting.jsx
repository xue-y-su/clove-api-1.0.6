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

import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Checkbox,
  Col,
  Input,
  InputNumber,
  Row,
  Select,
  Spin,
  Switch,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  API,
  buildPlaygroundOptionUpdates,
  showError,
  showSuccess,
  toBoolean,
} from '../../helpers';

const DEFAULTS = {
  enabled: false,
  apiKeyConfigured: false,
  defaultDepth: 'basic',
  maxResults: 5,
  maxContextLength: 4000,
  nativeModels: 'gpt-4.1*,gpt-4o*,gpt-5*',
};

const OPTION_FIELDS = {
  PlaygroundTavilyEnabled: 'enabled',
  PlaygroundTavilyAPIKeyConfigured: 'apiKeyConfigured',
  PlaygroundTavilyDefaultDepth: 'defaultDepth',
  PlaygroundTavilyMaxResults: 'maxResults',
  PlaygroundTavilyMaxContextLength: 'maxContextLength',
  PlaygroundNativeWebSearchModels: 'nativeModels',
};

const toFormValues = (defaults) => ({
  ...defaults,
  apiKey: '',
  clearAPIKey: false,
});

const PlaygroundSetting = () => {
  const { t } = useTranslation();
  const [defaults, setDefaults] = useState(DEFAULTS);
  const [values, setValues] = useState(() => toFormValues(DEFAULTS));
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const loadSettings = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/option/');
      if (!response.data?.success) {
        throw new Error(response.data?.message || t('加载设置失败'));
      }
      const nextDefaults = { ...DEFAULTS };
      (response.data.data || []).forEach((option) => {
        const field = OPTION_FIELDS[option.key];
        if (!field) return;
        if (field === 'enabled' || field === 'apiKeyConfigured') {
          nextDefaults[field] = toBoolean(option.value);
          return;
        }
        if (field === 'maxResults' || field === 'maxContextLength') {
          const parsed = Number(option.value);
          if (Number.isFinite(parsed)) nextDefaults[field] = parsed;
          return;
        }
        nextDefaults[field] = option.value || '';
      });
      setDefaults(nextDefaults);
      setValues(toFormValues(nextDefaults));
    } catch (error) {
      showError(error?.message || t('加载设置失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const setValue = (key, value) => {
    setValues((current) => ({ ...current, [key]: value }));
  };

  const handleSave = async () => {
    if (values.maxResults < 1 || values.maxResults > 10) {
      showError(t('最大搜索结果数必须在 1 到 10 之间'));
      return;
    }
    if (values.maxContextLength < 500 || values.maxContextLength > 20000) {
      showError(t('最大搜索上下文长度必须在 500 到 20000 之间'));
      return;
    }

    setSaving(true);
    try {
      const updates = buildPlaygroundOptionUpdates(defaults, values);
      if (updates.length === 0) {
        showSuccess(t('没有需要保存的更改'));
        return;
      }
      for (const update of updates) {
        const response = await API.put('/api/option/', update);
        if (!response.data?.success) {
          throw new Error(response.data?.message || t('保存设置失败'));
        }
      }
      showSuccess(t('游乐场设置已保存'));
      await loadSettings();
    } catch (error) {
      showError(error?.message || t('保存设置失败'));
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className='flex min-h-48 items-center justify-center'>
        <Spin size='large' />
      </div>
    );
  }

  return (
    <Card className='mt-2' bordered={false}>
      <div className='mb-5 flex flex-wrap items-center justify-between gap-3'>
        <div>
          <Typography.Title heading={4} className='!mb-1'>
            {t('游乐场设置')}
          </Typography.Title>
        </div>
        <Button
          type='primary'
          theme='solid'
          icon={<Save size={15} />}
          loading={saving}
          onClick={() => void handleSave()}
        >
          {t('保存设置')}
        </Button>
      </div>

      <div className='mb-5 flex items-center justify-between gap-4 rounded-md border border-semi-color-border p-3'>
        <div>
          <Typography.Text strong>{t('启用 Tavily 回退搜索')}</Typography.Text>
          <Typography.Text type='tertiary' size='small' className='block'>
            {t('仅在当前路由模型不支持原生联网时使用')}
          </Typography.Text>
        </div>
        <Switch
          checked={values.enabled}
          onChange={(checked) => setValue('enabled', checked)}
        />
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12}>
          <label className='grid gap-2 text-sm font-medium'>
            <span className='flex items-center justify-between gap-2'>
              {t('Tavily API Key')}
              <Typography.Text type='tertiary' size='small'>
                {values.apiKeyConfigured ? t('已配置') : t('未配置')}
              </Typography.Text>
            </span>
            <Input
              mode='password'
              value={values.apiKey}
              maxLength={4096}
              disabled={values.clearAPIKey}
              placeholder={
                values.apiKeyConfigured
                  ? t('输入新密钥以替换现有密钥')
                  : t('输入 Tavily API Key')
              }
              onChange={(value) => setValue('apiKey', value)}
            />
            {values.apiKeyConfigured && (
              <Checkbox
                checked={values.clearAPIKey}
                onChange={(event) => {
                  const shouldClear = event.target.checked;
                  setValues((current) => ({
                    ...current,
                    apiKey: shouldClear ? '' : current.apiKey,
                    clearAPIKey: shouldClear,
                    enabled: shouldClear ? false : current.enabled,
                  }));
                }}
              >
                {t('清除已配置密钥')}
              </Checkbox>
            )}
          </label>
        </Col>

        <Col xs={24} md={12}>
          <label className='grid gap-2 text-sm font-medium'>
            {t('默认搜索深度')}
            <Select
              value={values.defaultDepth}
              onChange={(value) => setValue('defaultDepth', value)}
              optionList={[
                { label: t('基础'), value: 'basic' },
                { label: t('高级'), value: 'advanced' },
              ]}
              style={{ width: '100%' }}
            />
          </label>
        </Col>

        <Col xs={24} md={12}>
          <label className='grid gap-2 text-sm font-medium'>
            {t('最大搜索结果数')}
            <InputNumber
              value={values.maxResults}
              min={1}
              max={10}
              onChange={(value) => setValue('maxResults', Number(value))}
              style={{ width: '100%' }}
            />
          </label>
        </Col>

        <Col xs={24} md={12}>
          <label className='grid gap-2 text-sm font-medium'>
            {t('最大搜索上下文长度')}
            <InputNumber
              value={values.maxContextLength}
              min={500}
              max={20000}
              step={100}
              onChange={(value) => setValue('maxContextLength', Number(value))}
              style={{ width: '100%' }}
            />
          </label>
        </Col>

        <Col xs={24}>
          <label className='grid gap-2 text-sm font-medium'>
            {t('原生联网模型匹配')}
            <TextArea
              value={values.nativeModels}
              maxCount={20000}
              autosize={{ minRows: 2, maxRows: 6 }}
              onChange={(value) => setValue('nativeModels', value)}
            />
          </label>
        </Col>
      </Row>
    </Card>
  );
};

export default PlaygroundSetting;
