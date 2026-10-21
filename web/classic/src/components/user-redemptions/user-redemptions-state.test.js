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
import * as redemptionState from './user-redemption-state.js';
import {
  buildCreateUserRedemptionRequest,
  getMinimumRedemptionAmount,
  getRedemptionAmountValidation,
  getUserRedemptionExpiryOptions,
  getUserRedemptionExpiryTranslation,
  getUserRedemptionKindTranslationKey,
  getUserRedemptionStatusTranslationKey,
  normalizeRedemptionAmount,
  sanitizeRedemptionAmountInput,
} from './user-redemption-state.js';

const zhCNLocale = JSON.parse(
  readFileSync(
    new URL('../../i18n/locales/zh-CN.json', import.meta.url),
    'utf8',
  ),
);

const activeLocales = Object.fromEntries(
  ['en', 'zh-CN', 'zh-TW', 'fr', 'ru', 'ja', 'vi'].map((localeName) => [
    localeName,
    JSON.parse(
      readFileSync(
        new URL(`../../i18n/locales/${localeName}.json`, import.meta.url),
        'utf8',
      ),
    ),
  ]),
);

test('uses the admin collection and governance endpoints for administrators', () => {
  const source = readFileSync(
    new URL('../../pages/UserRedemption/index.jsx', import.meta.url),
    'utf8',
  );
  const apiSource = readFileSync(new URL('./api.js', import.meta.url), 'utf8');

  expect(apiSource).toContain("'/api/user-redemption/admin/'");
  expect(apiSource).toContain('`/api/user-redemption/admin/${id}/disable`');
  expect(apiSource).toContain('`/api/user-redemption/admin/${id}/enable`');
  expect(apiSource).toContain('`/api/user-redemption/admin/${id}/terminate`');
  expect(apiSource).toContain('`/api/user-redemption/admin/${id}/delete`');
  expect(apiSource).toContain("'/api/user-redemption/admin/batch-settlement'");
  expect(source).toContain('selectedIds');
  expect(source).toContain('Batch settlement');
  expect(source).toContain('showCreator');
});

test('uses red packet terminology and reliable shared copy', () => {
  const createSource = readFileSync(
    new URL('./CreateUserRedemptionModal.jsx', import.meta.url),
    'utf8',
  );
  const tableSource = readFileSync(
    new URL('./UserRedemptionsTable.jsx', import.meta.url),
    'utf8',
  );

  expect(createSource).toContain("t('Red packet')");
  expect(createSource).not.toContain("t('Random packet')");
  expect(tableSource).toContain('await copy(row.code)');
  expect(tableSource).not.toContain('navigator.clipboard');
  expect(tableSource).toContain('onView');
  expect(tableSource).toContain('getUserRedemptionKindTranslationKey');
  expect(tableSource).toContain('getUserRedemptionStatusTranslationKey');
});

test('uses the configured recipient ceiling before raising packet amount', () => {
  expect(normalizeRedemptionAmount('random', 3, 999, 1, 12)).toBe(12);
});

test('defines subscription redemption copy in every active locale', () => {
  const keys = [
    'All reward types',
    'Allow subscription codes',
    'Balance',
    'Each user can claim one subscription from this code.',
    'Enter your subscription redemption code',
    'Fixed units',
    'Forfeit remaining units',
    'No balance-purchasable subscription plans are available.',
    'Per unit',
    'Plan',
    'Redeem subscription',
    'Refund pending',
    'Refund remaining units',
    'Reward type',
    'Select a subscription plan',
    'Subscription',
    'Subscription redeemed successfully',
    'The full subscription cost is reserved from your wallet immediately.',
    'Total cost',
    'Units',
    'Unused subscription units are refunded automatically after expiry or cancellation.',
    'Unused subscription units were refunded automatically',
    'Unused subscription units will be refunded to your wallet automatically.',
  ];

  for (const locale of Object.values(activeLocales)) {
    for (const key of keys) {
      expect(Object.hasOwn(locale.translation, key)).toBe(true);
    }
  }

  expect(zhCNLocale.translation['Reward type']).toBe('奖励类型');
  expect(zhCNLocale.translation.Subscription).toBe('订阅');
  expect(zhCNLocale.translation['Fixed units']).toBe('固定份数');
  expect(zhCNLocale.translation['Refund pending']).toBe('待退款');
  expect(
    activeLocales['zh-TW'].translation[
      'Unused subscription units will be refunded to your wallet automatically.'
    ],
  ).toBe('未使用的訂閱份數將自動退回錢包。');
});

test('builds discriminated balance and subscription creation requests', () => {
  expect(
    buildCreateUserRedemptionRequest({
      rewardType: 'subscription',
      planId: 7,
      units: 2,
      expiresAt: 1000,
    }),
  ).toEqual({
    reward_type: 'subscription',
    plan_id: 7,
    units: 2,
    expires_at: 1000,
  });
  expect(
    buildCreateUserRedemptionRequest({
      rewardType: 'balance',
      kind: 'single',
      totalQuota: '500000',
      maxClaims: 1,
      expiresAt: 1000,
    }),
  ).toEqual({
    reward_type: 'balance',
    kind: 'single',
    total_quota: '500000',
    max_claims: 1,
    expires_at: 1000,
  });
});

test('supports subscription labels and excludes them from batch settlement', () => {
  expect(getUserRedemptionKindTranslationKey('fixed')).toBe('Fixed units');
  expect(getUserRedemptionStatusTranslationKey('refund_pending')).toBe(
    'Refund pending',
  );
  expect(
    redemptionState.canSettleUserRedemption?.({
      deleted: false,
      reward_type: 'balance',
      status: 'active',
    }),
  ).toBe(true);
  expect(
    redemptionState.canSettleUserRedemption?.({
      deleted: false,
      reward_type: 'subscription',
      status: 'active',
    }),
  ).toBe(false);
});

test('wires subscription creation and snapshots through classic views', () => {
  const createSource = readFileSync(
    new URL('./CreateUserRedemptionModal.jsx', import.meta.url),
    'utf8',
  );
  const tableSource = readFileSync(
    new URL('./UserRedemptionsTable.jsx', import.meta.url),
    'utf8',
  );
  const detailSource = readFileSync(
    new URL('./UserRedemptionDetailModal.jsx', import.meta.url),
    'utf8',
  );

  expect(createSource).toContain('subscriptionPlans');
  expect(createSource).toContain('buildCreateUserRedemptionRequest');
  expect(createSource).toContain("rewardType === 'subscription'");
  expect(tableSource).toContain('getUserRedemptionRewardTranslationKey');
  expect(tableSource).toContain('row.plan_title');
  expect(detailSource).toContain('getUserRedemptionRewardTranslationKey');
  expect(detailSource).toContain('detail.code.plan_title');
});

test('uses a dedicated mobile layout instead of clipping the desktop table', () => {
  const tableSource = readFileSync(
    new URL('./UserRedemptionsTable.jsx', import.meta.url),
    'utf8',
  );

  expect(tableSource).toContain('useIsMobile');
  expect(tableSource).toContain('if (isMobile)');
  expect(tableSource).toContain("className='divide-y rounded-md border'");
  expect(tableSource).toContain('break-all');
});

test('keeps the user redemption modal within the mobile viewport', () => {
  const modalSource = readFileSync(
    new URL('./MyUserRedemptionsModal.jsx', import.meta.url),
    'utf8',
  );

  expect(modalSource).toContain('useIsMobile');
  expect(modalSource).toContain(
    "width={isMobile ? 'calc(100vw - 32px)' : 1040}",
  );
  expect(modalSource).toContain(
    "maxHeight: isMobile ? 'calc(100vh - 160px)' : '75vh'",
  );
  expect(modalSource).toContain("overflowX: 'hidden'");
  expect(modalSource).toContain("overflowY: 'auto'");
  expect(modalSource).toContain(
    "className='mt-3 flex justify-center sm:justify-end'",
  );
  expect(modalSource).toContain('showTotal={!isMobile}');
});

test('keeps the create redemption modal within the mobile viewport', () => {
  const modalSource = readFileSync(
    new URL('./CreateUserRedemptionModal.jsx', import.meta.url),
    'utf8',
  );

  expect(modalSource).toContain('useIsMobile');
  expect(modalSource).toContain(
    "width={isMobile ? 'calc(100vw - 24px)' : 560}",
  );
  expect(modalSource).toContain(
    "maxHeight: isMobile ? 'calc(100vh - 160px)' : '75vh'",
  );
  expect(modalSource).toContain("overflowX: 'hidden'");
  expect(modalSource).toContain("overflowY: 'auto'");
});

test('shares explicit translation keys across redemption list and detail views', () => {
  const detailSource = readFileSync(
    new URL('./UserRedemptionDetailModal.jsx', import.meta.url),
    'utf8',
  );

  expect(getUserRedemptionKindTranslationKey('single')).toBe('Single');
  expect(getUserRedemptionKindTranslationKey('random')).toBe('Red packet');
  expect(getUserRedemptionStatusTranslationKey('active')).toBe('Active');
  expect(getUserRedemptionStatusTranslationKey('disabled')).toBe('Disabled');
  expect(getUserRedemptionStatusTranslationKey('exhausted')).toBe('Exhausted');
  expect(getUserRedemptionStatusTranslationKey('expired_pending')).toBe(
    'Pending settlement',
  );
  expect(getUserRedemptionStatusTranslationKey('refunded')).toBe('Refunded');
  expect(getUserRedemptionStatusTranslationKey('forfeited')).toBe('Forfeited');
  expect(getUserRedemptionStatusTranslationKey('deleted')).toBe('Deleted');
  expect(getUserRedemptionStatusTranslationKey('unknown')).toBe('unknown');
  expect(detailSource).toContain('getUserRedemptionStatusTranslationKey');
  expect(detailSource).not.toContain('t(detail.code.status)');
});

test('does not repeat the deleted badge when deleted is already the terminal status', () => {
  const tableSource = readFileSync(
    new URL('./UserRedemptionsTable.jsx', import.meta.url),
    'utf8',
  );

  expect(redemptionState.shouldShowDeletedBadge?.('deleted', true)).toBe(false);
  expect(redemptionState.shouldShowDeletedBadge?.('refunded', true)).toBe(true);
  expect(redemptionState.shouldShowDeletedBadge?.('deleted', false)).toBe(
    false,
  );
  expect(tableSource.match(/shouldShowDeletedBadge/g)?.length).toBe(3);
});

test('localizes every visible user-redemption action and detail label in Chinese', () => {
  const translations = {
    'Cancel redemption code': '取消兑换码',
    'Cancel redemption code?': '确认取消兑换码？',
    'Claim history': '领取记录',
    Claimed: '已领取',
    'Claimed at': '领取时间',
    Confirm: '确认',
    'Copied to clipboard': '已复制到剪贴板',
    Creator: '创建者',
    Delete: '删除',
    'Delete reason': '删除原因',
    Deleted: '已删除',
    Disable: '禁用',
    'Disabled reason': '禁用原因',
    Enable: '启用',
    'Failed to copy to clipboard': '复制到剪贴板失败',
    'Forfeit remaining balance': '没收剩余额度',
    'No claims yet.': '暂无领取记录。',
    'The unclaimed balance will be submitted for administrator settlement.':
      '未领取额度将提交管理员结算。',
    Recipient: '领取人',
    'Redemption Code Details': '兑换码详情',
    'Redemption code submitted for administrator settlement':
      '兑换码已提交管理员结算',
    'Refund remaining balance': '退还剩余额度',
    Refunded: '已退款',
    Remaining: '剩余',
    'Request failed': '请求失败',
    Settlement: '结算方式',
    'Terminal reason': '终止原因',
    Terminate: '终止',
    Total: '总额',
    'Unable to load redemption code': '无法加载兑换码',
    'Unknown user': '未知用户',
    View: '查看',
    forfeit: '没收',
    random: '随机',
    refund: '退款',
  };

  for (const [key, translation] of Object.entries(translations)) {
    expect(zhCNLocale.translation[key]).toBe(translation);
  }
});

test('localizes every administrator redemption filter control in Chinese', () => {
  const translations = {
    'All modes': '全部模式',
    'All records': '所有记录',
    'All statuses': '全部状态',
    'Created from': '创建时间起',
    'Created to': '创建时间止',
    'Creator username': '创建者用户名',
    'Expires from': '过期时间起',
    'Expires to': '过期时间止',
    'Not deleted': '未删除',
    Reset: '重置',
    Search: '搜索',
  };

  for (const [key, translation] of Object.entries(translations)) {
    expect(zhCNLocale.translation[key]).toBe(translation);
  }
});

test('defines every administrator redemption filter control in each active locale', () => {
  const keys = [
    'All modes',
    'All records',
    'All statuses',
    'Created from',
    'Created to',
    'Creator username',
    'Expires from',
    'Expires to',
    'Not deleted',
    'Reset',
    'Search',
  ];

  for (const locale of Object.values(activeLocales)) {
    for (const key of keys) {
      expect(Object.hasOwn(locale.translation, key)).toBe(true);
    }
  }
});

test('localizes the administrator redemption page description in every active locale', () => {
  const key =
    'Audit and govern wallet-funded redemption codes created by users.';

  expect(zhCNLocale.translation[key]).toBe(
    '审计并管理由用户创建且由钱包额度支持的兑换码。',
  );

  for (const locale of Object.values(activeLocales)) {
    expect(Object.hasOwn(locale.translation, key)).toBe(true);
  }
});

test('exposes full administrator governance and filters', () => {
  const source = readFileSync(
    new URL('../../pages/UserRedemption/index.jsx', import.meta.url),
    'utf8',
  );

  expect(source).toContain('AdminUserRedemptionGovernanceModal');
  expect(source).toContain('UserRedemptionDetailModal');
  expect(source).toContain('creator_username');
  expect(source).toContain("deleted: 'all'");
  expect(source).toContain('onDisable');
  expect(source).toContain('onEnable');
  expect(source).toContain('onTerminate');
  expect(source).toContain('onDelete');
  expect(source).not.toContain('showCreator={admin}');
});

test('shows the creator id in the administrator table', () => {
  const source = readFileSync(
    new URL('./UserRedemptionsTable.jsx', import.meta.url),
    'utf8',
  );

  expect(source).toContain("t('Creator ID')");
  expect(source).toContain('row.creator_user_id');
});

test('refreshes wallet balance after cancellation', () => {
  const modalSource = readFileSync(
    new URL('./MyUserRedemptionsModal.jsx', import.meta.url),
    'utf8',
  );
  const walletSource = readFileSync(
    new URL('../topup/index.jsx', import.meta.url),
    'utf8',
  );

  expect(modalSource).toContain('await onBalanceChange?.()');
  expect(walletSource).toContain('onBalanceChange={getUserQuota}');
});

test('uses localized status labels and governance titles', () => {
  const pageSource = readFileSync(
    new URL('../../pages/UserRedemption/index.jsx', import.meta.url),
    'utf8',
  );
  const governanceSource = readFileSync(
    new URL('./AdminUserRedemptionGovernanceModal.jsx', import.meta.url),
    'utf8',
  );

  expect(pageSource).toContain("label: 'Pending settlement'");
  expect(pageSource).toContain("label: 'Refunded'");
  expect(pageSource).toContain("label: 'Forfeited'");
  expect(pageSource).not.toContain('{t(status)}');
  expect(governanceSource).toContain("t('终止')");
  expect(governanceSource).not.toContain("t('Terminate redemption code')");
});

test('states the distinct cancellation rules for balance and subscription codes', () => {
  const createSource = readFileSync(
    new URL('./CreateUserRedemptionModal.jsx', import.meta.url),
    'utf8',
  );
  const listSource = readFileSync(
    new URL('./MyUserRedemptionsModal.jsx', import.meta.url),
    'utf8',
  );

  expect(createSource).toContain('submitted for administrator settlement');
  expect(createSource).toContain(
    'Unused subscription units are refunded automatically',
  );
  expect(listSource).toContain('submitted for administrator settlement');
  expect(listSource).toContain(
    'Unused subscription units will be refunded to your wallet automatically.',
  );
});

function canCancel(status) {
  return status === 'active';
}

function buildCreatePayload({ kind, amount, maxClaims, hours }) {
  return {
    kind,
    amount,
    max_claims: kind === 'random' ? maxClaims : 1,
    expires_at: Math.floor(Date.now() / 1000) + Math.max(1, hours) * 3600,
  };
}

describe('classic user-redemptions state', () => {
  test('active only for cancel', () => {
    expect(canCancel('active')).toBe(true);
    expect(canCancel('completed')).toBe(false);
  });

  test('payload for single and random', () => {
    expect(
      buildCreatePayload({ kind: 'single', amount: 1, maxClaims: 5, hours: 2 })
        .max_claims,
    ).toBe(1);
    expect(
      buildCreatePayload({ kind: 'random', amount: 3, maxClaims: 6, hours: 2 })
        .max_claims,
    ).toBe(6);
  });

  test('uses the configured wallet display unit for minimum amounts', () => {
    expect(getMinimumRedemptionAmount('single', 9, 7.2)).toBe(7.2);
    expect(getMinimumRedemptionAmount('random', 3, 7.2)).toBe(21.6);
    expect(getMinimumRedemptionAmount('single', 9, 500_000)).toBe(500_000);
  });

  test('submits raw quota as an exact decimal string', () => {
    const source = readFileSync(
      new URL('./CreateUserRedemptionModal.jsx', import.meta.url),
      'utf8',
    );

    expect(source).toContain('displayAmountToQuota');
    expect(source).toContain('buildCreateUserRedemptionRequest');
    expect(source).toContain('totalQuota: String(');
    expect(source).toContain('displayAmountToQuota(');
    expect(source).not.toContain('amount: normalizeRedemptionAmount(');
  });

  test('explains an invalid amount before disabling redemption creation', () => {
    expect(getRedemptionAmountValidation('single', 0, 1)).toEqual({
      key: 'Amount must be at least {{amount}}.',
      values: { amount: 1 },
    });
    expect(getRedemptionAmountValidation('random', 1, 2)).toEqual({
      key: 'Red packet total must be at least {{amount}} for {{count}} recipients.',
      values: { amount: 2, count: 2 },
    });
    expect(getRedemptionAmountValidation('random', 1, 3, 7.2)).toEqual({
      key: 'Red packet total must be at least {{amount}} for {{count}} recipients.',
      values: { amount: 21.6, count: 3 },
    });
    expect(getRedemptionAmountValidation('random', 2, 2)).toBeNull();
  });

  test('removes floating-point noise from the amount field', () => {
    expect(sanitizeRedemptionAmountInput(0.10000000149011612)).toBe(0.1);
    expect(sanitizeRedemptionAmountInput(12.345)).toBe(12.35);
  });

  test('offers the configured expiry ceiling and labels partial days as hours', () => {
    expect(getUserRedemptionExpiryOptions(2)).toEqual([1, 2]);
    expect(getUserRedemptionExpiryOptions(36)).toEqual([1, 24, 36]);
    expect(getUserRedemptionExpiryOptions(720)).toEqual([1, 24, 168, 720]);
    expect(getUserRedemptionExpiryTranslation(2)).toEqual({
      key: '{{count}} hours',
      values: { count: 2 },
    });
    expect(getUserRedemptionExpiryTranslation(24)).toEqual({
      key: '{{count}} days',
      values: { count: 1 },
    });
  });
});
