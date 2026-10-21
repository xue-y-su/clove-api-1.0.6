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
import { existsSync, readFileSync } from 'node:fs'

import {
  getUserRedemptionDetailQueryKey,
  getUserRedemptionListQueryKey,
  normalizeUserRedemptionDetail,
  type UserRedemptionCode,
} from './api'
import {
  calculateSubscriptionRedemptionCost,
  getMinimumRedemptionAmount,
  getRedemptionAmountValidation,
  getUserRedemptionExpiryOptions,
  getUserRedemptionExpiryTranslation,
  normalizeRedemptionAmount,
  sanitizeRedemptionAmountInput,
} from './create-state'
import {
  canDeleteUserRedemption,
  canSettleUserRedemption,
  USER_REDEMPTION_KIND_LABELS,
  USER_REDEMPTION_REWARD_LABELS,
  USER_REDEMPTION_STATUS_LABELS,
} from './types'

function canCancel(item: Pick<UserRedemptionCode, 'status'>) {
  return item.status === 'active'
}

function buildCreatePayload(input: {
  kind: 'single' | 'random'
  amount: number
  maxClaims: number
  hours: number
}) {
  return {
    kind: input.kind,
    amount: input.amount,
    max_claims: input.kind === 'random' ? input.maxClaims : 1,
    expires_at: Math.floor(Date.now() / 1000) + Math.max(1, input.hours) * 3600,
  }
}

describe('user-redemptions state', () => {
  test('translates subscription reservation copy in every active locale', () => {
    const localeNames = ['en', 'zh', 'zh-TW', 'fr', 'ru', 'ja', 'vi']
    const key =
      'The full subscription cost is reserved from your wallet immediately.'

    for (const localeName of localeNames) {
      const locale = JSON.parse(
        readFileSync(
          new URL(`../../i18n/locales/${localeName}.json`, import.meta.url),
          'utf8'
        )
      ) as { translation: Record<string, string> }
      expect(Object.hasOwn(locale.translation, key)).toBe(true)
    }

    const zhLocale = JSON.parse(
      readFileSync(
        new URL('../../i18n/locales/zh.json', import.meta.url),
        'utf8'
      )
    ) as { translation: Record<string, string> }
    expect(zhLocale.translation[key]).toBe(
      '创建后将立即从钱包冻结全部订阅费用。'
    )
  })

  test('formats stored quota through the site display unit', () => {
    const source = readFileSync(
      new URL('./components/user-redemptions-table.tsx', import.meta.url),
      'utf8'
    )
    expect(source).toContain('formatQuota(item.claimed_quota)')
    expect(source).toContain('formatQuota(item.total_quota)')
  })

  test('normalizes a missing claim history from an older API node', () => {
    const detail = normalizeUserRedemptionDetail({
      code: {} as UserRedemptionCode,
    })

    expect(detail.claims).toEqual([])
  })

  test('keeps a detail cache entry distinct from the matching list page', () => {
    expect(getUserRedemptionListQueryKey(false, 1)).not.toEqual(
      getUserRedemptionDetailQueryKey(false, 1)
    )
  })

  test('only active codes can be cancelled', () => {
    expect(canCancel({ status: 'active' })).toBe(true)
    expect(canCancel({ status: 'exhausted' })).toBe(false)
    expect(canCancel({ status: 'expired_pending' })).toBe(false)
    expect(canCancel({ status: 'refunded' })).toBe(false)
  })

  test('does not offer delete for an already deleted code', () => {
    expect(canDeleteUserRedemption({ deleted: false })).toBe(true)
    expect(canDeleteUserRedemption({ deleted: true })).toBe(false)
  })

  test('detail status labels use translated display keys instead of raw enums', () => {
    const source = readFileSync(
      new URL(
        './components/user-redemption-detail-dialog.tsx',
        import.meta.url
      ),
      'utf8'
    )
    expect(source).toContain(
      'USER_REDEMPTION_STATUS_LABELS[detail.code.status]'
    )
    expect(source).not.toContain('t(detail.code.status)')
  })

  test('uses the approved red packet label in list and detail views', () => {
    expect(USER_REDEMPTION_KIND_LABELS.random).toBe('Red packet')
    expect(USER_REDEMPTION_KIND_LABELS.fixed).toBe('Fixed units')
    expect(USER_REDEMPTION_REWARD_LABELS.subscription).toBe('Subscription')
  })

  test('uses explicit terminal and pending-settlement status labels', () => {
    expect(USER_REDEMPTION_STATUS_LABELS.exhausted).toBe('Exhausted')
    expect(USER_REDEMPTION_STATUS_LABELS.expired_pending).toBe(
      'Pending settlement'
    )
    expect(USER_REDEMPTION_STATUS_LABELS.refund_pending).toBe('Refund pending')
    expect(USER_REDEMPTION_STATUS_LABELS.refunded).toBe('Refunded')
    expect(USER_REDEMPTION_STATUS_LABELS.forfeited).toBe('Forfeited')
    expect(USER_REDEMPTION_STATUS_LABELS.deleted).toBe('Deleted')
  })

  test('only balance rewards participate in administrator batch settlement', () => {
    expect(
      canSettleUserRedemption({
        deleted: false,
        reward_type: 'balance',
        status: 'active',
      })
    ).toBe(true)
    expect(
      canSettleUserRedemption({
        deleted: false,
        reward_type: 'subscription',
        status: 'active',
      })
    ).toBe(false)
  })

  test('computes subscription redemption wallet cost with the server rounding rule', () => {
    expect(calculateSubscriptionRedemptionCost(1.234, 3, 500_000)).toEqual({
      unitQuota: 617_000,
      totalQuota: 1_851_000,
    })
    expect(calculateSubscriptionRedemptionCost(0, 4, 500_000)).toEqual({
      unitQuota: 0,
      totalQuota: 0,
    })
  })

  test('wires subscription creation and plan snapshots through the default UI', () => {
    const createSource = readFileSync(
      new URL('./components/create-dialog.tsx', import.meta.url),
      'utf8'
    )
    const tableSource = readFileSync(
      new URL('./components/user-redemptions-table.tsx', import.meta.url),
      'utf8'
    )
    const detailSource = readFileSync(
      new URL(
        './components/user-redemption-detail-dialog.tsx',
        import.meta.url
      ),
      'utf8'
    )

    expect(createSource).toContain('getPublicPlans')
    expect(createSource).toContain('buildCreateUserRedemptionRequest')
    expect(createSource).toContain("rewardType: 'subscription'")
    expect(createSource).toContain('subscriptionEnabled')
    expect(tableSource).toContain('USER_REDEMPTION_REWARD_LABELS')
    expect(tableSource).toContain('item.plan_title')
    expect(detailSource).toContain('USER_REDEMPTION_REWARD_LABELS')
    expect(detailSource).toContain('detail.code.plan_title')
  })

  test('create payload clamps single max claims to 1', () => {
    const single = buildCreatePayload({
      kind: 'single',
      amount: 2,
      maxClaims: 9,
      hours: 2,
    })
    expect(single.max_claims).toBe(1)
    expect(single.kind).toBe('single')
    const random = buildCreatePayload({
      kind: 'random',
      amount: 5,
      maxClaims: 4,
      hours: 3,
    })
    expect(random.max_claims).toBe(4)
    expect(random.expires_at).toBeGreaterThan(Math.floor(Date.now() / 1000))
  })

  test('keeps redemption actions in the wallet instead of this management page', () => {
    const source = readFileSync(new URL('./index.tsx', import.meta.url), 'utf8')
    expect(source).not.toContain('ClaimUserRedemptionDialog')
    expect(source).not.toContain("t('Redeem')")
  })

  test('uses the admin collection and cancel endpoints for administrators', () => {
    const apiSource = readFileSync(new URL('./api.ts', import.meta.url), 'utf8')
    const pageSource = readFileSync(
      new URL('./index.tsx', import.meta.url),
      'utf8'
    )

    expect(apiSource).toContain("'/api/user-redemption/admin/'")
    expect(apiSource).toContain('`/api/user-redemption/admin/${id}/cancel`')
    expect(apiSource).toContain("settlement?: 'refund' | 'forfeit'")
    expect(apiSource).toContain('reason?: string')
    expect(apiSource).toContain('settlement: options.settlement')
    expect(apiSource).toContain('reason,')
    expect(pageSource).toContain('admin: true')
    expect(pageSource).toContain('showCreator')
  })

  test('uses the standard page, select, and round selection controls', () => {
    const pageSource = readFileSync(
      new URL('./index.tsx', import.meta.url),
      'utf8'
    )
    const tableSource = readFileSync(
      new URL('./components/user-redemptions-table.tsx', import.meta.url),
      'utf8'
    )

    expect(pageSource).toContain(
      "import { SectionPageLayout } from '@/components/layout'"
    )
    expect(pageSource).toContain("from '@/components/ui/select'")
    expect(pageSource).not.toContain('<select')
    expect(tableSource).toContain(
      "import { Checkbox } from '@/components/ui/checkbox'"
    )
    expect(tableSource).not.toContain("type='checkbox'")
    expect(tableSource.match(/rounded-full/g)?.length).toBe(3)
  })

  test('shows the creator id in the administrator table', () => {
    const source = readFileSync(
      new URL('./components/user-redemptions-table.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain("t('Creator ID')")
    expect(source).toContain('item.creator_user_id')
  })

  test('shows creation time and exposes an explicit administrator refresh', () => {
    const tableSource = readFileSync(
      new URL('./components/user-redemptions-table.tsx', import.meta.url),
      'utf8'
    )
    const pageSource = readFileSync(
      new URL('./index.tsx', import.meta.url),
      'utf8'
    )
    const walletSource = readFileSync(
      new URL('../wallet/index.tsx', import.meta.url),
      'utf8'
    )

    expect(tableSource).toContain("t('Created')")
    expect(
      tableSource.match(/formatTimestamp\(item\.created_at\)/g)?.length
    ).toBe(2)
    expect(pageSource).toContain('RefreshCw')
    expect(pageSource).toContain('setReloadVersion((current) => current + 1)')
    expect(walletSource).toContain(
      "queryClient.invalidateQueries({ queryKey: ['user-redemptions'] })"
    )
  })

  test('uses two shared date ranges and keeps inclusive local-day bounds', () => {
    const pageSource = readFileSync(
      new URL('./index.tsx', import.meta.url),
      'utf8'
    )

    expect(pageSource.match(/<DateRangePicker/g) ?? []).toHaveLength(2)
    expect(pageSource).toContain("startOf('day')")
    expect(pageSource).toContain("endOf('day')")
    expect(pageSource).not.toMatch(/type=['"]date['"]/)
  })

  test('keeps a random packet total at or above its claim count', () => {
    expect(getMinimumRedemptionAmount('single', 9)).toBe(1)
    expect(getMinimumRedemptionAmount('random', 6)).toBe(6)
    expect(normalizeRedemptionAmount('random', 3, 6)).toBe(6)
    expect(normalizeRedemptionAmount('random', 8, 6)).toBe(8)
  })

  test('uses the configured recipient ceiling before raising packet amount', () => {
    expect(normalizeRedemptionAmount('random', 3, 999, 1, 12)).toBe(12)
  })

  test('uses the configured wallet display unit for minimum amounts', () => {
    expect(getMinimumRedemptionAmount('single', 9, 7.2)).toBe(7.2)
    expect(getMinimumRedemptionAmount('random', 3, 7.2)).toBe(21.6)
    expect(getMinimumRedemptionAmount('single', 9, 500_000)).toBe(500_000)
    expect(normalizeRedemptionAmount('random', 8, 2, 7.2)).toBe(14.4)
  })

  test('refreshes the initial amount after currency config hydration', () => {
    const source = readFileSync(
      new URL('./components/create-dialog.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain(
      "import { useEffect, useMemo, useState } from 'react'"
    )
    expect(source).toContain('if (!props.open) return')
    expect(source).toContain('setAmount(minimumUnitAmount)')
    expect(source).toContain('minimumUnitAmount,')
    expect(source).toContain('props.open,')
  })

  test('submits raw quota as an exact decimal string', () => {
    const source = readFileSync(
      new URL('./components/create-dialog.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain('parseQuotaFromDollars')
    expect(source).toContain('buildCreateUserRedemptionRequest')
    expect(source).toContain('totalQuota: String(')
    expect(source).toContain('parseQuotaFromDollars(')
    expect(source).not.toContain('amount: normalizeRedemptionAmount(')
  })

  test('explains an invalid amount before disabling redemption creation', () => {
    expect(getRedemptionAmountValidation('single', 0, 1)).toEqual({
      key: 'Amount must be at least {{amount}}.',
      values: { amount: 1 },
    })
    expect(getRedemptionAmountValidation('random', 1, 2)).toEqual({
      key: 'Red packet total must be at least {{amount}} for {{count}} recipients.',
      values: { amount: 2, count: 2 },
    })
    expect(getRedemptionAmountValidation('random', 1, 3, 7.2)).toEqual({
      key: 'Red packet total must be at least {{amount}} for {{count}} recipients.',
      values: { amount: 21.6, count: 3 },
    })
    expect(getRedemptionAmountValidation('random', 2, 2)).toBeNull()
  })

  test('removes floating-point noise from the amount field', () => {
    expect(sanitizeRedemptionAmountInput(0.10000000149011612)).toBe(0.1)
    expect(sanitizeRedemptionAmountInput(12.345)).toBe(12.35)
  })

  test('offers the configured expiry ceiling and labels partial days as hours', () => {
    expect(getUserRedemptionExpiryOptions(2)).toEqual([1, 2])
    expect(getUserRedemptionExpiryOptions(36)).toEqual([1, 24, 36])
    expect(getUserRedemptionExpiryOptions(720)).toEqual([1, 24, 168, 720])
    expect(getUserRedemptionExpiryTranslation(2)).toEqual({
      key: '{{count}} hours',
      values: { count: 2 },
    })
    expect(getUserRedemptionExpiryTranslation(24)).toEqual({
      key: '{{count}} days',
      values: { count: 1 },
    })
  })

  test('uses governance DTO fields and administrator mutation endpoints', () => {
    const apiSource = readFileSync(new URL('./api.ts', import.meta.url), 'utf8')

    expect(apiSource).toContain('remaining_quota: number')
    expect(apiSource).toContain('forfeited_quota: number')
    expect(apiSource).toContain('deleted: boolean')
    expect(apiSource).toContain('creator_username?: string')
    expect(apiSource).toContain('`/api/user-redemption/admin/${id}/disable`')
    expect(apiSource).toContain('`/api/user-redemption/admin/${id}/enable`')
    expect(apiSource).toContain('`/api/user-redemption/admin/${id}/terminate`')
    expect(apiSource).toContain('`/api/user-redemption/admin/${id}/delete`')
  })

  test('uses the shared clipboard helper and exposes detail actions', () => {
    const source = readFileSync(
      new URL('./components/user-redemptions-table.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain('useCopyToClipboard')
    expect(source).toContain('tableProps.onView')
    expect(source).not.toContain('navigator.clipboard')
  })

  test('uses a dedicated mobile layout instead of clipping the desktop table', () => {
    const source = readFileSync(
      new URL('./components/user-redemptions-table.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain(
      "className='hidden max-w-full overflow-x-auto rounded-md border md:block'"
    )
    expect(source).toContain("className='w-full min-w-[1320px] text-sm'")
    expect(source).toContain("className='divide-y rounded-md border md:hidden'")
    expect(source).toContain('[overflow-wrap:anywhere]')
  })

  test('exposes full administrator governance from the audit page', () => {
    const pageSource = readFileSync(
      new URL('./index.tsx', import.meta.url),
      'utf8'
    )

    expect(pageSource).toContain('AdminUserRedemptionGovernanceDialog')
    expect(pageSource).toContain('UserRedemptionDetailDialog')
    expect(pageSource).toContain('creator_username')
    expect(pageSource).toContain("deleted: 'all'")
    expect(pageSource).toContain('onDisable')
    expect(pageSource).toContain('onEnable')
    expect(pageSource).toContain('onTerminate')
    expect(pageSource).toContain('onDelete')
    expect(pageSource).not.toContain('onCancel={onCancel}')
  })

  test('requires an administrator to choose settlement for live codes', () => {
    const dialogUrl = new URL(
      './components/admin-governance-dialog.tsx',
      import.meta.url
    )
    expect(existsSync(dialogUrl)).toBe(true)
    if (!existsSync(dialogUrl)) return

    const source = readFileSync(dialogUrl, 'utf8')
    expect(source).toContain("'refund'")
    expect(source).toContain("'forfeit'")
    expect(source).toContain('settlementRequired')
    expect(source).toContain('reason.trim()')
    expect(source).toContain("props.target?.reward_type === 'subscription'")
    expect(source).toContain("t('Refund remaining units')")
  })

  test('states the distinct balance and subscription cancellation rules', () => {
    const createSource = readFileSync(
      new URL('./components/create-dialog.tsx', import.meta.url),
      'utf8'
    )
    const listSource = readFileSync(
      new URL('./components/my-user-redemptions-dialog.tsx', import.meta.url),
      'utf8'
    )

    expect(createSource).toContain('submitted for administrator settlement')
    expect(listSource).toContain('submitted for administrator settlement')
    expect(createSource).toContain(
      'Unused subscription units are refunded automatically'
    )
    expect(listSource).toContain(
      'Unused subscription units will be refunded to your wallet automatically.'
    )
  })
})
