import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const assetsPath = path.resolve(
  __dirname,
  '..',
  'web',
  'snow',
  'dist',
  'assets'
)
const appBundleNames = fs
  .readdirSync(assetsPath)
  .filter((name) => /^App3D-[a-z0-9]+\.js$/.test(name))

if (appBundleNames.length !== 1) {
  throw new Error(
    `expected one Snow App3D bundle, found ${appBundleNames.length}`
  )
}

const bundlePath = path.join(assetsPath, appBundleNames[0])
const corsDependentLoader = 't.crossOrigin="anonymous",t.onload=()=>i(t)'
const regularImageLoader = 't.onload=()=>i(t)'

const source = fs.readFileSync(bundlePath, 'utf8')
const occurrenceCount = source.split(corsDependentLoader).length - 1

if (occurrenceCount === 0 && source.includes(regularImageLoader)) {
  console.log('Snow branding patch already applied')
  process.exit(0)
}

if (occurrenceCount !== 1) {
  throw new Error(
    `expected one Snow branding image loader, found ${occurrenceCount}`
  )
}

fs.writeFileSync(
  bundlePath,
  source.replace(corsDependentLoader, regularImageLoader)
)
console.log('Snow branding patch applied')
