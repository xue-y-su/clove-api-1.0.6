import assert from 'node:assert/strict'
import { existsSync, readFileSync, statSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { gzipSync } from 'node:zlib'

const DEFAULT_MAX_RAW_BYTES = 10 * 1024 * 1024
const DEFAULT_MAX_GZIP_BYTES = 1024 * 1024
const DEVTOOLS_MARKERS = [
  'TanStackRouterDevtools',
  'ReactQueryDevtools',
  '@tanstack/react-query-devtools',
  '@tanstack/react-router-devtools',
  'TANSTACK',
]

function extractInitialScriptSources(html) {
  const sources = []
  const scriptPattern = /<script\b[^>]*\bsrc\s*=\s*(["'])(.*?)\1[^>]*>/gi
  for (const match of html.matchAll(scriptPattern)) {
    const source = match[2].trim()
    if (
      !source ||
      source.startsWith('//') ||
      /^[a-z][a-z\d+.-]*:/i.test(source)
    ) {
      continue
    }
    sources.push(source)
  }
  return [...new Set(sources)]
}

function resolveAssetPath(distDir, source) {
  const pathname = decodeURIComponent(source.split(/[?#]/, 1)[0])
  const relativePath = pathname.replace(/^[/\\]+/, '')
  const absolutePath = path.resolve(distDir, relativePath)
  const relativeToDist = path.relative(distDir, absolutePath)
  assert(
    relativeToDist &&
      !relativeToDist.startsWith('..') &&
      !path.isAbsolute(relativeToDist),
    `Initial asset escapes dist directory: ${source}`
  )
  assert(existsSync(absolutePath), `Initial asset is missing: ${source}`)
  assert(statSync(absolutePath).isFile(), `Initial asset is not a file: ${source}`)
  return { absolutePath, relativePath: relativeToDist.replaceAll('\\', '/') }
}

export function verifyProductionBundles(distPath, options = {}) {
  const distDir = path.resolve(distPath)
  const indexPath = path.join(distDir, 'index.html')
  assert(existsSync(indexPath), `Production index is missing: ${indexPath}`)

  const html = readFileSync(indexPath, 'utf8')
  const initialAssets = extractInitialScriptSources(html).map((source) =>
    resolveAssetPath(distDir, source)
  )
  assert(initialAssets.length > 0, 'Production index has no initial JavaScript')

  let initialRawBytes = 0
  let initialGzipBytes = 0
  let bundleText = ''
  for (const asset of initialAssets) {
    const content = readFileSync(asset.absolutePath)
    initialRawBytes += content.byteLength
    initialGzipBytes += gzipSync(content, { level: 9 }).byteLength
    bundleText += content.toString('utf8')
  }

  const maxRawBytes = options.maxRawBytes ?? DEFAULT_MAX_RAW_BYTES
  const maxGzipBytes = options.maxGzipBytes ?? DEFAULT_MAX_GZIP_BYTES
  assert(
    initialRawBytes < maxRawBytes,
    `Initial raw JavaScript is ${initialRawBytes} bytes; limit is below ${maxRawBytes} bytes`
  )
  assert(
    initialGzipBytes < maxGzipBytes,
    `Initial gzip JavaScript is ${initialGzipBytes} bytes; limit is below ${maxGzipBytes} bytes`
  )

  for (const marker of DEVTOOLS_MARKERS) {
    assert(!bundleText.includes(marker), `Production bundle contains ${marker}`)
  }

  return {
    initialAssets: initialAssets.map((asset) => asset.relativePath),
    initialGzipBytes,
    initialRawBytes,
  }
}

const invokedPath = process.argv[1] ? pathToFileURL(path.resolve(process.argv[1])) : null
if (invokedPath?.href === import.meta.url) {
  try {
    const report = verifyProductionBundles(process.argv[2] || 'dist')
    console.log(
      `Production bundles verified: ${report.initialAssets.length} initial scripts, ${report.initialRawBytes} raw bytes, ${report.initialGzipBytes} gzip bytes`
    )
  } catch (error) {
    console.error(error instanceof Error ? error.message : error)
    process.exitCode = 1
  }
}
