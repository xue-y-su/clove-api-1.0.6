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

function safeQuota(value) {
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function toPercent(value, total) {
  if (total <= 0) return 0;
  return Math.min(100, Math.max(0, (value / total) * 100));
}

export function getTokenCallableQuota(balance) {
  return balance.token_callable_quota ?? balance.current_available;
}

export function getBalanceProgressSegments(balance) {
  const callable = getTokenCallableQuota(balance);
  if (callable.unlimited) {
    return { available: 100, used: 0, restricted: 0 };
  }

  const available = safeQuota(callable.value);
  const used = safeQuota(balance.token_used);
  const restricted = balance.token_unlimited
    ? 0
    : safeQuota(balance.restricted);
  const total = balance.token_unlimited
    ? available + used
    : safeQuota(balance.token_total);

  if (total <= 0) {
    return { available: 0, used: 0, restricted: 0 };
  }

  return {
    available: toPercent(available, total),
    used: toPercent(used, total),
    restricted: toPercent(restricted, total),
  };
}

export function getBalanceMetricValues(balance) {
  return {
    available: getTokenCallableQuota(balance),
    used: safeQuota(balance.token_used),
    third: balance.token_unlimited
      ? { kind: 'unlimited', value: 0 }
      : { kind: 'restricted', value: safeQuota(balance.restricted) },
  };
}

export function formatBalanceQuota(amount, display, unlimitedLabel, locale) {
  if (amount.unlimited) return unlimitedLabel;

  const value = safeQuota(amount.value);
  if (display.type === 'TOKENS') {
    return new Intl.NumberFormat(locale, {
      maximumFractionDigits: 0,
    }).format(value);
  }

  const quotaPerUnit =
    Number.isFinite(display.quota_per_unit) && display.quota_per_unit > 0
      ? display.quota_per_unit
      : 1;
  const exchangeRate =
    Number.isFinite(display.exchange_rate) && display.exchange_rate > 0
      ? display.exchange_rate
      : 1;
  const converted = (value / quotaPerUnit) * exchangeRate;
  const digits = Math.abs(converted) < 1 && converted !== 0 ? 4 : 2;
  const formatted = new Intl.NumberFormat(locale, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(converted);

  return `${display.symbol}${formatted}`;
}

export function getBalanceQueryErrorPresentation(code, retryAfterSeconds) {
  if (code === 'invalid_request') {
    return {
      titleKey: '请输入 API Key',
      descriptionKey: '请输入需要查询余额的完整 API Key。',
      tone: 'danger',
    };
  }
  if (code === 'invalid_api_key') {
    return {
      titleKey: '余额查询失败',
      descriptionKey: 'API 密钥无效，请检查后重新输入。',
      tone: 'danger',
    };
  }
  if (code === 'rate_limited') {
    if (Number.isFinite(retryAfterSeconds) && Number(retryAfterSeconds) > 0) {
      return {
        titleKey: '请求过于频繁',
        descriptionKey: '{{seconds}} 秒后可重新查询。',
        tone: 'warning',
        values: { seconds: Number(retryAfterSeconds) },
      };
    }
    return {
      titleKey: '请求过于频繁',
      descriptionKey: '请求过于频繁，请稍后重试。',
      tone: 'warning',
    };
  }
  return {
    titleKey: '暂时无法查询余额',
    descriptionKey: '余额查询服务暂时不可用，请稍后重试。',
    tone: 'info',
  };
}

const statusExplanationKeys = {
  disabled: '此 API 密钥已被禁用。',
  expired: '此 API 密钥已过期。',
  exhausted: '此 API 密钥已无剩余额度。',
  user_disabled: '此 API 密钥所属用户已被禁用。',
};

export function getBalanceStatusExplanationKey(status) {
  return statusExplanationKeys[status] || null;
}

export function shouldShowWalletBalance(balance, billingPreference) {
  return Boolean(balance.wallet);
}
