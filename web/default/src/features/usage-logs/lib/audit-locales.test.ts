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
import { readdir, readFile } from 'node:fs/promises'

import { AUDIT_TEMPLATES } from './format'

function backendAuditActions(source: string): string[] {
  const start = source.indexOf('var auditContentTemplates')
  const end = source.indexOf('// auditContentEN', start)
  const block = source.slice(start, end)
  return [...block.matchAll(/^\s*"([^"]+)":/gm)].map((match) => match[1])
}

async function userRedemptionAuditActions(): Promise<string[]> {
  const serviceDirectory = new URL(
    '../../../../../../service/',
    import.meta.url
  )
  const filenames = (await readdir(serviceDirectory)).filter((filename) =>
    /^user_redemption.*\.go$/.test(filename)
  )
  const sources = await Promise.all(
    filenames.map((filename) =>
      readFile(new URL(filename, serviceDirectory), 'utf8')
    )
  )

  return [
    ...new Set(
      sources.flatMap((source) =>
        [...source.matchAll(/"(user\.redemption(?:\.[a-z_]+)+)"/g)].map(
          (match) => match[1]
        )
      )
    ),
  ]
}

const REQUIRED_ACTIONS = [
  'announcement.acknowledge',
  'announcement.media_upload',
  'announcement.settings_update',
  'compliance.full_ip_read',
  'ip_audit_detail_read',
  'ip_audit_read',
  'invitation.incentive_settings_update',
  'log.full_ip_read',
  'log.full_ip_stat_read',
  'security.settings_update',
  'source_attribution.full_ip_read',
  'user.login_failed',
  'user.login_success',
  'user.logout',
  'user.password_setup',
  'user.profile_update',
  'user.register',
  'user.security_audit_read',
  'wallet.adjust',
  'user.redemption.create',
  'user.redemption.claim',
  'user.redemption.cancel_pending',
  'user.redemption.expire_pending',
  'user.redemption.subscription.create',
  'user.redemption.subscription.claim',
  'user.redemption.subscription.plan_disabled',
  'user.redemption.subscription.refund_pending',
  'user.redemption.subscription.refund',
  'user.redemption.subscription.forfeit',
  'user.redemption.cancel',
  'user.redemption.expire_refund',
  'user.redemption.disable',
  'user.redemption.enable',
  'user.redemption.terminate_refund',
  'user.redemption.terminate_forfeit',
  'user.redemption.delete',
] as const

const AUDIT_LOCALE_KEYS = [
  'Wallet {{category}} changed by {{delta}}; balance from {{before}} to {{after}}; reference {{reference_type}} {{reference_id}}',
  'Top-up',
  'Administrator adjustment',
  'API usage',
  'API pre-charge',
  'API settlement charge',
  'API settlement refund',
  'API refund',
  'Task usage',
  'Task charge',
  'Task refund',
  'Subscription overflow charge',
  'Subscription overflow settlement refund',
  'Subscription overflow refund',
  'Daily check-in',
  'Subscription purchase',
  'Invitation gift',
  'Invitation reward',
  'Inviter reward',
  'Redemption code',
  'Redemption code funding',
  'Redemption code claim',
  'Redemption code refund',
  'Other wallet adjustment',
  'Unrecognized operation',
  'Password',
  'Approved',
  'Rejected',
  'Withdrawn',
  'Viewed API token key (ID: {{token_id}})',
  'Viewed an API token key',
] as const

const AUDIT_MESSAGE_KEYS = new Set([
  ...Object.values(AUDIT_TEMPLATES),
  ...AUDIT_LOCALE_KEYS,
])

function interpolationVariables(value: string): string[] {
  return [...value.matchAll(/{{\s*([^},\s]+)[^}]*}}/g)]
    .map((match) => match[1])
    .sort()
}

describe('usage log audit locale coverage', () => {
  test('maps every production audit action added by CloveAPI', () => {
    for (const action of REQUIRED_ACTIONS) {
      expect(AUDIT_TEMPLATES[action], action).toBeTruthy()
    }
  })

  test('covers every backend managed-audit action', async () => {
    const source = await Bun.file(
      new URL('../../../../../../controller/audit.go', import.meta.url)
    ).text()
    for (const action of backendAuditActions(source)) {
      expect(AUDIT_TEMPLATES[action], action).toBeTruthy()
    }
  })

  test('covers every user-redemption audit action emitted by the backend', async () => {
    for (const action of await userRedemptionAuditActions()) {
      expect(AUDIT_TEMPLATES[action], action).toBeTruthy()
    }
  })

  test('every locale translates every audit template', async () => {
    for (const localeFile of [
      'fr.json',
      'ja.json',
      'ru.json',
      'vi.json',
      'zh-TW.json',
      'zh.json',
    ]) {
      const locale = (await Bun.file(
        new URL(`../../../i18n/locales/${localeFile}`, import.meta.url)
      ).json()) as { translation: Record<string, string> }
      for (const template of new Set(Object.values(AUDIT_TEMPLATES))) {
        expect(
          locale.translation[template],
          `${localeFile}: ${template}`
        ).toBeTruthy()
        expect(
          locale.translation[template],
          `${localeFile}: ${template}`
        ).not.toBe(template)
      }
      for (const key of AUDIT_LOCALE_KEYS) {
        expect(locale.translation[key], `${localeFile}: ${key}`).toBeTruthy()
        expect(locale.translation[key], `${localeFile}: ${key}`).not.toBe(key)
      }
    }
  })

  test('preserves every audit interpolation variable in every locale', async () => {
    for (const localeFile of [
      'en.json',
      'fr.json',
      'ja.json',
      'ru.json',
      'vi.json',
      'zh-TW.json',
      'zh.json',
    ]) {
      const locale = (await Bun.file(
        new URL(`../../../i18n/locales/${localeFile}`, import.meta.url)
      ).json()) as { translation: Record<string, string> }

      for (const key of AUDIT_MESSAGE_KEYS) {
        expect(
          interpolationVariables(locale.translation[key]),
          `${localeFile}: ${key}`
        ).toEqual(interpolationVariables(key))
        expect(locale.translation[key], `${localeFile}: ${key}`).not.toMatch(
          /[\r\n]/
        )
      }
    }
  })

  test('does not copy Simplified Chinese audit messages into other locales', async () => {
    const simplifiedChinese = (await Bun.file(
      new URL('../../../i18n/locales/zh.json', import.meta.url)
    ).json()) as { translation: Record<string, string> }

    for (const localeFile of ['fr.json', 'ja.json', 'ru.json', 'vi.json']) {
      const locale = (await Bun.file(
        new URL(`../../../i18n/locales/${localeFile}`, import.meta.url)
      ).json()) as { translation: Record<string, string> }

      for (const key of AUDIT_MESSAGE_KEYS) {
        expect(locale.translation[key], `${localeFile}: ${key}`).not.toBe(
          simplifiedChinese.translation[key]
        )
      }
    }
  })

  test('keeps English audit messages in the English source form', async () => {
    const locale = (await Bun.file(
      new URL('../../../i18n/locales/en.json', import.meta.url)
    ).json()) as { translation: Record<string, string> }

    for (const key of AUDIT_MESSAGE_KEYS) {
      expect(locale.translation[key], key).toBe(key)
    }
  })
})
