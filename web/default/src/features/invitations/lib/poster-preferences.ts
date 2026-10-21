/*
Copyright (C) 2023-2026 QuantumNous

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
  type InvitationPosterDesignV2,
} from '../../../../../shared/invitation-poster-design.js'

export const POSTER_PREFERENCES_STORAGE_KEY = POSTER_DESIGN_STORAGE_KEY
export type PosterPreferences = InvitationPosterDesignV2
export const DEFAULT_POSTER_PREFERENCES = createDefaultInvitationPosterDesign()

type PosterPreferenceStorage = Pick<Storage, 'getItem'>

function browserStorage(): PosterPreferenceStorage | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  } catch {
    return null
  }
}

export function readPosterPreferences(
  storage: PosterPreferenceStorage | null = browserStorage()
): PosterPreferences {
  try {
    const raw = storage?.getItem(POSTER_DESIGN_STORAGE_KEY) || 'null'
    return normalizeInvitationPosterDesign(JSON.parse(raw)).design
  } catch {
    return createDefaultInvitationPosterDesign()
  }
}

export function countPosterCharacters(value: string): number {
  return [...value].length
}

export function limitPosterCharacters(value: string, limit: number): string {
  return [...value].slice(0, limit).join('')
}
