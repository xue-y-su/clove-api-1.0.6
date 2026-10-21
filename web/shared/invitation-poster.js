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
  POSTER_SIZES,
  createDefaultInvitationPosterDesign,
} from "./invitation-poster-design.js";
import {
  getInvitationPosterCoverSourceRect,
  resolveInvitationPosterScene,
} from "./invitation-poster-scene.js";

export { POSTER_SIZES };

const DEFAULT_SIZE = "3:4";
const FONT_STACK = '"Public Sans", "PingFang SC", "Microsoft YaHei", sans-serif';
const COLORS = Object.freeze({
  clean: "#F2F5F4",
  ink: "#17201D",
  muted: "#5C6964",
  border: "#CBD6D2",
  white: "#FFFFFF",
  green: "#0B7965",
  coral: "#D96B4D",
  coralPaper: "#F3DDD5",
  inkBackground: "#18221F",
});

export function getInvitationPosterLayout(size) {
  const posterSize = Object.hasOwn(POSTER_SIZES, size) ? size : DEFAULT_SIZE;
  const dimensions = POSTER_SIZES[posterSize];
  const design = createDefaultInvitationPosterDesign();
  const modules = design.layouts[posterSize];
  const qrPlate = {
    x: modules.qr.x,
    y: modules.qr.y,
    size: Math.min(modules.qr.width, modules.qr.height),
  };
  const inset = Math.max(24, Math.ceil(qrPlate.size * 0.07));
  return {
    dimensions,
    frame: {
      x: 72,
      y: 184,
      width: dimensions.width - 144,
      height: dimensions.height - 344,
    },
    qrPlate,
    qr: {
      x: qrPlate.x + inset,
      y: qrPlate.y + inset,
      size: qrPlate.size - inset * 2,
    },
  };
}

export function resolveInvitationPosterCopy(input) {
  const siteName = String(input.siteName || "").trim();
  const title =
    String(input.title || "").trim() || String(input.defaultTitle || "").trim();
  const subtitle =
    String(input.subtitle || "").trim() ||
    String(input.defaultSubtitle || "").trim();
  return {
    title: replaceTemplate(title, "{站点名称}", siteName),
    subtitle: replaceTemplate(subtitle, "{站点名称}", siteName),
  };
}

export function resolveInvitationPosterLogoUrl(value, locationOrigin = "") {
  const raw = String(value || "").trim();
  if (!raw) return "";

  let parsed;
  let currentOrigin;
  try {
    currentOrigin = new URL(
      locationOrigin || "https://invitation-poster.invalid",
    );
    parsed = new URL(raw, currentOrigin);
  } catch {
    return "";
  }
  if (parsed.protocol === "data:" || parsed.protocol === "blob:") return raw;
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return "";
  return parsed.origin === currentOrigin.origin
    ? raw
    : "/api/invitation/poster-logo";
}

export async function renderInvitationPoster(input, dependencies = {}) {
  if (!input?.qrCanvas) throw new Error("Invitation QR canvas is required");
  const normalizedInput = normalizeRenderInput(input);
  const scene = resolveInvitationPosterScene(
    normalizedInput.design,
    normalizedInput.runtime,
  );
  await (dependencies.readyFonts || waitForBrowserFonts)();
  throwIfAborted(normalizedInput.signal);

  const canvas = (dependencies.createCanvas || defaultCreateCanvas)();
  canvas.width = scene.dimensions.width;
  canvas.height = scene.dimensions.height;
  const context = canvas.getContext("2d");
  if (!context) throw new Error("Canvas 2D is unavailable");

  let logo = null;
  const logoUrl = String(normalizedInput.runtime.logoUrl || "").trim();
  if (logoUrl) {
    try {
      logo = await (
        dependencies.loadLogoImage ||
        dependencies.loadImage ||
        loadBrowserImage
      )(logoUrl, undefined, normalizedInput.signal);
      throwIfAborted(normalizedInput.signal);
    } catch (error) {
      if (normalizedInput.signal?.aborted || error?.name === "AbortError") {
        throw error;
      }
      logo = null;
    }
  }
  const background = await loadPosterBackground(
    scene,
    normalizedInput.backgroundAsset,
    normalizedInput.signal,
    dependencies.loadBackgroundImage || loadBrowserImage,
  );
  throwIfAborted(normalizedInput.signal);

  drawInvitationPosterBackground(context, scene, background);
  const colors = resolveForegroundColors(scene.background);
  for (const command of scene.modules) {
    drawInvitationPosterModule(context, command, {
      colors,
      logo,
      qrCanvas: normalizedInput.qrCanvas,
      runtime: normalizedInput.runtime,
    });
  }
  return canvas;
}

function normalizeRenderInput(input) {
  if (input.design && input.runtime) return input;

  const design = createDefaultInvitationPosterDesign();
  design.size = Object.hasOwn(POSTER_SIZES, input.size)
    ? input.size
    : DEFAULT_SIZE;
  const displayName = String(input.displayName || "").trim();
  design.showNickname = Boolean(input.showNickname && displayName);
  const inviterTemplate = String(input.inviterTemplate || "{昵称}");
  return {
    design,
    qrCanvas: input.qrCanvas,
    signal: input.signal,
    backgroundAsset: null,
    runtime: {
      siteName: String(input.siteName || ""),
      logoUrl: String(input.logoUrl || ""),
      title: String(input.title || ""),
      subtitle: String(input.subtitle || ""),
      defaultTitle: String(input.defaultTitle || ""),
      defaultSubtitle: String(input.defaultSubtitle || ""),
      displayName,
      inviterText: replaceTemplate(inviterTemplate, "{昵称}", displayName),
      invitationCode: String(input.invitationCode || ""),
      siteDomain: String(input.siteDomain || ""),
      labels: {
        eyebrow: String(input.labels?.eyebrow || "INVITATION"),
        code: String(input.labels?.code || "CODE"),
        footer: String(input.labels?.footer || ""),
      },
    },
  };
}

function drawInvitationPosterBackground(context, scene, background) {
  const recipe = scene.background;
  if (recipe.kind === "upload" && background) {
    const width = Number(background.naturalWidth || background.width);
    const height = Number(background.naturalHeight || background.height);
    if (width > 0 && height > 0) {
      const source = getInvitationPosterCoverSourceRect(
        { width, height },
        scene.dimensions,
        recipe.cropBySize[scene.size],
      );
      context.drawImage(
        background,
        source.sx,
        source.sy,
        source.sw,
        source.sh,
        0,
        0,
        scene.dimensions.width,
        scene.dimensions.height,
      );
      context.fillStyle = "rgba(255, 255, 255, 0.58)";
      context.fillRect(0, 0, scene.dimensions.width, scene.dimensions.height);
      return;
    }
  }
  if (recipe.kind === "solid") {
    context.fillStyle = recipe.color;
    context.fillRect(0, 0, scene.dimensions.width, scene.dimensions.height);
    return;
  }
  if (recipe.kind === "builtin" && recipe.id === "ink-contrast") {
    drawInkContrastBackground(context, scene.dimensions);
    return;
  }
  if (recipe.kind === "builtin" && recipe.id === "coral-paper") {
    drawCoralPaperBackground(context, scene.dimensions);
    return;
  }
  drawCleanLightBackground(context, scene.dimensions);
}

function drawCleanLightBackground(context, dimensions) {
  context.fillStyle = COLORS.clean;
  context.fillRect(0, 0, dimensions.width, dimensions.height);
  context.fillStyle = COLORS.green;
  context.fillRect(0, 0, dimensions.width, 16);
  context.fillStyle = COLORS.coral;
  context.fillRect(dimensions.width - 220, 0, 220, 16);
}

function drawInkContrastBackground(context, dimensions) {
  context.fillStyle = COLORS.inkBackground;
  context.fillRect(0, 0, dimensions.width, dimensions.height);
  context.fillStyle = COLORS.coral;
  context.fillRect(0, 0, 18, dimensions.height);
  context.fillStyle = COLORS.green;
  context.fillRect(18, 0, 18, dimensions.height);
}

function drawCoralPaperBackground(context, dimensions) {
  context.fillStyle = COLORS.coralPaper;
  context.fillRect(0, 0, dimensions.width, dimensions.height);
  context.fillStyle = COLORS.ink;
  context.fillRect(0, dimensions.height - 18, dimensions.width, 18);
  context.fillStyle = COLORS.green;
  context.fillRect(72, 0, 220, 12);
}

function drawInvitationPosterModule(context, command, state) {
  switch (command.id) {
    case "brand":
      drawBrandModule(context, command, state);
      break;
    case "eyebrow":
      drawSingleLineModule(context, command, {
        color: state.colors.accent,
        font: `700 24px ${FONT_STACK}`,
        baselineOffset: 30,
      });
      break;
    case "inviter":
      drawSingleLineModule(context, command, {
        color: state.colors.muted,
        font: `500 28px ${FONT_STACK}`,
        baselineOffset: 32,
      });
      break;
    case "title":
      drawWrappedModule(context, command, {
        color: state.colors.ink,
        font: `700 52px ${FONT_STACK}`,
        lineHeight: 60,
      });
      break;
    case "subtitle":
      drawWrappedModule(context, command, {
        color: state.colors.muted,
        font: `400 30px ${FONT_STACK}`,
        lineHeight: 38,
      });
      break;
    case "footer":
      drawSingleLineModule(context, command, {
        color: state.colors.accent,
        font: `600 24px ${FONT_STACK}`,
        baselineOffset: 32,
      });
      break;
    case "inviteCode":
      drawInvitationCodeModule(context, command, state);
      break;
    case "domain":
      drawWrappedModule(context, command, {
        color: state.colors.muted,
        font: `500 22px ${FONT_STACK}`,
        lineHeight: 28,
      });
      break;
    case "qr":
      drawQrModule(context, command.rect, state.qrCanvas);
      break;
  }
}

function drawBrandModule(context, command, state) {
  const { rect, text } = command;
  const iconSize = Math.min(64, rect.height);
  const iconX = rect.x;
  const iconY = rect.y + Math.round((rect.height - iconSize) / 2);
  roundedRect(context, iconX, iconY, iconSize, iconSize, 8);
  context.fillStyle = COLORS.white;
  context.fill();
  context.strokeStyle = COLORS.border;
  context.lineWidth = 2;
  context.stroke();

  if (state.logo) {
    const width = Number(state.logo.naturalWidth || state.logo.width || iconSize);
    const height = Number(state.logo.naturalHeight || state.logo.height || iconSize);
    const scale = Math.min((iconSize - 12) / width, (iconSize - 12) / height);
    const drawWidth = Math.max(1, Math.round(width * scale));
    const drawHeight = Math.max(1, Math.round(height * scale));
    context.drawImage(
      state.logo,
      iconX + Math.round((iconSize - drawWidth) / 2),
      iconY + Math.round((iconSize - drawHeight) / 2),
      drawWidth,
      drawHeight,
    );
  } else {
    context.fillStyle = COLORS.green;
    context.font = `700 30px ${FONT_STACK}`;
    context.textAlign = "center";
    context.textBaseline = "middle";
    context.fillText(
      Array.from(String(text || "API"))[0]?.toUpperCase() || "A",
      iconX + Math.round(iconSize / 2),
      iconY + Math.round(iconSize / 2) + 1,
      iconSize - 12,
    );
  }

  context.fillStyle = state.colors.ink;
  context.font = `700 36px ${FONT_STACK}`;
  context.textAlign = "left";
  context.textBaseline = "alphabetic";
  drawWrappedText(context, text, {
    x: iconX + iconSize + 24,
    y: rect.y + 49,
    width: Math.max(1, rect.width - iconSize - 24),
    lineHeight: 40,
    maxLines: 1,
  });
}

function drawSingleLineModule(context, command, options) {
  context.fillStyle = options.color;
  context.font = options.font;
  context.textAlign = "left";
  context.textBaseline = "alphabetic";
  const text = fitSingleLine(context, String(command.text || ""), command.rect.width);
  context.fillText(
    text,
    command.rect.x,
    command.rect.y + Math.min(command.rect.height, options.baselineOffset),
    command.rect.width,
  );
}

function drawWrappedModule(context, command, options) {
  context.fillStyle = options.color;
  context.font = options.font;
  context.textAlign = "left";
  context.textBaseline = "alphabetic";
  const maxLines = Math.max(1, Math.floor(command.rect.height / options.lineHeight));
  drawWrappedText(context, command.text, {
    x: command.rect.x,
    y: command.rect.y + Math.min(options.lineHeight - 8, command.rect.height),
    width: command.rect.width,
    lineHeight: options.lineHeight,
    maxLines,
  });
}

function drawInvitationCodeModule(context, command, state) {
  const label = String(state.runtime.labels.code || "CODE");
  context.textAlign = "left";
  context.textBaseline = "alphabetic";
  context.fillStyle = state.colors.muted;
  context.font = `600 20px ${FONT_STACK}`;
  context.fillText(label, command.rect.x, command.rect.y + 22, command.rect.width);
  context.fillStyle = state.colors.ink;
  context.font = `700 32px ${FONT_STACK}`;
  context.fillText(
    fitSingleLine(context, command.text, command.rect.width),
    command.rect.x,
    command.rect.y + Math.min(command.rect.height - 4, 62),
    command.rect.width,
  );
}

function drawQrModule(context, rect, qrCanvas) {
  roundedRect(context, rect.x, rect.y, rect.width, rect.height, 8);
  context.fillStyle = COLORS.white;
  context.fill();
  context.strokeStyle = COLORS.border;
  context.lineWidth = 2;
  context.stroke();
  const inset = Math.max(24, Math.ceil(Math.min(rect.width, rect.height) * 0.07));
  const size = Math.max(1, Math.floor(Math.min(rect.width, rect.height) - inset * 2));
  const x = Math.round(rect.x + (rect.width - size) / 2);
  const y = Math.round(rect.y + (rect.height - size) / 2);
  context.imageSmoothingEnabled = false;
  context.drawImage(qrCanvas, x, y, size, size);
  context.imageSmoothingEnabled = true;
}

function resolveForegroundColors(background) {
  if (background.kind === "builtin" && background.id === "ink-contrast") {
    return { ink: COLORS.white, muted: "#D7E0DC", accent: "#79D6C4" };
  }
  if (background.kind === "solid" && isDarkColor(background.color)) {
    return { ink: COLORS.white, muted: "#E0E6E3", accent: "#8FE3D1" };
  }
  return { ink: COLORS.ink, muted: COLORS.muted, accent: COLORS.green };
}

function isDarkColor(color) {
  const red = Number.parseInt(color.slice(1, 3), 16);
  const green = Number.parseInt(color.slice(3, 5), 16);
  const blue = Number.parseInt(color.slice(5, 7), 16);
  return (red * 299 + green * 587 + blue * 114) / 1000 < 140;
}

async function loadPosterBackground(scene, asset, signal, loader) {
  if (
    scene.background.kind !== "upload" ||
    !asset ||
    asset.assetId !== scene.background.assetId ||
    !asset.objectUrl
  ) {
    return null;
  }
  return loadOptionalImage({ url: asset.objectUrl, signal, loader });
}

async function loadOptionalImage({ url, signal, loader }) {
  const source = String(url || "").trim();
  if (!source) return null;
  try {
    const image = await loader(source, undefined, signal);
    throwIfAborted(signal);
    return image;
  } catch (error) {
    if (signal?.aborted || error?.name === "AbortError") throw error;
    return null;
  }
}

function throwIfAborted(signal) {
  if (signal?.aborted) throw posterLogoAbortError();
}

function posterLogoAbortError() {
  const error = new Error("Logo loading aborted");
  error.name = "AbortError";
  return error;
}

function defaultCreateCanvas() {
  if (typeof document === "undefined") throw new Error("Canvas is unavailable");
  return document.createElement("canvas");
}

async function waitForBrowserFonts() {
  if (typeof document !== "undefined" && document.fonts?.ready) {
    await document.fonts.ready;
  }
}

function loadBrowserImage(url, timeoutMs = 6000, signal) {
  return new Promise((resolve, reject) => {
    if (typeof Image === "undefined") {
      reject(new Error("Image loading is unavailable"));
      return;
    }
    const image = new Image();
    let settled = false;
    let timer;
    const cleanup = () => {
      if (timer !== undefined) clearTimeout(timer);
      image.onload = null;
      image.onerror = null;
      signal?.removeEventListener("abort", abort);
    };
    const settle = (callback, clearSource = false) => {
      if (settled) return;
      settled = true;
      cleanup();
      if (clearSource) image.src = "";
      callback();
    };
    const abort = () => settle(() => reject(posterLogoAbortError()), true);
    if (signal?.aborted) {
      abort();
      return;
    }
    signal?.addEventListener("abort", abort, { once: true });
    timer = setTimeout(
      () => settle(() => reject(new Error("Image loading timed out")), true),
      timeoutMs,
    );
    image.crossOrigin = "anonymous";
    image.onload = () => settle(() => resolve(image));
    image.onerror = () =>
      settle(() => reject(new Error("Image loading failed")), true);
    image.src = url;
  });
}

function replaceTemplate(value, placeholder, replacement) {
  return String(value).split(placeholder).join(replacement);
}

function roundedRect(context, x, y, width, height, radius) {
  const safeRadius = Math.min(radius, width / 2, height / 2);
  context.beginPath();
  context.moveTo(x + safeRadius, y);
  context.lineTo(x + width - safeRadius, y);
  context.quadraticCurveTo(x + width, y, x + width, y + safeRadius);
  context.lineTo(x + width, y + height - safeRadius);
  context.quadraticCurveTo(x + width, y + height, x + width - safeRadius, y + height);
  context.lineTo(x + safeRadius, y + height);
  context.quadraticCurveTo(x, y + height, x, y + height - safeRadius);
  context.lineTo(x, y + safeRadius);
  context.quadraticCurveTo(x, y, x + safeRadius, y);
  context.closePath();
}

function fitSingleLine(context, text, maxWidth) {
  const normalized = String(text || "").replace(/\s+/g, " ").trim();
  if (context.measureText(normalized).width <= maxWidth) return normalized;
  return ellipsize(context, normalized, maxWidth);
}

function ellipsize(context, text, maxWidth) {
  const characters = Array.from(text);
  while (
    characters.length > 0 &&
    context.measureText(`${characters.join("")}...`).width > maxWidth
  ) {
    characters.pop();
  }
  return `${characters.join("").trimEnd()}...`;
}

function wrapText(context, text, maxWidth, maxLines) {
  const normalized = String(text || "").replace(/\s+/g, " ").trim();
  if (!normalized) return [];

  const characters = Array.from(normalized);
  const lines = [];
  let current = "";
  let index = 0;
  for (; index < characters.length; index += 1) {
    const candidate = current + characters[index];
    if (current && context.measureText(candidate).width > maxWidth) {
      lines.push(current.trimEnd());
      current = characters[index].trimStart();
      if (lines.length === maxLines) break;
    } else {
      current = candidate;
    }
  }
  if (lines.length < maxLines && current) lines.push(current.trimEnd());
  if (index < characters.length && lines.length > 0) {
    lines[lines.length - 1] = ellipsize(
      context,
      lines[lines.length - 1],
      maxWidth,
    );
  }
  return lines.slice(0, maxLines);
}

function drawWrappedText(context, text, options) {
  const lines = wrapText(context, text, options.width, options.maxLines);
  lines.forEach((line, index) => {
    context.fillText(
      line,
      options.x,
      options.y + index * options.lineHeight,
      options.width,
    );
  });
  return lines;
}

export function canvasToPngBlob(canvas) {
  return new Promise((resolve, reject) => {
    if (!canvas || typeof canvas.toBlob !== "function") {
      reject(new Error("PNG export is unavailable"));
      return;
    }
    try {
      canvas.toBlob((blob) => {
        if (!blob) {
          reject(new Error("PNG export returned an empty file"));
          return;
        }
        resolve(blob);
      }, "image/png");
    } catch (error) {
      reject(error instanceof Error ? error : new Error("PNG export failed"));
    }
  });
}

function sanitizeFilenamePart(value, fallback) {
  const sanitized = String(value || "")
    .trim()
    .replace(/[<>:"/\\|?*\u0000-\u001f]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^[.\s-]+|[.\s-]+$/g, "");
  return sanitized || fallback;
}

export function createInvitationPosterFilename(siteName, invitationCode) {
  const site = sanitizeFilenamePart(siteName, "site");
  const code = sanitizeFilenamePart(invitationCode, "invite");
  return `${site}-invitation-${code}.png`;
}

export function downloadPngBlob(blob, filename) {
  if (typeof document === "undefined" || typeof URL === "undefined") {
    throw new Error("PNG download is unavailable");
  }
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
