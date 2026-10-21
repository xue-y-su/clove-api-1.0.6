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
import { POSTER_SIZES } from "./invitation-poster-design.js";

const MAX_BYTES = 10 * 1024 * 1024;
const MIN_DIMENSION = 256;
const MAX_DIMENSION = 8192;
const MAX_AREA = 32_000_000;
const SUPPORTED_TYPES = new Set(["image/png", "image/jpeg", "image/webp"]);

export async function validateInvitationPosterUpload(file, dependencies = {}) {
  if (!(file instanceof Blob) || !SUPPORTED_TYPES.has(file.type)) {
    throw new Error("Use a PNG, JPEG, or WebP image");
  }
  if (file.size > MAX_BYTES) {
    throw new Error("Image must be 10 MiB or smaller");
  }

  const bytes = new Uint8Array(await file.arrayBuffer());
  const detectedType = detectRasterMimeType(bytes);
  if (detectedType !== file.type) {
    throw new Error("File type does not match its contents");
  }
  if (containsAnimation(bytes, detectedType)) {
    throw new Error("Animated images are not supported");
  }

  const decodeImage = dependencies.decodeImage || globalThis.createImageBitmap;
  if (typeof decodeImage !== "function") {
    throw new Error("Image decoding is unavailable");
  }
  const decoded = await decodeImage(file);
  try {
    if (decoded.width < MIN_DIMENSION || decoded.height < MIN_DIMENSION) {
      throw new Error("Image dimensions must be at least 256 pixels");
    }
    if (decoded.width > MAX_DIMENSION || decoded.height > MAX_DIMENSION) {
      throw new Error("Image dimensions must not exceed 8192 pixels");
    }
    if (decoded.width * decoded.height > MAX_AREA) {
      throw new Error("Image area exceeds 32 megapixels");
    }
    return {
      blob: file,
      mimeType: detectedType,
      width: decoded.width,
      height: decoded.height,
    };
  } finally {
    decoded.close?.();
  }
}

export function createInvitationPosterObjectUrlLease(blob, urlApi = URL) {
  const url = urlApi.createObjectURL(blob);
  let released = false;
  return {
    url,
    release() {
      if (released) return;
      released = true;
      urlApi.revokeObjectURL(url);
    },
  };
}

export function isInvitationPosterResolutionLow(asset, size) {
  const dimensions = POSTER_SIZES[size] || POSTER_SIZES["3:4"];
  return asset.width < dimensions.width || asset.height < dimensions.height;
}

function detectRasterMimeType(bytes) {
  if (
    bytes.length >= 8 &&
    [137, 80, 78, 71, 13, 10, 26, 10].every(
      (value, index) => bytes[index] === value,
    )
  ) {
    return "image/png";
  }
  if (
    bytes.length >= 3 &&
    bytes[0] === 0xff &&
    bytes[1] === 0xd8 &&
    bytes[2] === 0xff
  ) {
    return "image/jpeg";
  }
  if (
    bytes.length >= 12 &&
    readAscii(bytes, 0, 4) === "RIFF" &&
    readAscii(bytes, 8, 4) === "WEBP"
  ) {
    return "image/webp";
  }
  return "";
}

function containsAnimation(bytes, mimeType) {
  if (mimeType === "image/png") return pngContainsAnimation(bytes);
  if (mimeType === "image/webp") return webpContainsAnimation(bytes);
  return false;
}

function pngContainsAnimation(bytes) {
  let offset = 8;
  while (offset + 12 <= bytes.length) {
    const length = readUint32BigEndian(bytes, offset);
    const type = readAscii(bytes, offset + 4, 4);
    if (type === "acTL") return true;
    const next = offset + 12 + length;
    if (next <= offset || next > bytes.length) return false;
    offset = next;
    if (type === "IEND") return false;
  }
  return false;
}

function webpContainsAnimation(bytes) {
  let offset = 12;
  while (offset + 8 <= bytes.length) {
    const type = readAscii(bytes, offset, 4);
    if (type === "ANIM" || type === "ANMF") return true;
    const length = readUint32LittleEndian(bytes, offset + 4);
    const next = offset + 8 + length + (length % 2);
    if (next <= offset || next > bytes.length) return false;
    offset = next;
  }
  return false;
}

function readAscii(bytes, offset, length) {
  return String.fromCharCode(...bytes.subarray(offset, offset + length));
}

function readUint32BigEndian(bytes, offset) {
  return (
    bytes[offset] * 0x1000000 +
    (bytes[offset + 1] << 16) +
    (bytes[offset + 2] << 8) +
    bytes[offset + 3]
  );
}

function readUint32LittleEndian(bytes, offset) {
  return (
    bytes[offset] +
    (bytes[offset + 1] << 8) +
    (bytes[offset + 2] << 16) +
    bytes[offset + 3] * 0x1000000
  );
}
