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

import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';

import {
  formatBalanceQuota,
  getBalanceMetricValues,
  getBalanceProgressSegments,
  getBalanceQueryErrorPresentation,
  getBalanceStatusExplanationKey,
  getTokenCallableQuota,
  shouldShowWalletBalance,
} from './balance';

const pageSource = readFileSync(
  new URL('./index.jsx', import.meta.url),
  'utf8',
);

function calculation(overrides = {}) {
  return {
    token_unlimited: false,
    token_total: 150,
    token_remaining: 100,
    token_used: 50,
    token_callable_quota: { unlimited: false, value: 60 },
    current_available: { unlimited: false, value: 1000 },
    restricted: 40,
    single_request_limit: { unlimited: false, value: 60 },
    participating_sources: ['subscription'],
    restriction_reason: 'subscription_quota',
    ...overrides,
  };
}

const display = {
  type: 'USD',
  quota_per_unit: 1,
  exchange_rate: 1,
  symbol: '$',
};

describe('classic token balance presentation', () => {
  test('accepts custom keys without implying an sk prefix', () => {
    expect(pageSource).toContain("placeholder={t('请输入 API Key')}");
    expect(pageSource).not.toContain("placeholder='sk-...'");
  });

  test('preserves finite available, used, and restricted proportions', () => {
    const result = getBalanceProgressSegments(calculation());

    expect(result.available).toBeCloseTo(40);
    expect(result.used).toBeCloseTo(100 / 3);
    expect(result.restricted).toBeCloseTo(80 / 3);
  });

  test('omits the restricted segment for an unlimited key', () => {
    const result = getBalanceProgressSegments(
      calculation({
        token_unlimited: true,
        token_total: 0,
        token_remaining: 0,
        token_used: 40,
        restricted: 0,
        restriction_reason: '',
      }),
    );

    expect(result).toEqual({ available: 60, used: 40, restricted: 0 });
  });

  test('switches the third metric between restricted and unlimited', () => {
    expect(getBalanceMetricValues(calculation()).third).toEqual({
      kind: 'restricted',
      value: 40,
    });
    expect(
      getBalanceMetricValues(
        calculation({ token_unlimited: true, restricted: 0 }),
      ).third,
    ).toEqual({ kind: 'unlimited', value: 0 });
  });

  test('uses token callable quota instead of wallet spendable balance for key availability', () => {
    expect(getTokenCallableQuota(calculation())).toEqual({
      unlimited: false,
      value: 60,
    });
    expect(getBalanceMetricValues(calculation()).available).toEqual({
      unlimited: false,
      value: 60,
    });
  });

  test('formats unlimited without a numeric sentinel', () => {
    expect(
      formatBalanceQuota({ unlimited: true, value: 0 }, display, '无限制'),
    ).toBe('无限制');
  });

  test('maps rate limits and unavailable key statuses', () => {
    expect(getBalanceQueryErrorPresentation('rate_limited', 60)).toEqual({
      titleKey: '请求过于频繁',
      descriptionKey: '{{seconds}} 秒后可重新查询。',
      tone: 'warning',
      values: { seconds: 60 },
    });
    expect(getBalanceStatusExplanationKey('expired')).toBe(
      '此 API 密钥已过期。',
    );
    expect(getBalanceStatusExplanationKey('active')).toBeNull();
  });

  test('shows wallet balance whenever the snapshot includes wallet funding', () => {
    const wallet = {
      available: { unlimited: false, value: 100 },
      used: 0,
      total: { unlimited: false, value: 100 },
    };

    expect(shouldShowWalletBalance({ wallet }, 'subscription_only')).toBe(true);
    expect(shouldShowWalletBalance({ wallet }, 'subscription_first')).toBe(
      true,
    );
    expect(shouldShowWalletBalance({}, 'wallet_only')).toBe(false);
  });
});
