import path from 'path';
import { createRequire } from 'module';
import { fileURLToPath } from 'url';
import { defineConfig, loadEnv } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const semiUiDir = path.resolve(
  path.dirname(require.resolve('@douyinfe/semi-ui')),
  '../..',
);
const semiFoundationDir = path.resolve(semiUiDir, '../semi-foundation');
const semiFoundationRequire = createRequire(
  path.join(semiFoundationDir, 'package.json'),
);
const dateFnsDir = path.dirname(
  semiFoundationRequire.resolve('date-fns/package.json'),
);
const dateFnsTzDir = path.dirname(
  semiFoundationRequire.resolve('date-fns-tz/package.json'),
);
const classicVChartDir = path.dirname(
  require.resolve('@visactor/vchart/package.json'),
);
const classicVChartRuntimeDir = path.join(classicVChartDir, 'node_modules');

export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ['VITE_'] });
  const clientServerUrl =
    process.env.VITE_REACT_APP_SERVER_URL ||
    env.rawPublicVars.VITE_REACT_APP_SERVER_URL ||
    '';
  const proxyServerUrl = clientServerUrl || 'http://localhost:3000';
  const isProd = envMode === 'production';
  const devProxy = Object.fromEntries(
    (['/api', '/mj', '/pg'] as const).map((key) => [
      key,
      { target: proxyServerUrl, changeOrigin: true },
    ]),
  ) as Record<string, { target: string; changeOrigin: boolean }>;

  return {
    plugins: [pluginReact()],
    source: {
      entry: {
        index: './src/index.jsx',
      },
      define: {
        'import.meta.env.VITE_REACT_APP_SERVER_URL':
          JSON.stringify(clientServerUrl),
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        '@douyinfe/semi-ui/dist/css/semi.css': path.resolve(
          semiUiDir,
          'dist/css/semi.css',
        ),
        'date-fns$': path.resolve(dateFnsDir, 'index.js'),
        'date-fns/': `${dateFnsDir}/`,
        'date-fns': dateFnsDir,
        'date-fns-tz': dateFnsTzDir,
        '@visactor/vchart$': classicVChartDir,
        '@visactor/vrender-core$': path.join(
          classicVChartRuntimeDir,
          '@visactor/vrender-core',
        ),
        '@visactor/vrender-kits$': path.join(
          classicVChartRuntimeDir,
          '@visactor/vrender-kits',
        ),
        '@visactor/vutils$': path.join(
          classicVChartRuntimeDir,
          '@visactor/vutils',
        ),
      },
    },
    html: {
      template: './index.html',
    },
    server: {
      host: '0.0.0.0',
      strictPort: false,
      historyApiFallback: true,
      proxy: devProxy,
    },
    output: {
      minify: isProd,
      target: 'web',
      distPath: {
        root: 'dist',
      },
    },
    performance: {
      removeConsole: isProd ? ['log'] : false,
      buildCache: {
        cacheDigest: [process.env.VITE_REACT_APP_VERSION],
      },
    },
    tools: {
      rspack: {
        module: {
          rules: [
            {
              test: /src[\\/].*\.js$/,
              type: 'javascript/auto',
              use: [
                {
                  loader: 'builtin:swc-loader',
                  options: {
                    jsc: {
                      parser: {
                        syntax: 'ecmascript',
                        jsx: true,
                      },
                      transform: {
                        react: {
                          runtime: 'automatic',
                          development: !isProd,
                          refresh: !isProd,
                        },
                      },
                    },
                  },
                },
              ],
            },
          ],
        },
      },
    },
  };
});
