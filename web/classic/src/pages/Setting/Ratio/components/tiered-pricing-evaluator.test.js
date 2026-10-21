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

import { evaluateTieredPricingExpression } from './tiered-pricing-evaluator';

const extraFields = [
  { var: 'cr', stateKey: 'cacheReadTokens' },
  { var: 'cc', stateKey: 'cacheCreateTokens' },
];

describe('classic tiered pricing evaluator', () => {
  test('evaluates allowed tier expressions without code generation', () => {
    expect(
      evaluateTieredPricingExpression(
        'p < 100 ? tier("small", p * 2 + c + cr) : tier("large", p * 3 + c)',
        10,
        4,
        { cacheReadTokens: 6 },
        extraFields,
      ),
    ).toEqual({ cost: 30, matchedTier: 'small', error: null });
  });

  test('counts only cache inputs in the len variable', () => {
    expect(
      evaluateTieredPricingExpression(
        'tier("length", len)',
        10,
        4,
        { cacheReadTokens: 6, imageTokens: 20 },
        [...extraFields, { var: 'img', stateKey: 'imageTokens' }],
      ),
    ).toEqual({ cost: 16, matchedTier: 'length', error: null });
  });

  test('rejects global and member expression execution', () => {
    const result = evaluateTieredPricingExpression(
      'globalThis.process.exit(1)',
      10,
      4,
      {},
      extraFields,
    );

    expect(result.cost).toBe(0);
    expect(result.matchedTier).toBe('');
    expect(result.error).toContain('Unsupported billing expression node');
  });
});
