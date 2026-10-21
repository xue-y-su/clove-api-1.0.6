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

import { createDefaultInvitationPosterDesign } from "./invitation-poster-design.js";

import {
  POSTER_SIZES,
  canvasToPngBlob,
  createInvitationPosterFilename,
  getInvitationPosterLayout,
  renderInvitationPoster,
  resolveInvitationPosterCopy,
  resolveInvitationPosterLogoUrl,
} from "./invitation-poster.js";

function createCanvasHarness(blob = new Blob(["png"], { type: "image/png" })) {
  const calls = [];
  const context = {
    fillStyle: "",
    strokeStyle: "",
    lineWidth: 1,
    font: "",
    textAlign: "left",
    textBaseline: "alphabetic",
    imageSmoothingEnabled: true,
    save() {},
    restore() {},
    beginPath() {},
    closePath() {},
    moveTo() {},
    lineTo() {},
    quadraticCurveTo() {},
    fill() {},
    stroke() {},
    fillRect(x, y, width, height) {
      calls.push({ type: "fillRect", x, y, width, height, fillStyle: this.fillStyle });
    },
    fillText(text, x, y, maxWidth) {
      calls.push({ type: "fillText", text, x, y, maxWidth, font: this.font });
    },
    drawImage(...args) {
      calls.push({ type: "drawImage", args });
    },
    measureText(text) {
      return { width: Array.from(text).length * 24 };
    },
  };
  const canvas = {
    width: 0,
    height: 0,
    getContext: () => context,
    toBlob: (callback) => callback(blob),
  };
  return { calls, canvas, createCanvas: () => canvas };
}

function fixture(overrides = {}) {
  return {
    size: "3:4",
    qrCanvas: { width: 360, height: 360 },
    siteName: "Clove API",
    logoUrl: "",
    title: "",
    subtitle: "",
    defaultTitle: "邀请你加入「{站点名称}」",
    defaultSubtitle: "扫码注册，邀请码将自动填写",
    displayName: "Alice",
    showNickname: true,
    inviterTemplate: "来自 {昵称} 的邀请",
    invitationCode: "CLOVE-2026",
    siteDomain: "api.example.test",
    labels: {
      eyebrow: "INVITATION",
      code: "邀请码",
      footer: "专属邀请",
    },
    ...overrides,
  };
}

function modularRuntime(overrides = {}) {
  return {
    siteName: "Clove API",
    logoUrl: "",
    title: "",
    subtitle: "",
    defaultTitle: "Join {站点名称}",
    defaultSubtitle: "Scan to register",
    displayName: "Alice",
    inviterText: "Invitation from Alice",
    invitationCode: "CLOVE-TEST",
    siteDomain: "127.0.0.1:18104",
    labels: {
      eyebrow: "Invitation",
      code: "Invitation code",
      footer: "Exclusive invitation",
    },
    ...overrides,
  };
}

describe("invitation poster logo URL resolution", () => {
  test("proxies only cross-origin HTTP(S) logos", () => {
    const locationOrigin = "https://clove.test";
    const cases = [
      ["https://img.test/logo.webp", "/api/invitation/poster-logo"],
      ["https://clove.test/logo.png", "https://clove.test/logo.png"],
      ["//img.test/logo.webp", "/api/invitation/poster-logo"],
      ["//clove.test/logo.png", "//clove.test/logo.png"],
      ["/\\\\img.test/logo.webp", "/api/invitation/poster-logo"],
      ["/logo.png", "/logo.png"],
      ["data:image/png;base64,AA==", "data:image/png;base64,AA=="],
      ["blob:https://clove.test/id", "blob:https://clove.test/id"],
      ["", ""],
      ["javascript:alert(1)", ""],
    ];

    for (const [value, expected] of cases) {
      expect(resolveInvitationPosterLogoUrl(value, locationOrigin)).toBe(expected);
    }
  });

});

describe("invitation poster renderer", () => {
  for (const [size, dimensions] of Object.entries(POSTER_SIZES)) {
    test(`renders ${size} at exact dimensions`, async () => {
      const harness = createCanvasHarness();
      const canvas = await renderInvitationPoster(fixture({ size }), {
        createCanvas: harness.createCanvas,
      });

      expect(canvas.width).toBe(dimensions.width);
      expect(canvas.height).toBe(dimensions.height);
    });
  }

  test("keeps the QR image inside a white quiet zone", async () => {
    const harness = createCanvasHarness();
    const qrCanvas = { width: 360, height: 360 };
    await renderInvitationPoster(fixture({ qrCanvas }), {
      createCanvas: harness.createCanvas,
    });

    const layout = getInvitationPosterLayout("3:4");
    const qrDraw = harness.calls.find(
      (call) => call.type === "drawImage" && call.args[0] === qrCanvas,
    );
    expect(qrDraw?.args.slice(1)).toEqual([
      layout.qr.x,
      layout.qr.y,
      layout.qr.size,
      layout.qr.size,
    ]);
    expect(layout.qr.x - layout.qrPlate.x).toBeGreaterThanOrEqual(24);
    expect(layout.qr.y - layout.qrPlate.y).toBeGreaterThanOrEqual(24);
    expect(
      layout.qrPlate.x + layout.qrPlate.size - layout.qr.x - layout.qr.size,
    ).toBeGreaterThanOrEqual(24);
  });

  test("draws a loaded brand logo separately from the QR image", async () => {
    const harness = createCanvasHarness();
    const qrCanvas = { width: 360, height: 360 };
    const logo = { naturalWidth: 200, naturalHeight: 100 };

    await renderInvitationPoster(
      fixture({ qrCanvas, logoUrl: "https://img.test/logo.webp" }),
      {
        createCanvas: harness.createCanvas,
        loadImage: async () => logo,
      },
    );

    expect(
      harness.calls.filter(
        (call) => call.type === "drawImage" && call.args[0] === logo,
      ),
    ).toHaveLength(1);
    expect(
      harness.calls.some(
        (call) => call.type === "drawImage" && call.args[0] === qrCanvas,
      ),
    ).toBe(true);
    expect(
      harness.calls.some((call) => call.type === "fillText" && call.text === "C"),
    ).toBe(false);
  });

  test("does not bypass a rejected proxy through the original cross-origin URL", async () => {
    const harness = createCanvasHarness();
    const loadedUrls = [];

    await renderInvitationPoster(
      fixture({
        logoUrl: "/api/invitation/poster-logo",
        logoFallbackUrl: "https://img.test/logo.svg",
      }),
      {
        createCanvas: harness.createCanvas,
        loadImage: async (url) => {
          loadedUrls.push(url);
          throw new Error("rejected by the bounded proxy");
        },
      },
    );

    expect(loadedUrls).toEqual(["/api/invitation/poster-logo"]);
    expect(
      harness.calls.some((call) => call.type === "fillText" && call.text === "C"),
    ).toBe(true);
  });

  test("waits beyond the backend deadline and clears a timed-out image request", async () => {
    const harness = createCanvasHarness();
    const previousImage = globalThis.Image;
    const previousSetTimeout = globalThis.setTimeout;
    const previousClearTimeout = globalThis.clearTimeout;
    let image;
    let scheduled;

    class PendingImage {
      constructor() {
        image = this;
        this.currentSrc = "";
      }

      set src(value) {
        this.currentSrc = value;
      }

      get src() {
        return this.currentSrc;
      }
    }

    globalThis.Image = PendingImage;
    globalThis.setTimeout = (callback, delay) => {
      scheduled = { callback, delay };
      return 1;
    };
    globalThis.clearTimeout = () => {};
    try {
      const rendering = renderInvitationPoster(
        fixture({ logoUrl: "/api/invitation/poster-logo" }),
        { createCanvas: harness.createCanvas },
      );
      for (let attempt = 0; attempt < 5 && !scheduled; attempt += 1) {
        await Promise.resolve();
      }
      expect(scheduled?.delay).toBeGreaterThan(5000);
      scheduled.callback();
      await rendering;
      expect(image.src).toBe("");
    } finally {
      globalThis.Image = previousImage;
      globalThis.setTimeout = previousSetTimeout;
      globalThis.clearTimeout = previousClearTimeout;
    }
  });

  test("aborts an in-flight browser image when poster rendering is cancelled", async () => {
    const harness = createCanvasHarness();
    const previousImage = globalThis.Image;
    const previousSetTimeout = globalThis.setTimeout;
    const previousClearTimeout = globalThis.clearTimeout;
    const controller = new AbortController();
    let image;
    let scheduled;
    let rendering;
    let outcome = "pending";

    class PendingImage {
      constructor() {
        image = this;
        this.currentSrc = "";
      }

      set src(value) {
        this.currentSrc = value;
      }

      get src() {
        return this.currentSrc;
      }
    }

    globalThis.Image = PendingImage;
    globalThis.setTimeout = (callback, delay) => {
      scheduled = { callback, delay };
      return 1;
    };
    globalThis.clearTimeout = () => {};
    try {
      rendering = renderInvitationPoster(
        fixture({
          logoUrl: "/api/invitation/poster-logo",
          signal: controller.signal,
        }),
        { createCanvas: harness.createCanvas },
      );
      rendering.then(
        () => {
          outcome = "resolved";
        },
        () => {
          outcome = "rejected";
        },
      );
      for (let attempt = 0; attempt < 5 && !scheduled; attempt += 1) {
        await Promise.resolve();
      }
      controller.abort();
      await Promise.resolve();
      await Promise.resolve();
      expect(outcome).toBe("rejected");
      expect(image.src).toBe("");
    } finally {
      if (outcome === "pending" && scheduled) scheduled.callback();
      await rendering?.catch(() => {});
      globalThis.Image = previousImage;
      globalThis.setTimeout = previousSetTimeout;
      globalThis.clearTimeout = previousClearTimeout;
    }
  });

  test("keeps wrapped title and subtitle above the compact QR row", async () => {
    const harness = createCanvasHarness();
    await renderInvitationPoster(
      fixture({
        title: "LONGTITLE ".repeat(30),
        subtitle: "LONGSUBTITLE ".repeat(40),
      }),
      { createCanvas: harness.createCanvas },
    );

    const layout = getInvitationPosterLayout("3:4");
    const longCopyCalls = harness.calls.filter(
      (call) =>
        call.type === "fillText" &&
        (call.text.includes("LONGTITLE") || call.text.includes("LONGSUBTITLE")),
    );
    expect(longCopyCalls.length).toBeGreaterThan(0);
    expect(longCopyCalls.every((call) => call.y < layout.qrPlate.y)).toBe(true);
  });

  test("omits the nickname line when nickname display is disabled", async () => {
    const harness = createCanvasHarness();
    await renderInvitationPoster(fixture({ showNickname: false }), {
      createCanvas: harness.createCanvas,
    });

    expect(
      harness.calls.some((call) => call.type === "fillText" && call.text.includes("Alice")),
    ).toBe(false);
  });

  test("falls back to text branding when logo loading fails", async () => {
    const harness = createCanvasHarness();
    const loadedUrls = [];
    await renderInvitationPoster(
      fixture({
        logoUrl: "/api/invitation/poster-logo",
        logoFallbackUrl: "https://cdn.test/logo.png",
      }),
      {
        createCanvas: harness.createCanvas,
        loadImage: async (url) => {
          loadedUrls.push(url);
          throw new Error("blocked");
        },
      },
    );

    expect(loadedUrls).toEqual(["/api/invitation/poster-logo"]);

    expect(
      harness.calls.some((call) => call.type === "fillText" && call.text === "Clove API"),
    ).toBe(true);
    expect(harness.calls.filter((call) => call.type === "drawImage")).toHaveLength(1);
  });

  test("waits for browser fonts before drawing", async () => {
    const harness = createCanvasHarness();
    const order = [];
    await renderInvitationPoster(fixture(), {
      createCanvas: () => {
        order.push("canvas");
        return harness.canvas;
      },
      readyFonts: async () => {
        order.push("fonts");
      },
    });

    expect(order).toEqual(["fonts", "canvas"]);
  });
});

describe("modular invitation poster renderer", () => {
  for (const [size, dimensions] of Object.entries(POSTER_SIZES)) {
    test(`renders the resolved ${size} scene at exact dimensions`, async () => {
      const design = createDefaultInvitationPosterDesign();
      design.size = size;
      const harness = createCanvasHarness();
      const qrCanvas = { width: 360, height: 360 };

      const canvas = await renderInvitationPoster(
        { design, runtime: modularRuntime(), qrCanvas },
        { createCanvas: harness.createCanvas, readyFonts: async () => {} },
      );

      expect([canvas.width, canvas.height]).toEqual([
        dimensions.width,
        dimensions.height,
      ]);
      expect(
        harness.calls.some(
          (call) => call.type === "drawImage" && call.args[0] === qrCanvas,
        ),
      ).toBe(true);
    });
  }

  test("loads uploaded bytes only through the background loader", async () => {
    const logoLoads = [];
    const backgroundLoads = [];
    const design = createDefaultInvitationPosterDesign();
    const crop = { focalX: 0.5, focalY: 0.5, zoom: 1 };
    design.background = {
      kind: "upload",
      assetId: "asset-1",
      cropBySize: {
        "4:5": { ...crop },
        "3:4": { ...crop },
        "9:16": { ...crop },
      },
    };
    const harness = createCanvasHarness();
    const backgroundAsset = {
      assetId: "asset-1",
      blob: new Blob(["png"], { type: "image/png" }),
      mimeType: "image/png",
      width: 512,
      height: 512,
      createdAt: 1,
      objectUrl: "blob:poster-background",
    };

    await renderInvitationPoster(
      {
        design,
        runtime: modularRuntime({ logoUrl: "/api/invitation/poster-logo" }),
        qrCanvas: { width: 360, height: 360 },
        backgroundAsset,
      },
      {
        createCanvas: harness.createCanvas,
        readyFonts: async () => {},
        loadLogoImage: async (url) => {
          logoLoads.push(url);
          return { width: 64, height: 64 };
        },
        loadBackgroundImage: async (url) => {
          backgroundLoads.push(url);
          return { width: 512, height: 512 };
        },
      },
    );

    expect(logoLoads).toEqual(["/api/invitation/poster-logo"]);
    expect(backgroundLoads).toEqual(["blob:poster-background"]);
    expect(
      harness.calls.some(
        (call) => call.type === "drawImage" && call.args.length === 9,
      ),
    ).toBe(true);
  });

  test("draws a normalized solid background before all modules", async () => {
    const design = createDefaultInvitationPosterDesign();
    design.background = { kind: "solid", color: "#1f6f62" };
    const harness = createCanvasHarness();

    await renderInvitationPoster(
      {
        design,
        runtime: modularRuntime(),
        qrCanvas: { width: 360, height: 360 },
      },
      { createCanvas: harness.createCanvas, readyFonts: async () => {} },
    );

    expect(harness.calls[0]).toMatchObject({
      type: "fillRect",
      x: 0,
      y: 0,
      fillStyle: "#1F6F62",
    });
  });
});

test("resolves custom copy and localized defaults with site placeholders", () => {
  expect(
    resolveInvitationPosterCopy({
      siteName: "Clove API",
      title: "加入 {站点名称}",
      subtitle: "",
      defaultTitle: "邀请你加入「{站点名称}」",
      defaultSubtitle: "扫码注册",
    }),
  ).toEqual({ title: "加入 Clove API", subtitle: "扫码注册" });
});

test("converts canvases to PNG blobs and rejects empty exports", async () => {
  const success = createCanvasHarness().canvas;
  await expect(canvasToPngBlob(success)).resolves.toMatchObject({ type: "image/png" });

  const empty = createCanvasHarness(null).canvas;
  await expect(canvasToPngBlob(empty)).rejects.toThrow("PNG");
});

test("creates a filesystem-safe poster filename", () => {
  expect(createInvitationPosterFilename("Clove / API:*?", "INV/001")).toBe(
    "Clove-API-invitation-INV-001.png",
  );
});
