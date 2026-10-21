import type {
  BackgroundCrop,
  InvitationPosterDesignV2,
  InvitationPosterSize,
  ModuleRect,
  PosterBackground,
  PosterCopyRuntime,
  PosterModuleId,
} from "./invitation-poster-design.js";

export interface InvitationPosterRuntime extends PosterCopyRuntime {
  logoUrl: string;
  displayName: string;
  inviterText: string;
  invitationCode: string;
  siteDomain: string;
}

export interface InvitationPosterSceneModule {
  id: PosterModuleId;
  rect: Readonly<ModuleRect>;
  text: string;
  protected: boolean;
}

export interface InvitationPosterScene {
  size: InvitationPosterSize;
  dimensions: Readonly<{ width: number; height: number }>;
  background: PosterBackground;
  modules: ReadonlyArray<Readonly<InvitationPosterSceneModule>>;
}

export function resolveInvitationPosterScene(
  design: InvitationPosterDesignV2,
  runtime: InvitationPosterRuntime,
): InvitationPosterScene;
export function getInvitationPosterCoverSourceRect(
  image: { width: number; height: number },
  canvas: { width: number; height: number },
  crop: BackgroundCrop,
): { sx: number; sy: number; sw: number; sh: number };
