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

import { describe, expect, it } from 'bun:test';

import {
  DEFAULT_RELAY_BILLING_SETTINGS,
  highPriceWarnings,
  hydrateRelayBillingSettings,
  serializeRelayBillingSettings,
  validateRelayBillingSettings,
} from './relay-billing-settings.js';

describe('relay billing settings state', () => {
  it('keeps explicit zero values while hydrating defaults', () => {
    const settings = hydrateRelayBillingSettings({
      tool_pricing: { prices: { web_search: 0 } },
      upstream_diagnostic: { dedupe_window_seconds: 0 },
    });
    expect(settings.tool_pricing.prices.web_search).toBe(0);
    expect(settings.upstream_diagnostic.dedupe_window_seconds).toBe(0);
  });

  it('validates preview and dedupe bounds', () => {
    const settings = hydrateRelayBillingSettings({
      upstream_diagnostic: { preview_chars: 255, dedupe_window_seconds: 3601 },
    });
    expect(validateRelayBillingSettings(settings)).toEqual([
      'preview_chars',
      'dedupe_window_seconds',
    ]);
  });

  it('round trips settings and reports high-price warnings', () => {
    const settings = hydrateRelayBillingSettings({
      alpha_search: { enabled: false },
      tool_pricing: { prices: { expensive: 1000 } },
    });
    expect(JSON.parse(serializeRelayBillingSettings(settings))).toEqual(
      settings,
    );
    expect(highPriceWarnings(settings)).toEqual(['price:expensive']);
    expect(DEFAULT_RELAY_BILLING_SETTINGS.alpha_search.enabled).toBe(true);
  });
});
