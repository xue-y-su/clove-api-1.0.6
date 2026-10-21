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
import { readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import ts from 'typescript'

const sourceDirectory = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..'
)
const localesDirectory = path.join(sourceDirectory, 'i18n', 'locales')
const localeFiles = [
  'en.json',
  'fr.json',
  'ja.json',
  'ru.json',
  'vi.json',
  'zh-TW.json',
  'zh.json',
]

function collectSourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      return entry.name === 'locales' ? [] : collectSourceFiles(fullPath)
    }
    if (
      !/\.(ts|tsx)$/.test(entry.name) ||
      /\.test\.(ts|tsx)$/.test(entry.name)
    ) {
      return []
    }
    return [fullPath]
  })
}

function collectLiteralTranslationKeys(): string[] {
  const keys = new Set<string>()

  for (const filePath of collectSourceFiles(sourceDirectory)) {
    const source = readFileSync(filePath, 'utf8')
    const sourceFile = ts.createSourceFile(
      filePath,
      source,
      ts.ScriptTarget.Latest,
      true,
      filePath.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS
    )

    const visit = (node: ts.Node) => {
      if (
        ts.isCallExpression(node) &&
        ts.isIdentifier(node.expression) &&
        node.expression.text === 't'
      ) {
        const argument = node.arguments[0]
        if (
          argument &&
          (ts.isStringLiteralLike(argument) ||
            ts.isNoSubstitutionTemplateLiteral(argument))
        ) {
          keys.add(argument.text)
        }
      }
      ts.forEachChild(node, visit)
    }

    visit(sourceFile)
  }

  return [...keys].sort()
}

test('every literal translation key exists in every production locale', () => {
  const keys = collectLiteralTranslationKeys()

  for (const localeFile of localeFiles) {
    const locale = JSON.parse(
      readFileSync(path.join(localesDirectory, localeFile), 'utf8')
    ) as { translation: Record<string, string> }
    const missing = keys.filter((key) => !locale.translation[key]?.trim())

    expect({ localeFile, missing }).toEqual({ localeFile, missing: [] })
  }
}, 60_000)
