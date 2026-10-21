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

export const IP_AUDIT_RANGES = ['7d', '30d', '90d', 'all', 'custom'];
export const IP_AUDIT_VIEWS = [
  'risk',
  'shared_api',
  'shared_registration',
  'authentication',
  'all',
];

const rangeSet = new Set(IP_AUDIT_RANGES);
const viewSet = new Set(IP_AUDIT_VIEWS);
const ipAuditRiskReasonTranslationKeys = {
  shared_registration_ip: '共享注册 IP',
  shared_api_ip: '共享 API IP',
  unauthorized_burst: '未授权请求突增',
};

const baiduIPThreatURL =
  'https://cloud.baidu.com/product-s/afd_s/ip-threat.html?s=';

export const buildIPThreatURL = (ip, allowed) => {
  const value = typeof ip === 'string' ? ip.trim() : '';
  if (!allowed || value === '') return null;
  return `${baiduIPThreatURL}${encodeURIComponent(value)}`;
};

const toPositiveInteger = (value, fallback) => {
  const number = Number.parseInt(value, 10);
  return Number.isInteger(number) && number > 0 ? number : fallback;
};

const toCount = (value) => {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? Math.trunc(number) : 0;
};

const toTimestamp = (value) => {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? Math.trunc(number) : 0;
};

const readParams = (value) =>
  value instanceof URLSearchParams ? value : new URLSearchParams(value || '');

export const normalizeLogTab = (tab, admin) =>
  admin && tab === 'ip-audit' ? 'ip-audit' : 'usage';

export const buildUsageLogSearchParams = (current, requestID) => {
  const normalizedRequestID =
    typeof requestID === 'string' ? requestID.trim() : '';
  if (!normalizedRequestID) return null;

  const params = new URLSearchParams(readParams(current));
  for (const key of ['view', 'start_timestamp', 'end_timestamp', 'page_size']) {
    params.delete(key);
  }
  params.set('tab', 'usage');
  params.set('range', 'all');
  params.set('p', '1');
  params.set('request_id', normalizedRequestID);
  return params;
};

export const readUsageLogDeepLink = (value) => {
  const params = readParams(value);
  const requestId = (params.get('request_id') || '').trim();
  return {
    requestId,
    allHistory: requestId !== '' && params.get('range') === 'all',
  };
};

export const readIPAuditQuery = (value) => {
  const params = readParams(value);
  let range = params.get('range') || '30d';
  if (!rangeSet.has(range)) {
    range = '30d';
  }

  let startTimestamp = 0;
  let endTimestamp = 0;
  if (range === 'custom') {
    startTimestamp = toTimestamp(params.get('start_timestamp'));
    endTimestamp = toTimestamp(params.get('end_timestamp'));
    if (!startTimestamp || !endTimestamp || startTimestamp > endTimestamp) {
      range = '30d';
      startTimestamp = 0;
      endTimestamp = 0;
    }
  }

  return {
    range,
    view: viewSet.has(params.get('view')) ? params.get('view') : 'risk',
    startTimestamp,
    endTimestamp,
    page: toPositiveInteger(params.get('p'), 1),
    pageSize: Math.min(toPositiveInteger(params.get('page_size'), 10), 100),
  };
};

export const updateIPAuditSearchParams = (current, patch) => {
  const params = new URLSearchParams(readParams(current));
  const filterChanged =
    Object.hasOwn(patch, 'range') ||
    Object.hasOwn(patch, 'view') ||
    Object.hasOwn(patch, 'startTimestamp') ||
    Object.hasOwn(patch, 'endTimestamp');

  if (Object.hasOwn(patch, 'range')) {
    const range = rangeSet.has(patch.range) ? patch.range : '30d';
    params.set('range', range);
    if (range !== 'custom') {
      params.delete('start_timestamp');
      params.delete('end_timestamp');
    }
  }
  if (Object.hasOwn(patch, 'view')) {
    params.set('view', viewSet.has(patch.view) ? patch.view : 'risk');
  }
  if (params.get('range') === 'custom') {
    if (Object.hasOwn(patch, 'startTimestamp')) {
      params.set('start_timestamp', String(toTimestamp(patch.startTimestamp)));
    }
    if (Object.hasOwn(patch, 'endTimestamp')) {
      params.set('end_timestamp', String(toTimestamp(patch.endTimestamp)));
    }
  } else {
    params.delete('start_timestamp');
    params.delete('end_timestamp');
  }
  if (Object.hasOwn(patch, 'pageSize')) {
    params.set(
      'page_size',
      String(Math.min(toPositiveInteger(patch.pageSize, 10), 100)),
    );
  }
  if (filterChanged || Object.hasOwn(patch, 'pageSize')) {
    params.set('p', '1');
  } else if (Object.hasOwn(patch, 'page')) {
    params.set('p', String(toPositiveInteger(patch.page, 1)));
  }

  return params;
};

const buildIPAuditParams = (query) => {
  const params = new URLSearchParams({
    range: query.range,
    view: viewSet.has(query.view) ? query.view : 'risk',
  });
  if (query.range === 'custom') {
    params.set('start_timestamp', String(query.startTimestamp));
    params.set('end_timestamp', String(query.endTimestamp));
  }
  return params;
};

export const buildIPAuditURL = (basePath, query) => {
  const params = buildIPAuditParams(query);
  params.set('p', String(query.page));
  params.set('page_size', String(query.pageSize));
  return `${basePath}?${params.toString()}`;
};

export const buildUserSecurityAuditURL = (userId, query) => {
  const params = buildIPAuditParams(query);
  params.set('p', String(query.page));
  params.set('page_size', String(query.pageSize));
  if (query.relatedPage) {
    params.set('related_page', String(toPositiveInteger(query.relatedPage, 1)));
  }
  if (query.relatedPageSize) {
    params.set(
      'related_page_size',
      String(Math.min(toPositiveInteger(query.relatedPageSize, 20), 100)),
    );
  }
  return `/api/user/${encodeURIComponent(userId)}/security-audit?${params.toString()}`;
};

export const buildIPAuditDetailURL = (ip, query) => {
  const params = buildIPAuditParams(query);
  if (query.userPage) {
    params.set('user_page', String(toPositiveInteger(query.userPage, 1)));
  }
  if (query.userPageSize) {
    params.set(
      'user_page_size',
      String(Math.min(toPositiveInteger(query.userPageSize, 20), 100)),
    );
  }
  if (query.timelinePage) {
    params.set(
      'timeline_page',
      String(toPositiveInteger(query.timelinePage, 1)),
    );
  }
  if (query.timelinePageSize) {
    params.set(
      'timeline_page_size',
      String(Math.min(toPositiveInteger(query.timelinePageSize, 20), 100)),
    );
  }
  params.set('ip', ip);
  return `/api/log/ip-audit/detail?${params.toString()}`;
};

export const formatIPAuditTimestamp = (timestamp, locale) =>
  timestamp ? new Date(timestamp * 1000).toLocaleString(locale) : '-';

const normalizeAuditTimeline = (timeline, includeUser) => {
  if (!Array.isArray(timeline)) {
    return [];
  }
  return timeline.map((event) => {
    const item = {
      created_at: toTimestamp(event?.created_at),
      kind: typeof event?.kind === 'string' ? event.kind : '',
      request_id: typeof event?.request_id === 'string' ? event.request_id : '',
    };
    if (includeUser) {
      item.user_id = toCount(event?.user_id);
      item.username = typeof event?.username === 'string' ? event.username : '';
    } else {
      item.ip = typeof event?.ip === 'string' ? event.ip : '';
    }
    return item;
  });
};

const normalizeAuditUsers = (users) => {
  if (!Array.isArray(users)) {
    return [];
  }
  return users.map((user) => ({
    user_id: toCount(user?.user_id),
    username: typeof user?.username === 'string' ? user.username : '',
  }));
};

const normalizeRelatedUsers = (users) =>
  normalizeAuditUsers(users).map((user, index) => ({
    ...user,
    shared_ip_count: toCount(users[index]?.shared_ip_count),
    registration_count: toCount(users[index]?.registration_count),
    api_call_count: toCount(users[index]?.api_call_count),
    authentication_count: toCount(users[index]?.authentication_count),
    anomaly_count: toCount(users[index]?.anomaly_count),
    last_seen_at: toTimestamp(users[index]?.last_seen_at),
  }));

export const normalizeIPAuditDetail = (data = {}) => ({
  enabled: data?.enabled !== false,
  ip: typeof data?.ip === 'string' ? data.ip : '',
  users: normalizeAuditUsers(data?.users),
  user_total: toCount(data?.user_total),
  user_page: toPositiveInteger(data?.user_page, 1),
  user_page_size: Math.min(toPositiveInteger(data?.user_page_size, 20), 100),
  user_has_more: data?.user_has_more === true,
  timeline: normalizeAuditTimeline(data?.timeline, true),
  timeline_total: toCount(data?.timeline_total),
  timeline_page: toPositiveInteger(data?.timeline_page, 1),
  timeline_page_size: Math.min(
    toPositiveInteger(data?.timeline_page_size, 20),
    100,
  ),
  timeline_has_more: data?.timeline_has_more === true,
});

export const getIPAuditRiskReasonTranslationKeys = (item = {}) => {
  if (!Array.isArray(item?.risk_reasons)) {
    return [];
  }
  return item.risk_reasons
    .map((reason) => ipAuditRiskReasonTranslationKeys[reason])
    .filter(Boolean);
};

export const normalizeIPAuditPage = (data = {}) => {
  const isRoot = data?.is_root === true;
  const canViewDetails = data?.can_view_details === true;
  const items = Array.isArray(data?.items) ? data.items : [];
  const sync = data?.sync && typeof data.sync === 'object' ? data.sync : {};
  return {
    enabled: data?.enabled !== false,
    items: items.map((item) => ({
      ip: typeof item?.ip === 'string' ? item.ip : '',
      proxy_artifact: item?.proxy_artifact === true,
      can_query_external_profile: item?.can_query_external_profile === true,
      risk_reasons: Array.isArray(item?.risk_reasons)
        ? item.risk_reasons.filter((reason) => typeof reason === 'string')
        : [],
      request_count: toCount(item?.request_count),
      registration_count: toCount(item?.registration_count),
      api_request_count: toCount(item?.api_request_count),
      error_count: toCount(item?.error_count),
      unauthorized_count: toCount(item?.unauthorized_count),
      login_success_count: toCount(item?.login_success_count),
      login_failure_count: toCount(item?.login_failure_count),
      logout_count: toCount(item?.logout_count),
      anomaly_count: toCount(item?.anomaly_count),
      risk_level: toCount(item?.risk_level),
      user_count: toCount(item?.user_count),
      registration_user_count: toCount(item?.registration_user_count),
      api_user_count: toCount(item?.api_user_count),
      authentication_user_count: toCount(item?.authentication_user_count),
      quota: toCount(item?.quota),
      first_seen_at: toTimestamp(item?.first_seen_at),
      last_seen_at: toTimestamp(item?.last_seen_at),
      users: canViewDetails ? normalizeAuditUsers(item?.users) : [],
      timeline: canViewDetails
        ? normalizeAuditTimeline(item?.timeline, true)
        : [],
    })),
    total: toCount(data?.total),
    page: toPositiveInteger(data?.page, 1),
    page_size: Math.min(toPositiveInteger(data?.page_size, 10), 100),
    range: rangeSet.has(data?.range) ? data.range : '30d',
    view: viewSet.has(data?.view) ? data.view : 'risk',
    start_timestamp: toTimestamp(data?.start_timestamp),
    end_timestamp: toTimestamp(data?.end_timestamp),
    is_root: isRoot,
    can_view_details: canViewDetails,
    proxy_warning:
      typeof data?.proxy_warning === 'string' ? data.proxy_warning : '',
    sync: {
      status:
        typeof sync.status === 'string' && sync.status !== ''
          ? sync.status
          : 'pending',
      ready: sync.ready === true,
      processed_logs: toCount(sync.processed_logs),
      processed_users: toCount(sync.processed_users),
    },
  };
};

export const normalizeUserSecurityAudit = (data = {}) => {
  const isRoot = data?.is_root === true && data?.projection === 'full';
  const summary =
    data?.summary && typeof data.summary === 'object' ? data.summary : {};
  const errorCount = toCount(summary.error_count);
  const loginFailureCount = toCount(summary.login_failure_count);
  return {
    enabled: data?.enabled !== false,
    target_user_id: toCount(data?.target_user_id),
    username: typeof data?.username === 'string' ? data.username : '',
    projection: isRoot ? 'full' : 'masked',
    is_root: isRoot,
    proxy_warning:
      typeof data?.proxy_warning === 'string' ? data.proxy_warning : '',
    summary: {
      registration_ip:
        typeof summary.registration_ip === 'string'
          ? summary.registration_ip
          : '',
      recent_ip: typeof summary.recent_ip === 'string' ? summary.recent_ip : '',
      api_request_count: toCount(summary.api_request_count),
      error_count: errorCount,
      login_success_count: toCount(summary.login_success_count),
      login_failure_count: loginFailureCount,
      logout_count: toCount(summary.logout_count),
      anomaly_count: toCount(
        summary.anomaly_count ?? errorCount + loginFailureCount,
      ),
      related_user_count: toCount(summary.related_user_count),
      last_activity_at: toTimestamp(summary.last_activity_at),
    },
    related_users: isRoot ? normalizeRelatedUsers(data?.related_users) : [],
    related_total: toCount(data?.related_total),
    related_page: toPositiveInteger(data?.related_page, 1),
    related_page_size: Math.min(
      toPositiveInteger(data?.related_page_size, 20),
      100,
    ),
    related_has_more: isRoot && data?.related_has_more === true,
    timeline: isRoot ? normalizeAuditTimeline(data?.timeline, false) : [],
    total: isRoot ? toCount(data?.total) : 0,
    page: toPositiveInteger(data?.page, 1),
    page_size: Math.min(toPositiveInteger(data?.page_size, 10), 100),
    range: rangeSet.has(data?.range) ? data.range : '30d',
    start_timestamp: toTimestamp(data?.start_timestamp),
    end_timestamp: toTimestamp(data?.end_timestamp),
  };
};

export const getUserSecurityAuditRequestView = (
  requestURL,
  requestState = {},
) => {
  if (!requestURL || requestState.requestURL !== requestURL) {
    return { data: null, error: '', loading: Boolean(requestURL) };
  }
  return {
    data: requestState.data ?? null,
    error:
      requestState.status === 'error' && typeof requestState.error === 'string'
        ? requestState.error
        : '',
    loading: requestState.status === 'loading',
  };
};

export const hasUserSecurityAuditActivity = (summary = {}) =>
  Boolean(
    summary.registration_ip ||
    summary.recent_ip ||
    summary.api_request_count ||
    summary.error_count ||
    summary.login_success_count ||
    summary.login_failure_count ||
    summary.logout_count ||
    summary.anomaly_count ||
    summary.related_user_count ||
    summary.last_activity_at,
  );

export const getIPAuditViewState = ({
  loading,
  error,
  enabled,
  items,
} = {}) => {
  if (loading) return 'loading';
  if (error) return 'error';
  if (enabled === false) return 'disabled';
  return Array.isArray(items) && items.length > 0 ? 'success' : 'empty';
};

export const getIPAuditPageCorrection = (data = {}) => {
  const items = Array.isArray(data?.items) ? data.items : [];
  const total = toCount(data?.total);
  if (items.length > 0 || total === 0) return null;

  const page = toPositiveInteger(data?.page, 1);
  const pageSize = Math.min(toPositiveInteger(data?.page_size, 10), 100);
  const lastPage = Math.max(1, Math.ceil(total / pageSize));
  return page > lastPage ? lastPage : null;
};

export const getUserSecurityAuditPageCorrection = (data = {}) =>
  getIPAuditPageCorrection({
    ...data,
    items: Array.isArray(data.timeline) ? data.timeline : [],
  });

export const getIPAuditErrorDescriptionKey = (error) => {
  const code = typeof error === 'string' ? error.trim() : '';
  if (code === 'ip_audit_forbidden') {
    return '您无权访问此页面，请联系管理员';
  }
  if (code === 'ip_audit_invalid_range') {
    return '请求参数无效';
  }
  if (code === 'ip_audit_projection_not_ready') {
    return 'IP 审计数据仍在准备中，请稍后重试。';
  }
  return '请稍后重试。';
};

export const getProxyWarningPresentation = (warning) => {
  if (warning === 'trusted_proxies_not_configured') {
    return {
      tone: 'warning',
      titleKey: '可信代理尚未配置',
      descriptionKey: '请配置 TRUSTED_PROXIES 后再将 IP 审计结果用于安全判断。',
    };
  }
  if (warning === 'trusted_proxies_unsafe') {
    return {
      tone: 'danger',
      titleKey: '可信代理配置不安全',
      descriptionKey: '当前 TRUSTED_PROXIES 范围过宽，客户端 IP 可能被伪造。',
    };
  }
  return null;
};
