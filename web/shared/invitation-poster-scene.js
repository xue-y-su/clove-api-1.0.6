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
  POSTER_MODULE_REGISTRY,
  POSTER_SIZES,
  getVisibleInvitationPosterModules,
  normalizeInvitationPosterDesign,
  resolveInvitationPosterDesignCopy,
} from "./invitation-poster-design.js";

const DRAW_ORDER = Object.freeze([
  "eyebrow",
  "inviter",
  "title",
  "subtitle",
  "footer",
  "brand",
  "inviteCode",
  "domain",
  "qr",
]);

export function resolveInvitationPosterScene(designInput, runtime) {
  const design = normalizeInvitationPosterDesign(designInput).design;
  const copy = resolveInvitationPosterDesignCopy(design, runtime);
  const visible = new Set(getVisibleInvitationPosterModules(design, copy));
  const protectedText = {
    brand: runtime.siteName,
    inviter: runtime.inviterText,
    inviteCode: runtime.invitationCode,
    domain: runtime.siteDomain,
  };
  const modules = DRAW_ORDER.filter((id) => visible.has(id)).map((id) =>
    Object.freeze({
      id,
      rect: Object.freeze({ ...design.layouts[design.size][id] }),
      text: protectedText[id] ?? copy[id] ?? "",
      protected: POSTER_MODULE_REGISTRY[id].mandatory,
    }),
  );
  return Object.freeze({
    size: design.size,
    dimensions: POSTER_SIZES[design.size],
    background: deepFreeze(structuredClone(design.background)),
    modules: Object.freeze(modules),
  });
}

export function getInvitationPosterCoverSourceRect(image, canvas, crop) {
  const coverWidth = Math.min(
    image.width,
    image.height * (canvas.width / canvas.height),
  );
  const coverHeight = coverWidth * (canvas.height / canvas.width);
  const sw = coverWidth / crop.zoom;
  const sh = coverHeight / crop.zoom;
  return {
    sx: Math.round((image.width - sw) * crop.focalX),
    sy: Math.round((image.height - sh) * crop.focalY),
    sw: Math.round(sw),
    sh: Math.round(sh),
  };
}

function deepFreeze(value) {
  if (!value || typeof value !== "object" || Object.isFrozen(value)) return value;
  for (const child of Object.values(value)) deepFreeze(child);
  return Object.freeze(value);
}
