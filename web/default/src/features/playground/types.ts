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
// Message types
export type MessageRole = 'user' | 'assistant' | 'system'

export type MessageStatus = 'loading' | 'streaming' | 'complete' | 'error'

export type PlaygroundMessageLayoutMode = 'alternating' | 'left'

export interface MessageVersion {
  id: string
  content: string
}

export interface Message {
  key: string
  from: MessageRole
  versions: MessageVersion[]
  attachments?: PlaygroundAttachment[]
  createdAt?: number
  startedAt?: number
  completedAt?: number
  durationMs?: number
  sources?: { href: string; title: string }[]
  reasoning?: {
    content: string
    duration: number
    startedAt?: number
    completedAt?: number
    durationMs?: number
  }
  isReasoningStreaming?: boolean
  isReasoningComplete?: boolean
  isContentComplete?: boolean
  status?: MessageStatus
  errorCode?: string | null
}

export type PlaygroundAttachmentKind = 'image' | 'text'

interface PlaygroundAttachmentBase {
  id: string
  kind: PlaygroundAttachmentKind
  name: string
  size: number
  mimeType: string
}

export interface PlaygroundImageAttachment extends PlaygroundAttachmentBase {
  kind: 'image'
  dataUrl?: string
}

export interface PlaygroundTextAttachment extends PlaygroundAttachmentBase {
  kind: 'text'
  text?: string
}

export type PlaygroundAttachment =
  | PlaygroundImageAttachment
  | PlaygroundTextAttachment

// API payload types
export interface ChatCompletionMessage {
  role: MessageRole
  content: string | ContentPart[]
}

export interface ContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: {
    url: string
  }
}

export interface ChatCompletionRequest {
  model: string
  group?: string
  messages: ChatCompletionMessage[]
  stream: boolean
  temperature?: number
  top_p?: number
  max_tokens?: number
  frequency_penalty?: number
  presence_penalty?: number
  seed?: number
  reasoning_effort?: PlaygroundReasoningEffortValue | 'none'
  tools?: ChatCompletionTool[]
  web_search_options?: WebSearchOptions
}

export interface ChatCompletionTool {
  type: 'web_search_preview'
  search_context_size?: WebSearchContextSize
}

export type WebSearchContextSize = 'low' | 'medium' | 'high'

export interface WebSearchOptions {
  search_context_size?: WebSearchContextSize
}

export type PlaygroundWebSearchMode = 'off' | 'basic' | 'advanced'

export type PlaygroundImageSize =
  | '1024x1024'
  | '1024x1536'
  | '1536x1024'
  | '1024x1792'
  | '1792x1024'

export type PlaygroundImageOutputFormat = 'url' | 'b64_json'

export interface ImageGenerationRequest {
  model: string
  group?: string
  prompt: string
  n?: number
  size?: PlaygroundImageSize
  response_format?: PlaygroundImageOutputFormat
  background?: string
  quality?: string
  style?: string
}

export interface PlaygroundSearchMetadataSource {
  title: string
  url: string
  domain: string
  snippet: string
}

export interface PlaygroundSearchMetadata {
  status: string
  path: string
  sources: PlaygroundSearchMetadataSource[]
}

export interface ImageGenerationData {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface ImageGenerationResponse {
  created?: number
  data?: ImageGenerationData[]
}

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    delta: {
      role?: MessageRole
      content?: string
      reasoning_content?: string
    }
    finish_reason: string | null
  }>
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    message: {
      role: MessageRole
      content: string
      reasoning_content?: string
    }
    finish_reason: string
  }>
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

// Configuration types
export type PlaygroundReasoningEffortValue = 'low' | 'medium' | 'high'
export type PlaygroundReasoningEffort = 'off' | PlaygroundReasoningEffortValue

export interface PlaygroundConfig {
  model: string
  group: string
  temperature: number
  top_p: number
  max_tokens: number
  frequency_penalty: number
  presence_penalty: number
  seed: number | null
  stream: boolean
  web_search: PlaygroundWebSearchMode
  reasoning_effort: PlaygroundReasoningEffort
}

export interface ParameterEnabled {
  temperature: boolean
  top_p: boolean
  max_tokens: boolean
  frequency_penalty: boolean
  presence_penalty: boolean
  seed: boolean
}

// Model and group options
export interface ModelOption {
  label: string
  value: string
}

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}
