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
import { describe, expect, test } from "bun:test";

import {
  POSTER_DESIGN_STORAGE_KEY,
  createDefaultInvitationPosterDesign,
} from "./invitation-poster-design.js";
import {
  POSTER_ORPHAN_MAX_AGE_MS,
  cleanupInvitationPosterBackgrounds,
  loadInvitationPosterState,
  saveInvitationPosterState,
} from "./invitation-poster-storage.js";

function uploadDesign(assetId) {
  const design = createDefaultInvitationPosterDesign();
  const crop = { focalX: 0.5, focalY: 0.5, zoom: 1 };
  design.background = {
    kind: "upload",
    assetId,
    cropBySize: {
      "4:5": { ...crop },
      "3:4": { ...crop },
      "9:16": { ...crop },
    },
  };
  return design;
}

function backgroundRecord(assetId, createdAt) {
  return {
    assetId,
    blob: new Blob(["png"], { type: "image/png" }),
    mimeType: "image/png",
    width: 512,
    height: 512,
    createdAt,
    objectUrl: `blob:${assetId}`,
  };
}

function memoryStorage(calls = [], initial = null) {
  let value = initial;
  return {
    getItem: () => value,
    setItem: (_key, next) => {
      calls.push("localStorage.set");
      value = next;
    },
  };
}

function memoryStorageWith(design) {
  return memoryStorage([], JSON.stringify(design));
}

function memoryBackgroundStore(calls, records = []) {
  const values = new Map(records.map((record) => [record.assetId, record]));
  return {
    persistent: true,
    get: async (id) => values.get(id),
    getAll: async () => [...values.values()],
    put: async (record) => {
      calls.push(`idb.put:${record.assetId}`);
      values.set(record.assetId, record);
    },
    delete: async (id) => {
      calls.push(`idb.delete:${id}`);
      values.delete(id);
    },
  };
}

describe("invitation poster local persistence", () => {
  test("stores a new blob before publishing its v2 reference", async () => {
    const calls = [];
    const store = memoryBackgroundStore(calls);
    const result = await saveInvitationPosterState({
      activeDesign: createDefaultInvitationPosterDesign(),
      activeAsset: null,
      draftDesign: uploadDesign("asset-new"),
      pendingUpload: backgroundRecord("asset-new", 100),
      storage: memoryStorage(calls),
      backgroundStore: store,
    });

    expect(calls.slice(0, 2)).toEqual([
      "idb.put:asset-new",
      "localStorage.set",
    ]);
    expect(result.persistence).toBe("saved");
    expect(await store.get("asset-new")).not.toHaveProperty("objectUrl");
  });

  test("never publishes a dangling reference when IndexedDB fails", async () => {
    const storage = memoryStorage([]);
    const before = storage.getItem(POSTER_DESIGN_STORAGE_KEY);
    const result = await saveInvitationPosterState({
      activeDesign: createDefaultInvitationPosterDesign(),
      activeAsset: null,
      draftDesign: uploadDesign("asset-new"),
      pendingUpload: backgroundRecord("asset-new", 100),
      storage,
      backgroundStore: {
        ...memoryBackgroundStore([]),
        persistent: false,
        put: async () => {
          throw new Error("quota");
        },
      },
    });

    expect(storage.getItem(POSTER_DESIGN_STORAGE_KEY)).toBe(before);
    expect(result.persistence).toBe("session-only-upload");
    expect(result.warning).toBe("indexeddb-unavailable");
  });

  test("keeps a session-only upload out of storage on later edits", async () => {
    const calls = [];
    const storage = memoryStorage(calls);
    const backgroundStore = {
      ...memoryBackgroundStore(calls),
      persistent: false,
      put: async () => {
        throw new Error("quota");
      },
    };
    const pendingUpload = backgroundRecord("session-only", 100);
    const first = await saveInvitationPosterState({
      activeDesign: createDefaultInvitationPosterDesign(),
      activeAsset: null,
      draftDesign: uploadDesign("session-only"),
      pendingUpload,
      storage,
      backgroundStore,
    });

    const second = await saveInvitationPosterState({
      activeDesign: first.design,
      activeAsset: first.asset,
      draftDesign: { ...first.design, size: "9:16" },
      pendingUpload: null,
      storage,
      backgroundStore,
    });

    expect(calls).not.toContain("localStorage.set");
    expect(storage.getItem(POSTER_DESIGN_STORAGE_KEY)).toBeNull();
    expect(second.persistence).toBe("session-only-upload");
    expect(second.warning).toBe("indexeddb-unavailable");
  });

  test("repairs only a missing upload background and preserves copy", async () => {
    const saved = uploadDesign("missing");
    saved.copy.title = { source: "custom", value: "Still here" };
    const result = await loadInvitationPosterState({
      storage: memoryStorageWith(saved),
      backgroundStore: memoryBackgroundStore([]),
    });

    expect(result.design.background).toEqual({
      kind: "builtin",
      id: "clean-light",
    });
    expect(result.design.copy.title.value).toBe("Still here");
    expect(result.warning).toBe("missing-upload");
  });

  test("repairs an undecodable blob without erasing layout or copy", async () => {
    const saved = uploadDesign("corrupt");
    saved.copy.subtitle = { source: "custom", value: "Keep this copy" };
    const result = await loadInvitationPosterState({
      storage: memoryStorageWith(saved),
      backgroundStore: memoryBackgroundStore(
        [],
        [backgroundRecord("corrupt", 100)],
      ),
      validateBackground: async () => {
        throw new Error("decode failed");
      },
    });

    expect(result.design.background).toEqual({
      kind: "builtin",
      id: "clean-light",
    });
    expect(result.design.copy.subtitle.value).toBe("Keep this copy");
  });

  test("deletes only expired unreferenced backgrounds", async () => {
    const calls = [];
    const now = POSTER_ORPHAN_MAX_AGE_MS * 3;
    const store = memoryBackgroundStore(calls, [
      backgroundRecord("referenced", 1),
      backgroundRecord("fresh", now - POSTER_ORPHAN_MAX_AGE_MS + 1),
      backgroundRecord("expired", now - POSTER_ORPHAN_MAX_AGE_MS - 1),
    ]);

    await cleanupInvitationPosterBackgrounds(store, "referenced", now);

    expect(calls).toEqual(["idb.delete:expired"]);
  });
});
