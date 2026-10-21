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
import { afterEach, expect, test } from 'bun:test'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import * as formatModule from './format'

const subscriptionSources = await Promise.all(
  [
    '../components/subscriptions-columns.tsx',
    '../components/dialogs/subscription-purchase-dialog.tsx',
    '../components/dialogs/user-subscriptions-dialog.tsx',
    '../../wallet/components/subscription-plans-card.tsx',
  ].map((path) => Bun.file(new URL(path, import.meta.url)).text())
)

const originalCurrency = useSystemConfigStore.getState().config.currency

afterEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: { ...originalCurrency },
    },
  }))
})

test('formats subscription prices with the configured custom currency symbol', () => {
  const formatter = (
    formatModule as typeof formatModule & {
      formatSubscriptionPrice?: (amount: unknown) => string
    }
  ).formatSubscriptionPrice

  expect(typeof formatter).toBe('function')
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CUSTOM',
        customCurrencySymbol: '✿',
        customCurrencyExchangeRate: 1,
      },
    },
  }))

  expect(formatter?.(12)).toBe('✿ 12.00')
  expect(formatter?.('12.5')).toBe('✿ 12.50')
  expect(formatter?.(Number.NaN)).toBe('✿ 0.00')
})

test('uses the shared currency formatter in every subscription price view', () => {
  for (const source of subscriptionSources) {
    expect(source).toContain('formatSubscriptionPrice')
  }
})
