/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { describe, expect, it } from 'bun:test'

import {
  DEFAULT_RELAY_BILLING_SETTINGS,
  highPriceWarnings,
  hydrateRelayBillingSettings,
  parseRelayBillingSettings,
  serializeRelayBillingSettings,
  validateRelayBillingSettings,
} from './relay-billing-settings-state'

describe('relay billing settings state', () => {
  it('hydrates missing fields without losing explicit zero values', () => {
    const settings = hydrateRelayBillingSettings({
      tool_pricing: { prices: { web_search: 0 } },
      upstream_diagnostic: { dedupe_window_seconds: 0 },
    })
    expect(settings.tool_pricing.prices.web_search).toBe(0)
    expect(settings.upstream_diagnostic.dedupe_window_seconds).toBe(0)
    expect(settings.upstream_diagnostic.preview_chars).toBe(2048)
  })

  it('rejects invalid JSON and invalid numeric bounds', () => {
    expect(parseRelayBillingSettings('{').error).toBeTruthy()
    const settings = hydrateRelayBillingSettings({
      upstream_diagnostic: { preview_chars: 255, dedupe_window_seconds: 3601 },
    })
    expect(validateRelayBillingSettings(settings)).toEqual([
      'preview_chars',
      'dedupe_window_seconds',
    ])
  })

  it('round trips modes, image matrix and model overrides', () => {
    const source = hydrateRelayBillingSettings({
      alpha_search: { enabled: false },
      tool_pricing: {
        enabled: false,
        multiplier_mode: 'full',
        image_pricing_mode: 'flat',
        image_model_mode_overrides: { 'gpt-image-1*': 'matrix' },
      },
      upstream_diagnostic: { capture_mode: 'all_errors' },
    })
    expect(
      parseRelayBillingSettings(serializeRelayBillingSettings(source))
    ).toEqual({
      settings: source,
      error: null,
    })
  })

  it('reports high prices without blocking them', () => {
    const settings = hydrateRelayBillingSettings({
      tool_pricing: {
        prices: { expensive: 1000 },
        image_price_matrix: { high: { '1024x1024': 10 } },
      },
    })
    expect(highPriceWarnings(settings)).toEqual([
      'price:expensive',
      'image:high/1024x1024',
    ])
    expect(DEFAULT_RELAY_BILLING_SETTINGS.alpha_search.enabled).toBe(true)
  })
})
