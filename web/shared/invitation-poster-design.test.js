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
  POSTER_DESIGN_VERSION,
  POSTER_SIZES,
  clientPointToPosterPoint,
  constrainInvitationPosterTransform,
  createDefaultInvitationPosterDesign,
  getVisibleInvitationPosterModules,
  moveInvitationPosterModule,
  normalizeInvitationPosterDesign,
  resolveInvitationPosterDesignCopy,
} from "./invitation-poster-design.js";

describe("invitation poster design schema", () => {
  test("creates independent layouts for every supported size", () => {
    const first = createDefaultInvitationPosterDesign();
    const second = createDefaultInvitationPosterDesign();

    expect(first.version).toBe(POSTER_DESIGN_VERSION);
    expect(first.size).toBe("3:4");
    expect(Object.keys(first.layouts)).toEqual(Object.keys(POSTER_SIZES));
    first.layouts["4:5"].brand.x = 999;
    expect(second.layouts["4:5"].brand.x).toBe(72);
  });

  test("migrates valid v1 values without persisting runtime identity", () => {
    const result = normalizeInvitationPosterDesign({
      version: 1,
      size: "4:5",
      showNickname: false,
      siteName: "must-not-survive",
      invitationCode: "must-not-survive",
      registrationUrl: "must-not-survive",
    });

    expect(result.sourceVersion).toBe(1);
    expect(result.design).toMatchObject({
      version: 2,
      size: "4:5",
      showNickname: false,
      background: { kind: "builtin", id: "clean-light" },
    });
    expect(Object.keys(result.design.layouts)).toEqual(["4:5", "3:4", "9:16"]);
    expect(JSON.stringify(result.design)).not.toMatch(
      /siteName|invitationCode|registrationUrl/,
    );
  });

  test("repairs only the invalid layout and preserves copy", () => {
    const saved = createDefaultInvitationPosterDesign();
    saved.copy.title = { source: "custom", value: "Join {站点名称}" };
    saved.layouts["4:5"].brand.x = Number.NaN;
    saved.layouts["9:16"].footer.y += 8;

    const result = normalizeInvitationPosterDesign(saved);

    expect(result.repairedSizes).toEqual(["4:5"]);
    expect(result.design.copy.title).toEqual({
      source: "custom",
      value: "Join {站点名称}",
    });
    expect(result.design.layouts["9:16"].footer.y).toBe(
      saved.layouts["9:16"].footer.y,
    );
  });

  test("uses Unicode code-point limits and preserves intentional empty copy", () => {
    const design = createDefaultInvitationPosterDesign();
    design.copy.eyebrow = { source: "custom", value: "😀".repeat(33) };
    design.copy.footer = { source: "custom", value: "" };

    const { design: normalized } = normalizeInvitationPosterDesign(design);

    expect(Array.from(normalized.copy.eyebrow.value)).toHaveLength(32);
    expect(normalized.copy.footer).toEqual({ source: "custom", value: "" });
    expect(
      resolveInvitationPosterDesignCopy(normalized, {
        siteName: "Clove",
        title: "",
        subtitle: "",
        defaultTitle: "Join {站点名称}",
        defaultSubtitle: "Scan now",
        labels: {
          eyebrow: "Invitation",
          footer: "Exclusive invitation",
          code: "Code",
        },
      }),
    ).toMatchObject({ title: "Join Clove", footer: "" });
  });

  test("normalizes backgrounds and ignores future or malformed documents", () => {
    const solid = createDefaultInvitationPosterDesign();
    solid.background = { kind: "solid", color: "#a0bc9f" };
    expect(normalizeInvitationPosterDesign(solid).design.background).toEqual({
      kind: "solid",
      color: "#A0BC9F",
    });

    const malformed = createDefaultInvitationPosterDesign();
    malformed.background = {
      kind: "upload",
      assetId: "asset-1",
      cropBySize: { "4:5": { focalX: 2, focalY: 0.5, zoom: 1 } },
    };
    expect(normalizeInvitationPosterDesign(malformed).design.background).toEqual({
      kind: "builtin",
      id: "clean-light",
    });

    expect(normalizeInvitationPosterDesign({ version: 3 }).sourceVersion).toBe(0);
  });

  test("keeps mandatory modules visible and hides empty optional copy", () => {
    const design = createDefaultInvitationPosterDesign();
    design.showNickname = false;
    design.copy.footer = { source: "custom", value: "" };

    expect(
      getVisibleInvitationPosterModules(design, {
        eyebrow: "Invitation",
        title: "Join",
        subtitle: "Scan",
        footer: "",
      }),
    ).toEqual([
      "brand",
      "eyebrow",
      "title",
      "subtitle",
      "qr",
      "inviteCode",
      "domain",
    ]);
  });

  test("stops a drag before the expanded QR guard and safe-area boundary", () => {
    const design = createDefaultInvitationPosterDesign();
    const from = design.layouts["3:4"].inviteCode;
    const result = constrainInvitationPosterTransform({
      design,
      size: "3:4",
      moduleId: "inviteCode",
      from,
      proposed: { ...from, x: 300 },
      resolvedCopy: {
        eyebrow: "Invitation",
        title: "Join",
        subtitle: "Scan",
        footer: "Footer",
      },
    });

    expect(result.accepted).toBe(false);
    expect(result.rect.x).toBeGreaterThanOrEqual(584);
    expect(result.rect).toEqual(
      expect.objectContaining({ width: 340, height: 96 }),
    );
  });

  test("locks QR to a square and maps pointer pixels to export coordinates", () => {
    const design = createDefaultInvitationPosterDesign();
    const qr = design.layouts["4:5"].qr;
    const resized = constrainInvitationPosterTransform({
      design,
      size: "4:5",
      moduleId: "qr",
      from: qr,
      proposed: { ...qr, width: 500, height: 390 },
      resolvedCopy: {
        eyebrow: "Invitation",
        title: "Join",
        subtitle: "Scan",
        footer: "Footer",
      },
    });

    expect(resized.rect.width).toBe(resized.rect.height);
    expect(
      clientPointToPosterPoint(
        160,
        200,
        { left: 10, top: 20, width: 300, height: 400 },
        POSTER_SIZES["4:5"],
      ),
    ).toEqual({ x: 540, y: 608 });
  });

  test("moves one module immutably and preserves width-only dimensions", () => {
    const design = createDefaultInvitationPosterDesign();
    const moved = moveInvitationPosterModule(
      design,
      "3:4",
      "footer",
      10,
      0,
      { eyebrow: "Invitation", title: "Join", subtitle: "Scan", footer: "Footer" },
    );

    expect(moved).not.toBe(design);
    expect(moved.layouts["3:4"].footer.x).toBe(
      design.layouts["3:4"].footer.x + 10,
    );
    expect(moved.layouts["3:4"].footer.height).toBe(
      design.layouts["3:4"].footer.height,
    );
    expect(moved.layouts["4:5"]).toEqual(design.layouts["4:5"]);
  });

  test("repairs a saved layout when visible modules collide", () => {
    const design = createDefaultInvitationPosterDesign();
    design.layouts["3:4"].footer = { ...design.layouts["3:4"].domain };

    const normalized = normalizeInvitationPosterDesign(design);

    expect(normalized.repairedSizes).toEqual(["3:4"]);
    expect(normalized.design.layouts["3:4"].footer).not.toEqual(
      design.layouts["3:4"].footer,
    );
  });
});
