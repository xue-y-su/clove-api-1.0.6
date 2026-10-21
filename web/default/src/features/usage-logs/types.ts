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
/**
 * Type definitions for usage logs
 */
import type { UsageLog } from './data/schema'

// ============================================================================
// Log Category Types
// ============================================================================

/**
 * Log category for different log types
 */
export type LogCategory = 'common' | 'drawing' | 'task'

// ============================================================================
// Filter Types
// ============================================================================

/**
 * Common filters (shared across all log types)
 */
export interface CommonFilters {
  startTime?: Date
  endTime?: Date
  channel?: string
}

/**
 * Common logs specific filters
 */
export interface CommonLogFilters extends CommonFilters {
  model?: string
  token?: string
  group?: string
  username?: string
  ip?: string
  requestId?: string
  upstreamRequestId?: string
}

/**
 * Drawing logs specific filters
 */
export interface DrawingLogFilters extends CommonFilters {
  mjId?: string
}

/**
 * Task logs specific filters
 */
export interface TaskLogFilters extends CommonFilters {
  taskId?: string
}

/**
 * Union type for all log filters
 */
export type LogFilters = CommonLogFilters | DrawingLogFilters | TaskLogFilters

// ============================================================================
// Common Logs Additional Types
// ============================================================================

/**
 * Parsed data from the 'other' field in usage logs
 */
export interface ChannelAffinityInfo {
  rule_name?: string
  selected_group?: string
  key_source?: string
  key_path?: string
  key_key?: string
  key_hint?: string
  key_fp?: string
  using_group?: string
}

export const USAGE_BILLING_PATH = {
  LOCAL: 'local',
  UPSTREAM: 'upstream',
  OPENAI: 'billing-usage-openai',
  OPENAI_ESTIMATED: 'billing-usage-openai-estimated',
  ANTHROPIC: 'billing-usage-anthropic',
  ANTHROPIC_ESTIMATED: 'billing-usage-anthropic-estimated',
  GEMINI: 'billing-usage-gemini',
  GEMINI_ESTIMATED: 'billing-usage-gemini-estimated',
} as const

export type UsageBillingPath =
  (typeof USAGE_BILLING_PATH)[keyof typeof USAGE_BILLING_PATH]

export interface ToolSurchargeItem {
  name: string
  count: number
  unit: string
  unit_price: number
  subtotal_quota: number
}

export interface LogOtherData {
  admin_info?: {
    is_multi_key?: boolean
    multi_key_index?: number
    use_channel?: number[]
    local_count_tokens?: boolean
    usage_billing_path?: UsageBillingPath | string
    channel_affinity?: ChannelAffinityInfo
    // Top-up audit fields (type=1, admin only)
    payment_method?: string
    callback_payment_method?: string
    caller_ip?: string
    server_ip?: string
    version?: string
    node_name?: string
    // Operator identity for audit logs (type=3, admin only)
    admin_username?: string
    admin_id?: number | string
    admin_role?: number
    auth_method?: 'session' | 'access_token' | string
    // Quota saturation marker: set when a quota conversion clamped at the
    // int32 bound (overflow/underflow) or hit a NaN fallback while computing
    // this request's charge. Admin-only (nested under admin_info).
    quota_saturation?: {
      op: string
      kind: 'overflow' | 'underflow' | 'nan'
      original: number
      clamped: number
    }
  }
  // Language-independent operation descriptor (audit/login logs).
  // Frontend renders localized content from action + params via i18n templates.
  op?: {
    action?: string
    params?: Record<string, string | number | boolean | string[]>
  }
  // Operation audit details written by the admin-audit fallback in authHelper (type=3, admin only)
  audit_info?: {
    method?: string
    route?: string
    path?: string
    status?: number
    success?: boolean
    params?: Record<string, string>
  }
  // Login audit fields (type=7); visible to the log owner
  login_method?: string
  user_agent?: string
  request_path?: string
  request_conversion?: string[]
  ws?: boolean
  audio?: boolean
  audio_input?: number
  audio_output?: number
  text_input?: number
  text_output?: number
  cache_tokens?: number
  cache_creation_tokens?: number
  cache_creation_tokens_5m?: number
  cache_creation_tokens_1h?: number
  claude?: boolean
  model_ratio?: number
  completion_ratio?: number
  model_price?: number
  group_ratio?: number
  user_group_ratio?: number
  cache_ratio?: number
  cache_creation_ratio?: number
  cache_creation_ratio_5m?: number
  cache_creation_ratio_1h?: number
  is_model_mapped?: boolean
  upstream_model_name?: string
  audio_ratio?: number
  audio_completion_ratio?: number
  frt?: number
  // Tiered (expression-based) billing fields, set by backend when
  // billing_mode === 'tiered_expr'. expr_b64 is the base64-encoded billing
  // expression and matched_tier is the label of the tier that fired.
  billing_mode?: string
  expr_b64?: string
  matched_tier?: string
  reasoning_effort?: string
  image?: boolean
  image_ratio?: number
  image_output?: number
  web_search?: boolean
  web_search_call_count?: number
  web_search_price?: number
  file_search?: boolean
  file_search_call_count?: number
  file_search_price?: number
  audio_input_seperate_price?: boolean
  audio_input_token_count?: number
  audio_input_price?: number
  image_generation_call?: boolean
  image_generation_call_price?: number
  is_system_prompt_overwritten?: boolean
  po?: string[]
  billing_source?: string
  billing_breakdown?: {
    model_quota: number
    tool_quota: number
    total_quota: number
  }
  tool_surcharges?: ToolSurchargeItem[]
  group?: string
  stream_status?: {
    status?: string
    end_reason?: string
    error_count?: number
    end_error?: string
    errors?: string[]
  }
  // Violation fee fields
  violation_fee?: boolean
  violation_fee_code?: string
  violation_fee_marker?: string
  fee_quota?: number
  // Reject / intercept reason (admin)
  reject_reason?: string
  // Task-related fields (for refund logs, type=6)
  is_task?: boolean
  task_id?: string
  reason?: string
  // Subscription billing fields
  subscription_plan_id?: string
  subscription_plan_title?: string
  subscription_id?: string
  subscription_pre_consumed?: number
  subscription_post_delta?: number
  subscription_consumed?: number
  subscription_remain?: number
  subscription_total?: number
}

/**
 * Log statistics data
 */
export interface LogStatistics {
  quota: number
  rpm: number
  tpm: number
}

// ============================================================================
// Drawing Logs (MjProxy) Types
// ============================================================================

export interface MidjourneyLog {
  id: number
  user_id: number
  channel_id: number
  code: number
  mj_id: string
  action: string // IMAGINE, UPSCALE, VARIATION, etc. (backend field name)
  submit_time: number // milliseconds
  finish_time?: number // milliseconds
  start_time?: number // milliseconds
  fail_reason?: string
  progress: string
  prompt: string
  prompt_en?: string
  description?: string
  buttons?: string
  properties?: string
  image_url?: string
  status: string // NOT_START, SUBMITTED, IN_PROGRESS, SUCCESS, FAILURE, MODAL
  other?: string
  created_at?: number
  updated_at?: number
}

// ============================================================================
// Task Logs Types
// ============================================================================

export interface TaskLog {
  id: number
  user_id: number
  username?: string
  platform: string // suno, kling, runway, etc.
  task_id: string
  action: string // MUSIC, LYRICS, GENERATE, TEXT_GENERATE, etc.
  channel_id: number
  submit_time: number // seconds
  finish_time?: number // seconds
  progress?: string
  progress_message_en?: string
  data?: string // JSON string
  fail_reason?: string
  status: string // NOT_START, SUBMITTED, IN_PROGRESS, SUCCESS, FAILURE, QUEUED, UNKNOWN
  other?: string
  created_at?: number
  updated_at?: number
}

// ============================================================================
// Common Log Types
// ============================================================================

export interface GetLogsParams {
  p?: number
  page_size?: number
  type?: number
  username?: string
  token_name?: string
  model_name?: string
  start_timestamp?: number
  end_timestamp?: number
  channel?: number
  group?: string
  ip?: string
  request_id?: string
  upstream_request_id?: string
}

export interface GetLogsResponse {
  success: boolean
  message?: string
  data?: {
    items: UsageLog[] | MidjourneyLog[] | TaskLog[]
    total: number
    page: number
    page_size: number
  }
}

export interface GetLogStatsParams {
  type?: number
  username?: string
  token_name?: string
  model_name?: string
  start_timestamp?: number
  end_timestamp?: number
  channel?: number
  group?: string
  ip?: string
  request_id?: string
  upstream_request_id?: string
}

export interface GetLogStatsResponse {
  success: boolean
  message?: string
  data?: LogStatistics
}

export interface IPAuditUser {
  user_id: number
  username: string
}

export interface IPAuditTimelineItem {
  created_at: number
  user_id: number
  username: string
  type: number
  kind: string
  request_id?: string
}

export interface IPAuditItem {
  ip: string
  proxy_artifact: boolean
  can_query_external_profile: boolean
  risk_reasons: string[]
  risk_level: number
  request_count: number
  registration_count: number
  api_request_count: number
  error_count: number
  unauthorized_count: number
  login_success_count: number
  login_failure_count: number
  logout_count: number
  anomaly_count: number
  user_count: number
  registration_user_count: number
  api_user_count: number
  authentication_user_count: number
  quota: number
  first_seen_at: number
  last_seen_at: number
  users?: IPAuditUser[]
  timeline?: IPAuditTimelineItem[]
}

export interface IPAuditSyncStatus {
  status: 'pending' | 'running' | 'ready' | 'failed' | 'disabled' | 'unknown'
  ready: boolean
  processed_logs: number
  processed_users: number
  started_at: number
  updated_at: number
}

export interface IPAuditDetail {
  enabled: boolean
  ip: string
  users: IPAuditUser[]
  user_total: number
  user_page: number
  user_page_size: number
  user_has_more: boolean
  timeline: IPAuditTimelineItem[]
  timeline_total: number
  timeline_page: number
  timeline_page_size: number
  timeline_has_more: boolean
  sync: IPAuditSyncStatus
}

export interface GetIPAuditParams {
  range?: '7d' | '30d' | '90d' | 'all' | 'custom'
  view?:
    | 'risk'
    | 'shared_api'
    | 'shared_registration'
    | 'authentication'
    | 'all'
  start_timestamp?: number
  end_timestamp?: number
  p?: number
  page_size?: number
  user_page?: number
  user_page_size?: number
  timeline_page?: number
  timeline_page_size?: number
  related_page?: number
  related_page_size?: number
}

export interface GetIPAuditResponse {
  success: boolean
  message?: string
  data?: {
    enabled: boolean
    items: IPAuditItem[]
    total: number
    page: number
    page_size: number
    range: '7d' | '30d' | '90d' | 'all' | 'custom'
    view:
      | 'risk'
      | 'shared_api'
      | 'shared_registration'
      | 'authentication'
      | 'all'
    start_timestamp: number
    end_timestamp: number
    is_root: boolean
    can_view_details: boolean
    proxy_warning?: string
    sync: IPAuditSyncStatus
  }
}

export interface GetIPAuditDetailResponse {
  success: boolean
  message?: string
  data?: IPAuditDetail
}

// ============================================================================
// Drawing Log Types
// ============================================================================

export interface GetMidjourneyLogsParams {
  p?: number
  page_size?: number
  channel_id?: string
  mj_id?: string
  start_timestamp?: number
  end_timestamp?: number
}

// ============================================================================
// Task Log Types
// ============================================================================

export interface GetTaskLogsParams {
  p?: number
  page_size?: number
  channel_id?: string
  task_id?: string
  start_timestamp?: number
  end_timestamp?: number
}

// ============================================================================
// Fetch Logs Configuration
// ============================================================================

/**
 * Configuration for fetching logs by category
 */
export interface FetchLogsConfig {
  logCategory: LogCategory
  isAdmin: boolean
  page: number
  pageSize: number
  searchParams: Record<string, unknown>
  columnFilters: Array<{ id: string; value: unknown }>
}

// ============================================================================
// User Info Types
// ============================================================================

export interface UserInfo {
  id: number
  username: string
  display_name?: string
  quota: number
  used_quota: number
  request_count: number
  group?: string
  aff_code?: string
  aff_count?: number
  aff_quota?: number
  remark?: string
}
