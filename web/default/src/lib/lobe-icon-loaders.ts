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
export interface LobeIconModule {
  default: unknown
}

type LobeIconLoader = () => Promise<LobeIconModule>

export const lobeIconLoaders = {
  Ai360: () => import('@lobehub/icons/es/Ai360/components/Mono.js'),
  Aws: () => import('@lobehub/icons/es/Aws/components/Mono.js'),
  Azure: () => import('@lobehub/icons/es/Azure/components/Mono.js'),
  Baichuan: () => import('@lobehub/icons/es/Baichuan/components/Mono.js'),
  Baidu: () => import('@lobehub/icons/es/Baidu/components/Mono.js'),
  Claude: () => import('@lobehub/icons/es/Claude/components/Mono.js'),
  Cline: () => import('@lobehub/icons/es/Cline/components/Mono.js'),
  Cloudflare: () => import('@lobehub/icons/es/Cloudflare/components/Mono.js'),
  Cohere: () => import('@lobehub/icons/es/Cohere/components/Mono.js'),
  Coze: () => import('@lobehub/icons/es/Coze/components/Mono.js'),
  DeepSeek: () => import('@lobehub/icons/es/DeepSeek/components/Mono.js'),
  Dify: () => import('@lobehub/icons/es/Dify/components/Mono.js'),
  Doubao: () => import('@lobehub/icons/es/Doubao/components/Mono.js'),
  FastGPT: () => import('@lobehub/icons/es/FastGPT/components/Mono.js'),
  Gemini: () => import('@lobehub/icons/es/Gemini/components/Mono.js'),
  Google: () => import('@lobehub/icons/es/Google/components/Mono.js'),
  Grok: () => import('@lobehub/icons/es/Grok/components/Mono.js'),
  Hunyuan: () => import('@lobehub/icons/es/Hunyuan/components/Mono.js'),
  InternLM: () => import('@lobehub/icons/es/InternLM/components/Mono.js'),
  Jimeng: () => import('@lobehub/icons/es/Jimeng/components/Mono.js'),
  Jina: () => import('@lobehub/icons/es/Jina/components/Mono.js'),
  Kling: () => import('@lobehub/icons/es/Kling/components/Mono.js'),
  LobeHub: () => import('@lobehub/icons/es/LobeHub/components/Mono.js'),
  Meta: () => import('@lobehub/icons/es/Meta/components/Mono.js'),
  Midjourney: () => import('@lobehub/icons/es/Midjourney/components/Mono.js'),
  Minimax: () => import('@lobehub/icons/es/Minimax/components/Mono.js'),
  Mistral: () => import('@lobehub/icons/es/Mistral/components/Mono.js'),
  Moonshot: () => import('@lobehub/icons/es/Moonshot/components/Mono.js'),
  NewAPI: () => import('@lobehub/icons/es/NewAPI/components/Mono.js'),
  Ollama: () => import('@lobehub/icons/es/Ollama/components/Mono.js'),
  OpenAI: () => import('@lobehub/icons/es/OpenAI/components/Mono.js'),
  OpenRouter: () => import('@lobehub/icons/es/OpenRouter/components/Mono.js'),
  OpenWebUI: () => import('@lobehub/icons/es/OpenWebUI/components/Mono.js'),
  Perplexity: () => import('@lobehub/icons/es/Perplexity/components/Mono.js'),
  Qwen: () => import('@lobehub/icons/es/Qwen/components/Mono.js'),
  Replicate: () => import('@lobehub/icons/es/Replicate/components/Mono.js'),
  SiliconCloud: () =>
    import('@lobehub/icons/es/SiliconCloud/components/Mono.js'),
  Spark: () => import('@lobehub/icons/es/Spark/components/Mono.js'),
  Stepfun: () => import('@lobehub/icons/es/Stepfun/components/Mono.js'),
  Suno: () => import('@lobehub/icons/es/Suno/components/Mono.js'),
  Vidu: () => import('@lobehub/icons/es/Vidu/components/Mono.js'),
  Volcengine: () => import('@lobehub/icons/es/Volcengine/components/Mono.js'),
  Wenxin: () => import('@lobehub/icons/es/Wenxin/components/Mono.js'),
  XAI: () => import('@lobehub/icons/es/XAI/components/Mono.js'),
  XiaomiMiMo: () => import('@lobehub/icons/es/XiaomiMiMo/components/Mono.js'),
  Xinference: () => import('@lobehub/icons/es/Xinference/components/Mono.js'),
  Yi: () => import('@lobehub/icons/es/Yi/components/Mono.js'),
  Zhipu: () => import('@lobehub/icons/es/Zhipu/components/Mono.js'),
} satisfies Record<string, LobeIconLoader>

const colorLoaders = {
  Ai360: () => import('@lobehub/icons/es/Ai360/components/Color.js'),
  Aws: () => import('@lobehub/icons/es/Aws/components/Color.js'),
  Azure: () => import('@lobehub/icons/es/Azure/components/Color.js'),
  Baichuan: () => import('@lobehub/icons/es/Baichuan/components/Color.js'),
  Baidu: () => import('@lobehub/icons/es/Baidu/components/Color.js'),
  Claude: () => import('@lobehub/icons/es/Claude/components/Color.js'),
  Cloudflare: () => import('@lobehub/icons/es/Cloudflare/components/Color.js'),
  Cohere: () => import('@lobehub/icons/es/Cohere/components/Color.js'),
  DeepSeek: () => import('@lobehub/icons/es/DeepSeek/components/Color.js'),
  Dify: () => import('@lobehub/icons/es/Dify/components/Color.js'),
  Doubao: () => import('@lobehub/icons/es/Doubao/components/Color.js'),
  FastGPT: () => import('@lobehub/icons/es/FastGPT/components/Color.js'),
  Gemini: () => import('@lobehub/icons/es/Gemini/components/Color.js'),
  Google: () => import('@lobehub/icons/es/Google/components/Color.js'),
  Hunyuan: () => import('@lobehub/icons/es/Hunyuan/components/Color.js'),
  InternLM: () => import('@lobehub/icons/es/InternLM/components/Color.js'),
  Jimeng: () => import('@lobehub/icons/es/Jimeng/components/Color.js'),
  Kling: () => import('@lobehub/icons/es/Kling/components/Color.js'),
  LobeHub: () => import('@lobehub/icons/es/LobeHub/components/Color.js'),
  Meta: () => import('@lobehub/icons/es/Meta/components/Color.js'),
  Minimax: () => import('@lobehub/icons/es/Minimax/components/Color.js'),
  Mistral: () => import('@lobehub/icons/es/Mistral/components/Color.js'),
  NewAPI: () => import('@lobehub/icons/es/NewAPI/components/Color.js'),
  Perplexity: () => import('@lobehub/icons/es/Perplexity/components/Color.js'),
  Qwen: () => import('@lobehub/icons/es/Qwen/components/Color.js'),
  SiliconCloud: () =>
    import('@lobehub/icons/es/SiliconCloud/components/Color.js'),
  Spark: () => import('@lobehub/icons/es/Spark/components/Color.js'),
  Stepfun: () => import('@lobehub/icons/es/Stepfun/components/Color.js'),
  Vidu: () => import('@lobehub/icons/es/Vidu/components/Color.js'),
  Volcengine: () => import('@lobehub/icons/es/Volcengine/components/Color.js'),
  Wenxin: () => import('@lobehub/icons/es/Wenxin/components/Color.js'),
  Xinference: () => import('@lobehub/icons/es/Xinference/components/Color.js'),
  Yi: () => import('@lobehub/icons/es/Yi/components/Color.js'),
  Zhipu: () => import('@lobehub/icons/es/Zhipu/components/Color.js'),
} satisfies Record<string, LobeIconLoader>

const modulePromises = new Map<string, Promise<LobeIconModule>>()

export function resolveLobeIconLoader(
  baseKey: string,
  variant?: string
): LobeIconLoader | undefined {
  if (!Object.hasOwn(lobeIconLoaders, baseKey)) return undefined

  if (variant === 'Avatar' || variant === 'Color') {
    const colorLoader = colorLoaders[baseKey as keyof typeof colorLoaders]
    if (colorLoader) return colorLoader
  }

  return lobeIconLoaders[baseKey as keyof typeof lobeIconLoaders]
}

export function loadLobeIconModule(
  baseKey: string,
  variant?: string
): Promise<LobeIconModule> | undefined {
  const loader = resolveLobeIconLoader(baseKey, variant)
  if (!loader) return undefined

  const baseLoader = lobeIconLoaders[baseKey as keyof typeof lobeIconLoaders]
  const cacheKey = `${baseKey}.${loader === baseLoader ? 'Mono' : 'Color'}`
  const cached = modulePromises.get(cacheKey)
  if (cached) return cached

  const promise = loader().catch((error: unknown) => {
    modulePromises.delete(cacheKey)
    throw error
  })
  modulePromises.set(cacheKey, promise)
  return promise
}
