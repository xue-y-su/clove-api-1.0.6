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

const source = readFileSync(
  new URL('./user-invitation-card.tsx', import.meta.url),
  'utf8'
)
const qrDialogSource = readFileSync(
  new URL('./invitation-qr-dialog.tsx', import.meta.url),
  'utf8'
)
const posterEditorSource = readFileSync(
  new URL('./invitation-poster-editor.tsx', import.meta.url),
  'utf8'
)
const adminDialogsSource = readFileSync(
  new URL('./admin-invitation-dialogs.tsx', import.meta.url),
  'utf8'
)
const apiSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const signUpSource = readFileSync(
  new URL('../../auth/sign-up/components/sign-up-form.tsx', import.meta.url),
  'utf8'
)
const zhLocale = JSON.parse(
  readFileSync(
    new URL('../../../i18n/locales/zh.json', import.meta.url),
    'utf8'
  )
) as { translation: Record<string, string> }

test('exposes separate copy actions for the code and registration link', () => {
  expect(source.match(/<CopyButton/g) ?? []).toHaveLength(2)
  expect(source).toContain("t('Invitation link')")
  expect(source).toContain('value={props.registrationUrl}')
  expect(source).toContain('revokeOwnInvitationCode')
  expect(source).toContain("t('Revoke invitation code')")
  expect(apiSource).toContain('/api/invitation/codes/')
})

test('validates prefilled invitation codes without racing stale requests', () => {
  expect(signUpSource).toContain('validateInvitationCode')
  expect(signUpSource).toContain('window.setTimeout')
  expect(signUpSource).toContain('AbortController')
  expect(signUpSource).toContain("status === 'valid'")
  expect(signUpSource).toContain("status === 'invalid'")
  expect(signUpSource).toContain("aria-live='polite'")
})

test('uses a status label that does not duplicate the translated state prefix', () => {
  expect(source).toContain("t('Invitation code status: {{status}}'")
  expect(zhLocale.translation['Invitation code status: {{status}}']).toBe(
    '邀请码状态：{{status}}'
  )

  const interpolate = (template: string, status: string) =>
    template.replace('{{status}}', status)
  expect(
    interpolate(
      zhLocale.translation['This invitation code is {{status}}'],
      '已撤销'
    )
  ).toBe('邀请码状态：已撤销')
})

test('collects invitation revoke reason in a confirmation dialog', () => {
  expect(source).toContain('InvitationRevokeDialog')
  expect(source).not.toContain("t('Cancelled by code owner')")
})

test('collects invitation application reason only in a dialog', () => {
  expect(source).toContain('InvitationApplicationDialog')
  expect(source).not.toContain('<Textarea')

  const dialogSource = readFileSync(
    new URL('./invitation-application-dialog.tsx', import.meta.url),
    'utf8'
  )
  expect(dialogSource).toContain('<Textarea')
  expect(dialogSource).toContain('reason.trim()')
})

test('keeps clipboard failure notifications enabled', () => {
  const copyButtonSource = readFileSync(
    new URL('../../../components/copy-button.tsx', import.meta.url),
    'utf8'
  )

  expect(copyButtonSource).not.toContain(
    'useCopyToClipboard({ notify: false })'
  )
})

test('translates the owner revoke confirmation title in Chinese', () => {
  const revokeDialogSource = readFileSync(
    new URL('./invitation-revoke-dialog.tsx', import.meta.url),
    'utf8'
  )
  expect(revokeDialogSource).toContain("t('Revoke invitation code?')")
  expect(zhLocale.translation['Revoke invitation code?']).toBe('撤销邀请码？')
})

test('renders and downloads a full configurable invitation poster', () => {
  expect(source).toContain('code={invitationQuery.data?.code?.code ??')
  expect(source).toContain('systemName={systemName}')
  expect(source).toContain('logo={logo}')
  expect(source).toContain('displayName={displayName}')
  expect(qrDialogSource).toContain('renderInvitationPoster')
  expect(qrDialogSource).toContain(
    `  resolveInvitationPosterLogoUrl,\n} from '../../../../../shared/invitation-poster.js'`
  )
  expect(qrDialogSource).toContain(
    `logoUrl: resolveInvitationPosterLogoUrl(\n        props.logo,\n        typeof window === 'undefined' ? '' : window.location.origin\n      )`
  )
  expect(qrDialogSource).not.toContain('resolveInvitationPosterLogoFallbackUrl')
  expect(qrDialogSource).toContain('const controller = new AbortController()')
  expect(qrDialogSource).toContain('signal: controller.signal')
  expect(qrDialogSource).toContain('controller.abort()')
  expect(qrDialogSource).toContain('canvasToPngBlob')
  expect(qrDialogSource).toContain('createInvitationPosterFilename')
  expect(qrDialogSource).toContain('design: renderedDesign')
  expect(qrDialogSource).toContain('runtime: posterRuntime')
  expect(qrDialogSource).toContain('backgroundAsset: renderedAsset')
  expect(posterEditorSource).toContain('<ToggleGroup')
  expect(posterEditorSource).toContain('<Switch')
  expect(qrDialogSource).toContain('size={360}')
  expect(qrDialogSource).toContain("level='H'")
  expect(qrDialogSource).toContain('marginSize={4}')
  expect(qrDialogSource).not.toContain('toDataURL')
})

test('reruns poster rendering after the portal mounts the QR canvas', () => {
  expect(qrDialogSource).toContain('const [qrCanvas, setQrCanvas] = useState')
  expect(qrDialogSource).toContain('ref={setQrCanvas}')
  expect(qrDialogSource).toContain('!qrCanvas) return')
  expect(qrDialogSource).toContain('qrCanvas,')
  expect(qrDialogSource).not.toContain('qrCanvasRef')
})

test('uses the shared bounded date-time picker for invitation expiry', () => {
  expect(adminDialogsSource).toContain('<DateTimePicker')
  expect(adminDialogsSource).toContain('minDate=')
  expect(adminDialogsSource).toContain('maxDate=')
  expect(adminDialogsSource).not.toMatch(/type=['"]datetime-local['"]/)
})
