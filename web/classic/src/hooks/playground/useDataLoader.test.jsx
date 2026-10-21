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
import { afterEach, beforeAll, describe, expect, mock, test } from 'bun:test';
import { JSDOM } from 'jsdom';
import React from 'react';
import { act } from 'react';
import { createRoot } from 'react-dom/client';

const displayedErrors = [];

mock.module('react-i18next', () => ({
  useTranslation: () => ({ t: (message) => message }),
}));

mock.module('../../helpers', () => ({
  API: {
    get: async () => ({
      data: { success: false, message: 'upstream load failed' },
    }),
  },
  processModelsData: () => ({ modelOptions: [], selectedModel: '' }),
  processGroupsData: () => [],
  showError: (message) => displayedErrors.push(message),
}));

beforeAll(() => {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', {
    url: 'https://app.example/',
  });
  globalThis.window = dom.window;
  globalThis.document = dom.window.document;
  globalThis.navigator = dom.window.navigator;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.IS_REACT_ACT_ENVIRONMENT = true;
});

afterEach(() => {
  displayedErrors.length = 0;
  document.body.replaceChildren();
});

describe('useDataLoader', () => {
  test('reports model and group loading failures without throwing', async () => {
    const { useDataLoader } = await import('./useDataLoader');
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    function Harness() {
      useDataLoader(
        { user: { group: 'default' } },
        { model: 'gpt-4o', group: 'default' },
        () => {},
        () => {},
        () => {},
      );
      return null;
    }

    await act(async () => {
      root.render(React.createElement(Harness));
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(displayedErrors).toEqual([
      'upstream load failed',
      'upstream load failed',
    ]);

    await act(async () => root.unmount());
  });
});
