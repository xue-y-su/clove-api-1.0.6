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
import { describe, expect, test } from "bun:test";

import {
  createInvitationPosterObjectUrlLease,
  isInvitationPosterResolutionLow,
  validateInvitationPosterUpload,
} from "./invitation-poster-upload.js";

const encoder = new TextEncoder();

function concatBytes(...parts) {
  const size = parts.reduce((total, part) => total + part.length, 0);
  const result = new Uint8Array(size);
  let offset = 0;
  for (const part of parts) {
    result.set(part, offset);
    offset += part.length;
  }
  return result;
}

function pngChunk(name, payload = new Uint8Array()) {
  const length = payload.length;
  return concatBytes(
    new Uint8Array([
      (length >>> 24) & 255,
      (length >>> 16) & 255,
      (length >>> 8) & 255,
      length & 255,
    ]),
    encoder.encode(name),
    payload,
    new Uint8Array(4),
  );
}

function pngFixture(...chunks) {
  return new File(
    [
      new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]),
      ...chunks,
      pngChunk("IEND"),
    ],
    "fixture.png",
    { type: "image/png" },
  );
}

function webpFixture(chunkName = "VP8 ") {
  return new File(
    [encoder.encode(`RIFF0000WEBP${chunkName}`), new Uint8Array(8)],
    "fixture.webp",
    { type: "image/webp" },
  );
}

function decoder(width, height, onClose = () => {}) {
  return {
    decodeImage: async () => ({ width, height, close: onClose }),
  };
}

describe("invitation poster upload validation", () => {
  test.each([
    ["image/png", pngFixture()],
    [
      "image/jpeg",
      new File([new Uint8Array([255, 216, 255, 224, 0, 0])], "fixture.jpg", {
        type: "image/jpeg",
      }),
    ],
    ["image/webp", webpFixture()],
  ])("accepts decoded %s within byte and pixel bounds", async (type, file) => {
    let closeCount = 0;
    const result = await validateInvitationPosterUpload(
      file,
      decoder(1024, 1024, () => {
        closeCount += 1;
      }),
    );

    expect(result).toMatchObject({ mimeType: type, width: 1024, height: 1024 });
    expect(closeCount).toBe(1);
  });

  test("rejects APNG, animated WebP, MIME mismatches, and excessive area", async () => {
    await expect(
      validateInvitationPosterUpload(
        pngFixture(pngChunk("acTL")),
        decoder(512, 512),
      ),
    ).rejects.toThrow("Animated images are not supported");
    await expect(
      validateInvitationPosterUpload(webpFixture("ANIM"), decoder(512, 512)),
    ).rejects.toThrow("Animated images are not supported");
    await expect(
      validateInvitationPosterUpload(
        new File([new Uint8Array([255, 216, 255, 224])], "fixture.png", {
          type: "image/png",
        }),
        decoder(512, 512),
      ),
    ).rejects.toThrow("File type does not match its contents");
    await expect(
      validateInvitationPosterUpload(pngFixture(), decoder(8192, 8192)),
    ).rejects.toThrow("Image area exceeds 32 megapixels");
  });

  test("rejects unsupported, oversized, and undersized images", async () => {
    await expect(
      validateInvitationPosterUpload(
        new File(["gif"], "fixture.gif", { type: "image/gif" }),
      ),
    ).rejects.toThrow("PNG, JPEG, or WebP");
    await expect(
      validateInvitationPosterUpload(
        new File([new Uint8Array(10 * 1024 * 1024 + 1)], "large.png", {
          type: "image/png",
        }),
      ),
    ).rejects.toThrow("10 MiB");
    await expect(
      validateInvitationPosterUpload(pngFixture(), decoder(255, 512)),
    ).rejects.toThrow("at least 256 pixels");
  });

  test("revokes an object URL exactly once", () => {
    const revoked = [];
    const lease = createInvitationPosterObjectUrlLease(new Blob(["image"]), {
      createObjectURL: () => "blob:poster",
      revokeObjectURL: (url) => revoked.push(url),
    });

    expect(lease.url).toBe("blob:poster");
    lease.release();
    lease.release();
    expect(revoked).toEqual(["blob:poster"]);
  });

  test("warns only when decoded pixels are below the selected export size", () => {
    const asset = { width: 1080, height: 1440 };
    expect(isInvitationPosterResolutionLow(asset, "3:4")).toBe(false);
    expect(isInvitationPosterResolutionLow(asset, "9:16")).toBe(true);
  });
});
