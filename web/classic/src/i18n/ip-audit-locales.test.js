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
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const localesDirectory = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  'locales',
);

const localeFiles = [
  'en.json',
  'fr.json',
  'ja.json',
  'ru.json',
  'vi.json',
  'zh-CN.json',
  'zh-TW.json',
  'zh.json',
];

const requiredKeys = [
  '安全与 IP 审计',
  '当前 TRUSTED_PROXIES 范围过宽，客户端 IP 可能被伪造。',
  '当前时间范围内没有带 IP 的审计事件。',
  '当前时间范围内没有注册、登录或 API 事件。',
  '当前页没有安全事件。',
  '登录成功',
  '登录失败',
  '登录事件',
  '异常',
  '风险',
  '关联账号',
  '关联账号数',
  '活动时间',
  '近 30 天',
  '近 7 天',
  '近 90 天',
  '近期异常',
  '可信代理配置不安全',
  '可信代理尚未配置',
  '其他活动',
  '请稍后重试。',
  '请求次数',
  '请配置 TRUSTED_PROXIES 后再将 IP 审计结果用于安全判断。',
  '全部历史',
  '审计时间范围',
  '事件类型',
  '事件统计',
  '首次',
  '无关联账号',
  '用户安全审计加载失败',
  '暂无 IP 审计记录',
  '暂无安全审计活动',
  '暂无时间线记录',
  '注册 IP',
  '最近',
  '最近 IP',
  '最近活动',
  'Admin 掩码视图',
  'API 账号',
  'API 错误',
  'API 调用',
  'IP 审计视图',
  'IP 活动时间线',
  'IP 审计',
  'IP 审计加载失败',
  'IP 审计数据仍在准备中，请稍后重试。',
  'Root 完整视图',
  '共享 API IP',
  '共享 IP',
  '共享注册 IP',
  '风险优先',
  '认证活动',
  '认证账号',
  '注册',
  '注册账号',
  '注册事件',
  '全部 IP 活动',
  '账号总数',
  'IP 审计已关闭',
  '管理员已关闭 IP 安全事件采集。',
  '相关账号',
  '相关账号数',
  '无相关账号',
  'API 调用归属所用 API Key 的所有者；登录、登出和注册事件归属直接执行认证行为的账号。',
  '基础设施 IP',
  '未授权（401）',
  '未授权请求突增',
  '查看外部 IP 画像',
];

describe('classic IP audit locale coverage', () => {
  test('defines every audit key explicitly in all eight locales', () => {
    expect(requiredKeys).toHaveLength(68);

    for (const localeFile of localeFiles) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8'),
      );
      const translations = locale.translation || locale;

      for (const key of requiredKeys) {
        expect(Object.hasOwn(translations, key)).toBe(true);
        expect(typeof translations[key]).toBe('string');
        expect(translations[key].trim()).not.toBe('');
      }
    }
  });
});
