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
import { expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

test('queries authenticated gates in parallel and renders retryable failures', () => {
  const layoutSource = readFileSync(
    fileURLToPath(new URL('./authenticated-layout.tsx', import.meta.url)),
    'utf8'
  )
  const complianceSource = readFileSync(
    fileURLToPath(
      new URL(
        '../../../features/compliance/compliance-gate.tsx',
        import.meta.url
      )
    ),
    'utf8'
  )
  const attributionSource = readFileSync(
    fileURLToPath(
      new URL(
        '../../../features/source-attribution/source-attribution-gate.tsx',
        import.meta.url
      )
    ),
    'utf8'
  )
  const complianceApiSource = readFileSync(
    fileURLToPath(
      new URL('../../../features/compliance/api.ts', import.meta.url)
    ),
    'utf8'
  )
  const attributionApiSource = readFileSync(
    fileURLToPath(
      new URL('../../../features/source-attribution/api.ts', import.meta.url)
    ),
    'utf8'
  )

  expect(layoutSource).toContain('queryEnabled={Boolean(userId)}')
  expect(layoutSource).toContain(
    "import { useCallback, useEffect, useRef, useState } from 'react'"
  )
  expect(layoutSource).toContain('const resolvedUserId = useRef(userId)')
  expect(layoutSource).toContain(
    'if (resolvedUserId.current === userId) return'
  )
  expect(layoutSource).toContain('resolvedUserId.current = userId')
  expect(layoutSource).toContain('displayEnabled={gateAccess.compliance}')
  expect(layoutSource).toContain(
    'displayEnabled={gateAccess.sourceAttribution}'
  )
  expect(layoutSource).not.toContain(
    "if (complianceResolution !== 'satisfied')"
  )
  expect(layoutSource).toContain('<GateLoadError')
  expect(layoutSource).toContain('gateQueryKey(gateAccess.failed)')
  expect(complianceSource).toContain("? 'error'")
  expect(attributionSource).toContain("? 'error'")
  expect(complianceSource).toContain('queryFn: ({ signal }) =>')
  expect(attributionSource).toContain('queryFn: ({ signal }) =>')
  expect(complianceApiSource).toContain('timeout: 10_000')
  expect(attributionApiSource).toContain('timeout: 10_000')
})
