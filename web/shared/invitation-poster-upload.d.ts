import type { InvitationPosterSize } from "./invitation-poster-design.js";

export interface ValidatedInvitationPosterUpload {
  blob: Blob;
  mimeType: "image/png" | "image/jpeg" | "image/webp";
  width: number;
  height: number;
}

export interface InvitationPosterObjectUrlLease {
  url: string;
  release(): void;
}

export function validateInvitationPosterUpload(
  file: File,
  dependencies?: {
    decodeImage?: (
      blob: Blob,
    ) => Promise<{ width: number; height: number; close?(): void }>;
  },
): Promise<ValidatedInvitationPosterUpload>;
export function createInvitationPosterObjectUrlLease(
  blob: Blob,
  urlApi?: Pick<typeof URL, "createObjectURL" | "revokeObjectURL">,
): InvitationPosterObjectUrlLease;
export function isInvitationPosterResolutionLow(
  asset: Pick<ValidatedInvitationPosterUpload, "width" | "height">,
  size: InvitationPosterSize,
): boolean;
