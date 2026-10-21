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
import { afterEach, describe, expect, test } from 'bun:test'

import { copyToClipboard } from './copy-to-clipboard'

const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window')
const originalDocument = Object.getOwnPropertyDescriptor(globalThis, 'document')
const originalNavigator = Object.getOwnPropertyDescriptor(
  globalThis,
  'navigator'
)

function restoreGlobal(
  key: 'window' | 'document' | 'navigator',
  descriptor: PropertyDescriptor | undefined
) {
  if (descriptor) {
    Object.defineProperty(globalThis, key, descriptor)
    return
  }
  Reflect.deleteProperty(globalThis, key)
}

afterEach(() => {
  restoreGlobal('window', originalWindow)
  restoreGlobal('document', originalDocument)
  restoreGlobal('navigator', originalNavigator)
})

describe('copyToClipboard', () => {
  test('does not run the fallback while the Clipboard API write is pending', async () => {
    let resolveClipboardWrite: () => void = () => undefined
    const clipboardWrite = new Promise<void>((resolve) => {
      resolveClipboardWrite = resolve
    })
    let fallbackCalled = false
    const textArea = {
      value: '',
      style: {} as CSSStyleDeclaration,
      setAttribute: () => undefined,
      focus: () => undefined,
      select: () => undefined,
      setSelectionRange: () => undefined,
    }
    const selection = {
      removeAllRanges: () => undefined,
      addRange: () => undefined,
    }

    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: { getSelection: () => selection },
    })
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: {
        body: {
          appendChild: () => undefined,
          removeChild: () => undefined,
        },
        createElement: () => textArea,
        createRange: () => ({ selectNodeContents: () => undefined }),
        execCommand: () => {
          fallbackCalled = true
          return true
        },
      },
    })
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: {
        clipboard: {
          writeText: () => clipboardWrite,
        },
      },
    })

    const resultPromise = copyToClipboard('fresh-code')
    const fallbackStartedBeforeResolution = fallbackCalled
    resolveClipboardWrite()

    expect(await resultPromise).toBe(true)
    expect(fallbackStartedBeforeResolution).toBe(false)
    expect(fallbackCalled).toBe(false)
  })

  test('uses the fallback after the Clipboard API rejects', async () => {
    let fallbackCalled = false
    const textArea = {
      value: '',
      style: {} as CSSStyleDeclaration,
      setAttribute: () => undefined,
      focus: () => undefined,
      select: () => undefined,
      setSelectionRange: () => undefined,
    }
    const selection = {
      removeAllRanges: () => undefined,
      addRange: () => undefined,
    }

    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: { getSelection: () => selection },
    })
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: {
        body: {
          appendChild: () => undefined,
          removeChild: () => undefined,
        },
        createElement: () => textArea,
        createRange: () => ({ selectNodeContents: () => undefined }),
        execCommand: () => {
          fallbackCalled = true
          return true
        },
      },
    })
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: {
        clipboard: {
          writeText: async () => {
            throw new Error('clipboard permission denied')
          },
        },
      },
    })

    expect(await copyToClipboard('fallback-code')).toBe(true)
    expect(fallbackCalled).toBe(true)
    expect(textArea.value).toBe('fallback-code')
  })

  test('uses the synchronous fallback first on HTTP pages', async () => {
    let clipboardWriteCalled = false
    let userActivationAvailable = true
    const textArea = {
      value: '',
      style: {} as CSSStyleDeclaration,
      setAttribute: () => undefined,
      focus: () => undefined,
      select: () => undefined,
      setSelectionRange: () => undefined,
    }
    const selection = {
      removeAllRanges: () => undefined,
      addRange: () => undefined,
    }

    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        getSelection: () => selection,
        location: { protocol: 'http:' },
      },
    })
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: {
        body: {
          appendChild: () => undefined,
          removeChild: () => undefined,
        },
        createElement: () => textArea,
        createRange: () => ({ selectNodeContents: () => undefined }),
        execCommand: () => userActivationAvailable,
      },
    })
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: {
        clipboard: {
          writeText: async () => {
            clipboardWriteCalled = true
            await Promise.resolve()
            userActivationAvailable = false
            throw new Error('clipboard permission denied')
          },
        },
      },
    })

    expect(await copyToClipboard('http-code')).toBe(true)
    expect(textArea.value).toBe('http-code')
    expect(clipboardWriteCalled).toBe(false)
  })
})
