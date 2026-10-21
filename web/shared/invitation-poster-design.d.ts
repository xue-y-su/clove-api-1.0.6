export type InvitationPosterSize = "4:5" | "3:4" | "9:16";
export type PosterModuleId =
  | "brand"
  | "eyebrow"
  | "inviter"
  | "title"
  | "subtitle"
  | "qr"
  | "inviteCode"
  | "domain"
  | "footer";
export type PosterCopyField = "eyebrow" | "title" | "subtitle" | "footer";
export type CopyValue =
  | { source: "site" }
  | { source: "custom"; value: string };
export type ModuleRect = { x: number; y: number; width: number; height: number };
export type BackgroundCrop = { focalX: number; focalY: number; zoom: number };
export type PosterBackground =
  | { kind: "builtin"; id: "clean-light" | "ink-contrast" | "coral-paper" }
  | { kind: "solid"; color: `#${string}` }
  | {
      kind: "upload";
      assetId: string;
      cropBySize: Record<InvitationPosterSize, BackgroundCrop>;
    };

export interface InvitationPosterDesignV2 {
  version: 2;
  size: InvitationPosterSize;
  showNickname: boolean;
  background: PosterBackground;
  copy: Record<PosterCopyField, CopyValue>;
  layouts: Record<InvitationPosterSize, Record<PosterModuleId, ModuleRect>>;
}

export interface PosterCopyRuntime {
  siteName: string;
  title: string;
  subtitle: string;
  defaultTitle: string;
  defaultSubtitle: string;
  labels: { eyebrow: string; code: string; footer: string };
}

export interface NormalizeInvitationPosterDesignResult {
  design: InvitationPosterDesignV2;
  sourceVersion: 0 | 1 | 2;
  repairedSizes: InvitationPosterSize[];
}

export interface ConstrainInvitationPosterTransformInput {
  design: InvitationPosterDesignV2;
  size: InvitationPosterSize;
  moduleId: PosterModuleId;
  from: ModuleRect;
  proposed: ModuleRect;
  resolvedCopy: Record<PosterCopyField, string>;
}

export const POSTER_DESIGN_VERSION: 2;
export const POSTER_DESIGN_STORAGE_KEY: "invitation-poster-preferences";
export const POSTER_SIZES: Readonly<
  Record<InvitationPosterSize, Readonly<{ width: number; height: number }>>
>;
export const POSTER_COPY_LIMITS: Readonly<Record<PosterCopyField, number>>;
export const POSTER_MODULE_REGISTRY: Readonly<
  Record<
    PosterModuleId,
    Readonly<{
      mandatory: boolean;
      resize: "aspect" | "width" | "square";
      minWidth: number;
      minHeight: number;
    }>
  >
>;
export function createDefaultInvitationPosterDesign(): InvitationPosterDesignV2;
export function normalizeInvitationPosterDesign(
  value: unknown,
): NormalizeInvitationPosterDesignResult;
export function resolveInvitationPosterDesignCopy(
  design: InvitationPosterDesignV2,
  runtime: PosterCopyRuntime,
): Record<PosterCopyField, string>;
export function getVisibleInvitationPosterModules(
  design: InvitationPosterDesignV2,
  copy: Record<PosterCopyField, string>,
): PosterModuleId[];
export function constrainInvitationPosterTransform(
  input: ConstrainInvitationPosterTransformInput,
): { accepted: boolean; rect: ModuleRect };
export function updateInvitationPosterModuleRect(
  design: InvitationPosterDesignV2,
  size: InvitationPosterSize,
  moduleId: PosterModuleId,
  rect: ModuleRect,
): InvitationPosterDesignV2;
export function moveInvitationPosterModule(
  design: InvitationPosterDesignV2,
  size: InvitationPosterSize,
  moduleId: PosterModuleId,
  dx: number,
  dy: number,
  copy: Record<PosterCopyField, string>,
): InvitationPosterDesignV2;
export function clientPointToPosterPoint(
  clientX: number,
  clientY: number,
  bounds: Pick<DOMRect, "left" | "top" | "width" | "height">,
  dimensions: { width: number; height: number },
): { x: number; y: number };
