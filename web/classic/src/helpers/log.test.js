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

import { describe, expect, test } from 'bun:test';
import { readdir, readFile } from 'node:fs/promises';

import { AUDIT_TEMPLATES, renderAuditContent } from './log';

function sourceObjectKeys(source, startMarker, endMarker, pattern) {
  const start = source.indexOf(startMarker);
  const end = source.indexOf(endMarker, start);
  const block = source.slice(start, end);
  return [...block.matchAll(pattern)].map((match) => match[1] || match[2]);
}

async function userRedemptionAuditActions() {
  const serviceDirectory = new URL('../../../../service/', import.meta.url);
  const filenames = (await readdir(serviceDirectory)).filter((filename) =>
    /^user_redemption.*\.go$/.test(filename),
  );
  const sources = await Promise.all(
    filenames.map((filename) =>
      readFile(new URL(filename, serviceDirectory), 'utf8'),
    ),
  );

  return [
    ...new Set(
      sources.flatMap((source) =>
        [...source.matchAll(/"(user\.redemption(?:\.[a-z_]+)+)"/g)].map(
          (match) => match[1],
        ),
      ),
    ),
  ];
}

function t(template, opts = {}) {
  return template.replace(/{{([^}]+)}}/g, (_, key) => String(opts[key] ?? ''));
}

const AUDIT_LOCALE_KEYS = [
  'Wallet {{category}} changed by {{delta}}; balance from {{before}} to {{after}}; reference {{reference_type}} {{reference_id}}',
  'Top-up',
  'Administrator adjustment',
  'API usage',
  'API pre-charge',
  'API settlement charge',
  'API settlement refund',
  'API refund',
  'Task usage',
  'Task charge',
  'Task refund',
  'Subscription overflow charge',
  'Subscription overflow settlement refund',
  'Subscription overflow refund',
  'Daily check-in',
  'Subscription purchase',
  'Invitation gift',
  'Invitation reward',
  'Redemption code',
  'Redemption code funding',
  'Redemption code claim',
  'Redemption code refund',
  'Other wallet adjustment',
  'Unrecognized operation',
  'Password',
  'Approved',
  'Rejected',
  'Withdrawn',
  'Viewed API token key (ID: {{token_id}})',
  'Viewed an API token key',
];

const SUBSCRIPTION_REDEMPTION_TEMPLATES = [
  'Created subscription redemption code {{id}}',
  'Claimed subscription redemption code {{id}}',
  'Closed subscription redemption code {{id}} because its plan was disabled',
  'Queued subscription redemption code {{id}} for refund',
  'Refunded subscription redemption code {{id}}',
  'Forfeited subscription redemption code {{id}}',
  'Queued user redemption code {{id}} for cancellation',
  'Queued expired user redemption code {{id}} for settlement',
];

describe('classic audit action rendering', () => {
  test('covers CloveAPI production audit actions with Chinese translations', async () => {
    const requiredActions = [
      'announcement.acknowledge',
      'announcement.media_upload',
      'announcement.settings_update',
      'ip_audit_detail_read',
      'ip_audit_read',
      'log.full_ip_read',
      'log.full_ip_stat_read',
      'security.settings_update',
      'user.login_failed',
      'user.login_success',
      'user.logout',
      'user.password_setup',
      'user.profile_update',
      'user.register',
      'user.security_audit_read',
      'wallet.adjust',
      'user.redemption.create',
      'user.redemption.claim',
      'user.redemption.cancel_pending',
      'user.redemption.expire_pending',
      'user.redemption.subscription.create',
      'user.redemption.subscription.claim',
      'user.redemption.subscription.plan_disabled',
      'user.redemption.subscription.refund_pending',
      'user.redemption.subscription.refund',
      'user.redemption.subscription.forfeit',
      'user.redemption.cancel',
      'user.redemption.expire_refund',
      'user.redemption.disable',
      'user.redemption.enable',
      'user.redemption.terminate_refund',
      'user.redemption.terminate_forfeit',
      'user.redemption.delete',
      'token.key_view',
      'token.secret_reveal_verify',
      'user.binding_clear_self',
    ];

    for (const action of requiredActions) {
      const template = AUDIT_TEMPLATES[action];
      expect(template, action).toBeTruthy();
    }

    for (const localeFile of ['zh-CN.json', 'zh-TW.json', 'zh.json']) {
      const locale = await Bun.file(
        new URL(`../i18n/locales/${localeFile}`, import.meta.url),
      ).json();
      for (const template of new Set(Object.values(AUDIT_TEMPLATES))) {
        expect(
          locale.translation[template],
          `${localeFile}: ${template}`,
        ).toBeTruthy();
        expect(
          locale.translation[template],
          `${localeFile}: ${template}`,
        ).not.toBe(template);
      }
      for (const key of AUDIT_LOCALE_KEYS) {
        expect(locale.translation[key], `${localeFile}: ${key}`).toBeTruthy();
        expect(locale.translation[key], `${localeFile}: ${key}`).not.toBe(key);
      }
    }
  });

  test('covers every Default and backend managed-audit action', async () => {
    const defaultSource = await Bun.file(
      new URL(
        '../../../default/src/features/usage-logs/lib/format.ts',
        import.meta.url,
      ),
    ).text();
    const defaultActions = sourceObjectKeys(
      defaultSource,
      'export const AUDIT_TEMPLATES',
      'export function renderAuditContent',
      /^\s*(?:'([^']+)'|([a-z_][a-z0-9_]*)):/gm,
    );
    const backendSource = await Bun.file(
      new URL('../../../../controller/audit.go', import.meta.url),
    ).text();
    const backendActions = sourceObjectKeys(
      backendSource,
      'var auditContentTemplates',
      '// auditContentEN',
      /^\s*"([^"]+)":/gm,
    );

    for (const action of new Set([...defaultActions, ...backendActions])) {
      expect(AUDIT_TEMPLATES[action], action).toBeTruthy();
    }
  });

  test('covers every user-redemption audit action emitted by the backend', async () => {
    for (const action of await userRedemptionAuditActions()) {
      expect(AUDIT_TEMPLATES[action], action).toBeTruthy();
    }
  });

  test('translates subscription-redemption audits in every locale', async () => {
    for (const localeFile of [
      'en.json',
      'fr.json',
      'ja.json',
      'ru.json',
      'vi.json',
      'zh-CN.json',
      'zh-TW.json',
      'zh.json',
    ]) {
      const locale = await Bun.file(
        new URL(`../i18n/locales/${localeFile}`, import.meta.url),
      ).json();

      for (const template of SUBSCRIPTION_REDEMPTION_TEMPLATES) {
        expect(
          locale.translation[template],
          `${localeFile}: ${template}`,
        ).toBeTruthy();
        if (localeFile === 'en.json') {
          expect(
            locale.translation[template],
            `${localeFile}: ${template}`,
          ).toBe(template);
        } else {
          expect(
            locale.translation[template],
            `${localeFile}: ${template}`,
          ).not.toBe(template);
        }
      }
    }
  });

  test.each([
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
  ])('renders %s from structured log data', (action, params, expected) => {
    expect(renderAuditContent({ op: { action, params } }, t)).toBe(expected);
  });

  test('renders detailed wallet changes from structured fields', () => {
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
        t,
      ),
    ).toBe(
      'Wallet Top-up changed by $25.00; balance from $100.00 to $125.00; reference topup trade-13',
    );
  });

  test('localizes known login methods and invitation review statuses', () => {
    const translations = {
      'Logged in successfully via {{method}}': '已通过 {{method}} 登录成功',
      'Logged in successfully via {{login_method}}':
        '已通过 {{login_method}} 登录成功',
      'Reviewed invitation application {{application_id}} for user {{target_user_id}}: {{status}}':
        '已审核用户 {{target_user_id}} 的邀请码申请 {{application_id}}：{{status}}',
      Password: '密码',
      Rejected: '已驳回',
    };
    const localize = (template, opts = {}) =>
      (translations[template] || template).replace(/{{([^}]+)}}/g, (_, key) =>
        String(opts[key] ?? ''),
      );

    expect(
      renderAuditContent(
        { op: { action: 'login', params: { method: 'password' } } },
        localize,
      ),
    ).toBe('已通过 密码 登录成功');
    expect(
      renderAuditContent(
        {
          op: {
            action: 'user.login_success',
            params: { login_method: 'password' },
          },
        },
        localize,
      ),
    ).toBe('已通过 密码 登录成功');
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
        localize,
      ),
    ).toBe('已审核用户 3007 的邀请码申请 19：已驳回');
  });

  test('renders historical token reveal logs without a missing-name placeholder', () => {
    expect(
      renderAuditContent(
        {
          op: {
            action: 'token.secret_reveal',
            params: { token_id: 99004 },
          },
        },
        t,
      ),
    ).toBe('Viewed API token key (ID: 99004)');
  });

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
          t,
        ),
      ).toBe(
        `Wallet ${expectedCategory} changed by ${delta}; balance from 100 to ${100 + delta}; reference relay_request req-1`,
      );
    },
  );

  test('hides raw content for unknown structured actions', () => {
    expect(renderAuditContent({ op: { action: 'unknown' } }, t)).toBe(
      'Unrecognized operation',
    );
    expect(renderAuditContent({}, t)).toBeNull();
  });
});
