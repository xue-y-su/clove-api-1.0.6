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

const read = (path: string) => Bun.file(new URL(path, import.meta.url)).text();

describe("initial page title", () => {
  test("uses Clove API before the default frontend boots", async () => {
    const html = await read("./default/index.html");

    expect(html).toContain("<title>Clove API</title>");
    expect(html).toContain('<meta name="title" content="Clove API" />');
  });

  test("uses Clove API before the classic frontend boots", async () => {
    const html = await read("./classic/index.html");

    expect(html).toContain("<title>Clove API</title>");
  });

  test("keeps administrator-configured runtime title overrides", async () => {
    const [defaultMain, classicLayout] = await Promise.all([
      read("./default/src/main.tsx"),
      read("./classic/src/components/layout/PageLayout.jsx"),
    ]);

    expect(defaultMain).toContain("document.title = name");
    expect(classicLayout).toContain("document.title = systemName");
  });
});
