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
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const walletSource = readFileSync(
  new URL('./index.jsx', import.meta.url),
  'utf8',
);
const sidebarSource = readFileSync(
  new URL('../layout/SiderBar.jsx', import.meta.url),
  'utf8',
);
const adminPageSource = readFileSync(
  new URL('../../pages/UserRedemption/index.jsx', import.meta.url),
  'utf8',
);
const redemptionModalSource = readFileSync(
  new URL('../user-redemptions/CreateUserRedemptionModal.jsx', import.meta.url),
  'utf8',
);
const rechargeSource = readFileSync(
  new URL('./RechargeCard.jsx', import.meta.url),
  'utf8',
);
const subscriptionSource = readFileSync(
  new URL('./SubscriptionPlansCard.jsx', import.meta.url),
  'utf8',
);
const zhCNLocale = JSON.parse(
  readFileSync(
    new URL('../../i18n/locales/zh-CN.json', import.meta.url),
    'utf8',
  ),
);
const zhLocale = JSON.parse(
  readFileSync(new URL('../../i18n/locales/zh.json', import.meta.url), 'utf8'),
);

describe('classic wallet redemption composition', () => {
  test('uses one server-detected redemption entry for every code type', () => {
    expect(walletSource).toContain("API.post('/api/redemption/claim'");
    expect(walletSource).toContain("result.reward_type === 'subscription'");
    expect(walletSource).not.toContain('redemptionRewardType');
    expect(walletSource).not.toContain('expected_reward_type');
    expect(rechargeSource).not.toContain('redemptionRewardType');
    expect(rechargeSource).toContain("placeholder={t('请输入兑换码')}");
    expect(rechargeSource).toContain("t('Redeem Code')");
    expect(zhLocale.translation['Redeem Code']).toBe('兑换码核销');
  });

  test('keeps creation and personal history inside the wallet', () => {
    expect(walletSource).toContain('CreateUserRedemptionModal');
    expect(walletSource).toContain('MyUserRedemptionsModal');
    expect(walletSource).toContain("t('Create Redemption Code')");
    expect(walletSource).toContain("t('My Redemption Codes')");
  });

  test('removes the wallet ledger user interface', () => {
    expect(walletSource).not.toContain('WalletLedgerTable');
  });

  test('keeps the tall subscription workflow in the wallet main flow', () => {
    expect(walletSource).toContain("className='grid grid-cols-1 gap-6'");
    expect(walletSource).not.toContain('lg:grid-cols-2');
  });

  test('keeps subscription-only preference visible when no subscription is active', () => {
    expect(subscriptionSource).toContain('value={billingPreference}');
    expect(subscriptionSource).not.toContain(
      'disableSubscriptionPreference && isSubscriptionPreference',
    );
    expect(subscriptionSource).toContain(
      'Subscription-only mode has no active subscription',
    );
  });

  test('keeps the standalone page in the admin navigation only', () => {
    const financeSection = sidebarSource.slice(
      sidebarSource.indexOf('const financeItems'),
      sidebarSource.indexOf('const adminItems'),
    );
    const adminSection = sidebarSource.slice(
      sidebarSource.indexOf('const adminItems'),
    );

    expect(financeSection).not.toContain("itemKey: 'user_redemption'");
    expect(adminSection).toContain("itemKey: 'user_redemption'");
    expect(adminPageSource).not.toContain('CreateUserRedemptionModal');
  });

  test('localizes every user-facing redemption creation instruction in Chinese', () => {
    const translations = {
      Close: '关闭',
      'Redemption code created': '兑换码已创建',
      'Copy it now and share it only with the recipient.':
        '请立即复制，并仅分享给领取人。',
      'Copy redemption code': '复制兑换码',
      'Create another code': '再创建一个兑换码',
      'Redemption mode': '兑换模式',
      'Red packet': '红包',
      Recipients: '领取人数',
      Validity: '有效期',
      Important: '注意事项',
      'The full amount is reserved from your wallet immediately.':
        '创建后将立即从钱包中冻结全部额度。',
      'A single code gives all remaining balance to the first recipient.':
        '单次兑换码会将全部剩余额度发放给首位领取人。',
      'Each user can claim a red packet only once; the final recipient receives the remainder.':
        '每位用户仅可领取一次红包，最后一位领取人将获得剩余额度。',
      'Unused balance is submitted for administrator settlement after expiry or when you cancel an active code.':
        '兑换码过期或主动取消后，未使用额度将进入管理员结算。',
      'Administrators may terminate a code with or without a refund.':
        '管理员可选择退款或不退款后终止兑换码。',
      'Treat the code as a transferable credential and share it carefully.':
        '兑换码可转让，请谨慎分享。',
      'Amount uses the same display unit as your wallet.':
        '金额与钱包使用相同的显示单位。',
    };

    for (const [key, translation] of Object.entries(translations)) {
      expect(redemptionModalSource).toContain(key);
      expect(zhCNLocale.translation[key]).toBe(translation);
    }

    expect(redemptionModalSource).toContain(
      'getUserRedemptionExpiryTranslation',
    );
    expect(zhCNLocale.translation['1 hour']).toBe('1 小时');
    expect(zhCNLocale.translation['{{count}} hours']).toBe('{{count}} 小时');
    expect(zhCNLocale.translation['{{count}} days']).toBe('{{count}} 天');
  });
});
