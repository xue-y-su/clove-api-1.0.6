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
export type QueryErrorAction =
  | 'session-expired'
  | 'report-server-error'
  | 'local'

const ASSET_RECOVERY_STORAGE_KEY = 'newapi:asset-recovery-at'
const ASSET_RECOVERY_WINDOW_MS = 30_000

type RecoveryStorage = Pick<Storage, 'getItem' | 'setItem'>

const ASSET_ERROR_MESSAGES = [
  'chunkloaderror',
  'error loading dynamically imported module',
  'failed to fetch dynamically imported module',
  'failed to load module script',
  'importing a module script failed',
  'loading chunk',
] as const

export function getQueryErrorAction(
  status: number | undefined
): QueryErrorAction {
  if (status === 401) return 'session-expired'
  if (status === 500) return 'report-server-error'
  return 'local'
}

export function isRecoverableAssetError(error: unknown): boolean {
  if (!(error instanceof Error)) return false
  if (error.name.toLowerCase() === 'chunkloaderror') return true

  const message = error.message.toLowerCase()
  return ASSET_ERROR_MESSAGES.some((fragment) => message.includes(fragment))
}

export function shouldReloadForAssetError(
  error: unknown,
  storage: RecoveryStorage | null,
  now = Date.now()
): boolean {
  if (!storage || !isRecoverableAssetError(error)) return false

  try {
    const previousAttempt = Number(storage.getItem(ASSET_RECOVERY_STORAGE_KEY))
    if (
      Number.isFinite(previousAttempt) &&
      now - previousAttempt >= 0 &&
      now - previousAttempt < ASSET_RECOVERY_WINDOW_MS
    ) {
      return false
    }

    storage.setItem(ASSET_RECOVERY_STORAGE_KEY, String(now))
    return true
  } catch {
    return false
  }
}
