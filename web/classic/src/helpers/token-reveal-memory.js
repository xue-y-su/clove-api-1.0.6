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

export function normalizeApiKeyCredential(key) {
  return String(key || '').trim();
}

export function createTokenRevealMemory(now = Date.now) {
  let verifiedUntilMs = 0;
  let keys = {};
  const clear = () => {
    verifiedUntilMs = 0;
    keys = {};
  };
  const valid = () => {
    if (verifiedUntilMs <= now()) {
      clear();
      return false;
    }
    return true;
  };
  return {
    clear,
    setVerifiedUntil(seconds) {
      const next = Number(seconds) * 1000;
      if (!Number.isFinite(next) || next <= now()) {
        clear();
        return;
      }
      verifiedUntilMs = next;
    },
    getVerifiedUntilMs: () => (valid() ? verifiedUntilMs : 0),
    setKey(id, key) {
      if (valid() && Number(id) > 0 && key) {
        keys = { ...keys, [id]: normalizeApiKeyCredential(key) };
      }
    },
    setKeys(values) {
      if (!valid()) return;
      const next = { ...keys };
      Object.entries(values || {}).forEach(([id, key]) => {
        if (Number(id) > 0 && key) next[id] = normalizeApiKeyCredential(key);
      });
      keys = next;
    },
    getKey: (id) => (valid() ? keys[id] : undefined),
    snapshot: () => (valid() ? { ...keys } : {}),
  };
}
