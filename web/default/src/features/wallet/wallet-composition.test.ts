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
import { readFileSync } from 'node:fs'

const defaultLocales = ['en', 'fr', 'ja', 'ru', 'vi', 'zh', 'zh-TW'].map(
  (locale) =>
    JSON.parse(
      readFileSync(
        new URL(`../../i18n/locales/${locale}.json`, import.meta.url),
        'utf8'
      )
    ) as { translation: Record<string, string> }
)

const walletSource = readFileSync(
  new URL('./index.tsx', import.meta.url),
  'utf8'
)
const rechargeFormSource = readFileSync(
  new URL('./components/recharge-form-card.tsx', import.meta.url),
  'utf8'
)
const redemptionHookSource = readFileSync(
  new URL('./hooks/use-redemption.ts', import.meta.url),
  'utf8'
)
const sidebarSource = readFileSync(
  new URL('../../hooks/use-sidebar-data.ts', import.meta.url),
  'utf8'
)
const adminPageSource = readFileSync(
  new URL('../user-redemptions/index.tsx', import.meta.url),
  'utf8'
)
const myRedemptionsSource = readFileSync(
  new URL(
    '../user-redemptions/components/my-user-redemptions-dialog.tsx',
    import.meta.url
  ),
  'utf8'
)
const subscriptionSource = readFileSync(
  new URL('./components/subscription-plans-card.tsx', import.meta.url),
  'utf8'
)
const affiliateRewardsSource = readFileSync(
  new URL('./components/affiliate-rewards-card.tsx', import.meta.url),
  'utf8'
)
const sidebarConfigSource = readFileSync(
  new URL('../../hooks/use-sidebar-config.ts', import.meta.url),
  'utf8'
)
const sidebarSettingsSource = readFileSync(
  new URL(
    '../system-settings/maintenance/sidebar-modules-section.tsx',
    import.meta.url
  ),
  'utf8'
)
const sidebarDefaultsSource = readFileSync(
  new URL('../system-settings/maintenance/config.ts', import.meta.url),
  'utf8'
)

describe('wallet redemption composition', () => {
  test('uses one server-detected redemption entry for balance and subscription codes', () => {
    expect(redemptionHookSource).toContain('claimRedemptionCode')
    expect(redemptionHookSource).toContain(
      "result.reward_type === 'subscription'"
    )
    expect(redemptionHookSource).not.toContain('claimUserRedemption')
    expect(redemptionHookSource).not.toContain('expectedRewardType')
    expect(walletSource).not.toContain('redemptionRewardType')
    expect(rechargeFormSource).not.toContain('redemptionRewardType')
    expect(rechargeFormSource).toContain("t('Enter your redemption code')")
    expect(rechargeFormSource).toContain("t('Redeem Code')")
  })

  test('localizes the subscription redemption success message in every locale', () => {
    for (const locale of defaultLocales) {
      expect(
        locale.translation['Subscription redeemed successfully']
      ).toBeTruthy()
    }
  })

  test('keeps preset pricing readable in sidebar-constrained content', () => {
    expect(rechargeFormSource).toContain('@lg/content:grid-cols-3')
    expect(rechargeFormSource).toContain('@3xl/content:grid-cols-4')
    expect(rechargeFormSource).toContain('h-auto min-h-20')
    expect(rechargeFormSource).not.toContain("{' • '}")
  })

  test('keeps user redemption creation and history inside the wallet', () => {
    expect(walletSource).toContain('CreateUserRedemptionDialog')
    expect(walletSource).toContain('MyUserRedemptionsDialog')
    expect(walletSource).toContain("t('Create Redemption Code')")
    expect(walletSource).toContain("t('My Redemption Codes')")
    expect(walletSource).not.toContain("to='/user-redemptions'")
  })

  test('removes the wallet ledger from the user interface', () => {
    expect(walletSource).not.toContain('WalletLedgerTable')
  })

  test('keeps the tall subscription workflow in the wallet main flow', () => {
    expect(walletSource).toContain("className='grid gap-4'")
    expect(walletSource).not.toContain(
      'xl:grid-cols-[minmax(0,1.05fr)_minmax(360px,0.95fr)]'
    )
    expect(subscriptionSource).toContain('xl:grid-cols-2')
  })

  test('keeps invitation rewards at content height', () => {
    expect(affiliateRewardsSource).not.toContain(
      "className='bg-muted/20 h-full"
    )
    expect(affiliateRewardsSource).toContain(
      "className='bg-muted/20 self-start"
    )
  })

  test('shows frozen invitation rewards without changing quota formatting', () => {
    expect(affiliateRewardsSource).toContain("t('Frozen')")
    expect(affiliateRewardsSource).toContain(
      'formatQuota(rewards?.frozen ?? 0)'
    )
  })

  test('submits aggregate invitation rewards for approval without crediting the wallet', () => {
    expect(affiliateRewardsSource).toContain('getInvitationRewardSummary')
    expect(affiliateRewardsSource).toContain('submitInvitationRewardPayout')
    expect(affiliateRewardsSource).toContain("t('Invited users')")
    expect(affiliateRewardsSource).toContain("t('Total earned')")
    expect(affiliateRewardsSource).toContain("t('Available')")
    expect(affiliateRewardsSource).toContain("t('Pending payout')")
    expect(affiliateRewardsSource).toContain("t('Paid')")
    expect(affiliateRewardsSource).toContain("t('Submit payout review')")
    expect(affiliateRewardsSource).toContain('(rewards?.available ?? 0) <= 0')
    expect(walletSource).not.toContain('TransferDialog')
    expect(walletSource).not.toContain('useAffiliate')
  })

  test('keeps subscription-only preference visible when no subscription is active', () => {
    expect(subscriptionSource).toContain('value={billingPreference}')
    expect(subscriptionSource).not.toContain(
      "disablePref && isSubPref ? 'wallet_first' : billingPreference"
    )
    expect(subscriptionSource).toContain(
      'Subscription-only mode has no active subscription'
    )
  })

  test('uses the saved subscription-first label in its no-subscription notice', () => {
    expect(subscriptionSource).toContain("pref: t('Subscription First')")
    expect(subscriptionSource).not.toContain(
      "pref:\n                    billingPreference === 'subscription_only'"
    )
  })

  test('refreshes wallet balance after a user cancels a redemption code', () => {
    expect(walletSource).toContain('onBalanceChange={fetchUser}')
    expect(myRedemptionsSource).toContain('await props.onBalanceChange?.()')
  })

  test('keeps user redemption administration in the admin section only', () => {
    const personalSection = sidebarSource.slice(
      sidebarSource.indexOf("id: 'personal'"),
      sidebarSource.indexOf("id: 'admin'")
    )
    const adminSection = sidebarSource.slice(
      sidebarSource.indexOf("id: 'admin'")
    )

    expect(personalSection).not.toContain("url: '/user-redemptions'")
    expect(adminSection).toContain("url: '/user-redemptions'")
    expect(adminPageSource).not.toContain('CreateUserRedemptionDialog')
    expect(sidebarConfigSource).toMatch(
      /admin:\s*\{[\s\S]*?user_redemption:\s*true/
    )
    expect(sidebarDefaultsSource).toMatch(
      /admin:\s*\{[\s\S]*?user_redemption:\s*true/
    )
    expect(sidebarSettingsSource).toContain('user_redemption: {')
  })
})
