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
export function normalizeApiKeyCredential(key: string): string {
  return key.trim()
}

export type TokenRevealMemory = ReturnType<typeof createTokenRevealMemory>

export function createTokenRevealMemory(now: () => number = Date.now) {
  let verifiedUntilMs = 0
  let keys: Record<number, string> = {}

  const clear = () => {
    verifiedUntilMs = 0
    keys = {}
  }

  const valid = () => {
    if (verifiedUntilMs <= now()) {
      clear()
      return false
    }
    return true
  }

  return {
    clear,
    setVerifiedUntil(verifiedUntilSeconds: number) {
      const next = Number(verifiedUntilSeconds) * 1000
      if (!Number.isFinite(next) || next <= now()) {
        clear()
        return
      }
      verifiedUntilMs = next
    },
    getVerifiedUntilMs() {
      return valid() ? verifiedUntilMs : 0
    },
    setKey(id: number, key: string) {
      if (!valid() || !Number.isInteger(id) || id <= 0 || !key.trim()) return
      keys = { ...keys, [id]: normalizeApiKeyCredential(key) }
    },
    setKeys(values: Record<number, string>) {
      if (!valid()) return
      const next = { ...keys }
      for (const [rawId, key] of Object.entries(values)) {
        const id = Number(rawId)
        if (Number.isInteger(id) && id > 0 && key.trim()) {
          next[id] = normalizeApiKeyCredential(key)
        }
      }
      keys = next
    },
    getKey(id: number) {
      return valid() ? keys[id] : undefined
    },
    snapshot() {
      return valid() ? { ...keys } : {}
    },
  }
}
