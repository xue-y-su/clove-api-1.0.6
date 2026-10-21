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
export function getInvitationLimitMetrics(
  user = {},
  unlimitedLabel = 'Unlimited',
) {
  const status = user.invite_limit || {};
  const unlimited = Boolean(status.unlimited || status.exempt);
  return {
    used: Number(status.used ?? user.aff_count ?? 0),
    limit: unlimited ? unlimitedLabel : String(status.limit ?? 0),
    remaining: unlimited ? unlimitedLabel : String(status.remaining ?? 0),
    exempt: Boolean(status.exempt),
  };
}
