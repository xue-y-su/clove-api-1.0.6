import type { InvitationPosterDesignV2 } from "./invitation-poster-design.js";

export interface InvitationPosterBackgroundRecord {
  assetId: string;
  blob: Blob;
  mimeType: "image/png" | "image/jpeg" | "image/webp";
  width: number;
  height: number;
  createdAt: number;
}

export interface InvitationPosterBackgroundResource
  extends InvitationPosterBackgroundRecord {
  objectUrl: string;
}

export interface InvitationPosterBackgroundStore {
  persistent: boolean;
  get(assetId: string): Promise<InvitationPosterBackgroundRecord | undefined>;
  getAll(): Promise<InvitationPosterBackgroundRecord[]>;
  put(record: InvitationPosterBackgroundRecord): Promise<void>;
  delete(assetId: string): Promise<void>;
  close?(): void;
}

export type InvitationPosterStorageWarning =
  | "storage-unavailable"
  | "missing-upload"
  | "indexeddb-unavailable"
  | "localstorage-unavailable";

export const POSTER_DATABASE_NAME: "clove-invitation-poster";
export const POSTER_DATABASE_VERSION: 1;
export const POSTER_BACKGROUND_STORE: "backgrounds";
export const POSTER_ORPHAN_MAX_AGE_MS: number;
export function openInvitationPosterBackgroundStore(
  factory?: IDBFactory,
): Promise<InvitationPosterBackgroundStore>;
export function createUnavailableInvitationPosterBackgroundStore(): InvitationPosterBackgroundStore;
export function loadInvitationPosterState(input: {
  storage: Pick<Storage, "getItem">;
  backgroundStore: InvitationPosterBackgroundStore;
  validateBackground?: (
    record: InvitationPosterBackgroundRecord,
  ) => Promise<void>;
}): Promise<{
  design: InvitationPosterDesignV2;
  asset: InvitationPosterBackgroundRecord | null;
  warning: InvitationPosterStorageWarning | null;
}>;
export function saveInvitationPosterState(input: {
  activeDesign: InvitationPosterDesignV2;
  activeAsset: InvitationPosterBackgroundResource | null;
  draftDesign: InvitationPosterDesignV2;
  pendingUpload: InvitationPosterBackgroundResource | null;
  storage: Pick<Storage, "setItem">;
  backgroundStore: InvitationPosterBackgroundStore;
}): Promise<{
  design: InvitationPosterDesignV2;
  asset: InvitationPosterBackgroundResource | null;
  persistence: "saved" | "memory-only" | "session-only-upload";
  warning: InvitationPosterStorageWarning | null;
}>;
export function cleanupInvitationPosterBackgrounds(
  store: InvitationPosterBackgroundStore,
  referencedAssetId?: string,
  now?: number,
): Promise<void>;
