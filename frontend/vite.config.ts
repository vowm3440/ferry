import { defineConfig, type Plugin } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// 不使用 @wailsio/runtime/plugins/vite：该插件依赖 wailsbindings 生成的 bindings 目录，
// 当前前端通过运行时 Call.ByName 调用后端，待主线程生成 bindings 后再启用。
/**
 * 内容安全策略（只写进构建产物；开发服务器的热更新需要更宽的权限，不加）。
 * 界面里没有任何注入点，这是第二道防线：即使将来出现一个，注入的脚本也执行不了，
 * 页面也连不到本机桥接以外的任何地方。
 * - 脚本只能来自应用自身，禁止内联与 eval；
 * - 样式允许内联（Svelte 模板里的 style 属性需要）；
 * - 图片允许 data:（二维码是后端生成的 SVG，以 data: 地址放进 <img>）；
 * - 网络请求只能发往同源（Wails 桥接 /wails/runtime）。
 */
const CONTENT_SECURITY_POLICY = [
  "default-src 'none'",
  "script-src 'self'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data:",
  "connect-src 'self'",
  "base-uri 'none'",
  "form-action 'none'",
].join('; ');

function contentSecurityPolicy(): Plugin {
  return {
    name: 'ferry-csp',
    apply: 'build',
    transformIndexHtml: () => [
      { tag: 'meta', attrs: { 'http-equiv': 'Content-Security-Policy', content: CONTENT_SECURITY_POLICY }, injectTo: 'head-prepend' },
    ],
  };
}

export default defineConfig({
  plugins: [svelte(), contentSecurityPolicy()],
  // Wails 的 AssetFileServerFS 从根路径提供静态资源，保持默认 base。
  base: '/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'chrome120',
    sourcemap: false,
    // 目标浏览器原生支持 modulepreload；不带 polyfill，产物里就没有任何自发的 fetch。
    modulePreload: { polyfill: false },
  },
  server: {
    host: '127.0.0.1',
    port: 9245,
    strictPort: true,
  },
});
