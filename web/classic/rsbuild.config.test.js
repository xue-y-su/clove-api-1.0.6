/*
Copyright (C) 2025 QuantumNous

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

import { expect, test } from 'bun:test';
import path from 'path';
import { createRequire } from 'module';

import createConfig from './rsbuild.config';

const require = createRequire(import.meta.url);

test('serves client-side routes through the development index', async () => {
  const config = await createConfig({ envMode: 'development' });

  expect(config.server?.historyApiFallback).toBe(true);
});

test('keeps the classic VChart theme on the classic VChart runtime', async () => {
  const config = await createConfig({ envMode: 'production' });
  const classicVChartDir = path.dirname(
    require.resolve('@visactor/vchart/package.json'),
  );
  const classicVChartRuntimeDir = path.join(classicVChartDir, 'node_modules');

  expect(config.resolve?.alias?.['@visactor/vchart$']).toBe(classicVChartDir);
  expect(config.resolve?.alias?.['@visactor/vrender-core$']).toBe(
    path.join(classicVChartRuntimeDir, '@visactor/vrender-core'),
  );
  expect(config.resolve?.alias?.['@visactor/vrender-kits$']).toBe(
    path.join(classicVChartRuntimeDir, '@visactor/vrender-kits'),
  );
  expect(config.resolve?.alias?.['@visactor/vutils$']).toBe(
    path.join(classicVChartRuntimeDir, '@visactor/vutils'),
  );
});
