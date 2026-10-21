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
import createDOMPurify from 'dompurify';
import { marked } from 'marked';

const sanitizeOptions = {
  ADD_ATTR: ['allowfullscreen', 'loading', 'referrerpolicy', 'rel', 'target'],
  FORBID_ATTR: ['srcdoc'],
  FORBID_TAGS: [
    'base',
    'embed',
    'form',
    'link',
    'meta',
    'object',
    'script',
    'style',
  ],
};

const iframeSandbox =
  'allow-forms allow-popups allow-popups-to-escape-sandbox allow-presentation';

function getDOMPurify() {
  if (typeof createDOMPurify === 'function') {
    return createDOMPurify(window);
  }
  return createDOMPurify;
}

function addExternalLinkAttributes(html) {
  const template = document.createElement('template');
  template.innerHTML = html;

  template.content.querySelectorAll('a[href]').forEach((link) => {
    const href = link.getAttribute('href');
    if (!href) return;

    let url;
    try {
      url = new URL(href, window.location.origin);
    } catch {
      link.removeAttribute('href');
      return;
    }

    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
      link.removeAttribute('href');
      return;
    }

    if (url.origin !== window.location.origin) {
      link.setAttribute('target', '_blank');
      link.setAttribute('rel', 'noopener noreferrer');
    }
  });

  template.content.querySelectorAll('iframe').forEach((frame) => {
    frame.removeAttribute('srcdoc');
    frame.setAttribute('sandbox', iframeSandbox);
    frame.setAttribute('referrerpolicy', 'no-referrer');
    if (!frame.hasAttribute('loading')) {
      frame.setAttribute('loading', 'lazy');
    }
  });

  return template.innerHTML;
}

export function sanitizeRichHtml(html = '') {
  const sanitizedHtml = getDOMPurify().sanitize(String(html), sanitizeOptions);
  return addExternalLinkAttributes(sanitizedHtml);
}

export function sanitizeMarkdown(markdown = '') {
  return sanitizeRichHtml(marked.parse(String(markdown)));
}
