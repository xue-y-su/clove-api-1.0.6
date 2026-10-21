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
export const MAX_INVITATION_REAPPLY_COOLDOWN_SECONDS = 30 * 24 * 60 * 60;
const DEFAULT_INVITATION_REAPPLY_COOLDOWN_SECONDS = 24 * 60 * 60;
export const INVITATION_POSTER_TITLE_MAX_CHARACTERS = 80;
export const INVITATION_POSTER_SUBTITLE_MAX_CHARACTERS = 160;

export function countPosterCharacters(value) {
  return Array.from(String(value || '')).length;
}

export function limitPosterCharacters(value, limit) {
  return Array.from(String(value || ''))
    .slice(0, limit)
    .join('');
}

function normalizePosterCopy(value, limit) {
  return limitPosterCharacters(String(value || '').trim(), limit);
}

function normalizeInvitationReapplyCooldown(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return DEFAULT_INVITATION_REAPPLY_COOLDOWN_SECONDS;
  }
  return Math.max(
    0,
    Math.min(MAX_INVITATION_REAPPLY_COOLDOWN_SECONDS, Math.trunc(parsed)),
  );
}

export function normalizeRegistrationSettings(settings = {}) {
  const rawCount = Number(settings.invite_limit_count);
  return {
    affRequired: Boolean(settings.aff_required),
    inviteLimitUnlimited: settings.invite_limit_unlimited !== false,
    inviteLimitCount: Number.isFinite(rawCount)
      ? Math.max(0, Math.trunc(rawCount))
      : 0,
    invitationApplicationsEnabled:
      settings.invitation_applications_enabled !== false,
    invitationReapplyCooldownSeconds: normalizeInvitationReapplyCooldown(
      settings.invitation_reapply_cooldown_seconds,
    ),
    invitationPosterTitle: normalizePosterCopy(
      settings.invitation_poster_title,
      INVITATION_POSTER_TITLE_MAX_CHARACTERS,
    ),
    invitationPosterSubtitle: normalizePosterCopy(
      settings.invitation_poster_subtitle,
      INVITATION_POSTER_SUBTITLE_MAX_CHARACTERS,
    ),
  };
}

export function buildRegistrationSettingsPayload(settings) {
  const rawCount = Number(settings.inviteLimitCount);
  return {
    aff_required: Boolean(settings.affRequired),
    invite_limit_unlimited: Boolean(settings.inviteLimitUnlimited),
    invite_limit_count: Number.isFinite(rawCount)
      ? Math.max(0, Math.trunc(rawCount))
      : 0,
    invitation_applications_enabled: Boolean(
      settings.invitationApplicationsEnabled,
    ),
    invitation_reapply_cooldown_seconds: normalizeInvitationReapplyCooldown(
      settings.invitationReapplyCooldownSeconds,
    ),
    invitation_poster_title: normalizePosterCopy(
      settings.invitationPosterTitle,
      INVITATION_POSTER_TITLE_MAX_CHARACTERS,
    ),
    invitation_poster_subtitle: normalizePosterCopy(
      settings.invitationPosterSubtitle,
      INVITATION_POSTER_SUBTITLE_MAX_CHARACTERS,
    ),
  };
}
