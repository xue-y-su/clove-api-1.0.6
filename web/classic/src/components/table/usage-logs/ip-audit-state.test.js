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
import { beforeAll, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

let subject = {};
const panelSource = readFileSync(
  new URL('./IPAuditPanel.jsx', import.meta.url),
  'utf8',
);
const columnSource = readFileSync(
  new URL('./UsageLogsColumnDefs.jsx', import.meta.url),
  'utf8',
);
const controlsSource = readFileSync(
  new URL('./IPAuditRangeControls.jsx', import.meta.url),
  'utf8',
);
const userModalSource = readFileSync(
  new URL('../users/modals/UserSecurityAuditModal.jsx', import.meta.url),
  'utf8',
);

beforeAll(async () => {
  try {
    subject = await import('./ip-audit-state.js');
  } catch {
    subject = {};
  }
});

describe('classic IP audit URL state', () => {
  test('builds a first-page all-history usage-log link for a timeline request', () => {
    const next = subject.buildUsageLogSearchParams?.(
      new URLSearchParams(
        'tab=ip-audit&range=custom&view=shared_api&start_timestamp=1&end_timestamp=2&p=9&page_size=50&scope=all',
      ),
      ' req-123 ',
    );

    expect(next?.toString()).toBe(
      'tab=usage&range=all&p=1&scope=all&request_id=req-123',
    );
    expect(subject.readUsageLogDeepLink?.(next)).toEqual({
      requestId: 'req-123',
      allHistory: true,
    });
  });

  test('ignores empty usage-log request IDs and non-history ranges', () => {
    expect(
      subject.buildUsageLogSearchParams?.(
        new URLSearchParams('tab=ip-audit&range=30d&p=4'),
        '   ',
      ),
    ).toBeNull();
    expect(
      subject.readUsageLogDeepLink?.(
        new URLSearchParams('request_id=req-123&range=30d'),
      ),
    ).toEqual({ requestId: 'req-123', allHistory: false });
  });

  test('opens IP details in an independent responsive side sheet', () => {
    expect(panelSource).toContain('SideSheet');
    expect(panelSource).toContain("width={isMobile ? '100%' : 720}");
    expect(panelSource).toContain('selectedIP');
    expect(panelSource).not.toContain('expandedRowRender');
    expect(panelSource).not.toContain('expandedRowKeys');
  });

  test('guards the IP audit tab to administrators', () => {
    expect(subject.normalizeLogTab?.('ip-audit', true)).toBe('ip-audit');
    expect(subject.normalizeLogTab?.('ip-audit', false)).toBe('usage');
    expect(subject.normalizeLogTab?.('unknown', true)).toBe('usage');
  });

  test('defaults to the server-compatible thirty-day query', () => {
    expect(subject.readIPAuditQuery?.(new URLSearchParams())).toEqual({
      range: '30d',
      view: 'risk',
      startTimestamp: 0,
      endTimestamp: 0,
      page: 1,
      pageSize: 10,
    });
  });

  test('normalizes projection progress without hiding partial audit rows', () => {
    expect(
      subject.normalizeIPAuditPage?.({
        enabled: true,
        items: [{ ip: '8.8.8.8' }],
        sync: {
          status: 'running',
          ready: false,
          processed_logs: 1250,
          processed_users: 25,
        },
      }).sync,
    ).toEqual({
      status: 'running',
      ready: false,
      processed_logs: 1250,
      processed_users: 25,
    });
    expect(panelSource).toContain('page.sync?.ready === false');
    expect(panelSource).toContain('当前显示已完成同步的数据');
  });

  test('supports every preset and valid custom timestamps', () => {
    for (const range of ['7d', '30d', '90d', 'all']) {
      expect(
        subject.readIPAuditQuery?.(new URLSearchParams(`range=${range}`)).range,
      ).toBe(range);
    }

    expect(
      subject.readIPAuditQuery?.(
        new URLSearchParams(
          'range=custom&start_timestamp=1700000000&end_timestamp=1700003600&p=3&page_size=50',
        ),
      ),
    ).toEqual({
      range: 'custom',
      view: 'risk',
      startTimestamp: 1700000000,
      endTimestamp: 1700003600,
      page: 3,
      pageSize: 50,
    });
  });

  test('replaces an invalid custom range with a safe default', () => {
    expect(
      subject.readIPAuditQuery?.(
        new URLSearchParams(
          'range=custom&start_timestamp=1700003600&end_timestamp=1700000000&p=-2&page_size=1000',
        ),
      ),
    ).toEqual({
      range: '30d',
      view: 'risk',
      startTimestamp: 0,
      endTimestamp: 0,
      page: 1,
      pageSize: 100,
    });
  });

  test('resets pagination when filters change and preserves unrelated URL state', () => {
    const next = subject.updateIPAuditSearchParams?.(
      new URLSearchParams(
        'tab=ip-audit&range=custom&view=shared_api&start_timestamp=1&end_timestamp=2&p=4&page_size=20&scope=all',
      ),
      { range: '90d' },
    );

    expect(next?.toString()).toBe(
      'tab=ip-audit&range=90d&view=shared_api&p=1&page_size=20&scope=all',
    );
  });

  test('removes custom timestamps when a preset control supplies zero values', () => {
    const next = subject.updateIPAuditSearchParams?.(
      new URLSearchParams(
        'tab=ip-audit&range=custom&start_timestamp=1&end_timestamp=2',
      ),
      { range: '7d', startTimestamp: 0, endTimestamp: 0 },
    );

    expect(next?.toString()).toBe('tab=ip-audit&range=7d&p=1');
  });

  test('builds encoded global and user detail endpoints', () => {
    const query = {
      range: 'custom',
      view: 'shared_registration',
      startTimestamp: 1700000000,
      endTimestamp: 1700003600,
      page: 2,
      pageSize: 20,
      relatedPage: 3,
      relatedPageSize: 50,
      userPage: 2,
      userPageSize: 50,
      timelinePage: 4,
      timelinePageSize: 10,
    };

    expect(subject.buildIPAuditURL?.('/api/log/ip-audit', query)).toBe(
      '/api/log/ip-audit?range=custom&view=shared_registration&start_timestamp=1700000000&end_timestamp=1700003600&p=2&page_size=20',
    );
    expect(subject.buildUserSecurityAuditURL?.(42, query)).toBe(
      '/api/user/42/security-audit?range=custom&view=shared_registration&start_timestamp=1700000000&end_timestamp=1700003600&p=2&page_size=20&related_page=3&related_page_size=50',
    );
    expect(subject.buildIPAuditDetailURL?.('::ffff:203.0.113.57', query)).toBe(
      '/api/log/ip-audit/detail?range=custom&view=shared_registration&start_timestamp=1700000000&end_timestamp=1700003600&user_page=2&user_page_size=50&timeline_page=4&timeline_page_size=10&ip=%3A%3Affff%3A203.0.113.57',
    );
  });
});

describe('classic IP audit response projection', () => {
  const rootPage = {
    enabled: true,
    is_root: true,
    can_view_details: true,
    total: 47,
    page: 2,
    page_size: 20,
    range: '30d',
    start_timestamp: 1700000000,
    end_timestamp: 1700003600,
    proxy_warning: 'trusted_proxies_unsafe',
    items: [
      {
        ip: '::ffff:203.0.113.57',
        risk_reasons: [
          'shared_registration_ip',
          'shared_api_ip',
          'anomalous_activity',
        ],
        request_count: 12,
        registration_count: 2,
        api_request_count: 6,
        error_count: 2,
        login_success_count: 3,
        login_failure_count: 1,
        logout_count: 2,
        anomaly_count: 3,
        user_count: 2,
        registration_user_count: 2,
        api_user_count: 1,
        authentication_user_count: 1,
        quota: 123,
        first_seen_at: 1700000000,
        last_seen_at: 1700003600,
        users: [{ user_id: 1, username: 'root' }],
        timeline: [
          {
            created_at: 1700003600,
            user_id: 1,
            username: 'root',
            kind: 'login_failure',
            content: 'must not cross the UI contract',
          },
        ],
      },
    ],
  };

  test('keeps server totals and stable Root details without raw content', () => {
    const page = subject.normalizeIPAuditPage?.(rootPage);

    expect(page.enabled).toBe(true);
    expect(page.total).toBe(47);
    expect(page.items).toHaveLength(1);
    expect(page.items[0]).toMatchObject({
      ip: '::ffff:203.0.113.57',
      registration_count: 2,
      api_request_count: 6,
      error_count: 2,
      login_success_count: 3,
      login_failure_count: 1,
      logout_count: 2,
      anomaly_count: 3,
      registration_user_count: 2,
      api_user_count: 1,
      authentication_user_count: 1,
      risk_reasons: [
        'shared_registration_ip',
        'shared_api_ip',
        'anomalous_activity',
      ],
    });
    expect(page.items[0].users).toEqual([{ user_id: 1, username: 'root' }]);
    expect(page.items[0].timeline[0]).toEqual({
      created_at: 1700003600,
      user_id: 1,
      username: 'root',
      kind: 'login_failure',
      request_id: '',
    });
    expect(page.items[0].timeline[0]).not.toHaveProperty('content');
  });

  test('keeps independent IP detail totals and pages', () => {
    expect(
      subject.normalizeIPAuditDetail?.({
        enabled: true,
        users: [{ user_id: 7, username: 'bob' }],
        user_total: 300,
        user_page: 2,
        user_page_size: 50,
        user_has_more: true,
        timeline: [{ created_at: 10, user_id: 7, kind: 'api_request' }],
        timeline_total: 81,
        timeline_page: 3,
        timeline_page_size: 20,
        timeline_has_more: true,
      }),
    ).toMatchObject({
      user_total: 300,
      user_page: 2,
      user_page_size: 50,
      user_has_more: true,
      timeline_total: 81,
      timeline_page: 3,
      timeline_page_size: 20,
      timeline_has_more: true,
    });
  });

  test('uses the server detail capability for masked Admin associations', () => {
    const page = subject.normalizeIPAuditPage?.({
      ...rootPage,
      is_root: false,
      can_view_details: true,
      items: [{ ...rootPage.items[0], ip: '203.0.113.*' }],
    });

    expect(page.items[0].ip).toBe('203.0.113.*');
    expect(page.can_view_details).toBe(true);
    expect(page.items[0].users).toEqual([{ user_id: 1, username: 'root' }]);
    expect(page.items[0].timeline).toHaveLength(1);
  });

  test('distinguishes loading, error, empty, and success states', () => {
    expect(subject.getIPAuditViewState?.({ loading: true })).toBe('loading');
    expect(subject.getIPAuditViewState?.({ error: 'failed' })).toBe('error');
    expect(subject.getIPAuditViewState?.({ enabled: false, items: [] })).toBe(
      'disabled',
    );
    expect(subject.getIPAuditViewState?.({ items: [] })).toBe('empty');
    expect(subject.getIPAuditViewState?.({ items: [{}] })).toBe('success');
  });

  test('renders a dedicated loading state before empty results', () => {
    expect(panelSource).toMatch(/const content =\s*viewState === 'loading'/);
    expect(panelSource).toContain("tip={t('加载中...')}");
  });

  test('renders the disabled state and wraps long IPv6 values', () => {
    expect(panelSource).toContain("viewState === 'disabled'");
    expect(userModalSource).toContain('data.enabled === false');
    expect(columnSource).toContain('min-w-0');
    expect(columnSource).toContain('whitespace-normal');
    expect(columnSource).toContain('[overflow-wrap:anywhere]');
  });

  test('wires risk views and independent detail pagination into the classic UI', () => {
    expect(controlsSource).toContain('value={query.view}');
    expect(controlsSource).toContain('onChange({ view: event.target.value })');
    expect(panelSource).toContain('query.view');
    expect(panelSource).toContain('userPage');
    expect(panelSource).toContain('timelinePage');
    expect(panelSource).toContain('state.data.user_total');
    expect(panelSource).toContain('state.data.timeline_total');
  });

  test('shows separate account populations instead of one ambiguous total', () => {
    expect(columnSource).toContain('record.registration_user_count');
    expect(columnSource).toContain('record.api_user_count');
    expect(columnSource).toContain('record.authentication_user_count');
    expect(columnSource).toContain('record.user_count');
  });

  test('shows unauthorized responses without promoting ordinary upstream errors', () => {
    expect(columnSource).toMatch(
      /t\('未授权（401）'\),\s*record\.unauthorized_count/,
    );
    expect(columnSource).not.toMatch(/t\('错误'\),\s*record\.error_count/);
  });

  test('maps stable risk reasons and exposes an explicit detail action', () => {
    expect(
      subject.getIPAuditRiskReasonTranslationKeys?.({
        risk_reasons: [
          'shared_registration_ip',
          'shared_api_ip',
          'unauthorized_burst',
          'unknown_reason',
        ],
      }),
    ).toEqual(['共享注册 IP', '共享 API IP', '未授权请求突增']);
    expect(panelSource).toContain("t('查看 IP 详情')");
    expect(panelSource).toContain('onClick={() => setSelectedIP(record.ip)}');
    expect(columnSource).toContain('buildIPThreatURL');
    expect(columnSource).toContain('can_query_external_profile');
    expect(columnSource).toContain("reasonKey === '未授权请求突增'");
  });

  test('preserves infrastructure IP classification and renders its badge', () => {
    const page = subject.normalizeIPAuditPage?.({
      items: [{ ip: '172.18.0.1', proxy_artifact: true }],
    });

    expect(page.items[0].proxy_artifact).toBe(true);
    expect(columnSource).toContain('record.proxy_artifact');
    expect(columnSource).toContain("t('基础设施 IP')");
  });

  test('redirects an empty out-of-range response to the last available page', () => {
    expect(
      subject.getIPAuditPageCorrection?.({
        items: [],
        total: 21,
        page: 9,
        page_size: 10,
      }),
    ).toBe(3);
    expect(
      subject.getIPAuditPageCorrection?.({
        items: [{}],
        total: 21,
        page: 9,
        page_size: 10,
      }),
    ).toBeNull();
    expect(
      subject.getIPAuditPageCorrection?.({
        items: [],
        total: 21,
        page: 2,
        page_size: 10,
      }),
    ).toBeNull();
    expect(
      subject.getIPAuditPageCorrection?.({
        items: [],
        total: 0,
        page: 9,
        page_size: 10,
      }),
    ).toBeNull();
  });

  test('maps audit failures without exposing internal error details', () => {
    expect(subject.getIPAuditErrorDescriptionKey?.('ip_audit_forbidden')).toBe(
      '您无权访问此页面，请联系管理员',
    );
    expect(
      subject.getIPAuditErrorDescriptionKey?.('ip_audit_invalid_range'),
    ).toBe('请求参数无效');
    expect(
      subject.getIPAuditErrorDescriptionKey?.('ip_audit_projection_not_ready'),
    ).toBe('IP 审计数据仍在准备中，请稍后重试。');
    expect(
      subject.getIPAuditErrorDescriptionKey?.(
        'dial tcp: password authentication failed',
      ),
    ).toBe('请稍后重试。');
  });

  test('maps both trusted-proxy warnings to actionable messages', () => {
    expect(
      subject.getProxyWarningPresentation?.('trusted_proxies_not_configured'),
    ).toMatchObject({ tone: 'warning', titleKey: '可信代理尚未配置' });
    expect(
      subject.getProxyWarningPresentation?.('trusted_proxies_unsafe'),
    ).toMatchObject({ tone: 'danger', titleKey: '可信代理配置不安全' });
    expect(subject.getProxyWarningPresentation?.('')).toBeNull();
  });

  test('formats audit timestamps with the active locale', () => {
    const timestamp = 1700000000;
    expect(subject.formatIPAuditTimestamp?.(timestamp, 'fr-FR')).toBe(
      new Date(timestamp * 1000).toLocaleString('fr-FR'),
    );
    expect(subject.formatIPAuditTimestamp?.(0, 'fr-FR')).toBe('-');
  });
});

test('localizes IP audit synchronization progress in every active locale', () => {
  const localeNames = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW', 'zh'];
  const keys = [
    'IP 审计数据同步中',
    '当前显示已完成同步的数据',
    '已处理历史日志',
  ];

  for (const localeName of localeNames) {
    const locale = JSON.parse(
      readFileSync(
        new URL(`../../../i18n/locales/${localeName}.json`, import.meta.url),
        'utf8',
      ),
    ).translation;
    for (const key of keys) {
      expect(typeof locale[key], `${localeName}: ${key}`).toBe('string');
      expect(locale[key].trim(), `${localeName}: ${key}`).not.toBe('');
    }
  }
});

describe('classic user security audit projection', () => {
  test('paginates related accounts and explains why they are related', () => {
    expect(userModalSource).toContain("view: 'risk'");
    expect(userModalSource).toContain('relatedPage: 1');
    expect(userModalSource).toContain('relatedPageSize: 20');
    expect(userModalSource).toContain('relatedUser.shared_ip_count');
    expect(userModalSource).toContain('relatedUser.registration_count');
    expect(userModalSource).toContain('relatedUser.api_call_count');
    expect(userModalSource).toContain('relatedUser.authentication_count');
    expect(userModalSource).toContain('total={data.related_total}');
  });

  test('hides a previous response while a different audit request loads', () => {
    const previousData = { username: 'alice' };
    const previousRequest = {
      requestURL: '/api/user/42/security-audit?range=30d&p=1&page_size=10',
      status: 'success',
      data: previousData,
      error: '',
    };

    expect(
      subject.getUserSecurityAuditRequestView?.(
        '/api/user/42/security-audit?range=90d&p=1&page_size=10',
        previousRequest,
      ),
    ).toEqual({ data: null, error: '', loading: true });
    expect(
      subject.getUserSecurityAuditRequestView?.(
        previousRequest.requestURL,
        previousRequest,
      ),
    ).toEqual({ data: previousData, error: '', loading: false });
  });

  test('treats a logout-only summary as security activity', () => {
    expect(subject.hasUserSecurityAuditActivity?.({ logout_count: 1 })).toBe(
      true,
    );
    expect(subject.hasUserSecurityAuditActivity?.({ logout_count: 0 })).toBe(
      false,
    );
  });

  test('treats association-only and recent summaries as security activity', () => {
    expect(
      subject.hasUserSecurityAuditActivity?.({ related_user_count: 1 }),
    ).toBe(true);
    expect(
      subject.hasUserSecurityAuditActivity?.({ last_activity_at: 1700000000 }),
    ).toBe(true);
  });

  test('corrects an empty out-of-range user timeline page', () => {
    expect(
      subject.getUserSecurityAuditPageCorrection?.({
        timeline: [],
        total: 21,
        page: 9,
        page_size: 10,
      }),
    ).toBe(3);
    expect(
      subject.getUserSecurityAuditPageCorrection?.({
        timeline: [{ kind: 'registration' }],
        total: 21,
        page: 9,
        page_size: 10,
      }),
    ).toBeNull();
  });

  test('keeps Root relations and timeline, but strips them from Admin responses', () => {
    const data = {
      target_user_id: 42,
      username: 'alice',
      projection: 'full',
      is_root: true,
      summary: { registration_ip: '203.0.113.42', anomaly_count: 2 },
      related_users: [
        {
          user_id: 7,
          username: 'bob',
          shared_ip_count: 2,
          registration_count: 1,
          api_call_count: 4,
          authentication_count: 3,
          anomaly_count: 1,
          last_seen_at: 1700003600,
        },
      ],
      related_total: 1,
      related_page: 1,
      related_page_size: 20,
      related_has_more: false,
      timeline: [
        {
          created_at: 1700003600,
          ip: '203.0.113.42',
          kind: 'api_error',
          request_id: 'req-1',
        },
      ],
      total: 12,
      page: 2,
      page_size: 10,
    };

    expect(subject.normalizeUserSecurityAudit?.(data)).toMatchObject({
      enabled: true,
      related_users: [
        {
          user_id: 7,
          username: 'bob',
          shared_ip_count: 2,
          registration_count: 1,
          api_call_count: 4,
          authentication_count: 3,
          anomaly_count: 1,
          last_seen_at: 1700003600,
        },
      ],
      related_total: 1,
      total: 12,
    });
    expect(
      subject.normalizeUserSecurityAudit?.({
        ...data,
        is_root: false,
        projection: 'masked',
      }),
    ).toMatchObject({ related_users: [], timeline: [], total: 0 });
  });

  test('preserves disabled policy in global and user projections', () => {
    expect(subject.normalizeIPAuditPage?.({ enabled: false })).toMatchObject({
      enabled: false,
      items: [],
    });
    expect(
      subject.normalizeUserSecurityAudit?.({ enabled: false }),
    ).toMatchObject({ enabled: false, related_users: [], timeline: [] });
  });

  test('preserves external profile permission and unauthorized counts', () => {
    expect(
      subject.normalizeIPAuditPage?.({
        is_root: true,
        can_view_details: true,
        items: [
          {
            ip: '8.8.8.8',
            can_query_external_profile: true,
            unauthorized_count: 10,
          },
        ],
      }).items[0],
    ).toMatchObject({
      can_query_external_profile: true,
      unauthorized_count: 10,
    });
    expect(subject.buildIPThreatURL?.('2001:4860:4860::8888', true)).toContain(
      encodeURIComponent('2001:4860:4860::8888'),
    );
    expect(subject.buildIPThreatURL?.('127.0.0.1', false)).toBeNull();
  });
});
