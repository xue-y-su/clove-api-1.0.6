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
import { afterEach, beforeAll, describe, expect, test } from 'bun:test';
import { JSDOM } from 'jsdom';
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import TelegramLoginButton from './TelegramLoginButton';

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
  document.body.replaceChildren();
  delete window.TelegramLoginWidget;
});

const renderWidget = async (props) => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<TelegramLoginButton {...props} />));
  return { container, root };
};

describe('TelegramLoginButton', () => {
  test('loads the Telegram widget with a React-owned auth callback', async () => {
    const authenticatedUsers = [];
    const { container, root } = await renderWidget({
      botName: 'clove_bot',
      buttonSize: 'medium',
      cornerRadius: 8,
      requestAccess: 'read',
      usePic: false,
      lang: 'zh-CN',
      dataOnauth: (user) => authenticatedUsers.push(user),
    });

    const script = container.querySelector('script');
    expect(script?.src).toBe('https://telegram.org/js/telegram-widget.js?22');
    expect(script?.getAttribute('data-telegram-login')).toBe('clove_bot');
    expect(script?.getAttribute('data-size')).toBe('medium');
    expect(script?.getAttribute('data-radius')).toBe('8');
    expect(script?.getAttribute('data-request-access')).toBe('read');
    expect(script?.getAttribute('data-userpic')).toBe('false');
    expect(script?.getAttribute('data-lang')).toBe('zh-CN');
    expect(script?.getAttribute('data-onauth')).toBe(
      'TelegramLoginWidget.dataOnauth(user)',
    );

    window.TelegramLoginWidget.dataOnauth({ id: 42 });
    expect(authenticatedUsers).toEqual([{ id: 42 }]);

    await act(async () => root.unmount());
  });

  test('uses Telegram redirect mode when an auth URL is supplied', async () => {
    const { container, root } = await renderWidget({
      botName: 'clove_bot',
      dataAuthUrl: '/api/oauth/telegram/bind',
    });

    const script = container.querySelector('script');
    expect(script?.getAttribute('data-auth-url')).toBe(
      '/api/oauth/telegram/bind',
    );
    expect(script?.hasAttribute('data-onauth')).toBe(false);

    await act(async () => root.unmount());
  });

  test('removes widget-generated nodes before reloading changed options', async () => {
    const { container, root } = await renderWidget({
      botName: 'clove_bot',
      buttonSize: 'large',
    });
    const firstScript = container.querySelector('script');
    const widgetFrame = document.createElement('iframe');
    firstScript?.parentElement?.appendChild(widgetFrame);

    await act(async () =>
      root.render(
        <TelegramLoginButton botName='clove_bot' buttonSize='small' />,
      ),
    );

    expect(container.querySelectorAll('iframe')).toHaveLength(0);
    expect(container.querySelectorAll('script')).toHaveLength(1);
    expect(container.querySelector('script')?.getAttribute('data-size')).toBe(
      'small',
    );

    await act(async () => root.unmount());
  });
});
