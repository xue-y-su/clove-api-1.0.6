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

const sourceRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..',
);

const read = (relativePath) =>
  fs.readFileSync(path.join(sourceRoot, relativePath), 'utf8');

describe('classic frontend feature parity', () => {
  test('localizes Root actions and the language button', () => {
    const columns = read('components/table/users/UsersColumnDefs.jsx');
    expect(columns).toContain("promote_root: t('提升为 Root')");
    expect(columns).toContain("demote_root: t('将 Root 降为管理员')");

    for (const localeName of [
      'en',
      'fr',
      'ja',
      'ru',
      'vi',
      'zh',
      'zh-CN',
      'zh-TW',
    ]) {
      const locale = JSON.parse(read(`i18n/locales/${localeName}.json`));
      const translations = locale.translation || locale;
      expect(translations['common.changeLanguage']).not.toBe(
        'common.changeLanguage',
      );
      expect(translations['提升为 Root']).toBeTruthy();
      expect(translations['将 Root 降为管理员']).toBeTruthy();

      for (const key of [
        '保存注册设置',
        '操作人 ID',
        '单独设置优先于系统默认值',
        '当前默认不限制邀请人数',
        '附件读取失败',
        '跟随系统',
        '管理员不受系统默认邀请限制',
        '截图',
        '默认不限制邀请人数',
        '默认邀请人数上限',
        '拍照',
        '上传文件',
        '上传照片',
        '数字 0 表示不能邀请任何人',
        '添加附件',
        '跳过时间',
        '跳过邀请码记录',
        '跳过原因',
        '请输入邀请码',
        '请输入跳过邀请码的原因',
        '强制邀请注册且未填写邀请码时必填',
        '填写邀请码时无需填写跳过原因',
        '邀请限制',
        '邀请限制模式',
        '邀请注册',
        '移除附件',
        '站点导航',
        '注册必须填写邀请码',
        '注册设置已保存',
        '自定义上限',
        '自定义邀请上限',
        '最后登录',
        '最多可添加 {{count}} 张图片',
        '图片 {{name}} 超过大小限制',
        '最多可添加 {{count}} 个文件',
        '文件 {{name}} 超过大小限制',
        '无法从 {{name}} 提取可读文本',
        '附件文本总长度超过限制',
        '无法读取文件 {{name}}',
        '暂不支持文件 {{name}}（{{type}}）',
        '无法绘制屏幕截图',
        '屏幕截图未生成图片',
        '当前浏览器不支持屏幕截图',
        '截图失败',
        '用户上传的图片 {{index}}',
        '图片加载失败',
      ]) {
        expect(translations[key], `${localeName}: ${key}`).toBeTruthy();
      }
    }
  });

  test('exposes invitation controls and records across classic admin and wallet views', () => {
    const walletInvitation = read(
      'components/invitations/UserInvitationCard.jsx',
    );
    expect(walletInvitation).toContain('getInvitationSelf');
    expect(walletInvitation).toContain('InvitationQRModal');
    expect(walletInvitation).toContain('withdrawInvitationApplication');
    const adminInvitation = read('components/invitations/AdminInvitations.jsx');
    expect(adminInvitation).toContain('reviewInvitationApplication');
    expect(adminInvitation).toContain('issueInvitationCode');
    expect(adminInvitation).toContain('revokeInvitationCode');
  });

  test('refreshes the classic user form after an atomic quota adjustment', () => {
    const editUser = read('components/table/users/modals/EditUserModal.jsx');

    expect(editUser).toContain('setInputs(normalizeUserEditValues(data))');
    expect(editUser).not.toContain('getInitValues()');
  });

  test('provides mobile navigation and all playground attachment commands', () => {
    expect(read('components/layout/headerbar/Navigation.jsx')).toContain(
      'MobileNavigationMenu',
    );
    const input = read('components/playground/CustomInputRender.jsx');
    for (const command of ['上传文件', '上传照片', '截图', '拍照']) {
      expect(input).toContain(command);
    }
    expect(input).toContain('onFilesSelected');
    expect(input).not.toContain('if (!imageEnabled)');

    const messageContent = read('components/playground/MessageContent.jsx');
    expect(messageContent).toContain('message.attachments');
    expect(messageContent).toContain('attachment.dataUrl');
    expect(messageContent).toContain('displayContent');
  });

  test('keeps the playground attachment button on a direct dropdown trigger', () => {
    const input = read('components/playground/CustomInputRender.jsx');
    const dropdownStart = input.indexOf("<Dropdown\n      trigger='click'");
    const triggerWrapper = input.indexOf(
      "<span className='inline-flex'>",
      dropdownStart,
    );
    const tooltip = input.indexOf(
      "<Tooltip content={t('添加附件')}>",
      triggerWrapper,
    );
    const button = input.indexOf('<Button', tooltip);
    const dropdownEnd = input.indexOf('</Dropdown>', button);

    expect(dropdownStart).toBeGreaterThan(-1);
    expect(triggerWrapper).toBeGreaterThan(dropdownStart);
    expect(tooltip).toBeGreaterThan(triggerWrapper);
    expect(button).toBeGreaterThan(tooltip);
    expect(dropdownEnd).toBeGreaterThan(button);
  });
});
