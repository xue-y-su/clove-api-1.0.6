/*
Copyright (C) 2023-2026 QuantumNous

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
import { describe, expect, test } from 'bun:test'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const localesDirectory = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  'locales'
)

const localeFiles = [
  'en.json',
  'fr.json',
  'ja.json',
  'ru.json',
  'vi.json',
  'zh-TW.json',
  'zh.json',
]

const requiredKeys = [
  'My Redemption Codes',
  'Wallet Ledger',
  'Records start on {{date}}. Earlier balance changes are not backfilled.',
  'Earlier balance changes are not backfilled.',
  'Failed to load wallet ledger',
  'No wallet ledger entries yet',
  'Refresh wallet ledger',
  'No codes yet.',
  'Update your display identity',
  'Avatar URL',
  'Save profile',
  'Home Page',
  'Original Home Page',
  'Snow House Home Page',
  'Choose which homepage is served at the root URL.',
  'Save playground settings',
  'Enable Tavily fallback search',
  'Used only when the selected routed model cannot perform native web search.',
  'Tavily API Key',
  'Configured',
  'Not configured',
  'Enter a new key to replace the configured key',
  'Enter Tavily API Key',
  'Clear the configured key',
  'Default search depth',
  'Basic',
  'Maximum search results',
  'Maximum search context length',
  'Native web search model patterns',
  'Image model allowlist',
  'Image model denylist',
  'Image model display order',
  'Additional image model allowlist',
  'Review applications and manage approved invitation codes.',
  'Create custom template',
  'Subject',
  'Ongoing',
  'Blue',
  'Green',
  'Cyan',
  'Purple',
  'Pink',
  'Red',
  'Orange',
  'Amber',
  'Yellow',
  'Lime',
  'Teal',
  'Indigo',
  'Violet',
  'Slate',
  'builtin',
  'custom',
  'Default verification',
  'Email template preview',
  'Request rule',
  'Flat',
  'Night discount (50%)',
  'Weekend discount (80%)',
  'Group Type',
  'Model Group',
  'Reusable sets of models you can attach to channels.',
  'Tag Group',
  'Collections of metadata tags for bulk operations.',
  'Endpoint Group',
  'HTTP endpoint mappings shared across providers.',
  'Prefill group updated',
  'Prefill group created',
  'Operation failed',
  'No description provided',
  '{{count}} item',
  '{{count}} items',
  '+{{count}} more',
  'No endpoint mappings configured.',
  'No items configured yet.',
  '{{count}} group',
  '{{count}} groups',
  'Please retry or refresh the page.',
  'Failed to delete group',
  'Off',
  'Low',
  'Medium',
  'High',
  'Failed to preview upstream diff',
  'Found {{count}} conflicts. Please resolve them first.',
  'Sync completed! Created {{created}} models, updated {{updated}}, and added {{vendors}} vendors.',
  'Sync failed',
  'Diff',
  'Field',
  'Local Value',
  'Upstream Value',
  'View diff',
  '{{count}} model with conflicts',
  '{{count}} models with conflicts',
  '{{count}} field showing',
  '{{count}} fields showing',
  '{{count}} selected',
  'Showing {{start}}-{{end}} of {{count}} field',
  'Showing {{start}}-{{end}} of {{count}} fields',
  'HD',
  'Vivid',
  'Natural',
  'Opaque',
  'Transparent',
  'Optional',
  'Decrement',
  'Increment',
  'Sans',
  'Serif',
  'Select {{label}}',
  '{{label}} option preview',
  'Goroutines',
  'Stripe Price ID',
  'Creem Product ID',
  'Waffo Pancake Product ID',
  'Scroll to bottom',
  'More pages',
  'Delete row',
  'Toggle password visibility',
  'Remove tag',
  'Pagination',
  'Breadcrumb',
  'Custom API Key',
  'Optional custom API Key',
  'Available only when creating a single API key',
  'This API key has been disabled because it may have been exposed. Create a new key and replace the old one.',
  'The model returned no displayable content',
]

const simplifiedChinese = {
  'My Redemption Codes': '我的兑换码',
  'Wallet Ledger': '钱包流水',
  'Records start on {{date}}. Earlier balance changes are not backfilled.':
    '记录始于 {{date}}，更早的余额变动不会补录。',
  'Earlier balance changes are not backfilled.': '更早的余额变动不会补录。',
  'Failed to load wallet ledger': '加载钱包流水失败',
  'No wallet ledger entries yet': '暂无钱包流水记录',
  'Refresh wallet ledger': '刷新钱包流水',
  'No codes yet.': '暂无兑换码。',
  'Update your display identity': '更新你的显示身份',
  'Avatar URL': '头像 URL',
  'Save profile': '保存个人资料',
  'Home Page': '主页',
  'Original Home Page': '原版主页',
  'Snow House Home Page': 'Snow House 主页',
  'Choose which homepage is served at the root URL.': '选择根 URL 提供的主页。',
  'Save playground settings': '保存游乐场设置',
  'Enable Tavily fallback search': '启用 Tavily 备用搜索',
  'Used only when the selected routed model cannot perform native web search.':
    '仅当所选路由模型无法执行原生联网搜索时使用。',
  'Tavily API Key': 'Tavily API 密钥',
  Configured: '已配置',
  'Not configured': '未配置',
  'Enter a new key to replace the configured key':
    '输入新密钥以替换已配置的密钥',
  'Enter Tavily API Key': '输入 Tavily API 密钥',
  'Clear the configured key': '清除已配置的密钥',
  'Default search depth': '默认搜索深度',
  Basic: '基础',
  'Maximum search results': '最大搜索结果数',
  'Maximum search context length': '最大搜索上下文长度',
  'Native web search model patterns': '原生联网搜索模型匹配规则',
  'Image model allowlist': '图像模型允许列表',
  'Image model denylist': '图像模型禁止列表',
  'Image model display order': '图像模型显示顺序',
  'Additional image model allowlist': '额外图像模型允许列表',
  'Review applications and manage approved invitation codes.':
    '审核申请并管理已批准的邀请码。',
  'Create custom template': '创建自定义模板',
  Subject: '主题',
  Ongoing: '进行中',
  Blue: '蓝色',
  Green: '绿色',
  Cyan: '青色',
  Purple: '紫色',
  Pink: '粉色',
  Red: '红色',
  Orange: '橙色',
  Amber: '琥珀色',
  Yellow: '黄色',
  Lime: '青柠色',
  Teal: '蓝绿色',
  Indigo: '靛蓝色',
  Violet: '紫罗兰色',
  Slate: '石板灰',
  builtin: '内置',
  custom: '自定义',
  'Default verification': '默认验证模板',
  'Email template preview': '邮件模板预览',
  'Request rule': '请求规则',
  Flat: '固定价格',
  'Night discount (50%)': '夜间折扣（50%）',
  'Weekend discount (80%)': '周末折扣（80%）',
  'Group Type': '分组类型',
  'Model Group': '模型分组',
  'Reusable sets of models you can attach to channels.':
    '可附加到渠道的可复用模型集合。',
  'Tag Group': '标签分组',
  'Collections of metadata tags for bulk operations.':
    '用于批量操作的元数据标签集合。',
  'Endpoint Group': '端点分组',
  'HTTP endpoint mappings shared across providers.':
    '可在供应商之间共享的 HTTP 端点映射。',
  'Prefill group updated': '预填分组已更新',
  'Prefill group created': '预填分组已创建',
  'Operation failed': '操作失败',
  'No description provided': '未提供描述',
  '{{count}} item': '{{count}} 个项目',
  '{{count}} items': '{{count}} 个项目',
  '+{{count}} more': '还有 {{count}} 项',
  'No endpoint mappings configured.': '尚未配置端点映射。',
  'No items configured yet.': '尚未配置项目。',
  '{{count}} group': '{{count}} 个分组',
  '{{count}} groups': '{{count}} 个分组',
  'Please retry or refresh the page.': '请重试或刷新页面。',
  'Failed to delete group': '删除分组失败',
  Off: '关闭',
  Low: '低',
  Medium: '中',
  High: '高',
  'Failed to preview upstream diff': '预览上游差异失败',
  'Found {{count}} conflicts. Please resolve them first.':
    '发现 {{count}} 个冲突，请先解决它们。',
  'Sync completed! Created {{created}} models, updated {{updated}}, and added {{vendors}} vendors.':
    '同步完成！已创建 {{created}} 个模型，更新 {{updated}} 个，并新增 {{vendors}} 个供应商。',
  'Sync failed': '同步失败',
  Diff: '差异',
  Field: '字段',
  'Local Value': '本地值',
  'Upstream Value': '上游值',
  'View diff': '查看差异',
  '{{count}} model with conflicts': '{{count}} 个存在冲突的模型',
  '{{count}} models with conflicts': '{{count}} 个存在冲突的模型',
  '{{count}} field showing': '正在显示 {{count}} 个字段',
  '{{count}} fields showing': '正在显示 {{count}} 个字段',
  '{{count}} selected': '已选择 {{count}} 项',
  'Showing {{start}}-{{end}} of {{count}} field':
    '正在显示第 {{start}}-{{end}} 项，共 {{count}} 个字段',
  'Showing {{start}}-{{end}} of {{count}} fields':
    '正在显示第 {{start}}-{{end}} 项，共 {{count}} 个字段',
  HD: '高清',
  Vivid: '鲜艳',
  Natural: '自然',
  Opaque: '不透明',
  Transparent: '透明',
  Optional: '可选',
  Decrement: '减少',
  Increment: '增加',
  Sans: '无衬线',
  Serif: '衬线',
  'Select {{label}}': '选择{{label}}',
  '{{label}} option preview': '{{label}}选项预览',
  Goroutines: 'Goroutine 数量',
  'Stripe Price ID': 'Stripe 价格 ID',
  'Creem Product ID': 'Creem 产品 ID',
  'Waffo Pancake Product ID': 'Waffo Pancake 产品 ID',
  'Scroll to bottom': '滚动到底部',
  'More pages': '更多页面',
  'Delete row': '删除行',
  'Toggle password visibility': '切换密码可见性',
  'Remove tag': '移除标签',
  Pagination: '分页',
  Breadcrumb: '面包屑导航',
  'Custom API Key': '自定义 API 密钥',
  'Optional custom API Key': '可选的自定义 API 密钥',
  'Available only when creating a single API key':
    '仅在创建单个 API 密钥时可用',
  'This API key has been disabled because it may have been exposed. Create a new key and replace the old one.':
    '此 API Key 因疑似泄露已被系统禁用，请创建并替换新的 Key。',
  'The model returned no displayable content': '模型未返回可显示内容',
} as const

const localizedFeatureKeys = [
  'Custom API Key',
  'Optional custom API Key',
  'Available only when creating a single API key',
  'This API key has been disabled because it may have been exposed. Create a new key and replace the old one.',
  'The model returned no displayable content',
] as const

const settlementErrorTranslations = {
  'en.json': {
    'Redemption code was not found': 'Redemption code was not found',
    'Redemption code is not eligible for settlement':
      'Redemption code is not eligible for settlement',
    'Redemption code has already been settled differently':
      'Redemption code has already been settled differently',
    'Settlement request is invalid': 'Settlement request is invalid',
    'Unable to settle this redemption code':
      'Unable to settle this redemption code',
  },
  'fr.json': {
    'Redemption code was not found': "Code d'échange introuvable",
    'Redemption code is not eligible for settlement':
      "Ce code d'échange ne peut pas être réglé dans son état actuel",
    'Redemption code has already been settled differently':
      "Ce code d'échange a déjà été réglé d'une autre manière",
    'Settlement request is invalid': 'La demande de règlement est invalide',
    'Unable to settle this redemption code':
      "Impossible de régler ce code d'échange",
  },
  'ja.json': {
    'Redemption code was not found': '交換コードが見つかりません',
    'Redemption code is not eligible for settlement':
      'この交換コードは現在精算できません',
    'Redemption code has already been settled differently':
      'この交換コードは別の方法ですでに精算されています',
    'Settlement request is invalid': '精算リクエストが無効です',
    'Unable to settle this redemption code': 'この交換コードを精算できません',
  },
  'ru.json': {
    'Redemption code was not found': 'Код погашения не найден',
    'Redemption code is not eligible for settlement':
      'Этот код погашения сейчас нельзя рассчитать',
    'Redemption code has already been settled differently':
      'Этот код погашения уже рассчитан другим способом',
    'Settlement request is invalid': 'Запрос на расчет недействителен',
    'Unable to settle this redemption code':
      'Не удалось рассчитать этот код погашения',
  },
  'vi.json': {
    'Redemption code was not found': 'Không tìm thấy mã đổi thưởng',
    'Redemption code is not eligible for settlement':
      'Mã đổi thưởng này hiện không đủ điều kiện quyết toán',
    'Redemption code has already been settled differently':
      'Mã đổi thưởng này đã được quyết toán theo cách khác',
    'Settlement request is invalid': 'Yêu cầu quyết toán không hợp lệ',
    'Unable to settle this redemption code':
      'Không thể quyết toán mã đổi thưởng này',
  },
  'zh-TW.json': {
    'Redemption code was not found': '找不到兌換碼',
    'Redemption code is not eligible for settlement': '此兌換碼目前無法結算',
    'Redemption code has already been settled differently':
      '此兌換碼已用其他方式結算',
    'Settlement request is invalid': '結算請求無效',
    'Unable to settle this redemption code': '無法結算此兌換碼',
  },
  'zh.json': {
    'Redemption code was not found': '未找到兑换码',
    'Redemption code is not eligible for settlement': '该兑换码当前不能结算',
    'Redemption code has already been settled differently':
      '该兑换码已按其他方式结算',
    'Settlement request is invalid': '结算请求无效',
    'Unable to settle this redemption code': '无法结算该兑换码',
  },
} as const

describe('production UI locale coverage', () => {
  test('keeps all runtime strings inside the translation namespace', () => {
    for (const localeFile of localeFiles) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8')
      ) as Record<string, unknown>

      expect(Object.keys(locale)).toEqual(['translation'])
    }
  })

  test('defines the audited UI keys in every shipped locale', () => {
    for (const localeFile of localeFiles) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8')
      ) as { translation?: Record<string, unknown> }

      for (const key of requiredKeys) {
        expect(Object.hasOwn(locale.translation ?? {}, key)).toBe(true)
        expect(typeof locale.translation?.[key]).toBe('string')
        expect(
          (locale.translation?.[key] as string | undefined)?.trim()
        ).not.toBe('')
      }
    }
  })

  test('ships real Simplified Chinese translations for audited UI', () => {
    const locale = JSON.parse(
      fs.readFileSync(path.join(localesDirectory, 'zh.json'), 'utf8')
    ) as { translation: Record<string, string> }

    for (const [key, value] of Object.entries(simplifiedChinese)) {
      expect(locale.translation[key]).toBe(value)
    }
  })

  test('does not ship English fallbacks for the new feature keys', () => {
    for (const localeFile of localeFiles.filter((file) => file !== 'en.json')) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8')
      ) as { translation: Record<string, string> }

      for (const key of localizedFeatureKeys) {
        expect(locale.translation[key]).not.toBe(key)
      }
    }
  })

  test('localizes redemption settlement errors in every locale', () => {
    for (const [localeFile, expected] of Object.entries(
      settlementErrorTranslations
    )) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8')
      ) as { translation: Record<string, string> }

      for (const [key, value] of Object.entries(expected)) {
        expect(locale.translation[key]).toBe(value)
      }
    }
  })
})
