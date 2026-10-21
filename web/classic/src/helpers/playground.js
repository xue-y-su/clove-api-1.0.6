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

const REASONING_EFFORTS = new Set(['off', 'low', 'medium', 'high']);

const normalizeModelName = (model) =>
  String(model || '')
    .trim()
    .toLowerCase();

export const processModelsData = (data, currentModel) => {
  const modelOptions = (Array.isArray(data) ? data : []).map((model) => ({
    label: model,
    value: model,
  }));

  const hasCurrentModel = modelOptions.some(
    (option) => option.value === currentModel,
  );
  const selectedModel =
    hasCurrentModel && modelOptions.length > 0
      ? currentModel
      : modelOptions[0]?.value;

  return { modelOptions, selectedModel };
};

export const resolvePlaygroundImageGroup = (groups, currentGroup) => {
  const groupOptions = Array.isArray(groups) ? groups : [];
  const hasCurrentGroup = groupOptions.some(
    (option) => option.value === currentGroup,
  );
  return hasCurrentGroup ? currentGroup : groupOptions[0]?.value || '';
};

export const buildPlaygroundOptionUpdates = (defaults, values) => {
  const replacementKey = String(values.apiKey || '').trim();
  const keepsConfiguredKey = defaults.apiKeyConfigured && !values.clearAPIKey;
  if (values.enabled && !keepsConfiguredKey && !replacementKey) {
    throw new Error('A Tavily API key is required before enabling search.');
  }
  if (values.clearAPIKey && values.enabled) {
    throw new Error('Disable Tavily before clearing the API key.');
  }

  const updates = [];
  if (values.clearAPIKey) {
    if (defaults.enabled) {
      updates.push({ key: 'PlaygroundTavilyEnabled', value: 'false' });
    }
    updates.push({ key: 'PlaygroundTavilyAPIKey', value: '' });
  } else if (replacementKey) {
    updates.push({ key: 'PlaygroundTavilyAPIKey', value: replacementKey });
  }

  const fields = [
    ['defaultDepth', 'PlaygroundTavilyDefaultDepth'],
    ['maxResults', 'PlaygroundTavilyMaxResults'],
    ['maxContextLength', 'PlaygroundTavilyMaxContextLength'],
    ['nativeModels', 'PlaygroundNativeWebSearchModels'],
  ];
  fields.forEach(([field, key]) => {
    if (field in defaults && values[field] !== defaults[field]) {
      updates.push({ key, value: String(values[field]) });
    }
  });
  if (!values.clearAPIKey && values.enabled !== defaults.enabled) {
    updates.push({
      key: 'PlaygroundTavilyEnabled',
      value: String(values.enabled),
    });
  }
  return updates;
};

export const supportsPlaygroundReasoning = (model) => {
  const normalized = normalizeModelName(model);
  return (
    /^(?:o1|o3|o4)(?:[-_]|$)/.test(normalized) ||
    /^gpt[-_ ]?5(?:[-_ ]|$)/.test(normalized) ||
    /^grok[-_ ]?3[-_ ]?mini(?:[-_ ]|$)/.test(normalized)
  );
};

export const buildPlaygroundChatFeatureFields = (inputs) => {
  const fields = {};
  if (REASONING_EFFORTS.has(inputs.reasoning_effort)) {
    fields.reasoning_effort =
      inputs.reasoning_effort === 'off' ? 'none' : inputs.reasoning_effort;
  }

  const searchMode = inputs.web_search === true ? 'basic' : inputs.web_search;
  const searchContextSize = {
    basic: 'low',
    advanced: 'high',
  }[searchMode];
  if (searchContextSize) {
    fields.tools = [
      {
        type: 'web_search_preview',
        search_context_size: searchContextSize,
      },
    ];
    fields.web_search_options = {
      search_context_size: searchContextSize,
    };
  }

  return fields;
};

const OPEN_THINK_TAG = '<think>';
const CLOSE_THINK_TAG = '</think>';

const longestDelimiterPrefixSuffix = (value, delimiter) => {
  const maxLength = Math.min(value.length, delimiter.length - 1);
  for (let length = maxLength; length > 0; length -= 1) {
    const suffix = value.slice(-length);
    if (delimiter.startsWith(suffix)) return suffix;
  }
  return '';
};

export const createThinkTagFilter = () => {
  let insideThink = false;
  let pendingDelimiter = '';

  return {
    push(chunk) {
      let input = pendingDelimiter + String(chunk || '');
      let visible = '';
      pendingDelimiter = '';

      while (input) {
        const delimiter = insideThink ? CLOSE_THINK_TAG : OPEN_THINK_TAG;
        const delimiterIndex = input.indexOf(delimiter);
        if (delimiterIndex >= 0) {
          if (!insideThink) visible += input.slice(0, delimiterIndex);
          input = input.slice(delimiterIndex + delimiter.length);
          insideThink = !insideThink;
          continue;
        }

        const pending = longestDelimiterPrefixSuffix(input, delimiter);
        const stableLength = input.length - pending.length;
        if (!insideThink) visible += input.slice(0, stableLength);
        pendingDelimiter = pending;
        break;
      }

      return visible;
    },
    finish() {
      insideThink = false;
      pendingDelimiter = '';
      return '';
    },
  };
};

const stripThinkContent = (content) => {
  const filter = createThinkTagFilter();
  const visible = filter.push(String(content || ''));
  filter.finish();
  return visible;
};

export const sanitizePlaygroundReasoningResponse = (
  response,
  emptyContent = '',
) => ({
  ...response,
  choices: (response?.choices || []).map((choice) => {
    const message = choice?.message;
    if (!message) return choice;
    const {
      reasoning_content: _reasoningContent,
      reasoning: _reasoning,
      ...visibleMessage
    } = message;
    return {
      ...choice,
      message: {
        ...visibleMessage,
        content: stripThinkContent(message.content).trim() || emptyContent,
      },
    };
  }),
});

export const parsePlaygroundStreamEvent = (data) => {
  const payload = JSON.parse(data);
  if (payload?.type === 'playground.metadata' && payload.search) {
    const sources = Array.isArray(payload.search.sources)
      ? payload.search.sources.flatMap((source) => {
          if (!source || typeof source !== 'object') return [];
          if (typeof source.url !== 'string' || !source.url.trim()) return [];
          return [
            {
              title: typeof source.title === 'string' ? source.title : '',
              url: source.url,
              domain: typeof source.domain === 'string' ? source.domain : '',
              snippet: typeof source.snippet === 'string' ? source.snippet : '',
            },
          ];
        })
      : [];
    return {
      metadata: { ...payload.search, sources },
      updates: [],
    };
  }

  const delta = payload?.choices?.[0]?.delta;
  if (!delta) return { metadata: null, updates: [] };

  const updates = [];
  const reasoning = delta.reasoning_content || delta.reasoning;
  if (reasoning) updates.push({ type: 'reasoning', chunk: reasoning });
  if (delta.content) updates.push({ type: 'content', chunk: delta.content });
  return { metadata: null, updates };
};

export const getPlaygroundImageCapabilities = (model) => {
  const normalized = normalizeModelName(model);
  if (/\bgpt[-_ ]?image(?:[-_ ]?\d+)?\b/.test(normalized)) {
    return {
      background: true,
      quality: ['auto', 'low', 'medium', 'high'],
      style: [],
    };
  }
  if (/\bdall[-_ ]?e[-_ ]?3\b/.test(normalized)) {
    return {
      background: false,
      quality: ['standard', 'hd'],
      style: ['vivid', 'natural'],
    };
  }
  return { background: false, quality: [], style: [] };
};

export const getPlaygroundImageErrorMessage = (
  error,
  fallback = 'Unknown error',
) => {
  const responseData = error?.response?.data;
  const providerError = responseData?.error;
  const providerMessage =
    (typeof providerError?.message === 'string' &&
      providerError.message.trim()) ||
    (typeof providerError === 'string' && providerError.trim()) ||
    '';
  if (
    Number(error?.response?.status) === 404 &&
    providerMessage.toLowerCase() === 'openai_error'
  ) {
    return 'The selected model or channel does not support image generation. Please choose another model.';
  }
  return (
    providerMessage ||
    (typeof responseData?.message === 'string' &&
      responseData.message.trim()) ||
    (typeof error?.message === 'string' && error.message.trim()) ||
    fallback
  );
};

export const buildPlaygroundImageRequest = (input) => {
  const capabilities = getPlaygroundImageCapabilities(input.model);
  const request = {
    group: input.group,
    model: input.model,
    n: input.count,
    prompt: input.prompt,
    response_format: 'url',
    size: input.size,
  };
  if (
    input.quality &&
    input.quality !== 'auto' &&
    capabilities.quality.includes(input.quality)
  ) {
    request.quality = input.quality;
  }
  if (input.style && capabilities.style.includes(input.style)) {
    request.style = input.style;
  }
  if (
    input.background &&
    input.background !== 'auto' &&
    capabilities.background
  ) {
    request.background = input.background;
  }
  return request;
};

export const createPlaygroundImagePreviews = (
  images,
  createObjectURL = (blob) => URL.createObjectURL(blob),
) =>
  images.map((image, index) => {
    if (!image.b64_json) {
      const src = image.url || '';
      return {
        ...image,
        id: `${src || image.revised_prompt || 'image'}:${index}`,
        src,
      };
    }

    const binary = atob(image.b64_json);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    const objectURL = createObjectURL(new Blob([bytes], { type: 'image/png' }));
    return {
      ...image,
      id: `${objectURL}:${index}`,
      objectURL,
      src: objectURL,
    };
  });

export const revokePlaygroundImagePreviews = (
  images,
  revokeObjectURL = (url) => URL.revokeObjectURL(url),
) => {
  images.forEach((image) => {
    if (image.objectURL) revokeObjectURL(image.objectURL);
  });
};
