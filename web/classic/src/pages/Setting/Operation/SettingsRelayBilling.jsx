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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  InputNumber,
  Radio,
  RadioGroup,
  Select,
  Spin,
  Switch,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { IconRefresh, IconSave } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';
import {
  DEFAULT_RELAY_BILLING_SETTINGS,
  highPriceWarnings,
  hydrateRelayBillingSettings,
  validateRelayBillingSettings,
} from './relay-billing-settings.js';

const { Text, Title } = Typography;

function json(value) {
  return JSON.stringify(value, null, 2);
}

export default function SettingsRelayBilling() {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(DEFAULT_RELAY_BILLING_SETTINGS);
  const [saved, setSaved] = useState(DEFAULT_RELAY_BILLING_SETTINGS);
  const [raw, setRaw] = useState({
    prices: '{}',
    image_price_matrix: '{}',
    image_model_mode_overrides: '{}',
  });
  const [jsonErrors, setJsonErrors] = useState({});
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const applyLoaded = (settings) => {
    const next = hydrateRelayBillingSettings(settings);
    setDraft(next);
    setSaved(next);
    setRaw({
      prices: json(next.tool_pricing.prices),
      image_price_matrix: json(next.tool_pricing.image_price_matrix),
      image_model_mode_overrides: json(
        next.tool_pricing.image_model_mode_overrides,
      ),
    });
    setJsonErrors({});
  };

  const refresh = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/option/relay-billing-settings');
      if (!res.data?.success || !res.data?.data)
        throw new Error(res.data?.message || t('加载失败'));
      applyLoaded(res.data.data);
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  const updateTool = (patch) =>
    setDraft((current) => ({
      ...current,
      tool_pricing: { ...current.tool_pricing, ...patch },
    }));
  const updateDiagnostic = (patch) =>
    setDraft((current) => ({
      ...current,
      upstream_diagnostic: { ...current.upstream_diagnostic, ...patch },
    }));

  const updateJson = (field, value) => {
    setRaw((current) => ({ ...current, [field]: value }));
    try {
      const parsed = JSON.parse(value);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed))
        throw new Error(t('JSON 必须是对象'));
      updateTool({ [field]: parsed });
      setJsonErrors((current) => {
        const next = { ...current };
        delete next[field];
        return next;
      });
    } catch (error) {
      setJsonErrors((current) => ({
        ...current,
        [field]: error.message || t('JSON 无效'),
      }));
    }
  };

  const errors = useMemo(
    () => [...validateRelayBillingSettings(draft), ...Object.keys(jsonErrors)],
    [draft, jsonErrors],
  );
  const warnings = useMemo(() => highPriceWarnings(draft), [draft]);

  const reset = () => applyLoaded(saved);

  const save = async () => {
    if (errors.length) {
      showError(t('请先修正 Relay Billing 设置'));
      return;
    }
    if (warnings.length && !window.confirm(t('存在较高价格，仍然保存吗？')))
      return;
    setSaving(true);
    try {
      const res = await API.put('/api/option/relay-billing-settings', draft);
      if (!res.data?.success || !res.data?.data)
        throw new Error(res.data?.message || t('保存失败'));
      applyLoaded(res.data.data);
      showSuccess(t('Relay Billing 设置已保存'));
    } catch (error) {
      showError(error.message || t('保存失败'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card title={<Title heading={6}>{t('Relay Billing 与诊断')}</Title>}>
      <Spin spinning={loading}>
        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 8,
            marginBottom: 16,
          }}
        >
          <Button
            icon={<IconRefresh />}
            theme='borderless'
            onClick={reset}
            disabled={saving}
          >
            {t('重置')}
          </Button>
          <Button
            icon={<IconSave />}
            theme='solid'
            type='primary'
            loading={saving}
            disabled={errors.length > 0}
            onClick={save}
          >
            {t('保存')}
          </Button>
        </div>
        {warnings.length > 0 && (
          <Banner
            type='warning'
            description={t('检测到高于确认阈值的价格，保存时会再次确认。')}
            style={{ marginBottom: 16 }}
          />
        )}
        <div style={{ display: 'grid', gap: 16 }}>
          <div style={{ display: 'grid', gap: 12 }}>
            <Switch
              checked={draft.alpha_search.enabled}
              onChange={(enabled) =>
                setDraft((current) => ({
                  ...current,
                  alpha_search: { enabled },
                }))
              }
            >
              {t('启用 Alpha Search')}
            </Switch>
            <Switch
              checked={draft.tool_pricing.enabled}
              onChange={(enabled) => updateTool({ enabled })}
            >
              {t('启用工具计价')}
            </Switch>
            <div style={{ display: 'grid', gap: 8 }}>
              <Text>{t('工具倍率模式')}</Text>
              <RadioGroup
                type='button'
                value={draft.tool_pricing.multiplier_mode}
                onChange={(event) =>
                  updateTool({ multiplier_mode: event.target.value })
                }
              >
                <Radio value='group_only'>{t('仅分组倍率')}</Radio>
                <Radio value='full'>{t('完整倍率')}</Radio>
              </RadioGroup>
            </div>
            <div style={{ display: 'grid', gap: 8 }}>
              <Text>{t('图片计价模式')}</Text>
              <RadioGroup
                type='button'
                value={draft.tool_pricing.image_pricing_mode}
                onChange={(event) =>
                  updateTool({ image_pricing_mode: event.target.value })
                }
              >
                <Radio value='matrix'>{t('质量/尺寸矩阵')}</Radio>
                <Radio value='flat'>{t('统一按次')}</Radio>
              </RadioGroup>
            </div>
          </div>
          {[
            ['prices', t('工具价格 JSON')],
            ['image_price_matrix', t('图片价格矩阵 JSON')],
            ['image_model_mode_overrides', t('图片模型模式覆盖 JSON')],
          ].map(([field, label]) => (
            <div key={field} style={{ display: 'grid', gap: 6 }}>
              <Text strong>{label}</Text>
              <TextArea
                value={raw[field]}
                onChange={(value) => updateJson(field, value)}
                autosize={{ minRows: 4, maxRows: 12 }}
                style={{ fontFamily: 'monospace', fontSize: 12 }}
              />
              {jsonErrors[field] && (
                <Text type='danger'>{jsonErrors[field]}</Text>
              )}
            </div>
          ))}
          <div style={{ display: 'grid', gap: 12 }}>
            <Switch
              checked={draft.upstream_diagnostic.enabled}
              onChange={(enabled) => updateDiagnostic({ enabled })}
            >
              {t('记录上游诊断')}
            </Switch>
            <div style={{ display: 'grid', gap: 8 }}>
              <Text>{t('捕获模式')}</Text>
              <Select
                value={draft.upstream_diagnostic.capture_mode}
                onChange={(capture_mode) => updateDiagnostic({ capture_mode })}
                optionList={[
                  { label: t('仅回退'), value: 'fallback_only' },
                  { label: t('全部错误'), value: 'all_errors' },
                ]}
                style={{ width: '100%' }}
              />
            </div>
            <div
              style={{
                display: 'grid',
                gap: 8,
                gridTemplateColumns: 'repeat(auto-fit,minmax(180px,1fr))',
              }}
            >
              <label>
                {t('预览字符数')}
                <InputNumber
                  min={256}
                  max={8192}
                  value={draft.upstream_diagnostic.preview_chars}
                  onChange={(preview_chars) =>
                    updateDiagnostic({ preview_chars })
                  }
                  style={{ width: '100%' }}
                />
              </label>
              <label>
                {t('去重窗口（秒）')}
                <InputNumber
                  min={0}
                  max={3600}
                  value={draft.upstream_diagnostic.dedupe_window_seconds}
                  onChange={(dedupe_window_seconds) =>
                    updateDiagnostic({ dedupe_window_seconds })
                  }
                  style={{ width: '100%' }}
                />
              </label>
            </div>
            <Switch
              checked={draft.upstream_diagnostic.sync_to_usage_log}
              onChange={(sync_to_usage_log) =>
                updateDiagnostic({ sync_to_usage_log })
              }
            >
              {t('同步到失败使用日志')}
            </Switch>
          </div>
        </div>
      </Spin>
    </Card>
  );
}
