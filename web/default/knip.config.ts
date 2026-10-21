import type { KnipConfig } from 'knip'

const config: KnipConfig = {
  entry: ['src/main.tsx', 'src/checks/*.mjs', 'src/**/*.d.ts'],
  ignore: ['src/components/ui/**'],
  include: [
    'files',
    'dependencies',
    'devDependencies',
    'optionalPeerDependencies',
    'unlisted',
    'binaries',
    'unresolved',
    'duplicates',
    'catalog',
  ],
}

export default config
