import type {
  InvitationPosterDesignV2,
  InvitationPosterSize,
} from "./invitation-poster-design.js";
import type { InvitationPosterRuntime } from "./invitation-poster-scene.js";
import type { InvitationPosterBackgroundResource } from "./invitation-poster-storage.js";

export interface InvitationPosterRenderInput {
  design: InvitationPosterDesignV2;
  runtime: InvitationPosterRuntime;
  qrCanvas: HTMLCanvasElement;
  backgroundAsset?: InvitationPosterBackgroundResource | null;
  signal?: AbortSignal;
}

export interface LegacyInvitationPosterInput {
  size: InvitationPosterSize;
  qrCanvas: HTMLCanvasElement;
  siteName: string;
  logoUrl?: string;
  signal?: AbortSignal;
  title?: string;
  subtitle?: string;
  defaultTitle: string;
  defaultSubtitle: string;
  displayName?: string;
  showNickname: boolean;
  inviterTemplate: string;
  invitationCode: string;
  siteDomain: string;
  labels: { eyebrow: string; code: string; footer: string };
}

export interface InvitationPosterLayout {
  dimensions: { width: number; height: number };
  frame: { x: number; y: number; width: number; height: number };
  qrPlate: { x: number; y: number; size: number };
  qr: { x: number; y: number; size: number };
}

export const POSTER_SIZES: Readonly<
  Record<InvitationPosterSize, Readonly<{ width: number; height: number }>>
>;
export function getInvitationPosterLayout(
  size: InvitationPosterSize,
): InvitationPosterLayout;
export function resolveInvitationPosterCopy(input: {
  siteName: string;
  title?: string;
  subtitle?: string;
  defaultTitle: string;
  defaultSubtitle: string;
}): { title: string; subtitle: string };
export function resolveInvitationPosterLogoUrl(
  value?: string,
  locationOrigin?: string,
): string;

export interface InvitationPosterRenderDependencies {
  createCanvas?: () => HTMLCanvasElement;
  loadLogoImage?: (
    url: string,
    timeoutMs?: number,
    signal?: AbortSignal,
  ) => Promise<CanvasImageSource>;
  loadBackgroundImage?: (
    objectUrl: string,
    timeoutMs?: number,
    signal?: AbortSignal,
  ) => Promise<CanvasImageSource>;
  loadImage?: (
    url: string,
    timeoutMs?: number,
    signal?: AbortSignal,
  ) => Promise<CanvasImageSource>;
  readyFonts?: () => Promise<void>;
}

export function renderInvitationPoster(
  input: InvitationPosterRenderInput | LegacyInvitationPosterInput,
  dependencies?: InvitationPosterRenderDependencies,
): Promise<HTMLCanvasElement>;
export function canvasToPngBlob(canvas: HTMLCanvasElement): Promise<Blob>;
export function createInvitationPosterFilename(
  siteName: string,
  invitationCode: string,
): string;
export function downloadPngBlob(blob: Blob, filename: string): void;
