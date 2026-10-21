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
const DEFAULT_USER_EDIT_VALUES = {
  username: '',
  display_name: '',
  password: '',
  github_id: '',
  oidc_id: '',
  discord_id: '',
  wechat_id: '',
  telegram_id: '',
  linux_do_id: '',
  email: '',
  quota: 0,
  quota_amount: 0,
  group: 'default',
  remark: '',
  invite_limit_mode: 'inherit',
  invite_limit_count: 0,
};

function normalizeInviteLimitMode(mode) {
  return ['inherit', 'limited', 'unlimited'].includes(mode) ? mode : 'inherit';
}

export function normalizeUserEditValues(user = {}) {
  const rawCount = Number(user.invite_limit_count);
  return {
    ...DEFAULT_USER_EDIT_VALUES,
    ...user,
    invite_limit_mode: normalizeInviteLimitMode(user.invite_limit_mode),
    invite_limit_count: Number.isFinite(rawCount)
      ? Math.max(0, Math.trunc(rawCount))
      : 0,
  };
}

export function buildUserEditPayload(values, userId) {
  const payload = { ...values };
  delete payload.quota;
  delete payload.quota_amount;
  payload.invite_limit_mode = normalizeInviteLimitMode(
    payload.invite_limit_mode,
  );
  payload.invite_limit_count = Math.max(
    0,
    Math.trunc(Number(payload.invite_limit_count) || 0),
  );
  if (userId) {
    payload.id = Number.parseInt(userId, 10);
  }
  return payload;
}
