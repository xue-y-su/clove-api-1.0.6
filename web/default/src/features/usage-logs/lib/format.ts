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
import type { StatusBadgeProps } from '@/components/status-badge'
import {
  BILLING_PRICING_VARS,
  normalizeTierLabel,
  parseTiersFromExpr,
  type ParsedTier,
} from '@/features/pricing/lib/billing-expr'

import type { UsageLog } from '../data/schema'
import type { LogOtherData } from '../types'

export { normalizeTierLabel }

const PARAM_OVERRIDE_ACTION_MAP: Record<string, string> = {
  set: 'Set',
  delete: 'Delete',
  copy: 'Copy',
  move: 'Move',
  append: 'Append',
  prepend: 'Prepend',
  trim_prefix: 'Trim Prefix',
  trim_suffix: 'Trim Suffix',
  ensure_prefix: 'Ensure Prefix',
  ensure_suffix: 'Ensure Suffix',
  trim_space: 'Trim Space',
  to_lower: 'To Lower',
  to_upper: 'To Upper',
  replace: 'Replace',
  regex_replace: 'Regex Replace',
  set_header: 'Set Header',
  delete_header: 'Delete Header',
  copy_header: 'Copy Header',
  move_header: 'Move Header',
  pass_headers: 'Pass Headers',
  sync_fields: 'Sync Fields',
  return_error: 'Return Error',
}

/**
 * Get localized label for a param override action
 */
export function getParamOverrideActionLabel(
  action: string,
  t: (key: string) => string
): string {
  const key = PARAM_OVERRIDE_ACTION_MAP[action.toLowerCase()]
  return key ? t(key) : action
}

/**
 * Parse a param override audit line into action and content
 */
export function parseAuditLine(
  line: string
): { action: string; content: string } | null {
  if (typeof line !== 'string') return null
  const firstSpace = line.indexOf(' ')
  if (firstSpace <= 0) return { action: line, content: line }
  return {
    action: line.slice(0, firstSpace),
    content: line.slice(firstSpace + 1),
  }
}

/**
 * Check if the log is a violation fee log
 */
export function isViolationFeeLog(other: LogOtherData | null): boolean {
  if (!other) return false
  return (
    other.violation_fee === true ||
    Boolean(other.violation_fee_code) ||
    Boolean(other.violation_fee_marker)
  )
}

/**
 * Parse the 'other' field from JSON string to object
 */
export function parseLogOther(other: string): LogOtherData | null {
  if (!other) return null
  try {
    return JSON.parse(other) as LogOtherData
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to parse log other field:', error)
    return null
  }
}

/**
 * Get time color based on duration (in seconds)
 */
export function getTimeColor(
  seconds: number
): 'success' | 'warning' | 'danger' {
  if (seconds < 10) return 'success'
  if (seconds < 30) return 'warning'
  return 'danger'
}

/**
 * Get first-response-token color based on latency (in seconds)
 */
export function getFirstResponseTimeColor(
  seconds: number
): 'success' | 'warning' | 'danger' {
  if (seconds < 5) return 'success'
  if (seconds < 10) return 'warning'
  return 'danger'
}

/**
 * Get throughput color based on generated tokens per second
 */
export function getThroughputColor(
  tokensPerSecond: number
): 'success' | 'warning' | 'danger' {
  if (tokensPerSecond >= 30) return 'success'
  if (tokensPerSecond >= 15) return 'warning'
  return 'danger'
}

/**
 * Get response color using throughput only when enough output tokens exist.
 */
export function getResponseTimeColor(
  seconds: number,
  completionTokens: number
): 'success' | 'warning' | 'danger' {
  if (completionTokens < 100 || seconds <= 0) return getTimeColor(seconds)
  return getThroughputColor(completionTokens / seconds)
}

/**
 * Format model name with mapping indicator
 */
export function formatModelName(log: UsageLog): {
  name: string
  isMapped: boolean
  actualModel?: string
} {
  const other = parseLogOther(log.other)
  const isMapped = !!(
    other?.is_model_mapped &&
    other?.upstream_model_name &&
    other.upstream_model_name !== ''
  )

  return {
    name: log.model_name,
    isMapped,
    actualModel: isMapped ? other.upstream_model_name : undefined,
  }
}

/**
 * Decode a base64-encoded billing expression. Safely returns an empty string
 * when the input is missing or malformed (e.g. legacy logs without expr_b64).
 */
export function decodeBillingExprB64(exprB64: string | undefined): string {
  if (!exprB64) return ''
  try {
    const binaryString =
      typeof window !== 'undefined'
        ? window.atob(exprB64)
        : Buffer.from(exprB64, 'base64').toString('binary')
    const bytes = new Uint8Array(binaryString.length)

    for (let i = 0; i < binaryString.length; i++) {
      bytes[i] = binaryString.charCodeAt(i)
    }

    if (typeof TextDecoder !== 'undefined') {
      return new TextDecoder().decode(bytes)
    }

    return decodeURIComponent(
      Array.prototype.map
        .call(bytes, (byte: number) => `%${byte.toString(16).padStart(2, '0')}`)
        .join('')
    )
  } catch {
    return ''
  }
}

/**
 * Resolve which parsed tier corresponds to the matched_tier label in a log
 * entry. Missing or unknown labels do not fall back to another tier because
 * that would display guessed unit prices.
 */
export function resolveMatchedTier(
  tiers: ParsedTier[],
  matchedLabel: string | undefined
): ParsedTier | null {
  if (tiers.length === 0) return null
  if (!matchedLabel) return null
  const found = tiers.find((tier) => {
    const l1 = normalizeTierLabel(tier.label)
    const l2 = normalizeTierLabel(matchedLabel)
    return l1 === l2 && l1 !== ''
  })
  return found || null
}

/**
 * Tiered pricing summary derived from an `other` log payload using the
 * billing-expression library. Returns null when the entry is not a tiered
 * billing log or the expression failed to parse.
 */
export interface TieredBillingSummary {
  tiers: ParsedTier[]
  tier: ParsedTier
  priceEntries: Array<{ field: string; shortLabel: string; price: number }>
}

/**
 * Whether the request payload reports any cache-related token usage. Used to
 * suppress cache pricing rows from the tiered breakdown when the request did
 * not exercise the cache path (mirrors the classic frontend behaviour).
 */
export function hasAnyCacheTokens(
  other: LogOtherData | null | undefined
): boolean {
  if (!other) return false
  return (
    (other.cache_tokens || 0) > 0 ||
    (other.cache_creation_tokens || 0) > 0 ||
    (other.cache_creation_tokens_5m || 0) > 0 ||
    (other.cache_creation_tokens_1h || 0) > 0
  )
}

export function getTieredBillingSummary(
  other: LogOtherData | null
): TieredBillingSummary | null {
  if (!other || other.billing_mode !== 'tiered_expr') return null
  const exprStr = decodeBillingExprB64(other.expr_b64)
  if (!exprStr) return null
  const tiers = parseTiersFromExpr(exprStr)
  const tier = resolveMatchedTier(tiers, other.matched_tier)
  if (!tier) return null

  const cacheTokensPresent = hasAnyCacheTokens(other)

  const priceEntries: TieredBillingSummary['priceEntries'] = []
  for (const v of BILLING_PRICING_VARS) {
    if (!v.field) continue
    if (v.group === 'cache' && !cacheTokensPresent) continue
    const raw = tier[v.field as keyof ParsedTier]
    const price = Number(raw)
    if (Number.isFinite(price) && price > 0) {
      priceEntries.push({
        field: v.field,
        shortLabel: v.shortLabel,
        price,
      })
    }
  }
  return { tiers, tier, priceEntries }
}

/**
 * Calculate duration and return formatted result with color variant
 * @param submitTime - Submit timestamp
 * @param finishTime - Finish timestamp
 * @param unit - Unit of the timestamps ('seconds' or 'milliseconds')
 */
export function formatDuration(
  submitTime?: number,
  finishTime?: number,
  unit: 'seconds' | 'milliseconds' = 'milliseconds'
): { durationSec: number; variant: StatusBadgeProps['variant'] } | null {
  if (!submitTime || !finishTime) return null

  const durationSec =
    unit === 'milliseconds'
      ? (finishTime - submitTime) / 1000
      : finishTime - submitTime

  return { durationSec, variant: durationSec > 60 ? 'red' : 'green' }
}

/**
 * Maps a language-independent audit/login operation `action` to an i18n
 * template string (the template itself is the i18n key, with {{placeholders}}).
 *
 * The backend stores only `action` + structured `params` in `other.op`; the UI
 * renders localized content at display time so audit/login logs are fully
 * translatable instead of being frozen to whatever language was written to DB.
 */
const WALLET_ADJUST_TEMPLATE =
  'Wallet {{category}} changed by {{delta}}; balance from {{before}} to {{after}}; reference {{reference_type}} {{reference_id}}'
const UNKNOWN_AUDIT_TEMPLATE = 'Unrecognized operation'
const TOKEN_REVEAL_BY_ID_TEMPLATE = 'Viewed API token key (ID: {{token_id}})'
const TOKEN_REVEAL_GENERIC_TEMPLATE = 'Viewed an API token key'
const LOGIN_METHOD_LABELS: Record<string, string> = {
  password: 'Password',
}
const INVITATION_REVIEW_STATUS_LABELS: Record<string, string> = {
  approved: 'Approved',
  paid: 'Paid',
  pending: 'Pending',
  rejected: 'Rejected',
  withdrawn: 'Withdrawn',
}
const WALLET_CATEGORY_LABELS: Record<string, string> = {
  admin_adjust: 'Administrator adjustment',
  admin_redemption: 'Redemption code',
  checkin: 'Daily check-in',
  invitation_gift: 'Invitation gift',
  invitation_reward: 'Invitation reward',
  relay: 'API usage',
  subscription_purchase: 'Subscription purchase',
  task: 'Task usage',
  topup: 'Top-up',
  user_redemption_claim: 'Redemption code claim',
  user_redemption_refund: 'Redemption code refund',
  user_redemption_reserve: 'Redemption code funding',
}
const WALLET_CONTEXT_LABELS: Record<string, string> = {
  'relay:pre': 'API pre-charge',
  'relay:refund': 'API refund',
  'relay:settle_credit': 'API settlement refund',
  'relay:settle_debit': 'API settlement charge',
  'relay:subscription_overflow_credit':
    'Subscription overflow settlement refund',
  'relay:subscription_overflow_debit': 'Subscription overflow charge',
  'relay:subscription_overflow_refund': 'Subscription overflow refund',
  'task:credit': 'Task refund',
  'task:debit': 'Task charge',
  'task:refund': 'Task refund',
}

export const AUDIT_TEMPLATES: Record<string, string> = {
  login: 'Logged in successfully via {{method}}',
  // User management
  'user.register': 'Registered a user account',
  'user.login_success': 'Logged in successfully via {{login_method}}',
  'user.login_failed':
    'Login failed via {{login_method}} ({{failure_category}})',
  'user.logout': 'Logged out',
  'user.create': 'Created user {{username}} (role {{role}})',
  'user.create.invite': 'Created user {{username}} with inviter {{inviter_id}}',
  'user.create.invite_skip':
    'Created user {{username}} without invitation code: {{aff_skip_reason}}',
  'user.register.invite':
    'User {{username}} registered with invitation code {{aff_code}} from inviter {{inviter_id}}',
  'user.update': 'Updated user {{username}} (ID: {{id}})',
  'user.delete': 'Deleted user {{username}} (ID: {{id}})',
  'user.manage': 'Performed {{action}} on user {{username}} (ID: {{id}})',
  'user.disable': 'Disabled user {{username}} (ID: {{id}})',
  'user.enable': 'Enabled user {{username}} (ID: {{id}})',
  'user.role_change':
    'Changed user {{username}} role from {{role_from}} to {{role_to}}',
  'user.quota_add': 'Increased user quota by {{quota}}',
  'user.quota_subtract': 'Decreased user quota by {{quota}}',
  'user.quota_override': 'Overrode user quota from {{from}} to {{to}}',
  'user.binding_clear': 'Cleared {{bindingType}} binding for user {{username}}',
  'user.binding_clear_self': 'Cleared {{binding_type}} binding',
  'user.password_setup': 'Set an account password',
  'user.profile_update': 'Updated personal profile',
  'user.security_audit_read':
    'Viewed security audit for user {{target_user_id}}',
  'user.2fa_disable': 'Force-disabled two-factor authentication for the user',
  'user.passkey_register': 'Registered a passkey',
  'user.passkey_delete': 'Deleted a passkey',
  'user.reset_passkey': 'Reset the user passkey',
  'user.oauth_unbind': 'Removed an OAuth binding for the user',
  'user.redeem_code': 'Redeemed a redemption code for {{quota}}',
  'user.topup_complete': 'Completed top-up order {{trade_no}}',
  // System settings
  'option.update': 'Updated system setting {{key}}',
  'option.update.relay_billing': 'Updated relay billing settings',
  'option.payment_compliance': 'Confirmed payment compliance',
  'option.reset_ratio': 'Reset model ratios',
  'option.clear_affinity_cache': 'Cleared channel affinity cache',
  'registration.settings_update': 'Updated invitation registration settings',
  'source_attribution.settings_update': 'Updated source attribution settings',
  'source_attribution.confirm': 'Confirmed source attribution {{option_name}}',
  'compliance.settings_update': 'Updated compliance statement settings',
  'compliance.confirm': 'Confirmed compliance statement {{version}}',
  'announcement.acknowledge': 'Acknowledged the latest announcement',
  'announcement.media_upload': 'Uploaded an announcement image',
  'announcement.settings_update': 'Updated announcement settings',
  'security.settings_update': 'Updated security settings',
  // Billing
  'billing.topup': 'Recorded top-up event {{trade_no}}',
  'billing.consume': 'Recorded consumption event {{request_id}}',
  'billing.refund': 'Recorded refund event {{request_id}}',
  // Sensitive queries
  'sensitive.token_balance_query':
    'Queried API token balance {{masked_key}} ({{result}})',
  'sensitive.ip_audit_view': 'Viewed IP audit details',
  ip_audit_read: 'Viewed the global IP audit',
  ip_audit_detail_read: 'Viewed IP audit details',
  'log.full_ip_read': 'Viewed full IP addresses in usage logs',
  'log.full_ip_stat_read': 'Queried usage statistics by full IP address',
  'compliance.full_ip_read':
    'Viewed full IP addresses in compliance confirmations',
  'source_attribution.full_ip_read':
    'Viewed full IP addresses in source attribution confirmations',
  // Playground
  'playground.web_search': 'Used playground web search with model {{model}}',
  'playground.image_generation':
    'Generated playground image with model {{model}}',
  // Invitation codes
  'invitation.application_created':
    'User {{target_user_id}} submitted invitation application {{application_id}}',
  'invitation.application_withdrawn':
    'User {{target_user_id}} withdrew invitation application {{application_id}}',
  'invitation.application_reviewed':
    'Reviewed invitation application {{application_id}} for user {{target_user_id}}: {{status}}',
  'invitation.code_issued':
    'Issued invitation code {{invite_code_id}} to user {{target_user_id}}',
  'invitation.code_revoked':
    'Revoked invitation code {{invite_code_id}} for user {{target_user_id}}',
  'invitation.code_used':
    'Invitation code {{invite_code_id}} registered user {{registered_user_id}}',
  'invitation.reward_review':
    'Reviewed invitation reward {{payout_id}} for user {{target_user_id}}: {{status}}',
  'invitation.reward_withdraw':
    'User {{target_user_id}} withdrew {{amount}} from approved invitation rewards',
  'invitation.reward_payout_submitted':
    'User {{target_user_id}} submitted invitation reward payout request {{request_id}} for {{amount}} across {{item_count}} rewards',
  'invitation.reward_payout_reviewed':
    'Reviewed invitation reward payout request {{request_id}} for user {{target_user_id}}: {{status}}',
  'invitation.reward_payout_frozen':
    'Froze invitation reward payout request {{request_id}} for user {{target_user_id}}: {{amount}}',
  'invitation.reward_payout_unfrozen':
    'Unfroze invitation reward payout request {{request_id}} for user {{target_user_id}}: {{amount}}',
  'invitation.incentive_settings_update':
    'Updated invitation incentive settings',
  // API tokens
  'token.custom_key_collision_suspected_compromise':
    'Disabled API key {{masked_key}} after a custom key collision',
  'token.create': 'Created API token {{name}} (ID: {{token_id}})',
  'token.update': 'Updated API token {{name}} (ID: {{token_id}})',
  'token.status_update': 'Updated API token {{name}} status to {{status}}',
  'token.delete': 'Deleted API token {{name}} (ID: {{token_id}})',
  'token.key_view': 'Viewed API token key {{name}} (ID: {{token_id}})',
  'token.secret_reveal': 'Viewed API token key {{name}} (ID: {{token_id}})',
  'token.secret_reveal_batch': 'Viewed {{item_count}} API token keys',
  'token.secret_reveal_verify': 'Verified password for API token key access',
  // Custom OAuth
  'custom_oauth.create': 'Created a custom OAuth provider',
  'custom_oauth.update': 'Updated a custom OAuth provider',
  'custom_oauth.delete': 'Deleted a custom OAuth provider',
  // Performance / cache
  'performance.clear_disk_cache': 'Cleared disk cache',
  'performance.gc': 'Triggered garbage collection',
  'performance.clear_logs': 'Cleared log files',
  // Channel
  'channel.create': 'Created channel {{name}} (type {{type}}, count {{count}})',
  'channel.update': 'Updated channel {{name}} (ID: {{id}})',
  'channel.status_update': 'Updated channel {{id}} status to {{status}}',
  'channel.status_update_batch':
    'Batch updated {{count}}/{{total}} channels to status {{status}}',
  'channel.delete': 'Deleted channel {{name}} (ID: {{id}})',
  'channel.delete_batch': 'Batch deleted {{count}} channels',
  'channel.delete_disabled': 'Deleted all disabled channels ({{count}})',
  'channel.key_view': 'Viewed channel key {{name}} (ID: {{id}})',
  'channel.tag_disable': 'Disabled channels with tag {{tag}}',
  'channel.tag_enable': 'Enabled channels with tag {{tag}}',
  'channel.tag_edit': 'Edited channels with tag {{tag}}',
  'channel.tag_batch_set': 'Batch set tag for {{count}} channels',
  'channel.copy':
    'Copied channel (source ID: {{sourceId}}) to {{name}} (new ID: {{id}})',
  'channel.multi_key_manage':
    'Multi-key management {{action}} on channel (ID: {{id}})',
  'channel.upstream_apply':
    'Applied upstream model changes to channel (ID: {{id}})',
  'channel.upstream_apply_all':
    'Applied upstream model changes to {{count}} channels',
  'channel.upstream_detect_all':
    'Queued upstream model detection task {{task_id}}',
  // Redemption codes
  'redemption.create':
    'Created {{count}} redemption codes named {{name}} ({{quota}} each)',
  'redemption.view': 'Viewed redemption code {{id}}',
  'redemption.update': 'Updated redemption code {{id}}',
  'redemption.status_update':
    'Updated redemption code {{id}} status to {{status}}',
  'redemption.delete': 'Deleted redemption code {{id}}',
  'redemption.delete_invalid': 'Deleted invalid redemption codes ({{count}})',
  // User-created redemption codes
  'user.redemption.create': 'Created user redemption code {{id}}',
  'user.redemption.claim': 'Claimed user redemption code {{id}}',
  'user.redemption.cancel_pending':
    'Queued user redemption code {{id}} for cancellation',
  'user.redemption.expire_pending':
    'Queued expired user redemption code {{id}} for settlement',
  'user.redemption.subscription.create':
    'Created subscription redemption code {{id}}',
  'user.redemption.subscription.claim':
    'Claimed subscription redemption code {{id}}',
  'user.redemption.subscription.plan_disabled':
    'Closed subscription redemption code {{id}} because its plan was disabled',
  'user.redemption.subscription.refund_pending':
    'Queued subscription redemption code {{id}} for refund',
  'user.redemption.subscription.refund':
    'Refunded subscription redemption code {{id}}',
  'user.redemption.subscription.forfeit':
    'Forfeited subscription redemption code {{id}}',
  'user.redemption.cancel': 'Cancelled user redemption code {{id}}',
  'user.redemption.expire_refund':
    'Refunded expired user redemption code {{id}}',
  'user.redemption.disable': 'Disabled user redemption code {{id}}',
  'user.redemption.enable': 'Enabled user redemption code {{id}}',
  'user.redemption.terminate_refund':
    'Terminated user redemption code {{id}} with a refund',
  'user.redemption.terminate_forfeit':
    'Terminated user redemption code {{id}} with forfeiture',
  'user.redemption.delete': 'Deleted user redemption code {{id}}',
  // Prefill groups
  'prefill_group.create': 'Created a prefill group',
  'prefill_group.update': 'Updated a prefill group',
  'prefill_group.delete': 'Deleted a prefill group',
  // Vendors
  'vendor.create': 'Created a vendor',
  'vendor.update': 'Updated a vendor',
  'vendor.delete': 'Deleted a vendor',
  // Model metadata
  'model.create': 'Created a model',
  'model.update': 'Updated a model',
  'model.delete': 'Deleted a model',
  'model.sync_upstream': 'Synced upstream models',
  // Deployments
  'deployment.create': 'Created a deployment',
  'deployment.update': 'Updated a deployment',
  'deployment.delete': 'Deleted a deployment',
  // Subscriptions
  'subscription.plan_create': 'Created a subscription plan',
  'subscription.plan_update': 'Updated a subscription plan',
  'subscription.bind': 'Bound a subscription',
  'subscription.plan_reset': 'Reset active subscriptions for plan {{plan_id}}',
  'subscription.user_plan_reset':
    'Reset active plan {{plan_id}} subscriptions for user {{target_user_id}}',
  // Logs
  'log.clear': 'Cleared historical logs',
  'wallet.adjust': 'Updated wallet balance',
  // Generic middleware fallback
  generic: '{{method}} {{route}}',
}

/**
 * Render the localized content of an audit/login log from its structured
 * `other.op` descriptor. Returns null only when no structured action exists.
 */
export function renderAuditContent(
  other: LogOtherData | null | undefined,
  t: (key: string, opts?: Record<string, unknown>) => string
): string | null {
  const op = other?.op
  if (!op?.action) return null
  const params = (op.params ?? {}) as Record<string, unknown>
  if (op.action === 'wallet.adjust') {
    const hasDetailedBalance = [
      params.delta_display,
      params.balance_before_display,
      params.balance_after_display,
    ].every((value) => value !== undefined && value !== null && value !== '')
    if (!hasDetailedBalance) {
      return t(AUDIT_TEMPLATES['wallet.adjust'], params)
    }
    const category = String(params.category ?? '')
    const referenceContext = String(params.reference_context ?? '')
    const delta = Number(params.delta)
    let categoryLabel = WALLET_CONTEXT_LABELS[`${category}:${referenceContext}`]
    if (
      !categoryLabel &&
      category === 'relay' &&
      referenceContext === 'post_consume' &&
      Number.isFinite(delta)
    ) {
      categoryLabel =
        delta > 0 ? 'API settlement refund' : 'API settlement charge'
    }
    if (!categoryLabel && category === 'task' && Number.isFinite(delta)) {
      categoryLabel = delta > 0 ? 'Task refund' : 'Task charge'
    }
    return t(WALLET_ADJUST_TEMPLATE, {
      ...params,
      category: t(
        categoryLabel ??
          WALLET_CATEGORY_LABELS[category] ??
          'Other wallet adjustment'
      ),
      delta: params.delta_display,
      before: params.balance_before_display,
      after: params.balance_after_display,
    })
  }
  const template = AUDIT_TEMPLATES[op.action]
  if (!template) return t(UNKNOWN_AUDIT_TEMPLATE)
  const localizedParams = { ...params }
  if (op.action === 'login') {
    const method = String(params.method ?? '')
    localizedParams.method = t(LOGIN_METHOD_LABELS[method] ?? method)
  }
  if (op.action === 'user.login_success' || op.action === 'user.login_failed') {
    const loginMethod = String(params.login_method ?? '')
    localizedParams.login_method = t(
      LOGIN_METHOD_LABELS[loginMethod] ?? loginMethod
    )
  }
  if (
    op.action === 'invitation.application_reviewed' ||
    op.action === 'invitation.reward_review' ||
    op.action === 'invitation.reward_payout_reviewed'
  ) {
    const status = String(params.status ?? '')
    localizedParams.status = t(
      INVITATION_REVIEW_STATUS_LABELS[status] ?? status
    )
  }
  if (
    (op.action === 'token.key_view' || op.action === 'token.secret_reveal') &&
    !String(params.name ?? '').trim()
  ) {
    if (params.token_id !== undefined && params.token_id !== null) {
      return t(TOKEN_REVEAL_BY_ID_TEMPLATE, localizedParams)
    }
    return t(TOKEN_REVEAL_GENERIC_TEMPLATE, localizedParams)
  }
  return t(template, localizedParams)
}
