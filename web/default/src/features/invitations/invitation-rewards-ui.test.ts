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

const pageSource = readFileSync(
  new URL('./components/invitation-reward-approvals-page.tsx', import.meta.url),
  'utf8'
)
const sidebarSource = readFileSync(
  new URL('../../hooks/use-sidebar-data.ts', import.meta.url),
  'utf8'
)
const sidebarConfigSource = readFileSync(
  new URL('../../hooks/use-sidebar-config.ts', import.meta.url),
  'utf8'
)
const routeSource = readFileSync(
  new URL(
    '../../routes/_authenticated/invitation-rewards/index.tsx',
    import.meta.url
  ),
  'utf8'
)
const invitationSettingsPath = new URL(
  '../system-settings/billing/invitation-incentive-section.tsx',
  import.meta.url
)
const invitationSettingsSource = existsSync(invitationSettingsPath)
  ? readFileSync(invitationSettingsPath, 'utf8')
  : ''
const billingRegistrySource = readFileSync(
  new URL('../system-settings/billing/section-registry.tsx', import.meta.url),
  'utf8'
)
const quotaSettingsSource = readFileSync(
  new URL(
    '../system-settings/general/quota-settings-section.tsx',
    import.meta.url
  ),
  'utf8'
)
const walletCardSource = readFileSync(
  new URL('../wallet/components/affiliate-rewards-card.tsx', import.meta.url),
  'utf8'
)

describe('invitation reward approvals', () => {
  test('keeps reward approval separate from invitation code administration', () => {
    expect(pageSource).toContain('listInvitationRewardPayoutRequests')
    expect(pageSource).toContain('reviewInvitationRewardPayoutRequest')
    expect(pageSource).toContain("t('Invitation reward approvals')")
    expect(pageSource).toContain("t('Approve payout request')")
    expect(pageSource).toContain("t('Reject payout request')")
    expect(pageSource).toContain("t('A rejection reason is required')")
  })

  test('keeps approval history visible with freeze details and mobile cards', () => {
    expect(pageSource).toContain("value='pending'")
    expect(pageSource).toContain("value='history'")
    expect(pageSource).toContain("'frozen'")
    expect(pageSource).toContain('getInvitationRewardPayoutRequestDetail')
    expect(pageSource).toContain('freezeInvitationRewardPayoutRequest')
    expect(pageSource).toContain('unfreezeInvitationRewardPayoutRequest')
    expect(pageSource).toContain("className='space-y-3 lg:hidden'")
    expect(pageSource).toContain(
      "className='hidden min-h-0 flex-1 overflow-auto rounded-md border lg:block'"
    )
    expect(pageSource).toContain("t('Approval history')")
  })

  test('keeps desktop reward columns separated at compact desktop widths', () => {
    expect(pageSource).toContain("<Table className='min-w-[1168px]'>")
    expect(pageSource).toContain(
      "className='block max-w-52 text-xs break-all whitespace-normal'"
    )
    expect(pageSource).toContain(
      "className='max-w-52 break-words whitespace-normal'"
    )
  })

  test('exposes the controlled administrator route after user redemption codes', () => {
    expect(sidebarSource).toContain("url: '/invitation-rewards'")
    expect(sidebarSource.indexOf("url: '/invitation-rewards'")).toBeGreaterThan(
      sidebarSource.indexOf("url: '/user-redemptions'")
    )
    expect(sidebarConfigSource).toContain('invitation_reward: true')
    expect(routeSource).toContain(
      "createFileRoute('/_authenticated/invitation-rewards/')"
    )
  })

  test('provides one dedicated invitation incentive settings section', () => {
    expect(invitationSettingsSource).toContain(
      'updateInvitationIncentiveSettings'
    )
    expect(invitationSettingsSource).toContain("name='cash_commission_rate'")
    expect(invitationSettingsSource).toContain("step='0.01'")
    expect(invitationSettingsSource).toContain('min={0}')
    expect(invitationSettingsSource).not.toMatch(
      /cash_commission_rate[\s\S]{0,500}max=\{?100/
    )
    expect(invitationSettingsSource).toContain(
      'const childrenDisabled = !enabled || isSaving'
    )
    expect(invitationSettingsSource).toContain('disabled={childrenDisabled}')
    expect(billingRegistrySource).toContain("id: 'invitation-incentive'")
    expect(billingRegistrySource).toContain("titleKey: 'Invitation incentives'")
  })

  test('moves registration reward amounts out of generic quota settings', () => {
    expect(quotaSettingsSource).not.toContain('QuotaForInviter')
    expect(quotaSettingsSource).not.toContain('QuotaForInvitee')
    expect(invitationSettingsSource).toContain("name='QuotaForInviter'")
    expect(invitationSettingsSource).toContain("name='QuotaForInvitee'")
  })

  test('shows exactly the current cash cashback rate when policy is visible', () => {
    expect(walletCardSource).toContain('cash_commission_visible')
    expect(walletCardSource).toContain('cash_commission_rate')
    expect(walletCardSource).toContain("t('Top-up cashback {{rate}}%'")
    expect(walletCardSource).toContain('rewards?.cash_commission_visible ? (')
  })

  test('labels commission evidence and supports copying long source IDs', () => {
    for (const label of [
      'Trigger type',
      'Trigger ID',
      'Eligible quota',
      'Commission rate',
      'Gross reward',
      'Reversed amount',
      'Net reward',
      'Reversal status',
    ]) {
      expect(pageSource).toContain(`t('${label}')`)
    }
    expect(pageSource).toContain('<CopyButton')
    expect(pageSource).toContain('value={reward.trigger_id}')
    expect(pageSource).toContain("className='min-w-0 break-all'")
    expect(pageSource).toContain('formatQuota(request.reversed_amount)')
    expect(pageSource).toContain('formatQuota(request.net_amount)')
  })
})
