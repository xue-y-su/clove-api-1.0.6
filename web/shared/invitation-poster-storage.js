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
} from "./invitation-poster-design.js";
import { validateInvitationPosterUpload } from "./invitation-poster-upload.js";

export const POSTER_DATABASE_NAME = "clove-invitation-poster";
export const POSTER_DATABASE_VERSION = 1;
export const POSTER_BACKGROUND_STORE = "backgrounds";
export const POSTER_ORPHAN_MAX_AGE_MS = 24 * 60 * 60 * 1000;

export async function openInvitationPosterBackgroundStore(factory = indexedDB) {
  const database = await openDatabase(
    factory,
    POSTER_DATABASE_NAME,
    POSTER_DATABASE_VERSION,
    (event) => {
      const database = event.target.result;
      if (!database.objectStoreNames.contains(POSTER_BACKGROUND_STORE)) {
        database.createObjectStore(POSTER_BACKGROUND_STORE, {
          keyPath: "assetId",
        });
      }
    },
  );
  return createBackgroundStore(database, POSTER_BACKGROUND_STORE);
}

export function createUnavailableInvitationPosterBackgroundStore() {
  return {
    persistent: false,
    get: async () => undefined,
    getAll: async () => [],
    put: async () => {
      throw new Error("IndexedDB is unavailable");
    },
    delete: async () => {},
  };
}

function openDatabase(factory, name, version, onUpgrade) {
  return new Promise((resolve, reject) => {
    const request = factory.open(name, version);
    request.onupgradeneeded = onUpgrade;
    request.onsuccess = () => resolve(request.result);
    request.onerror = () =>
      reject(request.error || new Error("IndexedDB open failed"));
    request.onblocked = () =>
      reject(new Error("IndexedDB upgrade was blocked"));
  });
}

function requestResult(request) {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () =>
      reject(request.error || new Error("IndexedDB request failed"));
  });
}

function createBackgroundStore(database, storeName) {
  const read = (method, argument) => {
    const transaction = database.transaction(storeName, "readonly");
    const store = transaction.objectStore(storeName);
    return requestResult(
      argument === undefined ? store[method]() : store[method](argument),
    );
  };
  const write = (method, argument) =>
    new Promise((resolve, reject) => {
      const transaction = database.transaction(storeName, "readwrite");
      transaction.objectStore(storeName)[method](argument);
      transaction.oncomplete = () => resolve();
      transaction.onerror = () =>
        reject(transaction.error || new Error("IndexedDB write failed"));
      transaction.onabort = () =>
        reject(transaction.error || new Error("IndexedDB write aborted"));
    });
  return {
    persistent: true,
    get: (assetId) => read("get", assetId),
    getAll: () => read("getAll"),
    put: (record) => write("put", record),
    delete: (assetId) => write("delete", assetId),
    close: () => database.close(),
  };
}

export async function saveInvitationPosterState(input) {
  const normalized = normalizeInvitationPosterDesign(input.draftDesign).design;
  const retainedAsset =
    normalized.background.kind === "upload" &&
    input.activeDesign.background.kind === "upload" &&
    normalized.background.assetId === input.activeDesign.background.assetId
      ? input.activeAsset
      : null;

  if (input.pendingUpload) {
    try {
      await input.backgroundStore.put(
        toStoredBackgroundRecord(input.pendingUpload),
      );
    } catch {
      return {
        design: normalized,
        asset: input.pendingUpload,
        persistence: "session-only-upload",
        warning: "indexeddb-unavailable",
      };
    }
  }

  if (!input.pendingUpload && normalized.background.kind === "upload") {
    try {
      const storedAsset = await input.backgroundStore.get(
        normalized.background.assetId,
      );
      if (!(storedAsset?.blob instanceof Blob)) {
        return {
          design: normalized,
          asset: retainedAsset,
          persistence: "session-only-upload",
          warning: "indexeddb-unavailable",
        };
      }
    } catch {
      return {
        design: normalized,
        asset: retainedAsset,
        persistence: "session-only-upload",
        warning: "indexeddb-unavailable",
      };
    }
  }

  try {
    input.storage.setItem(
      POSTER_DESIGN_STORAGE_KEY,
      JSON.stringify(normalized),
    );
  } catch {
    if (input.pendingUpload) {
      await input.backgroundStore
        .delete(input.pendingUpload.assetId)
        .catch(() => {});
    }
    return {
      design: normalized,
      asset: input.pendingUpload || retainedAsset,
      persistence: "memory-only",
      warning: "localstorage-unavailable",
    };
  }

  await deleteReplacedUpload(
    input.activeDesign,
    normalized,
    input.backgroundStore,
  );
  return {
    design: normalized,
    asset: input.pendingUpload || retainedAsset,
    persistence: "saved",
    warning: null,
  };
}

function toStoredBackgroundRecord(asset) {
  return {
    assetId: asset.assetId,
    blob: asset.blob,
    mimeType: asset.mimeType,
    width: asset.width,
    height: asset.height,
    createdAt: asset.createdAt,
  };
}

async function deleteReplacedUpload(previous, next, store) {
  if (previous.background.kind !== "upload") return;
  if (
    next.background.kind === "upload" &&
    next.background.assetId === previous.background.assetId
  ) {
    return;
  }
  await store.delete(previous.background.assetId).catch(() => {});
}

export async function loadInvitationPosterState(input) {
  let raw;
  try {
    raw = input.storage.getItem(POSTER_DESIGN_STORAGE_KEY);
  } catch {
    return {
      design: createDefaultInvitationPosterDesign(),
      asset: null,
      warning: "storage-unavailable",
    };
  }

  let parsed = null;
  try {
    parsed = JSON.parse(raw || "null");
  } catch {}
  const design = normalizeInvitationPosterDesign(parsed).design;
  if (design.background.kind !== "upload") {
    return { design, asset: null, warning: null };
  }

  try {
    const asset = await input.backgroundStore.get(design.background.assetId);
    if (asset?.blob instanceof Blob) {
      await (
        input.validateBackground || validateStoredInvitationPosterBackground
      )(asset);
      return { design, asset, warning: null };
    }
  } catch {}
  return {
    design: {
      ...design,
      background: { kind: "builtin", id: "clean-light" },
    },
    asset: null,
    warning: "missing-upload",
  };
}

async function validateStoredInvitationPosterBackground(record) {
  const extension =
    record.mimeType === "image/jpeg" ? "jpg" : record.mimeType.split("/")[1];
  const file = new File([record.blob], `poster-background.${extension}`, {
    type: record.mimeType,
  });
  const decoded = await validateInvitationPosterUpload(file);
  if (decoded.width !== record.width || decoded.height !== record.height) {
    throw new Error("Stored image dimensions changed");
  }
}

export async function cleanupInvitationPosterBackgrounds(
  store,
  referencedAssetId,
  now = Date.now(),
) {
  const records = await store.getAll();
  await Promise.all(
    records
      .filter(
        (record) =>
          record.assetId !== referencedAssetId &&
          now - record.createdAt > POSTER_ORPHAN_MAX_AGE_MS,
      )
      .map((record) => store.delete(record.assetId).catch(() => {})),
  );
}
