import {defineConfig} from 'vite';
import {readFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';

const here = (p) => fileURLToPath(new URL(p, import.meta.url));
const APP = '\0mardo-app';
const RUNTIME = '\0mardo-runtime';

// `vite build --mode ios` makes the page for the iPhone app: where the page imports Wails' generated
// bindings it gets the bridge to the Go core instead (src/ios/bridge.js). The list of methods is read
// from the generated bindings, so the two cannot drift apart.
function iosBridge() {
    return {
        name: 'mardo-ios-bridge',
        enforce: 'pre',
        resolveId(id) {
            if (/wailsjs\/go\/main\/App$/.test(id)) return APP;
            if (/wailsjs\/runtime\/runtime$/.test(id)) return RUNTIME;
        },
        load(id) {
            const bridge = JSON.stringify(here('./src/ios/bridge.js'));
            if (id === RUNTIME) return `export * from ${bridge};`;
            if (id !== APP) return;
            const names = [...readFileSync(here('./wailsjs/go/main/App.js'), 'utf8').matchAll(/^export function (\w+)/gm)].map((m) => m[1]);
            return `import {call} from ${bridge};\n` + names.map((n) => `export const ${n} = (...args) => call(${JSON.stringify(n)}, args);`).join('\n');
        },
    };
}

export default defineConfig(({mode}) => (mode === 'ios'
    ? {base: './', plugins: [iosBridge()], build: {outDir: 'dist-ios', emptyOutDir: true}}
    : {}));
