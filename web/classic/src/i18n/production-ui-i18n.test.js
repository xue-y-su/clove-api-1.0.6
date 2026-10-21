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

import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

function readSource(relativePath) {
  return readFileSync(new URL(relativePath, import.meta.url), 'utf8');
}

const requiredKeys = [
  '温度',
  '核采样阈值',
  '频率惩罚',
  '存在惩罚',
  '最大令牌数',
  '随机种子',
  '预览',
  '请求',
  '安全检查',
  '计量功能',
  '绘图进度',
  '任务进度',
  '额度使用',
  'HTML预览',
  '令牌框架',
];

const userRedemptionKeys = [
  'My Redemption Codes',
  'Redeem',
  'Create Code',
  'Code',
  'Kind',
  'Status',
  'Quota',
  'Claims',
  'Expires',
  'Actions',
  'Copy',
  'Cancel',
  'No codes yet.',
  'load failed',
  'cancel failed',
  'Created',
  'create failed',
  'Create',
  'Single',
  'Random packet',
  'Amount',
  'Max claims',
  'Expires in hours',
  'Redeemed',
  'claim failed',
  'Redeem Code',
  'Enter redemption code',
  'Active',
  'Disabled',
  'Completed',
  'Cancelled',
  'Forfeited',
  'Expired',
];

const classicEnglishUiKeys = [
  'Logo',
  'Provider icon',
  'Happy New Year!!! 🎉',
  'New Year',
  'Creem Official Site',
  'failed to load templates',
  'Email Templates',
  'Manage built-in and custom verification email templates.',
  'Default',
  'preview failed',
  'Preview',
  'Default updated',
  'set default failed',
  'Set Default',
  'Create custom template',
  'Name',
  'Subject',
  'Default verification',
  'Built-in',
  'Custom',
];

const localizedChineseTechnicalKeys = [
  'Configuration',
  'Discovery scopes',
  'Discovery claims',
  'Authorization Endpoint',
  'Token Endpoint',
  'User Info Endpoint',
  'OAuth Client ID',
  'OAuth Client Secret',
  'Tavily API Key',
  'Prompt tokens',
  'Cached tokens',
  'Prompt cache hit tokens',
  'Completion tokens',
  'Total tokens',
  'Reasoning Effort',
];

describe('classic production UI translations', () => {
  test('translates playground controls and debug labels', () => {
    const parameters = readSource(
      '../components/playground/ParameterControl.jsx',
    );
    const debugPanel = readSource('../components/playground/DebugPanel.jsx');

    for (const key of requiredKeys.slice(0, 6)) {
      expect(parameters).toContain(`t('${key}')`);
    }
    expect(debugPanel).toContain("title={t('预览')}");
    expect(debugPanel).toContain("title={t('请求')}");
    expect(debugPanel).toContain("title={t('响应')}");
  });

  test('translates shared titles, progress labels, and usage metadata', () => {
    const markdown = readSource(
      '../components/common/markdown/MarkdownRenderer.jsx',
    );
    const checkin = readSource(
      '../components/settings/personal/cards/CheckinCalendar.jsx',
    );
    const chat = readSource('../pages/Chat/index.jsx');
    const codexUsage = readSource(
      '../components/table/channels/modals/CodexUsageModal.jsx',
    );
    const drawingLogs = readSource(
      '../components/table/mj-logs/MjLogsColumnDefs.jsx',
    );
    const taskLogs = readSource(
      '../components/table/task-logs/TaskLogsColumnDefs.jsx',
    );
    const tokens = readSource(
      '../components/table/tokens/TokensColumnDefs.jsx',
    );
    const users = readSource('../components/table/users/UsersColumnDefs.jsx');

    expect(markdown).toContain("title={t('HTML预览')}");
    expect(checkin).toContain("title={t('安全检查')}");
    expect(chat).toContain("title={t('令牌框架')}");
    expect(codexUsage).toContain("{tt('计量功能')}");
    expect(drawingLogs).toContain("aria-label={t('绘图进度')}");
    expect(taskLogs).toContain("aria-label={t('任务进度')}");
    expect(tokens).toContain("alt={t('未知')}");
    expect(tokens).toContain("aria-label={t('额度使用')}");
    expect(users).toContain("aria-label={t('额度使用')}");
  });

  test('ships the required labels in every classic locale', () => {
    for (const locale of [
      'en',
      'fr',
      'ja',
      'ru',
      'vi',
      'zh',
      'zh-CN',
      'zh-TW',
    ]) {
      const data = JSON.parse(readSource(`./locales/${locale}.json`));
      const missing = requiredKeys.filter(
        (key) => !data.translation?.[key]?.trim(),
      );
      expect({ locale, missing }).toEqual({ locale, missing: [] });
    }
  });

  test('ships the complete user redemption page in every classic locale', () => {
    for (const locale of ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW']) {
      const data = JSON.parse(readSource(`./locales/${locale}.json`));
      const missing = userRedemptionKeys.filter(
        (key) => !data.translation?.[key]?.trim(),
      );
      expect({ locale, missing }).toEqual({ locale, missing: [] });
    }
  });

  test('ships remaining English-key controls in every classic locale', () => {
    const emailTemplates = readSource(
      '../pages/Setting/Operation/SettingsEmailTemplates.jsx',
    );
    expect(emailTemplates).toContain('getEmailTemplateName(item, t)');
    expect(emailTemplates).toContain('EMAIL_TEMPLATE_KIND_LABELS[item.kind]');

    for (const locale of ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW']) {
      const data = JSON.parse(readSource(`./locales/${locale}.json`));
      const missing = classicEnglishUiKeys.filter(
        (key) => !data.translation?.[key]?.trim(),
      );
      expect({ locale, missing }).toEqual({ locale, missing: [] });
    }
  });

  test('localizes technical labels that otherwise read as English-only', () => {
    for (const locale of ['zh-CN', 'zh-TW']) {
      const data = JSON.parse(readSource(`./locales/${locale}.json`));
      const untranslated = localizedChineseTechnicalKeys.filter((key) => {
        const value = data.translation?.[key]?.trim();
        return !value || value === key;
      });
      expect({ locale, untranslated }).toEqual({ locale, untranslated: [] });
    }
  });
});
