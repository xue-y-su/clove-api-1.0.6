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

import type { LogOtherData } from '../types'
import { renderAuditContent } from './format'

function t(template: string, opts?: Record<string, unknown>): string {
  return template.replaceAll(/{{([^}]+)}}/g, (_, key: string) =>
    String(opts?.[key] ?? '')
  )
}

describe('usage log audit action rendering', () => {
  test.each([
    [
      'token.create',
      { name: 'main', token_id: 7 },
      'Created API token main (ID: 7)',
    ],
    [
      'token.secret_reveal',
      { name: 'main', token_id: 7 },
      'Viewed API token key main (ID: 7)',
    ],
    [
      'token.status_update',
      { name: 'main', status: 2 },
      'Updated API token main status to 2',
    ],
    ['redemption.view', { id: 9 }, 'Viewed redemption code 9'],
    [
      'redemption.status_update',
      { id: 9, status: 2 },
      'Updated redemption code 9 status to 2',
    ],
    [
      'user.disable',
      { username: 'alice', id: 3 },
      'Disabled user alice (ID: 3)',
    ],
    [
      'user.role_change',
      { username: 'alice', role_from: 1, role_to: 10 },
      'Changed user alice role from 1 to 10',
    ],
    [
      'user.binding_clear_self',
      { binding_type: 'github' },
      'Cleared github binding',
    ],
    [
      'user.redeem_code',
      { quota: '$1.00' },
      'Redeemed a redemption code for $1.00',
    ],
    [
      'channel.status_update',
      { id: 4, status: 1 },
      'Updated channel 4 status to 1',
    ],
    [
      'channel.status_update_batch',
      { count: 2, total: 3, status: 2 },
      'Batch updated 2/3 channels to status 2',
    ],
    [
      'channel.upstream_detect_all',
      { task_id: 'task-1' },
      'Queued upstream model detection task task-1',
    ],
    [
      'subscription.plan_reset',
      { plan_id: 8 },
      'Reset active subscriptions for plan 8',
    ],
    [
      'subscription.user_plan_reset',
      { plan_id: 8, target_user_id: 5 },
      'Reset active plan 8 subscriptions for user 5',
    ],
    ['billing.topup', { trade_no: 'T123' }, 'Recorded top-up event T123'],
    [
      'billing.consume',
      { request_id: 'req-1' },
      'Recorded consumption event req-1',
    ],
    ['billing.refund', { request_id: 'req-1' }, 'Recorded refund event req-1'],
    [
      'sensitive.token_balance_query',
      { masked_key: 'sk-abcd**********wxyz', result: 'success' },
      'Queried API token balance sk-abcd**********wxyz (success)',
    ],
    [
      'playground.web_search',
      { model: 'gpt-4.1' },
      'Used playground web search with model gpt-4.1',
    ],
    [
      'playground.image_generation',
      { model: 'gpt-image-1' },
      'Generated playground image with model gpt-image-1',
    ],
    [
      'invitation.application_created',
      { target_user_id: 5, application_id: 11 },
      'User 5 submitted invitation application 11',
    ],
    [
      'invitation.application_withdrawn',
      { target_user_id: 5, application_id: 11 },
      'User 5 withdrew invitation application 11',
    ],
    [
      'invitation.application_reviewed',
      { target_user_id: 5, application_id: 11, status: 'approved' },
      'Reviewed invitation application 11 for user 5: Approved',
    ],
    [
      'invitation.code_issued',
      { target_user_id: 5, invite_code_id: 17 },
      'Issued invitation code 17 to user 5',
    ],
    [
      'invitation.code_revoked',
      { target_user_id: 5, invite_code_id: 17 },
      'Revoked invitation code 17 for user 5',
    ],
    [
      'invitation.code_used',
      { invite_code_id: 17, registered_user_id: 23 },
      'Invitation code 17 registered user 23',
    ],
    [
      'invitation.reward_payout_frozen',
      { request_id: 29, target_user_id: 5, amount: 6400 },
      'Froze invitation reward payout request 29 for user 5: 6400',
    ],
    [
      'invitation.reward_payout_unfrozen',
      { request_id: 29, target_user_id: 5, amount: 6400 },
      'Unfroze invitation reward payout request 29 for user 5: 6400',
    ],
    [
      'invitation.incentive_settings_update',
      { enabled_from: true, enabled_to: false },
      'Updated invitation incentive settings',
    ],
  ])('renders %s audit content', (action, params, expected) => {
    const other: LogOtherData = { op: { action, params } }

    expect(renderAuditContent(other, t)).toBe(expected)
  })

  test('renders wallet adjustments with reason, delta, balances, and reference', () => {
    const other: LogOtherData = {
      op: {
        action: 'wallet.adjust',
        params: {
          category: 'topup',
          delta_display: '$25.00',
          balance_before_display: '$100.00',
          balance_after_display: '$125.00',
          reference_type: 'topup',
          reference_id: 'trade-13',
        },
      },
    }

    expect(renderAuditContent(other, t)).toBe(
      'Wallet Top-up changed by $25.00; balance from $100.00 to $125.00; reference topup trade-13'
    )
  })

  test('localizes the wallet adjustment category', () => {
    const translations: Record<string, string> = {
      'Top-up': '充值',
      'Wallet {{category}} changed by {{delta}}; balance from {{before}} to {{after}}; reference {{reference_type}} {{reference_id}}':
        '钱包{{category}}{{delta}}；余额从{{before}}变为{{after}}；关联{{reference_type}} {{reference_id}}',
    }
    const localize = (template: string, opts?: Record<string, unknown>) =>
      (translations[template] ?? template).replaceAll(
        /{{([^}]+)}}/g,
        (_, key: string) => String(opts?.[key] ?? '')
      )

    expect(
      renderAuditContent(
        {
          op: {
            action: 'wallet.adjust',
            params: {
              category: 'topup',
              delta_display: '$25.00',
              balance_before_display: '$100.00',
              balance_after_display: '$125.00',
              reference_type: 'topup',
              reference_id: 'trade-13',
            },
          },
        },
        localize
      )
    ).toBe('钱包充值$25.00；余额从$100.00变为$125.00；关联topup trade-13')
  })

  test('localizes known login methods and invitation review statuses', () => {
    const translations: Record<string, string> = {
      'Logged in successfully via {{method}}': '已通过 {{method}} 登录成功',
      'Logged in successfully via {{login_method}}':
        '已通过 {{login_method}} 登录成功',
      'Reviewed invitation application {{application_id}} for user {{target_user_id}}: {{status}}':
        '已审核用户 {{target_user_id}} 的邀请码申请 {{application_id}}：{{status}}',
      Password: '密码',
      Rejected: '已驳回',
    }
    const localize = (template: string, opts?: Record<string, unknown>) =>
      (translations[template] ?? template).replaceAll(
        /{{([^}]+)}}/g,
        (_, key: string) => String(opts?.[key] ?? '')
      )

    expect(
      renderAuditContent(
        { op: { action: 'login', params: { method: 'password' } } },
        localize
      )
    ).toBe('已通过 密码 登录成功')
    expect(
      renderAuditContent(
        {
          op: {
            action: 'user.login_success',
            params: { login_method: 'password' },
          },
        },
        localize
      )
    ).toBe('已通过 密码 登录成功')
    expect(
      renderAuditContent(
        {
          op: {
            action: 'invitation.application_reviewed',
            params: {
              target_user_id: 3007,
              application_id: 19,
              status: 'rejected',
            },
          },
        },
        localize
      )
    ).toBe('已审核用户 3007 的邀请码申请 19：已驳回')
  })

  test('renders historical token reveal logs without a missing-name placeholder', () => {
    expect(
      renderAuditContent(
        {
          op: {
            action: 'token.secret_reveal',
            params: { token_id: 99004 },
          },
        },
        t
      )
    ).toBe('Viewed API token key (ID: 99004)')
  })

  test.each([
    ['pre', -25, 'API pre-charge'],
    ['settle_debit', -5, 'API settlement charge'],
    ['settle_credit', 5, 'API settlement refund'],
    ['refund', 25, 'API refund'],
  ])(
    'renders relay wallet context %s with the correct direction',
    (referenceContext, delta, expectedCategory) => {
      expect(
        renderAuditContent(
          {
            op: {
              action: 'wallet.adjust',
              params: {
                category: 'relay',
                delta,
                delta_display: String(delta),
                balance_before_display: '100',
                balance_after_display: String(100 + delta),
                reference_type: 'relay_request',
                reference_id: 'req-1',
                reference_context: referenceContext,
              },
            },
          },
          t
        )
      ).toBe(
        `Wallet ${expectedCategory} changed by ${delta}; balance from 100 to ${100 + delta}; reference relay_request req-1`
      )
    }
  )

  test('does not expose raw content for an unknown structured action', () => {
    expect(
      renderAuditContent({ op: { action: 'future.action', params: {} } }, t)
    ).toBe('Unrecognized operation')
    expect(renderAuditContent(null, t)).toBeNull()
  })
})
