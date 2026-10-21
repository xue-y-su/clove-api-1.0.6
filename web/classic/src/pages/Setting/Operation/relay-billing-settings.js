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

export const DEFAULT_RELAY_BILLING_SETTINGS = {
  alpha_search: { enabled: true },
  tool_pricing: {
    enabled: true,
    prices: {
      web_search: 10,
      web_search_preview: 10,
      file_search: 2.5,
      google_search: 14,
    },
    multiplier_mode: 'group_only',
    image_pricing_mode: 'matrix',
    image_price_matrix: {
      low: { '1024x1024': 0.011, '1024x1536': 0.016, '1536x1024': 0.016 },
      medium: { '1024x1024': 0.042, '1024x1536': 0.063, '1536x1024': 0.063 },
      high: { '1024x1024': 0.167, '1024x1536': 0.25, '1536x1024': 0.25 },
    },
    image_model_mode_overrides: {},
  },
  upstream_diagnostic: {
    enabled: true,
    capture_mode: 'fallback_only',
    preview_chars: 2048,
    sync_to_usage_log: false,
    dedupe_window_seconds: 60,
  },
};

const clone = (value) => JSON.parse(JSON.stringify(value));

export function hydrateRelayBillingSettings(input) {
  const source = input || {};
  const tool = source.tool_pricing || {};
  const diagnostic = source.upstream_diagnostic || {};
  return {
    alpha_search: { enabled: source.alpha_search?.enabled ?? true },
    tool_pricing: {
      enabled: tool.enabled ?? true,
      prices: {
        ...DEFAULT_RELAY_BILLING_SETTINGS.tool_pricing.prices,
        ...(tool.prices || {}),
      },
      multiplier_mode: tool.multiplier_mode || 'group_only',
      image_pricing_mode: tool.image_pricing_mode || 'matrix',
      image_price_matrix: {
        ...clone(
          DEFAULT_RELAY_BILLING_SETTINGS.tool_pricing.image_price_matrix,
        ),
        ...(tool.image_price_matrix || {}),
      },
      image_model_mode_overrides: {
        ...(tool.image_model_mode_overrides || {}),
      },
    },
    upstream_diagnostic: {
      enabled: diagnostic.enabled ?? true,
      capture_mode: diagnostic.capture_mode || 'fallback_only',
      preview_chars: diagnostic.preview_chars ?? 2048,
      sync_to_usage_log: diagnostic.sync_to_usage_log ?? false,
      dedupe_window_seconds: diagnostic.dedupe_window_seconds ?? 60,
    },
  };
}

function validNumber(value) {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0;
}

export function validateRelayBillingSettings(settings) {
  const errors = [];
  const tool = settings.tool_pricing;
  if (!['group_only', 'full'].includes(tool.multiplier_mode))
    errors.push('multiplier_mode');
  if (!['matrix', 'flat'].includes(tool.image_pricing_mode))
    errors.push('image_pricing_mode');
  Object.entries(tool.prices || {}).forEach(([key, value]) => {
    if (!key.trim() || !validNumber(value)) errors.push(`price:${key}`);
  });
  Object.entries(tool.image_price_matrix || {}).forEach(([quality, sizes]) => {
    Object.entries(sizes || {}).forEach(([size, value]) => {
      if (!quality.trim() || !size.trim() || !validNumber(value))
        errors.push(`image:${quality}/${size}`);
    });
  });
  Object.entries(tool.image_model_mode_overrides || {}).forEach(
    ([prefix, mode]) => {
      if (!prefix.trim() || !['matrix', 'flat'].includes(mode))
        errors.push(`override:${prefix}`);
    },
  );
  const diagnostic = settings.upstream_diagnostic;
  if (!['fallback_only', 'all_errors'].includes(diagnostic.capture_mode))
    errors.push('capture_mode');
  if (
    !Number.isInteger(diagnostic.preview_chars) ||
    diagnostic.preview_chars < 256 ||
    diagnostic.preview_chars > 8192
  )
    errors.push('preview_chars');
  if (
    !Number.isInteger(diagnostic.dedupe_window_seconds) ||
    diagnostic.dedupe_window_seconds < 0 ||
    diagnostic.dedupe_window_seconds > 3600
  )
    errors.push('dedupe_window_seconds');
  return errors;
}

export function highPriceWarnings(settings) {
  const warnings = [];
  Object.entries(settings.tool_pricing.prices || {}).forEach(([key, value]) => {
    if (value >= 1000) warnings.push(`price:${key}`);
  });
  Object.entries(settings.tool_pricing.image_price_matrix || {}).forEach(
    ([quality, sizes]) => {
      Object.entries(sizes || {}).forEach(([size, value]) => {
        if (value >= 10) warnings.push(`image:${quality}/${size}`);
      });
    },
  );
  return warnings;
}

export function serializeRelayBillingSettings(settings) {
  return JSON.stringify(hydrateRelayBillingSettings(settings));
}
