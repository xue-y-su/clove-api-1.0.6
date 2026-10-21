import { readFileSync } from 'node:fs'

const expectedVersion = 'v1.0.6'
const actualVersion = readFileSync(new URL('./VERSION', import.meta.url), 'utf8').trim()

if (actualVersion !== expectedVersion) {
  console.error(
    `Expected VERSION to be ${expectedVersion}, got ${actualVersion || '<empty>'}`,
  )
  process.exit(1)
}
