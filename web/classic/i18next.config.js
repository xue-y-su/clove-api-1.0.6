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

// Keep protocol literals, product names, and literal templates out of the
// translation catalog without weakening lint checks for any other text.
const exactLintAllowlist = new Map([
  [
    'src/helpers/render.jsx',
    new Set([
      'interpolation:Interpolation parameter "ratio" was not provided',
      'interpolation:Interpolation parameter "cacheRatio" was not provided',
      'interpolation:Interpolation parameter "cacheCreationRatio5m" was not provided',
      'interpolation:Interpolation parameter "cacheCreationRatio1h" was not provided',
      'interpolation:Interpolation parameter "cacheCreationRatio" was not provided',
      'interpolation:Interpolation parameter "imageRatio" was not provided',
    ]),
  ],
  [
    'src/pages/About/index.jsx',
    new Set([
      'hardcoded:NewAPI',
      'hardcoded:QuantumNous',
      'hardcoded:One API v0.5.4',
      'hardcoded:© 2023',
      'hardcoded:JustSong',
    ]),
  ],
  ['src/components/topup/RechargeCard.jsx', new Set(['hardcoded:Waffo'])],
  [
    'src/components/settings/SystemSetting.jsx',
    new Set(['hardcoded:new-api-worker']),
  ],
  [
    'src/components/settings/CustomOAuthSetting.jsx',
    new Set([
      'interpolation:Interpolation parameter "required" was not provided',
      'interpolation:Interpolation parameter "current" was not provided',
      'interpolation:Interpolation parameter "provider" was not provided',
      'interpolation:Interpolation parameter "field" was not provided',
      'interpolation:Interpolation parameter "op" was not provided',
      'interpolation:Interpolation parameter "current.path" was not provided',
    ]),
  ],
  [
    'src/components/layout/Footer.jsx',
    new Set([
      'hardcoded:One API',
      'hardcoded:MjProxy',
      'hardcoded:new-api-key-tool',
      'hardcoded:new-api-horizon',
      'hardcoded:CoAI',
      'hardcoded:GPT-Load',
      'hardcoded:New API',
    ]),
  ],
  ['src/components/playground/SSEViewer.jsx', new Set(['hardcoded:[DONE]'])],
  [
    'src/components/playground/CustomRequestEditor.jsx',
    new Set(['hardcoded:{"model": "gpt-4o", "messages": [...], ...}']),
  ],
  [
    'src/components/playground/CodeViewer.jsx',
    new Set(['hardcoded:(+', 'hardcoded:K)']),
  ],
  [
    'src/pages/Setting/Model/SettingModelDeployment.jsx',
    new Set(['hardcoded:io.net']),
  ],
  [
    'src/components/topup/modals/SubscriptionPurchaseModal.jsx',
    new Set(['hardcoded:Stripe', 'hardcoded:Creem']),
  ],
  [
    'src/components/layout/headerbar/NewYearButton.jsx',
    new Set(['hardcoded:🎉']),
  ],
  [
    'src/pages/Setting/Ratio/components/TieredPricingEditor.jsx',
    new Set(['hardcoded:),']),
  ],
  [
    'src/components/setup/components/steps/DatabaseStep.jsx',
    new Set(['hardcoded:💡']),
  ],
  [
    'src/components/table/channels/modals/EditTagModal.jsx',
    new Set(['hardcoded:{api_key}']),
  ],
  [
    'src/components/table/channels/modals/EditChannelModal.jsx',
    new Set(['hardcoded:Doubao Coding Plan', 'hardcoded:{api_key}']),
  ],
  [
    'src/components/table/channels/modals/CodexUsageModal.jsx',
    new Set(['hardcoded:metered_feature']),
  ],
  [
    'src/components/table/model-deployments/modals/ViewDetailsModal.jsx',
    new Set(['hardcoded:🌍']),
  ],
  [
    'src/components/settings/personal/cards/NotificationSettings.jsx',
    new Set([
      'hardcoded:type:',
      'hardcoded:title:',
      'hardcoded:content:',
      'hardcoded:values:',
      'hardcoded:timestamp:',
      'hardcoded:?sound=alarm&group=quota',
      'hardcoded:title',
      'hardcoded:content',
      'hardcoded:Bark',
      'hardcoded:Gotify',
      'interpolation:Interpolation parameter "value" was not provided',
      'interpolation:Interpolation parameter "title" was not provided',
      'interpolation:Interpolation parameter "content" was not provided',
    ]),
  ],
]);

const exactLintAllowlistPlugin = {
  name: 'classic-exact-lint-allowlist',
  lintOnResult(filePath, issues) {
    const normalizedPath = filePath.replaceAll('\\', '/');
    const allowlistEntry = [...exactLintAllowlist.entries()].find(
      ([relativePath]) => normalizedPath.endsWith(relativePath),
    );

    if (!allowlistEntry) {
      return issues;
    }

    const allowedIssues = allowlistEntry[1];
    return issues.filter(
      (issue) =>
        !allowedIssues.has(`${issue.type ?? 'hardcoded'}:${issue.text}`),
    );
  },
};

/** @type {import('i18next-cli').I18nextToolkitConfig} */
export default {
  locales: ['zh-CN', 'zh-TW', 'en', 'fr', 'ru', 'ja', 'vi'],
  extract: {
    input: ['src/**/*.{js,jsx,ts,tsx}'],
    ignore: ['src/i18n/**/*'],
    output: 'src/i18n/locales/{{language}}.json',
    ignoredAttributes: [
      'accept',
      'align',
      'aria-label',
      'autoComplete',
      'className',
      'clipRule',
      'color',
      'crossOrigin',
      'data-index',
      'data-name',
      'data-testid',
      'data-type',
      'defaultActiveKey',
      'direction',
      'editorType',
      'field',
      'fill',
      'fillRule',
      'height',
      'hoverStyle',
      'htmlType',
      'id',
      'itemKey',
      'key',
      'keyPrefix',
      'layout',
      'margin',
      'maxHeight',
      'mode',
      'name',
      'overflow',
      'placement',
      'position',
      'rel',
      'role',
      'rowKey',
      'searchPosition',
      'selectedStyle',
      'shape',
      'size',
      'style',
      'theme',
      'trigger',
      'uploadTrigger',
      'validateStatus',
      'value',
      'viewBox',
      'width',
    ],
    sort: true,
    disablePlurals: false,
    removeUnusedKeys: false,
    nsSeparator: false,
    keySeparator: false,
    mergeNamespaces: true,
  },
  lint: {
    checkInterpolationParams: true,
  },
  plugins: [exactLintAllowlistPlugin],
};
