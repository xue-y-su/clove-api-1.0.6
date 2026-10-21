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

import * as formatModule from './subscriptionFormat';

const subscriptionSources = await Promise.all(
  [
    '../components/topup/SubscriptionPlansCard.jsx',
    '../components/topup/modals/SubscriptionPurchaseModal.jsx',
    '../components/table/subscriptions/SubscriptionsColumnDefs.jsx',
    '../components/table/users/modals/UserSubscriptionsModal.jsx',
  ].map((path) => Bun.file(new URL(path, import.meta.url)).text()),
);

test('formats subscription prices as RMB amounts', () => {
  expect(typeof formatModule.formatSubscriptionPrice).toBe('function');
  expect(formatModule.formatSubscriptionPrice?.(12)).toBe('¥12.00');
  expect(formatModule.formatSubscriptionPrice?.('12.5')).toBe('¥12.50');
  expect(formatModule.formatSubscriptionPrice?.(Number.NaN)).toBe('¥0.00');
});

test('uses the shared RMB formatter in every subscription price view', () => {
  for (const source of subscriptionSources) {
    expect(source).toContain('formatSubscriptionPrice');
  }
});
