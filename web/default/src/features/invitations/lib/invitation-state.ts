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
export type InvitationViewState =
  | 'loading'
  | 'error'
  | 'empty'
  | 'available'
  | 'pending'
  | 'rejected'
  | 'active'
  | 'terminal'

type ApplicationState = {
  status: string
}

type CodeState = {
  status: string
  max_uses?: number
  used_count?: number
}

type InvitationState = {
  application?: ApplicationState | null
  code?: CodeState | null
  can_apply?: boolean
}

const MAX_INVITATION_VALIDITY_MS = 365 * 24 * 60 * 60 * 1000
const MAX_REAPPLY_REFRESH_DELAY_MS = 24 * 60 * 60 * 1000

export type InvitationInputError = 'too_long' | 'invalid_short_code'

export function getInvitationInputError(
  value: string
): InvitationInputError | null {
  const code = value.trim()
  if (!code) return null
  if (code.length > 64) return 'too_long'
  if (
    code.length === 8 &&
    !/^[0-9A-HJKMNP-TV-Z]{8}$/.test(code.toUpperCase())
  ) {
    return 'invalid_short_code'
  }
  return null
}

export function getInvitationReapplyRefreshDelay(
  reapplyAt: number,
  canApply: boolean,
  nowMs = Date.now()
): number | null {
  if (canApply || !Number.isFinite(reapplyAt) || reapplyAt <= 0) return null
  const delay = reapplyAt * 1000 - nowMs + 250
  return Math.min(MAX_REAPPLY_REFRESH_DELAY_MS, Math.max(1_000, delay))
}

export function getInvitationViewState(
  state: InvitationState | null | undefined,
  loading: boolean,
  failed: boolean
): InvitationViewState {
  if (loading) return 'loading'
  if (failed) return 'error'
  if (state?.code?.status === 'active') return 'active'
  if (state?.application?.status === 'pending') return 'pending'
  if (state?.application?.status === 'rejected') {
    return state.can_apply ? 'available' : 'rejected'
  }
  if (state?.code) return 'terminal'
  return 'empty'
}

export function buildInvitationRegistrationUrl(
  origin: string,
  code: string
): string {
  const url = new URL('/register', origin)
  url.searchParams.set('aff', code)
  return url.toString()
}

export function getInvitationRemainingUses(code: CodeState): number {
  return Math.max(0, (code.max_uses ?? 0) - (code.used_count ?? 0))
}

export function formatInvitationRewardStatus(
  inviterStatus: string,
  inviteeStatus: string,
  t: (key: string) => string
): string {
  return `${t(invitationStatusTranslationKey(inviterStatus))} / ${t(
    invitationStatusTranslationKey(inviteeStatus)
  )}`
}

function invitationStatusTranslationKey(status: string): string {
  switch (status) {
    case 'active':
      return 'Active'
    case 'exhausted':
      return 'Exhausted'
    case 'expired':
      return 'Expired'
    case 'revoked':
      return 'Revoked'
    case 'pending':
      return 'Pending'
    case 'pending_review':
      return 'Pending review'
    case 'approved':
      return 'Approved'
    case 'rejected':
      return 'Rejected'
    case 'withdrawn':
      return 'Withdrawn'
    case 'completed':
      return 'Completed'
    case 'skipped':
      return 'Skipped'
    case 'failed':
      return 'Failed'
    default:
      return status
  }
}

export function formatInvitationLocalDateTime(timestampMs: number): string {
  const date = new Date(timestampMs)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate()
  )}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function buildInvitationIssueSpec(
  kind: 'single' | 'multi',
  maxUses: number,
  expiresAt: string,
  nowMs = Date.now()
): { kind: 'single' | 'multi'; max_uses: number; expires_at: number } | null {
  const expiryMs = new Date(expiresAt).getTime()
  const normalizedMaxUses = kind === 'single' ? 1 : Number(maxUses)
  if (
    !Number.isFinite(expiryMs) ||
    expiryMs <= nowMs ||
    expiryMs > nowMs + MAX_INVITATION_VALIDITY_MS ||
    !Number.isInteger(normalizedMaxUses) ||
    (kind === 'multi' && (normalizedMaxUses < 2 || normalizedMaxUses > 100))
  ) {
    return null
  }
  return {
    kind,
    max_uses: normalizedMaxUses,
    expires_at: Math.floor(expiryMs / 1000),
  }
}

export function getInvitationErrorMessage(
  error: unknown,
  fallback: string
): string {
  return error instanceof Error && error.message.trim()
    ? error.message
    : fallback
}
