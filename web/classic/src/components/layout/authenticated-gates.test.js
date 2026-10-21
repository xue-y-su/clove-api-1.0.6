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
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { getAuthenticatedGateAccess } from './authenticated-gates';

describe('classic authenticated blocking gate order', () => {
  test.each([
    [false, 'satisfied', 'satisfied', [false, false, false, null]],
    [true, 'required', 'satisfied', [true, false, false, null]],
    [true, 'satisfied', 'required', [true, true, false, null]],
    [true, 'satisfied', 'satisfied', [true, true, true, null]],
    [true, 'error', 'satisfied', [true, false, false, 'compliance']],
    [true, 'satisfied', 'error', [true, false, false, 'sourceAttribution']],
  ])(
    'serializes gate access',
    (signedIn, compliance, sourceAttribution, expected) => {
      const access = getAuthenticatedGateAccess({
        signedIn,
        compliance,
        sourceAttribution,
      });
      expect([
        access.compliance,
        access.sourceAttribution,
        access.announcement,
        access.failed,
      ]).toEqual(expected);
    },
  );

  test('queries both checks together but displays them in order', () => {
    const source = readFileSync(
      fileURLToPath(new URL('./AuthenticatedGateStack.jsx', import.meta.url)),
      'utf8',
    );

    expect(source).toContain('queryEnabled={Boolean(userId)}');
    expect(source).toContain('displayEnabled={access.compliance}');
    expect(source).toContain('displayEnabled={access.sourceAttribution}');
    expect(source).not.toContain("if (complianceResolution !== 'satisfied')");
  });

  test('fails both classic checks after the bounded request timeout', () => {
    const complianceSource = readFileSync(
      fileURLToPath(
        new URL('../compliance/ComplianceGate.jsx', import.meta.url),
      ),
      'utf8',
    );
    const attributionSource = readFileSync(
      fileURLToPath(
        new URL(
          '../source-attribution/SourceAttributionGate.jsx',
          import.meta.url,
        ),
      ),
      'utf8',
    );

    expect(complianceSource).toContain('timeout: 10000');
    expect(attributionSource).toContain('timeout: 10000');
    expect(complianceSource).toContain("? 'error'");
    expect(attributionSource).toContain("? 'error'");
  });

  test('renders a retry action for the failed classic check', () => {
    const source = readFileSync(
      fileURLToPath(new URL('./AuthenticatedGateStack.jsx', import.meta.url)),
      'utf8',
    );

    expect(source).toContain('<GateLoadError');
    expect(source).toContain('onRetry={retryFailedGate}');
  });
});
