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

export const POSTER_DESIGN_VERSION = 2;
export const POSTER_DESIGN_STORAGE_KEY = "invitation-poster-preferences";

export const POSTER_SIZES = Object.freeze({
  "4:5": Object.freeze({ width: 1080, height: 1350 }),
  "3:4": Object.freeze({ width: 1080, height: 1440 }),
  "9:16": Object.freeze({ width: 1080, height: 1920 }),
});

export const POSTER_COPY_LIMITS = Object.freeze({
  eyebrow: 32,
  title: 80,
  subtitle: 160,
  footer: 64,
});

export const POSTER_MODULE_REGISTRY = Object.freeze({
  brand: Object.freeze({ mandatory: true, resize: "aspect", minWidth: 240, minHeight: 72 }),
  eyebrow: Object.freeze({ mandatory: false, resize: "width", minWidth: 160, minHeight: 40 }),
  inviter: Object.freeze({ mandatory: false, resize: "width", minWidth: 240, minHeight: 44 }),
  title: Object.freeze({ mandatory: false, resize: "width", minWidth: 320, minHeight: 120 }),
  subtitle: Object.freeze({ mandatory: false, resize: "width", minWidth: 320, minHeight: 80 }),
  qr: Object.freeze({ mandatory: true, resize: "square", minWidth: 360, minHeight: 360 }),
  inviteCode: Object.freeze({ mandatory: true, resize: "width", minWidth: 240, minHeight: 80 }),
  domain: Object.freeze({ mandatory: true, resize: "width", minWidth: 240, minHeight: 48 }),
  footer: Object.freeze({ mandatory: false, resize: "width", minWidth: 240, minHeight: 48 }),
});

const CANONICAL_LAYOUTS = Object.freeze({
  "4:5": Object.freeze({
    brand: { x: 72, y: 72, width: 640, height: 72 },
    eyebrow: { x: 124, y: 258, width: 300, height: 40 },
    inviter: { x: 124, y: 314, width: 832, height: 44 },
    title: { x: 124, y: 374, width: 832, height: 120 },
    subtitle: { x: 124, y: 510, width: 832, height: 80 },
    qr: { x: 124, y: 630, width: 420, height: 420 },
    inviteCode: { x: 604, y: 668, width: 340, height: 96 },
    domain: { x: 604, y: 780, width: 340, height: 64 },
    footer: { x: 124, y: 1090, width: 832, height: 48 },
  }),
  "3:4": Object.freeze({
    brand: { x: 72, y: 72, width: 640, height: 72 },
    eyebrow: { x: 124, y: 258, width: 300, height: 40 },
    inviter: { x: 124, y: 314, width: 832, height: 44 },
    title: { x: 124, y: 374, width: 832, height: 120 },
    subtitle: { x: 124, y: 510, width: 832, height: 80 },
    qr: { x: 124, y: 700, width: 420, height: 420 },
    inviteCode: { x: 604, y: 742, width: 340, height: 96 },
    domain: { x: 604, y: 854, width: 340, height: 64 },
    footer: { x: 124, y: 1172, width: 832, height: 48 },
  }),
  "9:16": Object.freeze({
    brand: { x: 72, y: 72, width: 640, height: 72 },
    eyebrow: { x: 124, y: 258, width: 300, height: 40 },
    inviter: { x: 124, y: 326, width: 832, height: 44 },
    title: { x: 124, y: 396, width: 832, height: 200 },
    subtitle: { x: 124, y: 620, width: 832, height: 120 },
    qr: { x: 330, y: 840, width: 420, height: 420 },
    inviteCode: { x: 124, y: 1320, width: 832, height: 80 },
    domain: { x: 124, y: 1416, width: 832, height: 48 },
    footer: { x: 124, y: 1580, width: 832, height: 48 },
  }),
});

export function createDefaultInvitationPosterDesign() {
  return structuredClone({
    version: POSTER_DESIGN_VERSION,
    size: "3:4",
    showNickname: true,
    background: { kind: "builtin", id: "clean-light" },
    copy: {
      eyebrow: { source: "site" },
      title: { source: "site" },
      subtitle: { source: "site" },
      footer: { source: "site" },
    },
    layouts: CANONICAL_LAYOUTS,
  });
}

export function normalizeInvitationPosterDesign(value) {
  const fallback = createDefaultInvitationPosterDesign();
  const sourceVersion = value?.version === 1 ? 1 : value?.version === 2 ? 2 : 0;
  if (sourceVersion === 0) {
    return { design: fallback, sourceVersion, repairedSizes: [] };
  }

  const candidate =
    sourceVersion === 1
      ? {
          ...fallback,
          size: Object.hasOwn(POSTER_SIZES, value.size) ? value.size : fallback.size,
          showNickname:
            typeof value.showNickname === "boolean" ? value.showNickname : true,
        }
      : value;
  const design = {
    ...fallback,
    size: Object.hasOwn(POSTER_SIZES, candidate.size) ? candidate.size : fallback.size,
    showNickname:
      typeof candidate.showNickname === "boolean" ? candidate.showNickname : true,
    background: normalizePosterBackground(candidate.background, fallback.background),
    copy: normalizePosterCopy(candidate.copy, fallback.copy),
  };
  const repairedSizes = [];
  design.layouts = Object.fromEntries(
    Object.keys(POSTER_SIZES).map((size) => {
      const layout = normalizePosterLayout(
        candidate.layouts?.[size],
        fallback.layouts[size],
      );
      if (!layout || !isPosterLayoutValid(design, size, layout)) {
        repairedSizes.push(size);
        return [size, structuredClone(fallback.layouts[size])];
      }
      return [size, layout];
    }),
  );
  return { design, sourceVersion, repairedSizes };
}

export function resolveInvitationPosterDesignCopy(design, runtime) {
  const sourceText = {
    eyebrow: runtime.labels.eyebrow,
    title: runtime.title || runtime.defaultTitle,
    subtitle: runtime.subtitle || runtime.defaultSubtitle,
    footer: runtime.labels.footer,
  };
  return Object.fromEntries(
    Object.keys(POSTER_COPY_LIMITS).map((key) => {
      const preference = design.copy[key];
      const text =
        preference.source === "custom" ? preference.value : sourceText[key];
      return [
        key,
        String(text || "").split("{站点名称}").join(runtime.siteName),
      ];
    }),
  );
}

function normalizePosterCopy(value, fallback) {
  return Object.fromEntries(
    Object.entries(POSTER_COPY_LIMITS).map(([key, limit]) => {
      const item = value?.[key];
      if (item?.source === "custom" && typeof item.value === "string") {
        return [
          key,
          {
            source: "custom",
            value: Array.from(item.value).slice(0, limit).join(""),
          },
        ];
      }
      return [
        key,
        item?.source === "site" ? { source: "site" } : { ...fallback[key] },
      ];
    }),
  );
}

function normalizePosterBackground(value, fallback) {
  if (
    value?.kind === "builtin" &&
    ["clean-light", "ink-contrast", "coral-paper"].includes(value.id)
  ) {
    return { kind: "builtin", id: value.id };
  }
  if (value?.kind === "solid" && /^#[0-9A-Fa-f]{6}$/.test(value.color)) {
    return { kind: "solid", color: value.color.toUpperCase() };
  }
  if (
    value?.kind === "upload" &&
    typeof value.assetId === "string" &&
    value.assetId.length > 0
  ) {
    const cropBySize = Object.fromEntries(
      Object.keys(POSTER_SIZES).map((size) => [
        size,
        normalizeCrop(value.cropBySize?.[size]),
      ]),
    );
    if (Object.values(cropBySize).every(Boolean)) {
      return { kind: "upload", assetId: value.assetId, cropBySize };
    }
  }
  return structuredClone(fallback);
}

function normalizeCrop(value) {
  return Number.isFinite(value?.focalX) &&
    value.focalX >= 0 &&
    value.focalX <= 1 &&
    Number.isFinite(value?.focalY) &&
    value.focalY >= 0 &&
    value.focalY <= 1 &&
    Number.isFinite(value?.zoom) &&
    value.zoom >= 1 &&
    value.zoom <= 4
    ? { focalX: value.focalX, focalY: value.focalY, zoom: value.zoom }
    : null;
}

function normalizePosterLayout(value, canonical) {
  if (value !== undefined && (value === null || typeof value !== "object")) {
    return null;
  }
  const layout = {};
  for (const moduleId of Object.keys(POSTER_MODULE_REGISTRY)) {
    const rect = value?.[moduleId];
    if (rect === undefined) {
      layout[moduleId] = { ...canonical[moduleId] };
      continue;
    }
    if (
      !["x", "y", "width", "height"].every(
        (key) => Number.isFinite(rect[key]) && Number.isInteger(rect[key]),
      )
    ) {
      return null;
    }
    layout[moduleId] = {
      x: rect.x,
      y: rect.y,
      width: rect.width,
      height: rect.height,
    };
  }
  return layout;
}

function isPosterLayoutWithinBounds(size, layout) {
  const dimensions = POSTER_SIZES[size];
  return Object.entries(layout).every(([moduleId, rect]) => {
    const rule = POSTER_MODULE_REGISTRY[moduleId];
    return (
      rect.width >= rule.minWidth &&
      rect.height >= rule.minHeight &&
      rect.x >= 72 &&
      rect.y >= 72 &&
      rect.x + rect.width <= dimensions.width - 72 &&
      rect.y + rect.height <= dimensions.height - 72
    );
  });
}

const SAFE_INSET = 72;
const COLLISION_GUTTER = 16;
const QR_GUARD = 24;

export function getVisibleInvitationPosterModules(design, copy) {
  return Object.keys(POSTER_MODULE_REGISTRY).filter((moduleId) => {
    if (POSTER_MODULE_REGISTRY[moduleId].mandatory) return true;
    if (moduleId === "inviter") return design.showNickname;
    return String(copy[moduleId] || "").length > 0;
  });
}

function rectanglesConflict(leftId, left, rightId, right) {
  const leftGuard = leftId === "qr" ? QR_GUARD : 0;
  const rightGuard = rightId === "qr" ? QR_GUARD : 0;
  return !(
    left.x + left.width + leftGuard + COLLISION_GUTTER <= right.x - rightGuard ||
    right.x + right.width + rightGuard + COLLISION_GUTTER <= left.x - leftGuard ||
    left.y + left.height + leftGuard + COLLISION_GUTTER <= right.y - rightGuard ||
    right.y + right.height + rightGuard + COLLISION_GUTTER <= left.y - leftGuard
  );
}

function normalizationCopy(design) {
  return Object.fromEntries(
    Object.keys(POSTER_COPY_LIMITS).map((key) => [
      key,
      design.copy[key].source === "custom" ? design.copy[key].value : key,
    ]),
  );
}

function isPosterLayoutValid(design, size, layout) {
  if (!isPosterLayoutWithinBounds(size, layout)) return false;
  const visible = getVisibleInvitationPosterModules(
    design,
    normalizationCopy(design),
  );
  return visible.every((leftId, leftIndex) =>
    visible
      .slice(leftIndex + 1)
      .every(
        (rightId) =>
          !rectanglesConflict(leftId, layout[leftId], rightId, layout[rightId]),
      ),
  );
}

function normalizeTransformForModule(moduleId, from, proposed) {
  const rule = POSTER_MODULE_REGISTRY[moduleId];
  if (rule.resize === "width") {
    return {
      ...proposed,
      width: Math.max(rule.minWidth, Math.round(proposed.width)),
      height: from.height,
      y: from.y,
    };
  }
  if (rule.resize === "square") {
    const size = Math.max(
      rule.minWidth,
      Math.round(Math.max(proposed.width, proposed.height)),
    );
    return { ...proposed, width: size, height: size };
  }
  const aspect = from.width / from.height;
  const width = Math.max(
    rule.minWidth,
    Math.round(rule.minHeight * aspect),
    Math.round(proposed.width),
  );
  return { ...proposed, width, height: Math.round(width / aspect) };
}

function clampRectToSafeArea(rect, dimensions, inset) {
  const maxX = dimensions.width - inset - rect.width;
  const maxY = dimensions.height - inset - rect.height;
  return {
    x: Math.round(maxX < inset ? inset : Math.min(Math.max(rect.x, inset), maxX)),
    y: Math.round(maxY < inset ? inset : Math.min(Math.max(rect.y, inset), maxY)),
    width: Math.round(rect.width),
    height: Math.round(rect.height),
  };
}

function interpolateRect(from, to, factor) {
  return Object.fromEntries(
    ["x", "y", "width", "height"].map((key) => [
      key,
      Math.round(from[key] + (to[key] - from[key]) * factor),
    ]),
  );
}

function isModuleRectValid({ design, size, moduleId, rect, resolvedCopy }) {
  const layout = { ...design.layouts[size], [moduleId]: rect };
  if (!isPosterLayoutWithinBounds(size, layout)) return false;
  return getVisibleInvitationPosterModules(design, resolvedCopy)
    .filter((otherId) => otherId !== moduleId)
    .every(
      (otherId) =>
        !rectanglesConflict(moduleId, rect, otherId, layout[otherId]),
    );
}

export function constrainInvitationPosterTransform(input) {
  const normalized = normalizeTransformForModule(
    input.moduleId,
    input.from,
    input.proposed,
  );
  const target = clampRectToSafeArea(
    normalized,
    POSTER_SIZES[input.size],
    SAFE_INSET,
  );
  if (isModuleRectValid({ ...input, rect: target })) {
    return { accepted: true, rect: target };
  }

  let lastValid = input.from;
  let validFactor = 0;
  let invalidFactor = 1;
  for (let index = 1; index <= 64; index += 1) {
    const factor = index / 64;
    const candidate = interpolateRect(input.from, target, factor);
    if (!isModuleRectValid({ ...input, rect: candidate })) {
      invalidFactor = factor;
      break;
    }
    lastValid = candidate;
    validFactor = factor;
  }
  for (let index = 0; index < 12; index += 1) {
    const factor = (validFactor + invalidFactor) / 2;
    const candidate = interpolateRect(input.from, target, factor);
    if (isModuleRectValid({ ...input, rect: candidate })) {
      validFactor = factor;
      lastValid = candidate;
    } else {
      invalidFactor = factor;
    }
  }
  return { accepted: false, rect: lastValid };
}

export function updateInvitationPosterModuleRect(design, size, moduleId, rect) {
  return {
    ...design,
    layouts: {
      ...design.layouts,
      [size]: {
        ...design.layouts[size],
        [moduleId]: { ...rect },
      },
    },
  };
}

export function moveInvitationPosterModule(
  design,
  size,
  moduleId,
  dx,
  dy,
  copy,
) {
  const from = design.layouts[size][moduleId];
  const constrained = constrainInvitationPosterTransform({
    design,
    size,
    moduleId,
    from,
    proposed: { ...from, x: from.x + dx, y: from.y + dy },
    resolvedCopy: copy,
  });
  return updateInvitationPosterModuleRect(
    design,
    size,
    moduleId,
    constrained.rect,
  );
}

export function clientPointToPosterPoint(
  clientX,
  clientY,
  bounds,
  dimensions,
) {
  return {
    x: Math.round(((clientX - bounds.left) / bounds.width) * dimensions.width),
    y: Math.round(((clientY - bounds.top) / bounds.height) * dimensions.height),
  };
}
