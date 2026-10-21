import { describe, expect, test } from 'bun:test'

import {
  getExplicitInterfaceLanguage,
  markExplicitInterfaceLanguage,
} from './language-preference'

describe('explicit interface language preference', () => {
  test('normalizes and persists the language selected in the browser', () => {
    const originalWindow = globalThis.window
    const values = new Map<string, string>()
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        localStorage: {
          getItem: (key: string) => values.get(key) ?? null,
          setItem: (key: string, value: string) => values.set(key, value),
        },
      },
    })

    markExplicitInterfaceLanguage('zh-CN')
    expect(getExplicitInterfaceLanguage()).toBe('zhCN')

    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: originalWindow,
    })
  })
})
