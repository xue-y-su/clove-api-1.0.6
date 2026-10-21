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
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import * as playgroundHelpers from './playground';

import {
  buildPlaygroundChatFeatureFields,
  buildPlaygroundImageRequest,
  buildPlaygroundOptionUpdates,
  createThinkTagFilter,
  createPlaygroundImagePreviews,
  getPlaygroundImageErrorMessage,
  getPlaygroundImageCapabilities,
  parsePlaygroundStreamEvent,
  processModelsData,
  revokePlaygroundImagePreviews,
  sanitizePlaygroundReasoningResponse,
} from './playground';

describe('classic playground protocol helpers', () => {
  test('gives every image option select an accessible name', () => {
    const panelSource = readFileSync(
      new URL(
        '../components/playground/PlaygroundImagePanel.jsx',
        import.meta.url,
      ),
      'utf8',
    );

    for (const label of ['分组', '模型', '尺寸', '质量', '风格', '背景']) {
      expect(panelSource).toContain(`aria-label={t('${label}')}`);
    }
  });

  test('does not expose a selectable image output format', () => {
    const panelSource = readFileSync(
      new URL(
        '../components/playground/PlaygroundImagePanel.jsx',
        import.meta.url,
      ),
      'utf8',
    );

    expect(panelSource).not.toContain('outputFormat');
    expect(panelSource).not.toContain('b64_json');
    expect(panelSource).not.toContain('输出格式');
  });

  test('normalizes nullish model payloads to an empty selection', () => {
    expect(processModelsData(null, 'gpt-4o')).toEqual({
      modelOptions: [],
      selectedModel: undefined,
    });
    expect(processModelsData(undefined, 'gpt-4o')).toEqual({
      modelOptions: [],
      selectedModel: undefined,
    });
  });

  test('orders Tavily key replacement before enabling', () => {
    const defaults = {
      enabled: false,
      apiKeyConfigured: false,
      maxResults: 5,
    };

    expect(
      buildPlaygroundOptionUpdates(defaults, {
        ...defaults,
        enabled: true,
        apiKey: 'tvly-new-key',
        clearAPIKey: false,
      }),
    ).toEqual([
      { key: 'PlaygroundTavilyAPIKey', value: 'tvly-new-key' },
      { key: 'PlaygroundTavilyEnabled', value: 'true' },
    ]);
  });

  test('does not expose removed image policy controls in settings', () => {
    const settingsSource = readFileSync(
      new URL('../components/settings/PlaygroundSetting.jsx', import.meta.url),
      'utf8',
    );

    expect(settingsSource).not.toContain('imageAllowlist');
    expect(settingsSource).not.toContain('imageDenylist');
    expect(settingsSource).not.toContain('imageDisplayOrder');
    expect(settingsSource).not.toContain('imageExtraAllowlist');
    expect(settingsSource).not.toContain('PlaygroundImageModelsAllowlist');
  });

  test('orders Tavily disable before key removal', () => {
    const defaults = {
      enabled: true,
      apiKeyConfigured: true,
      maxResults: 5,
    };

    expect(
      buildPlaygroundOptionUpdates(defaults, {
        ...defaults,
        enabled: false,
        apiKey: '',
        clearAPIKey: true,
      }),
    ).toEqual([
      { key: 'PlaygroundTavilyEnabled', value: 'false' },
      { key: 'PlaygroundTavilyAPIKey', value: '' },
    ]);
  });

  test('maps search depth and supported reasoning to chat fields', () => {
    expect(
      buildPlaygroundChatFeatureFields({
        model: 'gpt-5-mini',
        reasoning_effort: 'high',
        web_search: 'advanced',
      }),
    ).toEqual({
      reasoning_effort: 'high',
      tools: [{ type: 'web_search_preview', search_context_size: 'high' }],
      web_search_options: { search_context_size: 'high' },
    });

    expect(
      buildPlaygroundChatFeatureFields({
        model: 'gpt-5-mini',
        reasoning_effort: 'off',
        web_search: 'off',
      }),
    ).toEqual({ reasoning_effort: 'none' });

    expect(
      buildPlaygroundChatFeatureFields({
        model: 'gpt-4o',
        reasoning_effort: 'high',
        web_search: 'off',
      }),
    ).toEqual({ reasoning_effort: 'high' });
  });

  test('extracts search metadata without treating it as model text', () => {
    expect(
      parsePlaygroundStreamEvent(
        JSON.stringify({
          type: 'playground.metadata',
          search: {
            status: 'ok',
            path: 'tavily',
            sources: [
              {
                title: 'Redis 8',
                url: 'https://redis.io/blog/redis-8',
                domain: 'redis.io',
                snippet: 'Release notes',
              },
            ],
          },
        }),
      ),
    ).toEqual({
      metadata: {
        status: 'ok',
        path: 'tavily',
        sources: [
          {
            title: 'Redis 8',
            url: 'https://redis.io/blog/redis-8',
            domain: 'redis.io',
            snippet: 'Release notes',
          },
        ],
      },
      updates: [],
    });
  });

  test('normalizes missing and malformed search sources', () => {
    expect(
      parsePlaygroundStreamEvent(
        JSON.stringify({
          type: 'playground.metadata',
          search: {
            status: 'ok',
            path: 'builtin',
            sources: [
              null,
              { title: 'Missing URL' },
              {
                title: 'Valid source',
                url: 'https://example.test/source',
                domain: 'example.test',
                snippet: 'Evidence',
              },
            ],
          },
        }),
      ),
    ).toEqual({
      metadata: {
        status: 'ok',
        path: 'builtin',
        sources: [
          {
            title: 'Valid source',
            url: 'https://example.test/source',
            domain: 'example.test',
            snippet: 'Evidence',
          },
        ],
      },
      updates: [],
    });

    expect(
      parsePlaygroundStreamEvent(
        JSON.stringify({
          type: 'playground.metadata',
          search: { status: 'ok', path: 'builtin' },
        }),
      ),
    ).toEqual({
      metadata: { status: 'ok', path: 'builtin', sources: [] },
      updates: [],
    });
  });

  test('keeps normal content and reasoning stream updates', () => {
    expect(
      parsePlaygroundStreamEvent(
        JSON.stringify({
          choices: [
            {
              delta: {
                content: 'Answer',
                reasoning_content: 'Thought',
              },
            },
          ],
        }),
      ),
    ).toEqual({
      metadata: null,
      updates: [
        { type: 'reasoning', chunk: 'Thought' },
        { type: 'content', chunk: 'Answer' },
      ],
    });
  });

  test('uses request-scoped reasoning suppression before state mutation', () => {
    const helperSource = readFileSync(
      new URL('./playground.js', import.meta.url),
      'utf8',
    );
    const requestSource = readFileSync(
      new URL('../hooks/playground/useApiRequest.jsx', import.meta.url),
      'utf8',
    );
    const settingsSource = readFileSync(
      new URL('../components/playground/SettingsPanel.jsx', import.meta.url),
      'utf8',
    );

    expect(helperSource).toContain('createThinkTagFilter');
    expect(helperSource).toContain('sanitizePlaygroundReasoningResponse');
    expect(requestSource).toContain("payload.reasoning_effort === 'none'");
    expect(requestSource).toContain('reasoningOutputFilter');
    expect(requestSource).toContain('sanitizedDebugEvent');
    expect(settingsSource).not.toContain(
      'supportsPlaygroundReasoning(inputs.model)',
    );
    expect(settingsSource).toContain(
      'Higher reasoning effort may use more tokens',
    );
  });

  test('removes complete, split, and dangling think output', () => {
    const complete = createThinkTagFilter();
    expect(complete.push('before<think>secret</think>after')).toBe(
      'beforeafter',
    );

    const split = createThinkTagFilter();
    expect(split.push('visible<thi')).toBe('visible');
    expect(split.push('nk>hidden</thi')).toBe('');
    expect(split.push('nk>done')).toBe('done');

    const dangling = createThinkTagFilter();
    expect(dangling.push('visible<think>hidden')).toBe('visible');
    expect(dangling.finish()).toBe('');
  });

  test('sanitizes non-stream reasoning before debug and message state', () => {
    expect(
      sanitizePlaygroundReasoningResponse({
        choices: [
          {
            message: {
              role: 'assistant',
              content: '<think>secret</think>Visible answer',
              reasoning_content: 'also secret',
              reasoning: 'legacy secret',
            },
          },
        ],
      }),
    ).toEqual({
      choices: [
        {
          message: {
            role: 'assistant',
            content: 'Visible answer',
          },
        },
      ],
    });
  });

  test('shows a fallback when hidden reasoning leaves no visible content', () => {
    const sanitized = sanitizePlaygroundReasoningResponse(
      {
        choices: [
          {
            message: {
              role: 'assistant',
              content: '<think>hidden only</think>',
              reasoning_content: 'hidden',
            },
          },
        ],
      },
      '模型未返回可显示内容',
    );

    expect(sanitized.choices[0].message.content).toBe('模型未返回可显示内容');
  });

  test('loads and searches only models in the selected classic image group', () => {
    const panelSource = readFileSync(
      new URL(
        '../components/playground/PlaygroundImagePanel.jsx',
        import.meta.url,
      ),
      'utf8',
    );

    expect(panelSource).not.toContain("endpoint_type: 'image-generation'");
    expect(panelSource).toContain('params: { group }');
    expect(panelSource).toContain('filter={selectFilter}');
    expect(panelSource).toContain("searchPosition='dropdown'");
    expect(panelSource).toContain("searchPlaceholder={t('搜索模型...')}");
    expect(panelSource).toContain('modelRequestRef.current += 1');
    expect(panelSource).not.toContain('可路由的生图模型');
    expect(panelSource).toContain('当前分组没有可用模型');
  });

  test('falls back when the account group is not selectable in image mode', () => {
    const groups = [
      { label: 'Default', value: 'default' },
      { label: 'Paid', value: 'paid' },
    ];

    expect(
      playgroundHelpers.resolvePlaygroundImageGroup?.(groups, 'Admin'),
    ).toBe('default');
    expect(
      playgroundHelpers.resolvePlaygroundImageGroup?.(groups, 'paid'),
    ).toBe('paid');
    expect(playgroundHelpers.resolvePlaygroundImageGroup?.([], 'Admin')).toBe(
      '',
    );
  });

  test('uses the shared clipboard fallback for image URLs', () => {
    const panelSource = readFileSync(
      new URL(
        '../components/playground/PlaygroundImagePanel.jsx',
        import.meta.url,
      ),
      'utf8',
    );

    expect(panelSource).toContain('await copy(url)');
    expect(panelSource).not.toContain('navigator.clipboard');
  });

  test('sends only model-supported image options', () => {
    expect(getPlaygroundImageCapabilities('dall-e-3')).toEqual({
      background: false,
      quality: ['standard', 'hd'],
      style: ['vivid', 'natural'],
    });
    expect(
      buildPlaygroundImageRequest({
        background: 'transparent',
        count: 2,
        group: 'default',
        model: 'dall-e-3',
        prompt: 'a snow house',
        quality: 'hd',
        size: '1024x1024',
        style: 'natural',
      }),
    ).toEqual({
      group: 'default',
      model: 'dall-e-3',
      n: 2,
      prompt: 'a snow house',
      quality: 'hd',
      response_format: 'url',
      size: '1024x1024',
      style: 'natural',
    });
  });

  test('omits provider-controlled auto options and always requests URLs', () => {
    expect(
      buildPlaygroundImageRequest({
        background: 'auto',
        count: 1,
        group: 'default',
        model: 'gpt-image-1',
        prompt: 'a quiet lake',
        quality: 'auto',
        size: '1024x1024',
      }),
    ).toEqual({
      group: 'default',
      model: 'gpt-image-1',
      n: 1,
      prompt: 'a quiet lake',
      response_format: 'url',
      size: '1024x1024',
    });
  });

  test('creates and revokes object URLs for base64 images', () => {
    const revoked = [];
    const previews = createPlaygroundImagePreviews(
      [
        { b64_json: 'aGVsbG8=', revised_prompt: 'hello' },
        { url: 'https://example.test/image.png' },
      ],
      () => 'blob:classic-preview',
    );

    expect(previews.map((item) => item.src)).toEqual([
      'blob:classic-preview',
      'https://example.test/image.png',
    ]);
    expect(previews.map((item) => item.id)).toEqual([
      'blob:classic-preview:0',
      'https://example.test/image.png:1',
    ]);

    revokePlaygroundImagePreviews(previews, (url) => revoked.push(url));
    expect(revoked).toEqual(['blob:classic-preview']);
  });

  test('prefers the upstream image error over the generic Axios status', () => {
    expect(
      getPlaygroundImageErrorMessage({
        message: 'Request failed with status code 503',
        response: {
          status: 503,
          data: { error: { message: 'upstream image service unavailable' } },
        },
      }),
    ).toBe('upstream image service unavailable');
  });

  test('falls back to the request error when the upstream has no message', () => {
    expect(
      getPlaygroundImageErrorMessage({
        message: 'Request failed with status code 503',
        response: { status: 503, data: {} },
      }),
    ).toBe('Request failed with status code 503');
  });

  test('replaces an opaque image-route 404 with actionable model guidance', () => {
    expect(
      getPlaygroundImageErrorMessage({
        message: 'Request failed with status code 404',
        response: {
          status: 404,
          data: { error: { message: 'openai_error' } },
        },
      }),
    ).toBe(
      'The selected model or channel does not support image generation. Please choose another model.',
    );
  });
});
