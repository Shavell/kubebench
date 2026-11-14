import { readFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'

const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8')) as {
  version: string
}

/** Backend the dev server proxies to (`npm run server:go` listens on :9876). */
const backend = process.env.KUBEBENCH_DEV_BACKEND ?? 'http://localhost:9876'

/** Public CDNs. KubeBench must work offline, so no emitted file may reference them. */
const CDN_HOSTS = /cdn\.jsdelivr\.net|unpkg\.com|cdnjs\.cloudflare\.com/

/**
 * @monaco-editor/loader keeps a jsDelivr URL as the default path of its AMD
 * fallback. The app hands it the locally bundled Monaco instead
 * (`loader.config({ monaco })` in src/shared/editor/monaco.ts), so that URL
 * is dead code; blank it so offline bundles carry no CDN reference at all.
 */
function dropMonacoLoaderCdnPath(): Plugin {
  return {
    name: 'kubebench:drop-monaco-loader-cdn-path',
    transform: {
      filter: { id: /[\\/]@monaco-editor[\\/]loader[\\/]/ },
      handler(code) {
        const stripped = code.replace(/https:\/\/cdn\.jsdelivr\.net\/npm\/monaco-editor@[^'"`]*/g, '')
        return stripped === code ? null : { code: stripped, map: null }
      },
    },
  }
}

/** Fails the production build when an emitted text file mentions a public CDN. */
function forbidCdnReferences(): Plugin {
  return {
    name: 'kubebench:forbid-cdn-references',
    apply: 'build',
    generateBundle(_options, bundle) {
      for (const file of Object.values(bundle)) {
        if (!/\.(m?js|css|html|json|svg)$/.test(file.fileName)) continue
        const text =
          file.type === 'chunk'
            ? file.code
            : typeof file.source === 'string'
              ? file.source
              : new TextDecoder().decode(file.source)
        const host = CDN_HOSTS.exec(text)?.[0]
        if (host) {
          this.error(`${file.fileName} references ${host}; bundle the resource locally (offline installs)`)
        }
      }
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), dropMonacoLoaderCdnPath(), forbidCdnReferences()],
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // No `changeOrigin`: the backend rejects mutations whose Origin does not
    // match the Host header (CSRF check), so the proxy must keep the
    // browser's Host.
    proxy: {
      // `ws` carries the exec terminal WebSocket; SSE streams pass through as-is.
      '/api': { target: backend, ws: true },
      // Files of external (runtime-loaded) plugins, design §6.8.
      '/plugins': backend,
    },
  },
})
