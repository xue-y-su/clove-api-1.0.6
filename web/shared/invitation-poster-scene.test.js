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
  POSTER_SIZES,
  createDefaultInvitationPosterDesign,
} from "./invitation-poster-design.js";
import {
  getInvitationPosterCoverSourceRect,
  resolveInvitationPosterScene,
} from "./invitation-poster-scene.js";

function runtime(overrides = {}) {
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

describe("invitation poster scene", () => {
  test("projects fresh protected runtime facts in fixed drawing order", () => {
    const design = createDefaultInvitationPosterDesign();
    design.copy.eyebrow = { source: "custom", value: "" };
    const scene = resolveInvitationPosterScene(
      design,
      runtime({
        siteName: "Current Clove",
        invitationCode: "LIVE-CODE",
        siteDomain: "api.example",
      }),
    );

    expect(scene.modules.map((module) => module.id)).toEqual([
      "inviter",
      "title",
      "subtitle",
      "footer",
      "brand",
      "inviteCode",
      "domain",
      "qr",
    ]);
    expect(scene.modules.find((module) => module.id === "brand").text).toContain(
      "Current Clove",
    );
    expect(scene.modules.find((module) => module.id === "inviteCode").text).toBe(
      "LIVE-CODE",
    );
    expect(scene.modules.some((module) => module.id === "eyebrow")).toBe(false);
    expect(scene.modules.find((module) => module.id === "qr").protected).toBe(true);
  });

  test("normalizes a malformed design before resolving protected facts", () => {
    const design = createDefaultInvitationPosterDesign();
    design.layouts["4:5"].brand.x = -1;
    design.size = "4:5";

    const scene = resolveInvitationPosterScene(design, runtime());

    expect(scene.dimensions).toEqual(POSTER_SIZES["4:5"]);
    expect(scene.modules.find((module) => module.id === "brand").rect.x).toBe(72);
    expect(Object.isFrozen(scene.modules)).toBe(true);
  });

  test("computes deterministic uploaded-image cover crops at focal boundaries", () => {
    expect(
      getInvitationPosterCoverSourceRect(
        { width: 2000, height: 1000 },
        POSTER_SIZES["3:4"],
        { focalX: 1, focalY: 0, zoom: 2 },
      ),
    ).toEqual({ sx: 1625, sy: 0, sw: 375, sh: 500 });
  });
});
