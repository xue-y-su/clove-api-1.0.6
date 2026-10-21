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
export function normalizeAnnouncementContent(content = ''): string {
  const lines = String(content).split('\n')
  let fenceMarker = ''

  for (let index = 0; index < lines.length; index += 1) {
    const fence = lines[index].match(/^\s{0,3}(`{3,}|~{3,})/)
    if (fence) {
      if (!fenceMarker) {
        fenceMarker = fence[1]
      } else if (
        fence[1][0] === fenceMarker[0] &&
        fence[1].length >= fenceMarker.length
      ) {
        fenceMarker = ''
      }
      continue
    }
    if (fenceMarker) continue

    const htmlStart = lines[index].match(
      /^( {4,}|\t+)(?:<([A-Za-z][\w:-]*)(?=\s|\/?>|$)|(?=<!--))/
    )
    if (!htmlStart) continue

    const indentation = htmlStart[1]
    const rootTag = htmlStart[2]
    const rootClosingTag = rootTag
      ? new RegExp(`^</${rootTag}\\s*>`, 'i')
      : /^-->/
    const inlineClosingTag = rootTag
      ? new RegExp(`</${rootTag}\\s*>`, 'i')
      : /-->/
    const singleLineTag = rootTag
      ? /^(?:area|base|br|col|embed|hr|img|input|link|meta|param|source|track|wbr)$/i.test(
          rootTag
        ) || /\/\s*>\s*$/.test(lines[index])
      : false
    let blockEnd = index
    while (blockEnd < lines.length) {
      const line = lines[blockEnd]
      if (line.trim() && !line.startsWith(indentation)) break
      if (line.startsWith(indentation)) {
        const normalizedLine = line.slice(indentation.length)
        lines[blockEnd] = normalizedLine
        if (
          (blockEnd === index &&
            (singleLineTag || inlineClosingTag.test(normalizedLine))) ||
          rootClosingTag.test(normalizedLine)
        ) {
          blockEnd += 1
          break
        }
      }
      blockEnd += 1
    }
    index = blockEnd - 1
  }

  return lines.join('\n')
}
