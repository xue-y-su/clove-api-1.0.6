import assert from 'node:assert/strict'
import path from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { verifyProductionBundles } from './verify-production-bundles.mjs'

const fixtureRoot = fileURLToPath(
  new URL('./testdata/verify-production-bundles/', import.meta.url)
)

function fixture(name) {
  return path.join(fixtureRoot, name)
}

test('counts initial scripts and ignores unreferenced lazy chunks', () => {
  const report = verifyProductionBundles(fixture('valid'), {
    maxGzipBytes: 1_000,
    maxRawBytes: 1_000,
  })

  assert.deepEqual(report.initialAssets, ['static/js/index.js'])
  assert.equal(report.initialRawBytes, 22)
  assert.ok(report.initialGzipBytes < 1_000)
})

test('rejects oversized initial scripts', () => {
  assert.throws(
    () =>
      verifyProductionBundles(fixture('oversized'), {
        maxGzipBytes: 1_000,
        maxRawBytes: 100,
      }),
    /initial raw JavaScript/i
  )
})

test('rejects production Devtools markers', () => {
  assert.throws(
    () => verifyProductionBundles(fixture('devtools-component')),
    /TanStackRouterDevtools/
  )
})

test('rejects Devtools package names and TANSTACK controls', () => {
  assert.throws(
    () => verifyProductionBundles(fixture('devtools-package')),
    /@tanstack\/react-query-devtools/
  )
  assert.throws(
    () => verifyProductionBundles(fixture('devtools-control')),
    /TANSTACK/
  )
})
