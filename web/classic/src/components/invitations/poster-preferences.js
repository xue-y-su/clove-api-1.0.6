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
import {
  POSTER_DESIGN_STORAGE_KEY,
  createDefaultInvitationPosterDesign,
  normalizeInvitationPosterDesign,
} from '../../../../shared/invitation-poster-design.js';

export const POSTER_PREFERENCES_STORAGE_KEY = POSTER_DESIGN_STORAGE_KEY;
export const DEFAULT_POSTER_PREFERENCES = createDefaultInvitationPosterDesign();

function browserStorage() {
  try {
    return typeof window === 'undefined' ? null : window.localStorage;
  } catch {
    return null;
  }
}

export function readPosterPreferences(storage = browserStorage()) {
  try {
    const raw = storage?.getItem(POSTER_DESIGN_STORAGE_KEY) || 'null';
    return normalizeInvitationPosterDesign(JSON.parse(raw)).design;
  } catch {
    return createDefaultInvitationPosterDesign();
  }
}

export function countPosterCharacters(value) {
  return [...value].length;
}

export function limitPosterCharacters(value, limit) {
  return [...value].slice(0, limit).join('');
}
