import { defineConfig } from 'vite'
import manifest from './plugin.json' with { type: 'json' }

/**
 * Builds the plugin as one IIFE script, `dist/index.js`, next to a copy of
 * `plugin.json` — the folder to install under $KUBEBENCH_PLUGINS_DIR.
 *
 * React is not bundled: `react` and `react/jsx-runtime` are mapped to the
 * globals KubeBench provides, so the plugin renders with the app's React.
 */
export default defineConfig({
  oxc: { jsx: { runtime: 'automatic' } },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    lib: {
      entry: 'src/index.tsx',
      formats: ['iife'],
      name: 'KubeBenchHelloCluster',
      fileName: () => manifest.entry,
    },
    rolldownOptions: {
      external: ['react', 'react/jsx-runtime'],
      output: {
        globals: {
          react: 'KubeBench.React',
          'react/jsx-runtime': 'KubeBench.jsxRuntime',
        },
      },
    },
  },
  plugins: [
    {
      name: 'kubebench-plugin-manifest',
      generateBundle() {
        this.emitFile({ type: 'asset', fileName: 'plugin.json', source: `${JSON.stringify(manifest, null, 2)}\n` })
      },
    },
  ],
})
