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
import { expect, test } from 'bun:test'

test('reserves mobile space below playground messages', async () => {
  const source = await Bun.file(
    new URL('./chat/playground-chat.tsx', import.meta.url)
  ).text()

  expect(source).toContain('pb-24 sm:pb-8')
})

test('keeps attachments independent from web search availability', async () => {
  const source = await Bun.file(
    new URL('./input/playground-input-tools.tsx', import.meta.url)
  ).text()

  expect(source).toContain('webSearchAvailable?: boolean')
  const attachControl = source.slice(
    source.indexOf("aria-label={t('Attach')}"),
    source.indexOf("aria-label={t('Web search')}")
  )
  const webSearchControl = source.slice(
    source.indexOf("aria-label={t('Web search')}"),
    source.indexOf("aria-label={t('Reasoning Effort')}")
  )
  expect(attachControl).toContain('disabled={disabled}')
  expect(attachControl).not.toContain('!webSearchAvailable')
  expect(webSearchControl).toContain(
    'disabled={disabled || !webSearchAvailable}'
  )
})

test('limits the file chooser to supported attachment formats', async () => {
  const source = await Bun.file(
    new URL('./input/playground-input.tsx', import.meta.url)
  ).text()

  const fileInputStart = source.indexOf("aria-label={t('Upload file')}")
  const fileInput = source.slice(
    fileInputStart,
    source.indexOf('<PromptInput', fileInputStart)
  )
  expect(fileInput).toContain("accept='.txt,.md")
  expect(fileInput).toContain('.pdf')
  expect(fileInput).toContain('.docx')
  expect(fileInput).toContain('.xlsx')
})
