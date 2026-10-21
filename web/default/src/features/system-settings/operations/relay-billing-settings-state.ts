/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import type {
  RelayBillingSettings,
  ToolPriceSetting,
  UpstreamDiagnosticSetting,
} from '../types'

export const DEFAULT_TOOL_PRICES: Record<string, number> = {
  web_search: 10,
  web_search_preview: 10,
  file_search: 2.5,
  google_search: 14,
}

export const DEFAULT_IMAGE_PRICE_MATRIX: Record<
  string,
  Record<string, number>
> = {
  low: { '1024x1024': 0.011, '1024x1536': 0.016, '1536x1024': 0.016 },
  medium: { '1024x1024': 0.042, '1024x1536': 0.063, '1536x1024': 0.063 },
  high: { '1024x1024': 0.167, '1024x1536': 0.25, '1536x1024': 0.25 },
}

export const DEFAULT_RELAY_BILLING_SETTINGS: RelayBillingSettings = {
  alpha_search: { enabled: true },
  tool_pricing: {
    enabled: true,
    prices: DEFAULT_TOOL_PRICES,
    multiplier_mode: 'group_only',
    image_pricing_mode: 'matrix',
    image_price_matrix: DEFAULT_IMAGE_PRICE_MATRIX,
    image_model_mode_overrides: {},
  },
  upstream_diagnostic: {
    enabled: true,
    capture_mode: 'fallback_only',
    preview_chars: 2048,
    sync_to_usage_log: false,
    dedupe_window_seconds: 60,
  },
}

const clone = <T>(value: T): T => {
  if (typeof structuredClone === 'function') return structuredClone(value)
  return JSON.parse(JSON.stringify(value)) as T
}

function finiteNonNegative(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
}

export type RelayBillingSettingsInput = {
  alpha_search?: Partial<RelayBillingSettings['alpha_search']>
  tool_pricing?: Partial<ToolPriceSetting>
  upstream_diagnostic?: Partial<UpstreamDiagnosticSetting>
}

export function hydrateRelayBillingSettings(
  input: RelayBillingSettingsInput | null | undefined
): RelayBillingSettings {
  const source = input ?? {}
  const tool = source.tool_pricing ?? {}
  const diagnostic = source.upstream_diagnostic ?? {}
  return {
    alpha_search: {
      enabled:
        source.alpha_search?.enabled ??
        DEFAULT_RELAY_BILLING_SETTINGS.alpha_search.enabled,
    },
    tool_pricing: {
      enabled:
        tool.enabled ?? DEFAULT_RELAY_BILLING_SETTINGS.tool_pricing.enabled,
      prices: { ...DEFAULT_TOOL_PRICES, ...(tool.prices ?? {}) },
      multiplier_mode: tool.multiplier_mode ?? 'group_only',
      image_pricing_mode: tool.image_pricing_mode ?? 'matrix',
      image_price_matrix: {
        ...clone(DEFAULT_IMAGE_PRICE_MATRIX),
        ...(tool.image_price_matrix ?? {}),
      },
      image_model_mode_overrides: {
        ...(tool.image_model_mode_overrides ?? {}),
      },
    },
    upstream_diagnostic: {
      enabled: diagnostic.enabled ?? true,
      capture_mode: diagnostic.capture_mode ?? 'fallback_only',
      preview_chars: diagnostic.preview_chars ?? 2048,
      sync_to_usage_log: diagnostic.sync_to_usage_log ?? false,
      dedupe_window_seconds: diagnostic.dedupe_window_seconds ?? 60,
    },
  }
}

export function validateRelayBillingSettings(
  settings: RelayBillingSettings
): string[] {
  const errors: string[] = []
  const tool = settings.tool_pricing
  if (!['group_only', 'full'].includes(tool.multiplier_mode)) {
    errors.push('multiplier_mode')
  }
  if (!['matrix', 'flat'].includes(tool.image_pricing_mode)) {
    errors.push('image_pricing_mode')
  }
  for (const [key, value] of Object.entries(tool.prices ?? {})) {
    if (!key.trim() || !finiteNonNegative(value)) errors.push(`price:${key}`)
  }
  for (const [quality, sizes] of Object.entries(
    tool.image_price_matrix ?? {}
  )) {
    if (!quality.trim() || !sizes) {
      errors.push(`image:${quality}`)
      continue
    }
    for (const [size, value] of Object.entries(sizes)) {
      if (!size.trim() || !finiteNonNegative(value)) {
        errors.push(`image:${quality}/${size}`)
      }
    }
  }
  for (const [prefix, mode] of Object.entries(
    tool.image_model_mode_overrides ?? {}
  )) {
    if (!prefix.trim() || !['matrix', 'flat'].includes(mode)) {
      errors.push(`override:${prefix}`)
    }
  }
  const diagnostic = settings.upstream_diagnostic
  if (!['fallback_only', 'all_errors'].includes(diagnostic.capture_mode)) {
    errors.push('capture_mode')
  }
  if (
    !Number.isInteger(diagnostic.preview_chars) ||
    diagnostic.preview_chars < 256 ||
    diagnostic.preview_chars > 8192
  ) {
    errors.push('preview_chars')
  }
  if (
    !Number.isInteger(diagnostic.dedupe_window_seconds) ||
    diagnostic.dedupe_window_seconds < 0 ||
    diagnostic.dedupe_window_seconds > 3600
  ) {
    errors.push('dedupe_window_seconds')
  }
  return errors
}

export function parseRelayBillingSettings(text: string): {
  settings: RelayBillingSettings
  error: string | null
} {
  try {
    const parsed = JSON.parse(text) as RelayBillingSettingsInput
    const settings = hydrateRelayBillingSettings(parsed)
    const errors = validateRelayBillingSettings(settings)
    return {
      settings,
      error: errors.length
        ? `Invalid relay billing fields: ${errors.join(', ')}`
        : null,
    }
  } catch (error) {
    return {
      settings: clone(DEFAULT_RELAY_BILLING_SETTINGS),
      error: error instanceof Error ? error.message : 'Invalid JSON',
    }
  }
}

export function highPriceWarnings(settings: RelayBillingSettings): string[] {
  const warnings: string[] = []
  for (const [key, value] of Object.entries(settings.tool_pricing.prices)) {
    if (value >= 1000) warnings.push(`price:${key}`)
  }
  for (const [quality, sizes] of Object.entries(
    settings.tool_pricing.image_price_matrix
  )) {
    for (const [size, value] of Object.entries(sizes)) {
      if (value >= 10) warnings.push(`image:${quality}/${size}`)
    }
  }
  return warnings
}

export function serializeRelayBillingSettings(
  settings: RelayBillingSettings
): string {
  return JSON.stringify(hydrateRelayBillingSettings(settings))
}

export type RelayBillingToolPrice = ToolPriceSetting
export type RelayBillingDiagnostic = UpstreamDiagnosticSetting
