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
import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const readSource = (path) =>
  readFileSync(new URL(path, import.meta.url), 'utf8');

test('classic mounts a global acknowledgement popup at the application layout', () => {
  const layoutSource = readSource('./PageLayout.jsx');
  const gateSource = readSource('./AuthenticatedGateStack.jsx');
  const modalSource = readSource('./GlobalAnnouncementModal.jsx');

  expect(layoutSource).toContain('AuthenticatedGateStack');
  expect(layoutSource).not.toContain('GlobalAnnouncementModal');
  expect(gateSource).toContain('GlobalAnnouncementModal');
  expect(gateSource).toContain('getAuthenticatedGateAccess');
  expect(modalSource).toContain('getGlobalAnnouncementState');
  expect(modalSource).toContain('acknowledgeGlobalAnnouncement');
  expect(modalSource).toContain('公告确认失败');
  expect(modalSource).toContain('maskClosable={false}');
  expect(modalSource).toContain("isMobile ? 'calc(100vw - 24px)' : 600");
});

test('classic administrator composer uses managed image upload instead of URL entry', () => {
  const source = readSource(
    '../../pages/Setting/Dashboard/SettingsAnnouncements.jsx',
  );

  expect(source).toContain('ImagePlus');
  expect(source).toContain('uploadAnnouncementMedia');
  expect(source).toContain('aspect-square');
  expect(source).toContain('announcementSettingsError');
  expect(source).toContain('loadAnnouncementSettings');
  expect(source).toContain('width={112}');
  expect(source).toContain('height={112}');
  expect(source).toContain("t('Announcements saved successfully')");
  expect(source).toContain("t('Failed to save announcements')");
  expect(source).toContain("t('Please complete all announcement fields')");
  expect(source).toContain("t('Please select items to delete')");
  expect(source).not.toContain("showSuccess('系统公告已更新')");
  expect(source).not.toContain("showError('系统公告更新失败')");
  expect(source).toContain('image/png,image/jpeg,image/gif,image/webp');
  expect(source).not.toContain('type="url"');
  expect(source).not.toContain('图片 URL');
  expect(source).not.toContain('maxCount={500}');
  expect(source).toContain(
    'new TextEncoder().encode(announcementForm.content)',
  );
  expect(source).toContain('64 * 1024');
  expect(source).not.toContain('content_format');
  expect(source).not.toContain('contentFormat');
  expect(source).not.toContain('popup_enabled');
  expect(source).toContain("t('此公告以弹窗形式展示')");
  expect(source).toContain("field='title'");
  expect(source).toContain("field='published'");
  expect(source).toContain("t('发布此公告')");
  expect(source).toContain('const isMobile = useIsMobile()');
  expect(source).toContain("width={isMobile ? 'calc(100vw - 24px)' : 520}");
  expect(source).toContain("width={isMobile ? 'calc(100vw - 24px)' : 800}");
  expect(source).not.toContain("t('Show unread announcements as a popup')");
});

test('classic renders every announcement through the markdown sanitizer', () => {
  const source = readSource('./GlobalAnnouncementModal.jsx');
  const panelSource = readSource('../dashboard/AnnouncementsPanel.jsx');
  const noticeSource = readSource('./NoticeModal.jsx');

  expect(source).toContain('sanitizeMarkdown');
  expect(source).not.toContain('sanitizeRichHtml');
  expect(source).not.toContain('content_format');
  expect(source).not.toContain('!state.popup_enabled');
  expect(source).toContain("t('查看公告图片')");
  expect(source).toContain('previewImageURL');
  expect(source).toContain('max-h-[85vh] max-w-[95vw] object-contain');
  expect(panelSource).toContain('sanitizeMarkdown');
  expect(panelSource).not.toContain('sanitizeRichHtml');
  expect(panelSource).not.toContain('content_format');
  expect(noticeSource).toContain('sanitizeMarkdown');
  expect(noticeSource).not.toContain('sanitizeRichHtml');
  expect(noticeSource).not.toContain('content_format');
  expect(source).toContain('announcement.title');
  expect(panelSource).toContain('item.title');
  expect(noticeSource).toContain('item.title');
  expect(panelSource).toContain('AnnouncementImage');
  expect(noticeSource).toContain('AnnouncementImage');
});

test('classic translates announcement upload and acknowledgement controls', () => {
  const keys = [
    '公告图片',
    '公告图片（可选）',
    '添加公告图片',
    '替换公告图片',
    '移除公告图片',
    '公告图片上传失败',
    '公告图片必须是 PNG、JPEG、GIF 或 WebP',
    '支持 PNG、JPEG、GIF 或 WebP，最大 5 MiB。',
    '正在确认公告',
    '公告确认失败，请重试',
    '知道了',
    '公告标题',
    '发布此公告',
    '未发布的公告仅管理员可见。',
  ];

  for (const localeFile of [
    'en.json',
    'fr.json',
    'ja.json',
    'ru.json',
    'vi.json',
    'zh-CN.json',
    'zh-TW.json',
    'zh.json',
  ]) {
    const locale = JSON.parse(
      readSource(`../../i18n/locales/${localeFile}`),
    ).translation;
    for (const key of keys) {
      expect(typeof locale[key]).toBe('string');
      expect(locale[key].trim().length).toBeGreaterThan(0);
      if (!localeFile.startsWith('zh')) {
        expect(locale[key]).not.toBe(key);
      }
    }
  }
});

test('classic translates the announcement content size limit', () => {
  const key = 'Announcement content must not exceed 64 KiB';
  for (const localeFile of [
    'en.json',
    'fr.json',
    'ja.json',
    'ru.json',
    'vi.json',
    'zh-CN.json',
    'zh-TW.json',
    'zh.json',
  ]) {
    const locale = JSON.parse(
      readSource(`../../i18n/locales/${localeFile}`),
    ).translation;
    expect(typeof locale[key]).toBe('string');
    expect(locale[key].trim().length).toBeGreaterThan(0);
    if (localeFile !== 'en.json') expect(locale[key]).not.toBe(key);
  }
});
