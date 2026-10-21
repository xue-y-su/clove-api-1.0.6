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

import en from './locales/en.json'
import fr from './locales/fr.json'
import ja from './locales/ja.json'
import ru from './locales/ru.json'
import vi from './locales/vi.json'
import zhTW from './locales/zh-TW.json'
import zhCN from './locales/zh.json'
import { STATIC_I18N_KEYS } from './static-keys'

const locales = { en, fr, ja, ru, vi, 'zh-TW': zhTW, zh: zhCN }

describe('static translation keys', () => {
  test('keeps dynamic invitation states and audit templates extractable', () => {
    const required = [
      'pending',
      'approved',
      'rejected',
      'withdrawn',
      'active',
      'exhausted',
      'expired',
      'revoked',
      'completed',
      'skipped',
      'failed',
      'single',
      'multi',
      'Invitation incentives',
      'Cancelled',
      'Cash top-up',
      'Cash subscription purchase',
      'Administrator balance code',
      'Administrator subscription code',
      'Not reversed',
      'Partially reversed',
      'Fully reversed',
      'User {{target_user_id}} submitted invitation application {{application_id}}',
      'Invitation code {{invite_code_id}} registered user {{registered_user_id}}',
    ]

    for (const key of required) {
      expect(STATIC_I18N_KEYS).toContain(key)
    }
  })

  test('keeps profile identity labels extractable', () => {
    expect(STATIC_I18N_KEYS).toContain('Avatar')
  })

  test('defines every dynamic static key in every production locale', () => {
    for (const [localeName, locale] of Object.entries(locales)) {
      for (const key of STATIC_I18N_KEYS) {
        expect(locale.translation[key], `${localeName}: ${key}`).toBeTruthy()
      }
    }
  })

  test('defines legacy Chinese navigation keys without falling back at runtime', () => {
    const legacyNavigationKeys = [
      '主页',
      '控制台',
      '模型广场',
      '排行榜',
      '余额查询',
      '文档',
      '关于',
    ]

    for (const key of legacyNavigationKeys) {
      expect(zhCN.translation).toHaveProperty(key)
    }
  })
})
