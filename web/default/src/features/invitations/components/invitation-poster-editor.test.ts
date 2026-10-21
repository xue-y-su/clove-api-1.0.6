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
import { expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'

const editorSource = readFileSync(
  new URL('./invitation-poster-editor.tsx', import.meta.url),
  'utf8'
)
const dialogSource = readFileSync(
  new URL('./invitation-qr-dialog.tsx', import.meta.url),
  'utf8'
)

test('keeps editing behind a pencil and uses shared transforms', () => {
  expect(dialogSource).toContain("mode === 'preview'")
  expect(dialogSource).toContain("aria-label={t('Edit poster')}")
  expect(dialogSource).toContain('<Pencil')
  expect(editorSource).toContain('constrainInvitationPosterTransform')
  expect(editorSource).toContain('clientPointToPosterPoint')
  expect(editorSource).toContain('setPointerCapture')
  expect(editorSource).toContain('event.shiftKey ? 10 : 1')
})

test('keeps download out of edit mode and reset reversible', () => {
  expect(dialogSource).toContain("mode === 'preview' ?")
  expect(dialogSource).toContain('saveInvitationPosterState')
  expect(dialogSource).toContain('createDefaultInvitationPosterDesign')
  expect(dialogSource).toContain('setDraft(structuredClone(activeDesign))')
  expect(dialogSource).toContain('setDiscardConfirmationOpen(true)')
})

test('keeps uploaded backgrounds browser-local and resource bounded', () => {
  expect(dialogSource).toContain('validateInvitationPosterUpload')
  expect(dialogSource).toContain('createInvitationPosterObjectUrlLease')
  expect(dialogSource).toContain('pendingUploadLease.current?.release()')
  expect(editorSource).toContain("accept='image/png,image/jpeg,image/webp'")
  expect(`${dialogSource}\n${editorSource}`).not.toMatch(
    /fetch\(|XMLHttpRequest|axios\./
  )
})

test('uses stable responsive overlays and protected module controls', () => {
  expect(editorSource).toContain('min-h-11 min-w-11')
  expect(editorSource).toContain('aria-selected={selected}')
  expect(editorSource).toContain('max-h-[45vh]')
  expect(editorSource).toContain('<Lock')
  expect(editorSource).toContain('<Slider')
  expect(editorSource).toContain('<Switch')
})

test('translates every editor control in every Default locale', () => {
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
  ]
  for (const locale of ['en', 'fr', 'ja', 'ru', 'vi', 'zh-TW', 'zh']) {
    const parsed = JSON.parse(
      readFileSync(
        new URL(`../../../i18n/locales/${locale}.json`, import.meta.url),
        'utf8'
      )
    ) as { translation: Record<string, string> }
    for (const key of keys) expect(parsed.translation[key]).toBeTruthy()
  }
})
