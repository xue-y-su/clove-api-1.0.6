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
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const sourceRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..',
);

const read = (relativePath) => {
  try {
    return fs.readFileSync(path.join(sourceRoot, relativePath), 'utf8');
  } catch {
    return '';
  }
};

describe('classic IP audit wiring', () => {
  test('adds the guarded global audit tab and four-state panel', () => {
    const logPage = read('pages/Log/index.jsx');
    const panel = read('components/table/usage-logs/IPAuditPanel.jsx');
    const logsTable = read('components/table/usage-logs/index.jsx');
    const logsHook = read('hooks/usage-logs/useUsageLogsData.jsx');

    expect(logPage).toContain('normalizeLogTab');
    expect(logPage).toContain('IPAuditPanel');
    expect(panel).toContain('getIPAuditViewState');
    expect(panel).toContain('buildIPAuditDetailURL');
    expect(panel).toContain('IPAuditDetailRow');
    expect(panel).toContain('proxy_warning');
    expect(panel).toContain('createCardProPagination');
    expect(panel).toContain("logout: '退出'");
    const columns = read('components/table/usage-logs/UsageLogsColumnDefs.jsx');
    const securityModal = read(
      'components/table/users/modals/UserSecurityAuditModal.jsx',
    );
    expect(columns).toContain('record.logout_count');
    expect(columns).toMatch(/t\(['"]请求次数['"]\),\s*record\.request_count/);
    expect(columns).not.toMatch(/t\(['"]总计['"]\),\s*record\.request_count/);
    expect(panel).toContain('formatIPAuditTimestamp');
    expect(columns).toContain('formatIPAuditTimestamp');
    expect(securityModal).toContain('formatIPAuditTimestamp');
    expect(panel).toContain('buildUsageLogSearchParams');
    expect(logPage).toContain('readUsageLogDeepLink');
    expect(logPage).toContain('initialQuery={usageLogDeepLink}');
    expect(logsTable).toContain('useLogsData({ initialQuery })');
    expect(logsHook).toContain('initialQuery?.allHistory');
    expect(logsHook).toContain(
      'formApi ? formApi.getValues() : formInitValues',
    );
    expect(columns).toMatch(/key: 'ip',[\s\S]*?width: 170/);
    expect(columns).toMatch(/key: 'events',[\s\S]*?width: 160/);
    expect(columns).toMatch(/key: 'login',[\s\S]*?width: 140/);
    expect(columns).toMatch(/key: 'risk',[\s\S]*?width: 190/);
    expect(columns).toMatch(/key: 'users',[\s\S]*?width: 170/);
    expect(panel).toContain('scroll={{ x: 1320 }}');
    expect(panel).not.toContain("scroll={{ x: 'max-content' }}");
  });

  test('keeps security IP data in a dedicated permission-scoped modal', () => {
    const table = read('components/table/users/UsersTable.jsx');
    const columns = read('components/table/users/UsersColumnDefs.jsx');
    const modal = read(
      'components/table/users/modals/UserSecurityAuditModal.jsx',
    );
    const state = read('components/table/usage-logs/ip-audit-state.js');
    const genericUserInfo = read(
      'components/table/usage-logs/modals/UserInfoModal.jsx',
    );

    expect(table).toContain('UserSecurityAuditModal');
    expect(columns).toContain('canViewUserSecurityAudit');
    expect(modal).toContain('buildUserSecurityAuditURL');
    expect(state).toContain('/security-audit');
    expect(genericUserInfo).not.toContain('registration_ip');
    expect(genericUserInfo).not.toContain('recent_ip');
  });
});
