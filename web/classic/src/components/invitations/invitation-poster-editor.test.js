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

const editorSource = readFileSync(
  new URL('./InvitationPosterEditor.jsx', import.meta.url),
  'utf8',
);
const modalSource = readFileSync(
  new URL('./InvitationQRModal.jsx', import.meta.url),
  'utf8',
);

test('keeps editing behind a pencil and delegates transforms to shared code', () => {
  expect(modalSource).toContain("const [mode, setMode] = useState('preview')");
  expect(modalSource).toContain("aria-label={t('Edit poster')}");
  expect(modalSource).toContain('<Pencil');
  expect(editorSource).toContain('constrainInvitationPosterTransform');
  expect(editorSource).toContain('clientPointToPosterPoint');
  expect(editorSource).toContain('setPointerCapture');
  expect(editorSource).toContain('event.shiftKey ? 10 : 1');
});

test('waits for persisted state before editing and serializes saves', () => {
  expect(modalSource).toContain(
    'const [hydrated, setHydrated] = useState(false)',
  );
  expect(modalSource).toContain('if (!hydrated) return');
  expect(modalSource).toContain('disabled={!hydrated}');
  expect(modalSource).toContain('if (!draft || savingRef.current) return');
  expect(modalSource).toContain('savingRef.current = true');
  expect(modalSource).toContain('loading={saving}');
  expect(modalSource).toContain('disabled={saving}');
});

test('shows keyboard focus and supports keyboard resizing', () => {
  expect(editorSource).toContain('focus-visible:ring-2');
  expect(editorSource).toContain('const resizeDirection = {');
  expect(editorSource).toContain('ArrowLeft: -step');
  expect(editorSource).toContain('ArrowDown: step');
  expect(editorSource).toContain('width: from.width + resizeDirection');
  expect(editorSource).toContain('height: from.height + resizeDirection');
});

test('keeps download in preview and makes reset reversible', () => {
  expect(modalSource).toContain("mode === 'preview'");
  expect(modalSource).toContain('saveInvitationPosterState');
  expect(modalSource).toContain('createDefaultInvitationPosterDesign');
  expect(modalSource).toContain('setDraft(structuredClone(activeDesign))');
  expect(modalSource).toContain('setDiscardConfirmationOpen(true)');
});

test('keeps uploaded backgrounds browser-local and releases resources', () => {
  expect(modalSource).toContain('validateInvitationPosterUpload');
  expect(modalSource).toContain('createInvitationPosterObjectUrlLease');
  expect(modalSource).toContain('pendingUploadLease.current?.release()');
  expect(editorSource).toContain("accept='image/png,image/jpeg,image/webp'");
  expect(`${modalSource}\n${editorSource}`).not.toMatch(
    /fetch\(|XMLHttpRequest|axios\./,
  );
});

test('uses responsive overlays and protected module controls', () => {
  expect(editorSource).toContain('min-h-11 min-w-11');
  expect(editorSource).toContain('aria-selected={selected}');
  expect(editorSource).toContain('max-h-[45vh]');
  expect(editorSource).toContain('<Lock');
  expect(editorSource).toContain('<Slider');
  expect(editorSource).toContain('<Switch');
});

test('translates every editor control in every Classic locale', async () => {
  const keys = [
    'Edit poster',
    'Poster background',
    'Clean light',
    'Ink contrast',
    'Coral paper',
    'Solid color',
    'Local image',
    'Move background',
    'Zoom',
    'Poster modules',
    'Eyebrow',
    'Inviter',
    'Invitation code',
    'QR code',
    'Reset poster',
    'Discard poster changes?',
    'Discard changes',
    'Keep editing',
    'This image may look soft at the selected size',
    'This background is available only until this dialog closes',
    'Poster changes will reset after reload',
    'Saved background was unavailable and has been reset',
    'Use a PNG, JPEG, or WebP image up to 10 MiB',
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
    const locale = await Bun.file(
      new URL(`../../i18n/locales/${localeFile}`, import.meta.url),
    ).json();
    for (const key of keys) {
      expect(locale.translation[key], `${localeFile}: ${key}`).toBeTruthy();
      if (localeFile !== 'en.json') {
        expect(locale.translation[key], `${localeFile}: ${key}`).not.toBe(key);
      }
    }
  }
});
