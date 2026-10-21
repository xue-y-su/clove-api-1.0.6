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

import {
  convertDetectedLanguage,
  normalizeInterfaceLanguage,
  toHtmlLanguageTag,
} from './languages'

describe('interface language helpers', () => {
  test('normalizes empty and generic Chinese values to a supported simplified Chinese locale', () => {
    expect(normalizeInterfaceLanguage()).toBe('zhCN')
    expect(normalizeInterfaceLanguage('')).toBe('zhCN')
    expect(normalizeInterfaceLanguage('zh')).toBe('zhCN')
    expect(normalizeInterfaceLanguage('zh-Hans')).toBe('zhCN')
    expect(normalizeInterfaceLanguage('zh_CN')).toBe('zhCN')
  })

  test('normalizes traditional Chinese values to the supported traditional locale', () => {
    expect(normalizeInterfaceLanguage('zh-TW')).toBe('zhTW')
    expect(normalizeInterfaceLanguage('zh_Hant')).toBe('zhTW')
    expect(normalizeInterfaceLanguage('zh-HK')).toBe('zhTW')
  })

  test('maps browser Chinese locales onto supported i18next resource names', () => {
    expect(convertDetectedLanguage('zh-CN')).toBe('zhCN')
    expect(convertDetectedLanguage('zh')).toBe('zhCN')
    expect(convertDetectedLanguage('zh-Hant-TW')).toBe('zhTW')
  })

  test('maps internal interface language codes to valid HTML language tags', () => {
    expect(toHtmlLanguageTag('zhCN')).toBe('zh-CN')
    expect(toHtmlLanguageTag('zhTW')).toBe('zh-TW')
    expect(toHtmlLanguageTag('fr')).toBe('fr')
  })
})
