import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(__dirname, '..')
const snowDist = path.join(root, 'web', 'snow', 'dist')

function walk(dir) {
  const out = []
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name)
    if (ent.isDirectory()) out.push(...walk(p))
    else out.push(p)
  }
  return out
}

if (!fs.existsSync(snowDist)) {
  console.error('missing web/snow/dist')
  process.exit(1)
}

const files = walk(snowDist).map((f) => path.relative(snowDist, f).replaceAll('\\', '/')).sort()
const totalBytes = files.reduce((sum, rel) => sum + fs.statSync(path.join(snowDist, rel)).size, 0)
const indexPath = path.join(snowDist, 'index.html')
const indexHtml = fs.readFileSync(indexPath, 'utf8')

const assetRefs = Array.from(indexHtml.matchAll(/(?:src|href)=["']([^"']+)["']/g)).map((m) => m[1])
const missing = []
for (const ref of assetRefs) {
  if (/^https?:\/\//i.test(ref) || ref.startsWith('data:')) continue
  let cleaned = ref.replace(/^\.\//, '')
  cleaned = cleaned.replace(/^\/snow-assets\//, '')
  cleaned = cleaned.replace(/^\//, '')
  const candidate = path.join(snowDist, cleaned)
  if (!fs.existsSync(candidate)) missing.push(ref)
}

// Scan main JS bundles for absolute external navigation that should stay same-origin.
const jsFiles = files.filter((f) => f.endsWith('.js'))
const externalNav = []
const corsDependentBranding = []
const allowedExternal = [/docs/i]
for (const rel of jsFiles) {
  const text = fs.readFileSync(path.join(snowDist, rel), 'utf8')
  const brandingWarningIndex = text.indexOf('Snow branding unavailable')
  if (brandingWarningIndex >= 0) {
    const brandingIntegration = text.slice(
      Math.max(0, brandingWarningIndex - 10_000),
      brandingWarningIndex
    )
    if (/\.crossOrigin\s*=\s*["']anonymous["']/.test(brandingIntegration)) {
      corsDependentBranding.push(rel)
    }
  }
  const urls = text.match(/https?:\/\/[a-zA-Z0-9._\-\/]+/g) || []
  for (const u of urls) {
    if (allowedExternal.some((re) => re.test(u))) continue
    // ignore source maps / known CDN asset hosts if any; flag navigation-like domains
    if (/github|google|twitter|facebook|linkedin|vercel|netlify/i.test(u)) {
      externalNav.push({ file: rel, url: u })
    }
  }
}

const report = {
  fileCount: files.length,
  totalBytes,
  entry: 'index.html',
  assetRefs: assetRefs.length,
  missing,
  externalNavSample: externalNav.slice(0, 20),
  corsDependentBranding,
  files,
}

console.log(JSON.stringify({
  fileCount: report.fileCount,
  totalBytes: report.totalBytes,
  assetRefs: report.assetRefs,
  missing: report.missing,
  externalNavCount: externalNav.length,
  corsDependentBranding: report.corsDependentBranding,
}, null, 2))

if (missing.length) {
  console.error('missing referenced assets:', missing)
  process.exit(2)
}

// Soft expectation around vendored package size; do not hard-fail if slightly different.
if (files.length < 50) {
  console.error(`expected a full vendored snow package, found only ${files.length} files`)
  process.exit(3)
}

if (corsDependentBranding.length) {
  console.error(
    'Snow branding must not require remote logo hosts to provide CORS headers:',
    corsDependentBranding
  )
  process.exit(4)
}

console.log('snow assets verification passed')
