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
import { describe, expect, test } from 'bun:test'

import type { PlaygroundAttachment } from '../../types'
import {
  buildPlaygroundMessageContent,
  buildPromptWithTextAttachments,
  isSupportedDocumentFile,
  readPlaygroundAttachments,
  sanitizeAttachmentsForStorage,
} from './attachment-utils'

describe('playground attachment utilities', () => {
  const textAttachment: PlaygroundAttachment = {
    id: 'text-1',
    kind: 'text',
    mimeType: 'text/markdown',
    name: 'notes.md',
    size: 16,
    text: '# Notes\nhello',
  }

  const imageAttachment: PlaygroundAttachment = {
    id: 'image-1',
    kind: 'image',
    mimeType: 'image/png',
    name: 'diagram.png',
    size: 32,
    dataUrl: 'data:image/png;base64,abc123',
  }

  test('adds text files as delimited prompt context', () => {
    expect(buildPromptWithTextAttachments('Summarize this.', [textAttachment]))
      .toBe(`Summarize this.

<attachments>
<attachment name="notes.md" type="text/markdown">
# Notes
hello
</attachment>
</attachments>`)
  })

  test('builds OpenAI-compatible image content parts', () => {
    expect(
      buildPlaygroundMessageContent('What is shown?', [imageAttachment])
    ).toEqual([
      { type: 'text', text: 'What is shown?' },
      {
        type: 'image_url',
        image_url: { url: 'data:image/png;base64,abc123' },
      },
    ])
  })

  test('accepts common document formats for file uploads', () => {
    expect(
      isSupportedDocumentFile(
        new File(['%PDF-1.4'], 'brief.pdf', { type: 'application/pdf' })
      )
    ).toBe(true)
    expect(
      isSupportedDocumentFile(
        new File(['docx'], 'notes.docx', {
          type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        })
      )
    ).toBe(true)
    expect(
      isSupportedDocumentFile(
        new File(['xlsx'], 'budget.xlsx', {
          type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        })
      )
    ).toBe(true)
  })

  test('reads spreadsheets as text context attachments', async () => {
    const xlsx = await import('@e965/xlsx')
    const workbook = xlsx.utils.book_new()
    const worksheet = xlsx.utils.aoa_to_sheet([
      ['Project', 'Status'],
      ['Upload', 'Ready'],
    ])
    xlsx.utils.book_append_sheet(workbook, worksheet, 'Plan')

    const bytes = xlsx.write(workbook, {
      bookType: 'xlsx',
      type: 'array',
    })
    const result = await readPlaygroundAttachments([
      new File([bytes], 'plan.xlsx', {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    ])

    expect(result.rejected).toEqual([])
    expect(result.attachments).toHaveLength(1)
    const attachment = result.attachments[0]
    expect(attachment?.kind).toBe('text')
    if (!attachment || attachment.kind !== 'text') {
      throw new Error(
        'Expected spreadsheet upload to become a text attachment.'
      )
    }
    expect(attachment.name).toBe('plan.xlsx')
    expect(attachment.text).toContain('[Sheet: Plan]')
    expect(attachment.text).toContain('Project,Status')
    expect(attachment.text).toContain('Upload,Ready')
  })

  test('strips attachment payloads before localStorage persistence', () => {
    expect(
      sanitizeAttachmentsForStorage([textAttachment, imageAttachment])
    ).toEqual([
      {
        id: 'text-1',
        kind: 'text',
        mimeType: 'text/markdown',
        name: 'notes.md',
        size: 16,
      },
      {
        id: 'image-1',
        kind: 'image',
        mimeType: 'image/png',
        name: 'diagram.png',
        size: 32,
      },
    ])
  })
})
