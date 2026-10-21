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

export function getLogOther(otherStr) {
  if (otherStr === undefined || otherStr === null || otherStr === '') {
    return {};
  }
  if (typeof otherStr === 'object') {
    return otherStr;
  }
  try {
    return JSON.parse(otherStr);
  } catch (e) {
    console.error(`Failed to parse record.other: "${otherStr}".`, e);
    return null;
  }
}

const WALLET_ADJUST_TEMPLATE =
  'Wallet {{category}} changed by {{delta}}; balance from {{before}} to {{after}}; reference {{reference_type}} {{reference_id}}';
const UNKNOWN_AUDIT_TEMPLATE = 'Unrecognized operation';
const TOKEN_REVEAL_BY_ID_TEMPLATE = 'Viewed API token key (ID: {{token_id}})';
const TOKEN_REVEAL_GENERIC_TEMPLATE = 'Viewed an API token key';
const LOGIN_METHOD_LABELS = {
  password: 'Password',
};
const INVITATION_REVIEW_STATUS_LABELS = {
  approved: 'Approved',
  paid: 'Paid',
  pending: 'Pending',
  rejected: 'Rejected',
  withdrawn: 'Withdrawn',
};
const WALLET_CATEGORY_LABELS = {
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
};
const WALLET_CONTEXT_LABELS = {
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
};

export const AUDIT_TEMPLATES = {
  login: 'Logged in successfully via {{method}}',
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
  'billing.topup': 'Recorded top-up event {{trade_no}}',
  'billing.consume': 'Recorded consumption event {{request_id}}',
  'billing.refund': 'Recorded refund event {{request_id}}',
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
  'playground.web_search': 'Used playground web search with model {{model}}',
  'playground.image_generation':
    'Generated playground image with model {{model}}',
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
  'custom_oauth.create': 'Created a custom OAuth provider',
  'custom_oauth.update': 'Updated a custom OAuth provider',
  'custom_oauth.delete': 'Deleted a custom OAuth provider',
  'performance.clear_disk_cache': 'Cleared disk cache',
  'performance.gc': 'Triggered garbage collection',
  'performance.clear_logs': 'Cleared log files',
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
  'redemption.create':
    'Created {{count}} redemption codes named {{name}} ({{quota}} each)',
  'redemption.view': 'Viewed redemption code {{id}}',
  'redemption.update': 'Updated redemption code {{id}}',
  'redemption.status_update':
    'Updated redemption code {{id}} status to {{status}}',
  'redemption.delete': 'Deleted redemption code {{id}}',
  'redemption.delete_invalid': 'Deleted invalid redemption codes ({{count}})',
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
  'prefill_group.create': 'Created a prefill group',
  'prefill_group.update': 'Updated a prefill group',
  'prefill_group.delete': 'Deleted a prefill group',
  'vendor.create': 'Created a vendor',
  'vendor.update': 'Updated a vendor',
  'vendor.delete': 'Deleted a vendor',
  'model.create': 'Created a model',
  'model.update': 'Updated a model',
  'model.delete': 'Deleted a model',
  'model.sync_upstream': 'Synced upstream models',
  'deployment.create': 'Created a deployment',
  'deployment.update': 'Updated a deployment',
  'deployment.delete': 'Deleted a deployment',
  'subscription.plan_create': 'Created a subscription plan',
  'subscription.plan_update': 'Updated a subscription plan',
  'subscription.bind': 'Bound a subscription',
  'subscription.plan_reset': 'Reset active subscriptions for plan {{plan_id}}',
  'subscription.user_plan_reset':
    'Reset active plan {{plan_id}} subscriptions for user {{target_user_id}}',
  'log.clear': 'Cleared historical logs',
  'wallet.adjust': 'Updated wallet balance',
  generic: '{{method}} {{route}}',
};

export function renderAuditContent(other, t) {
  const action = other?.op?.action;
  if (!action) {
    return null;
  }
  const params = other.op.params || {};
  if (action === 'wallet.adjust') {
    const hasDetailedBalance = [
      params.delta_display,
      params.balance_before_display,
      params.balance_after_display,
    ].every((value) => value !== undefined && value !== null && value !== '');
    if (!hasDetailedBalance) {
      return t(AUDIT_TEMPLATES['wallet.adjust'], params);
    }
    const category = String(params.category || '');
    const referenceContext = String(params.reference_context || '');
    const delta = Number(params.delta);
    let categoryLabel =
      WALLET_CONTEXT_LABELS[`${category}:${referenceContext}`];
    if (
      !categoryLabel &&
      category === 'relay' &&
      referenceContext === 'post_consume' &&
      Number.isFinite(delta)
    ) {
      categoryLabel =
        delta > 0 ? 'API settlement refund' : 'API settlement charge';
    }
    if (!categoryLabel && category === 'task' && Number.isFinite(delta)) {
      categoryLabel = delta > 0 ? 'Task refund' : 'Task charge';
    }
    return t(WALLET_ADJUST_TEMPLATE, {
      ...params,
      category: t(
        categoryLabel ||
          WALLET_CATEGORY_LABELS[category] ||
          'Other wallet adjustment',
      ),
      delta: params.delta_display,
      before: params.balance_before_display,
      after: params.balance_after_display,
    });
  }
  const template = AUDIT_TEMPLATES[action];
  if (!template) {
    return t(UNKNOWN_AUDIT_TEMPLATE);
  }
  const localizedParams = { ...params };
  if (action === 'login') {
    const method = String(params.method || '');
    localizedParams.method = t(LOGIN_METHOD_LABELS[method] || method);
  }
  if (action === 'user.login_success' || action === 'user.login_failed') {
    const loginMethod = String(params.login_method || '');
    localizedParams.login_method = t(
      LOGIN_METHOD_LABELS[loginMethod] || loginMethod,
    );
  }
  if (
    action === 'invitation.application_reviewed' ||
    action === 'invitation.reward_review' ||
    action === 'invitation.reward_payout_reviewed'
  ) {
    const status = String(params.status || '');
    localizedParams.status = t(
      INVITATION_REVIEW_STATUS_LABELS[status] || status,
    );
  }
  if (
    (action === 'token.key_view' || action === 'token.secret_reveal') &&
    !String(params.name || '').trim()
  ) {
    if (params.token_id !== undefined && params.token_id !== null) {
      return t(TOKEN_REVEAL_BY_ID_TEMPLATE, localizedParams);
    }
    return t(TOKEN_REVEAL_GENERIC_TEMPLATE, localizedParams);
  }
  return t(template, localizedParams);
}
