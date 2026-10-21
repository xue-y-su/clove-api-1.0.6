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
import { nanoid } from 'nanoid'

import type {
  ContentPart,
  PlaygroundAttachment,
  PlaygroundTextAttachment,
} from '../../types'

export const MAX_IMAGE_ATTACHMENTS = 6
export const MAX_IMAGE_ATTACHMENT_BYTES = 8 * 1024 * 1024
export const MAX_TEXT_ATTACHMENTS = 8
export const MAX_TEXT_ATTACHMENT_BYTES = 512 * 1024
export const MAX_DOCUMENT_ATTACHMENT_BYTES = 16 * 1024 * 1024
export const MAX_TEXT_ATTACHMENT_CHARS = 80_000
export const MAX_TOTAL_TEXT_ATTACHMENT_CHARS = 120_000

const SUPPORTED_TEXT_EXTENSIONS = new Set([
  'txt',
  'md',
  'markdown',
  'json',
  'csv',
  'tsv',
  'log',
  'yaml',
  'yml',
  'xml',
])

const IMAGE_MIME_PREFIX = 'image/'

const DOCUMENT_EXTENSIONS = new Set(['pdf', 'docx', 'xls', 'xlsx', 'ods'])

const DOCUMENT_MIME_TYPES = new Set([
  'application/pdf',
  'application/msword',
  'application/vnd.ms-excel',
  'application/vnd.oasis.opendocument.spreadsheet',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
])

type AttachmentRejectedMessage = {
  key: string
  values?: Record<string, number | string>
}

type AttachmentReadResult =
  | {
      attachments: PlaygroundAttachment[]
      rejected: AttachmentRejectedMessage[]
    }
  | {
      attachments: []
      rejected: AttachmentRejectedMessage[]
    }

function getFileExtension(fileName: string): string {
  const lastDotIndex = fileName.lastIndexOf('.')

  if (lastDotIndex === -1) {
    return ''
  }

  return fileName.slice(lastDotIndex + 1).toLowerCase()
}

function normalizeMimeType(file: File): string {
  return file.type || 'application/octet-stream'
}

function isTextLikeMimeType(mimeType: string): boolean {
  return (
    mimeType.startsWith('text/') ||
    mimeType === 'application/json' ||
    mimeType === 'application/xml' ||
    mimeType === 'application/x-yaml'
  )
}

function escapeAttachmentAttribute(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
}

function truncateTextAttachment(text: string): string {
  if (text.length <= MAX_TEXT_ATTACHMENT_CHARS) {
    return text
  }

  return `${text.slice(0, MAX_TEXT_ATTACHMENT_CHARS)}\n\n[Attachment truncated]`
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()

    reader.addEventListener(
      'load',
      () => {
        if (typeof reader.result !== 'string') {
          reject(new Error('Failed to read image attachment.'))
          return
        }

        resolve(reader.result)
      },
      { once: true }
    )
    reader.addEventListener('error', () => reject(reader.error), { once: true })
    reader.readAsDataURL(file)
  })
}

function hasTextAttachmentPayload(
  attachment: PlaygroundAttachment
): attachment is PlaygroundTextAttachment & { text: string } {
  return attachment.kind === 'text' && Boolean(attachment.text)
}

function isPdfFile(file: File): boolean {
  return (
    normalizeMimeType(file) === 'application/pdf' ||
    getFileExtension(file.name) === 'pdf'
  )
}

function isDocxFile(file: File): boolean {
  return (
    normalizeMimeType(file) ===
      'application/vnd.openxmlformats-officedocument.wordprocessingml.document' ||
    getFileExtension(file.name) === 'docx'
  )
}

function isSpreadsheetFile(file: File): boolean {
  const mimeType = normalizeMimeType(file)
  const extension = getFileExtension(file.name)

  return (
    mimeType === 'application/vnd.ms-excel' ||
    mimeType ===
      'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' ||
    mimeType === 'application/vnd.oasis.opendocument.spreadsheet' ||
    extension === 'xls' ||
    extension === 'xlsx' ||
    extension === 'ods'
  )
}

async function readPdfText(file: File): Promise<string> {
  const pdfjs = await import('pdfjs-dist/legacy/build/pdf.mjs')
  pdfjs.GlobalWorkerOptions.workerSrc ||= new URL(
    'pdfjs-dist/legacy/build/pdf.worker.mjs',
    import.meta.url
  ).toString()

  const loadingTask = pdfjs.getDocument({
    data: new Uint8Array(await file.arrayBuffer()),
  })
  const pdf = await loadingTask.promise
  const pages: string[] = []

  try {
    for (let pageNumber = 1; pageNumber <= pdf.numPages; pageNumber += 1) {
      const page = await pdf.getPage(pageNumber)
      const content = await page.getTextContent()
      const pageText = content.items
        .map((item) => ('str' in item ? item.str : ''))
        .join(' ')
        .replaceAll(/\s+/g, ' ')
        .trim()

      if (pageText) {
        pages.push(`[Page ${pageNumber}]\n${pageText}`)
      }
      page.cleanup()
    }
  } finally {
    await loadingTask.destroy()
  }

  return pages.join('\n\n')
}

async function readDocxText(file: File): Promise<string> {
  const mammoth = await import('mammoth')
  const result = await mammoth.extractRawText({
    arrayBuffer: await file.arrayBuffer(),
  })

  return result.value.trim()
}

async function readSpreadsheetText(file: File): Promise<string> {
  const xlsx = await import('@e965/xlsx')
  const workbook = xlsx.read(await file.arrayBuffer(), {
    type: 'array',
  })
  const sheets: string[] = []

  for (const sheetName of workbook.SheetNames) {
    const sheet = workbook.Sheets[sheetName]
    const csv = xlsx.utils
      .sheet_to_csv(sheet, {
        blankrows: false,
      })
      .trim()

    if (csv) {
      sheets.push(`[Sheet: ${sheetName}]\n${csv}`)
    }
  }

  return sheets.join('\n\n')
}

async function readDocumentText(file: File): Promise<string> {
  if (isPdfFile(file)) {
    return readPdfText(file)
  }

  if (isDocxFile(file)) {
    return readDocxText(file)
  }

  if (isSpreadsheetFile(file)) {
    return readSpreadsheetText(file)
  }

  return ''
}

export function isSupportedImageFile(file: File): boolean {
  return normalizeMimeType(file).startsWith(IMAGE_MIME_PREFIX)
}

export function isSupportedDocumentFile(file: File): boolean {
  const mimeType = normalizeMimeType(file)
  const extension = getFileExtension(file.name)

  return DOCUMENT_MIME_TYPES.has(mimeType) || DOCUMENT_EXTENSIONS.has(extension)
}

export function isSupportedTextFile(file: File): boolean {
  const mimeType = normalizeMimeType(file)

  if (isTextLikeMimeType(mimeType)) {
    return true
  }

  return SUPPORTED_TEXT_EXTENSIONS.has(getFileExtension(file.name))
}

export function buildUnsupportedAttachmentMessage(
  file: File
): AttachmentRejectedMessage {
  const extension = getFileExtension(file.name)
  const typeLabel = file.type || (extension ? `.${extension}` : 'unknown')

  return {
    key: '{{name}} is not supported yet ({{type}}). Use images, PDFs, Word or Excel documents, or plain text files.',
    values: {
      name: file.name,
      type: typeLabel,
    },
  }
}

export async function readPlaygroundAttachments(
  files: File[] | FileList,
  existingAttachments: PlaygroundAttachment[] = []
): Promise<AttachmentReadResult> {
  const acceptedAttachments: PlaygroundAttachment[] = []
  const rejected: AttachmentRejectedMessage[] = []
  let imageCount = existingAttachments.filter(
    (attachment) => attachment.kind === 'image'
  ).length
  let textCount = existingAttachments.filter(
    (attachment) => attachment.kind === 'text'
  ).length
  let totalTextChars = existingAttachments.reduce(
    (total, attachment) =>
      attachment.kind === 'text'
        ? total + (attachment.text?.length ?? 0)
        : total,
    0
  )

  for (const file of files) {
    if (isSupportedImageFile(file)) {
      if (imageCount >= MAX_IMAGE_ATTACHMENTS) {
        rejected.push({
          key: 'Only {{count}} images can be attached.',
          values: { count: MAX_IMAGE_ATTACHMENTS },
        })
        continue
      }

      if (file.size > MAX_IMAGE_ATTACHMENT_BYTES) {
        rejected.push({
          key: '{{name}} is too large for an image attachment.',
          values: { name: file.name },
        })
        continue
      }

      acceptedAttachments.push({
        id: nanoid(),
        kind: 'image',
        mimeType: normalizeMimeType(file),
        name: file.name || 'image',
        size: file.size,
        dataUrl: await readFileAsDataUrl(file),
      })
      imageCount += 1
      continue
    }

    if (isSupportedTextFile(file)) {
      if (textCount >= MAX_TEXT_ATTACHMENTS) {
        rejected.push({
          key: 'Only {{count}} files can be attached as text context.',
          values: { count: MAX_TEXT_ATTACHMENTS },
        })
        continue
      }

      if (file.size > MAX_TEXT_ATTACHMENT_BYTES) {
        rejected.push({
          key: '{{name}} is too large for a text attachment.',
          values: { name: file.name },
        })
        continue
      }

      const text = truncateTextAttachment(await file.text())
      if (totalTextChars + text.length > MAX_TOTAL_TEXT_ATTACHMENT_CHARS) {
        rejected.push({
          key: 'Attached text files are too long for one message.',
        })
        continue
      }

      acceptedAttachments.push({
        id: nanoid(),
        kind: 'text',
        mimeType: normalizeMimeType(file),
        name: file.name || 'text',
        size: file.size,
        text,
      })
      textCount += 1
      totalTextChars += text.length
      continue
    }

    if (isSupportedDocumentFile(file)) {
      if (textCount >= MAX_TEXT_ATTACHMENTS) {
        rejected.push({
          key: 'Only {{count}} files can be attached as text context.',
          values: { count: MAX_TEXT_ATTACHMENTS },
        })
        continue
      }

      if (file.size > MAX_DOCUMENT_ATTACHMENT_BYTES) {
        rejected.push({
          key: '{{name}} is too large for a document attachment.',
          values: { name: file.name },
        })
        continue
      }

      try {
        const text = truncateTextAttachment(await readDocumentText(file))
        if (!text.trim()) {
          rejected.push({
            key: 'Could not extract readable text from {{name}}.',
            values: { name: file.name },
          })
          continue
        }

        if (totalTextChars + text.length > MAX_TOTAL_TEXT_ATTACHMENT_CHARS) {
          rejected.push({
            key: 'Attached text files are too long for one message.',
          })
          continue
        }

        acceptedAttachments.push({
          id: nanoid(),
          kind: 'text',
          mimeType: normalizeMimeType(file),
          name: file.name || 'document',
          size: file.size,
          text,
        })
        textCount += 1
        totalTextChars += text.length
      } catch {
        rejected.push({
          key: '{{name}} could not be read as a document.',
          values: { name: file.name },
        })
      }
      continue
    }

    rejected.push(buildUnsupportedAttachmentMessage(file))
  }

  return {
    attachments: acceptedAttachments,
    rejected,
  }
}

export function buildPromptWithTextAttachments(
  text: string,
  attachments: PlaygroundAttachment[] = []
): string {
  const textAttachments = attachments.filter(hasTextAttachmentPayload)

  if (textAttachments.length === 0) {
    return text
  }

  const attachmentContext = textAttachments
    .map((attachment) => {
      const safeName = escapeAttachmentAttribute(attachment.name)
      const safeMimeType = escapeAttachmentAttribute(attachment.mimeType)

      return `<attachment name="${safeName}" type="${safeMimeType}">
${attachment.text}
</attachment>`
    })
    .join('\n')

  const contextBlock = `<attachments>
${attachmentContext}
</attachments>`

  return text.trim().length > 0 ? `${text}\n\n${contextBlock}` : contextBlock
}

export function getImageAttachmentDataUrls(
  attachments: PlaygroundAttachment[] = []
): string[] {
  return attachments.flatMap((attachment) =>
    attachment.kind === 'image' && attachment.dataUrl
      ? [attachment.dataUrl]
      : []
  )
}

export function buildPlaygroundMessageContent(
  text: string,
  attachments: PlaygroundAttachment[] = []
): string | ContentPart[] {
  const prompt = buildPromptWithTextAttachments(text, attachments)
  const imageUrls = getImageAttachmentDataUrls(attachments)

  if (imageUrls.length === 0) {
    return prompt
  }

  return [
    {
      type: 'text',
      text: prompt || '',
    },
    ...imageUrls.map((url) => ({
      type: 'image_url' as const,
      image_url: { url },
    })),
  ]
}

export function sanitizeAttachmentsForStorage(
  attachments: PlaygroundAttachment[] = []
): PlaygroundAttachment[] {
  return attachments.map((attachment) => ({
    id: attachment.id,
    kind: attachment.kind,
    mimeType: attachment.mimeType,
    name: attachment.name,
    size: attachment.size,
  })) as PlaygroundAttachment[]
}

export function hasAttachmentPayload(
  attachment: PlaygroundAttachment
): boolean {
  if (attachment.kind === 'image') {
    return Boolean(attachment.dataUrl)
  }

  return Boolean(attachment.text)
}

export function formatAttachmentSize(size: number): string {
  if (!Number.isFinite(size) || size <= 0) {
    return '0 B'
  }

  if (size < 1024) {
    return `${size} B`
  }

  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`
  }

  return `${(size / 1024 / 1024).toFixed(1)} MB`
}
