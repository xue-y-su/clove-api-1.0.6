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

import {
  buildInvitationIssueSpec,
  buildInvitationRegistrationUrl,
  formatInvitationLocalDateTime,
  formatInvitationRewardStatus,
  getInvitationErrorMessage,
  getInvitationInputError,
  getInvitationReapplyRefreshDelay,
  getInvitationRemainingUses,
  getInvitationViewState,
} from './invitation-state'

const emptyState = {
  application: null,
  code: null,
  uses: [],
  can_apply: true,
  reapply_at: 0,
}

describe('invitation presentation state', () => {
  test('prioritizes loading and error before invitation records', () => {
    expect(getInvitationViewState(emptyState, true, false)).toBe('loading')
    expect(getInvitationViewState(emptyState, false, true)).toBe('error')
  })

  test('maps each application and code lifecycle to a stable view', () => {
    expect(getInvitationViewState(emptyState, false, false)).toBe('empty')
    expect(
      getInvitationViewState(
        {
          ...emptyState,
          application: { status: 'pending' },
          can_apply: false,
        },
        false,
        false
      )
    ).toBe('pending')
    expect(
      getInvitationViewState(
        {
          ...emptyState,
          application: { status: 'pending' },
          code: { status: 'revoked' },
        },
        false,
        false
      )
    ).toBe('pending')
    expect(
      getInvitationViewState(
        {
          ...emptyState,
          application: { status: 'rejected' },
          can_apply: false,
        },
        false,
        false
      )
    ).toBe('rejected')
    expect(
      getInvitationViewState(
        {
          ...emptyState,
          application: { status: 'rejected' },
          can_apply: true,
        },
        false,
        false
      )
    ).toBe('available')
    expect(
      getInvitationViewState(
        {
          ...emptyState,
          code: { status: 'active', max_uses: 5, used_count: 2 },
          can_apply: false,
        },
        false,
        false
      )
    ).toBe('active')
    expect(
      getInvitationViewState(
        {
          ...emptyState,
          code: { status: 'exhausted', max_uses: 1, used_count: 1 },
        },
        false,
        false
      )
    ).toBe('terminal')
  })

  test('schedules one refresh at the invitation reapply boundary', () => {
    const nowMs = 1_800_000_000_000
    expect(getInvitationReapplyRefreshDelay(0, false, nowMs)).toBeNull()
    expect(
      getInvitationReapplyRefreshDelay(nowMs / 1000 + 60, true, nowMs)
    ).toBeNull()
    expect(
      getInvitationReapplyRefreshDelay(nowMs / 1000 + 60, false, nowMs)
    ).toBe(60_250)
    expect(
      getInvitationReapplyRefreshDelay(nowMs / 1000 - 1, false, nowMs)
    ).toBe(1_000)
  })

  test('wires the reapply boundary to an automatic invitation refresh', () => {
    const source = readFileSync(
      new URL('../components/user-invitation-card.tsx', import.meta.url),
      'utf8'
    )
    expect(source).toContain('getInvitationReapplyRefreshDelay')
    expect(source).toContain('window.setTimeout')
    expect(source).toContain('invitationQuery.refetch()')
  })

  test('builds encoded registration links and clamps remaining uses', () => {
    expect(
      buildInvitationRegistrationUrl('https://example.com/', 'inv_a+b/c')
    ).toBe('https://example.com/register?aff=inv_a%2Bb%2Fc')
    expect(getInvitationRemainingUses({ max_uses: 3, used_count: 1 })).toBe(2)
    expect(getInvitationRemainingUses({ max_uses: 1, used_count: 4 })).toBe(0)
  })

  test('validates short-code shape without rejecting legacy codes while typing', () => {
    expect(getInvitationInputError('')).toBeNull()
    expect(getInvitationInputError('abcd2345')).toBeNull()
    expect(getInvitationInputError('ABCDI234')).toBe('invalid_short_code')
    expect(getInvitationInputError('inv_aBcDefGhijkLmNoPqRsTuvWx')).toBeNull()
    expect(getInvitationInputError('x'.repeat(65))).toBe('too_long')
  })

  test('localizes both invitation reward statuses', () => {
    expect(
      formatInvitationRewardStatus('pending', 'completed', (key) => `[${key}]`)
    ).toBe('[Pending] / [Completed]')
    expect(
      formatInvitationRewardStatus(
        'pending_review',
        'withdrawn',
        (key) => `[${key}]`
      )
    ).toBe('[Pending review] / [Withdrawn]')
  })

  test('formats local expiry values without a UTC shift', () => {
    const localTime = new Date(2026, 0, 2, 3, 4).getTime()
    expect(formatInvitationLocalDateTime(localTime)).toBe('2026-01-02T03:04')
  })

  test('normalizes valid issue specs and rejects invalid limits', () => {
    const now = new Date(2026, 0, 1, 12, 0).getTime()
    const validExpiry = formatInvitationLocalDateTime(now + 24 * 60 * 60 * 1000)
    expect(buildInvitationIssueSpec('single', 99, validExpiry, now)).toEqual({
      kind: 'single',
      max_uses: 1,
      expires_at: Math.floor(new Date(validExpiry).getTime() / 1000),
    })
    expect(buildInvitationIssueSpec('multi', 2, validExpiry, now)).toEqual({
      kind: 'multi',
      max_uses: 2,
      expires_at: Math.floor(new Date(validExpiry).getTime() / 1000),
    })
    expect(buildInvitationIssueSpec('multi', 1, validExpiry, now)).toBeNull()
    expect(buildInvitationIssueSpec('multi', 101, validExpiry, now)).toBeNull()
    expect(buildInvitationIssueSpec('single', 1, 'not-a-date', now)).toBeNull()
    expect(
      buildInvitationIssueSpec(
        'single',
        1,
        formatInvitationLocalDateTime(now + 366 * 24 * 60 * 60 * 1000),
        now
      )
    ).toBeNull()
  })

  test('returns useful mutation errors with a safe fallback', () => {
    expect(getInvitationErrorMessage(new Error('network down'), 'failed')).toBe(
      'network down'
    )
    expect(getInvitationErrorMessage({}, 'failed')).toBe('failed')
  })
})
