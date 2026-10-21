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

export function buildProfileIdentityPatch(current, draft) {
  const patch = {};
  const displayName = draft.displayName.trim();
  const avatarURL = draft.avatarURL.trim();
  if (displayName !== current.displayName.trim()) {
    patch.display_name = displayName;
  }
  if (avatarURL !== current.avatarURL.trim()) {
    patch.avatar_url = avatarURL;
  }
  return patch;
}

export function formatDisplayNameCooldownRemaining(
  nextChangeAt,
  nowSeconds = Math.floor(Date.now() / 1000),
  locale,
) {
  if (typeof nextChangeAt !== 'number' || nextChangeAt <= nowSeconds) {
    return '';
  }
  const remainingSeconds = nextChangeAt - nowSeconds;
  let value;
  let unit;
  if (remainingSeconds >= 24 * 60 * 60) {
    value = Math.ceil(remainingSeconds / (24 * 60 * 60));
    unit = 'day';
  } else if (remainingSeconds >= 60 * 60) {
    value = Math.ceil(remainingSeconds / (60 * 60));
    unit = 'hour';
  } else {
    value = Math.max(1, Math.ceil(remainingSeconds / 60));
    unit = 'minute';
  }
  return new Intl.RelativeTimeFormat(locale, { numeric: 'always' }).format(
    value,
    unit,
  );
}

const supportedAvatarTypes = new Set([
  'image/jpeg',
  'image/png',
  'image/webp',
  'image/gif',
]);

function avatarUploadResponseURL(payload) {
  if (!payload || typeof payload !== 'object') return '';
  if (typeof payload.url === 'string') return payload.url.trim();
  if (typeof payload.data === 'string') return payload.data.trim();
  if (payload.data && typeof payload.data === 'object') {
    if (typeof payload.data.url === 'string') return payload.data.url.trim();
    if (typeof payload.data.image_url === 'string') {
      return payload.data.image_url.trim();
    }
  }
  return '';
}

function isAllowedAvatarURL(url, allowedDomains) {
  try {
    const parsed = new URL(url);
    if (
      parsed.protocol !== 'https:' ||
      parsed.username ||
      parsed.password ||
      parsed.port ||
      parsed.hash
    ) {
      return false;
    }
    const host = parsed.hostname.toLowerCase();
    return allowedDomains.some(
      (domain) => String(domain).trim().toLowerCase() === host,
    );
  } catch {
    return false;
  }
}

export async function uploadAvatarToImageHost(
  file,
  config,
  fetcher = fetch,
  options = {},
) {
  if (!supportedAvatarTypes.has(String(file.type || '').toLowerCase())) {
    throw new Error('头像必须是 JPEG、PNG、WebP 或 GIF 图片');
  }
  if (!config?.enabled || !String(config.endpoint || '').trim()) {
    throw new Error('头像上传暂不可用');
  }
  if (options.signal?.aborted) {
    throw new Error('头像上传已取消');
  }
  const controller = new AbortController();
  let timedOut = false;
  const abortUpload = () => controller.abort();
  options.signal?.addEventListener('abort', abortUpload, { once: true });
  const timeoutId = globalThis.setTimeout(
    () => {
      timedOut = true;
      controller.abort();
    },
    Math.max(1, options.timeoutMs ?? 30000),
  );
  const formData = new FormData();
  formData.append('image', file, file.name);
  try {
    let response;
    try {
      response = await fetcher(config.endpoint, {
        method: 'POST',
        body: formData,
        signal: controller.signal,
      });
    } catch {
      if (timedOut) throw new Error('头像上传超时，请重试');
      if (options.signal?.aborted) throw new Error('头像上传已取消');
      throw new Error('无法连接头像图床，请检查网络或稍后重试');
    }
    let payload;
    try {
      payload = await response.json();
    } catch {
      if (timedOut) throw new Error('头像上传超时，请重试');
      if (options.signal?.aborted) throw new Error('头像上传已取消');
      throw new Error('图床返回了无效响应');
    }
    if (!response.ok || payload?.success === false) {
      throw new Error(payload?.message || payload?.error || '头像上传失败');
    }
    const url = avatarUploadResponseURL(payload);
    if (!isAllowedAvatarURL(url, config.allowed_domains || [])) {
      throw new Error('图床返回了不受信任的地址');
    }
    return url;
  } finally {
    globalThis.clearTimeout(timeoutId);
    options.signal?.removeEventListener('abort', abortUpload);
  }
}

export function isDisplayNameCoolingDown(
  nextChangeAt,
  nowSeconds = Math.floor(Date.now() / 1000),
) {
  return typeof nextChangeAt === 'number' && nextChangeAt > nowSeconds;
}
