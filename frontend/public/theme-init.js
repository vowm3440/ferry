// 在样式加载前确定主题，避免首帧闪烁。与 src/lib/stores/theme.svelte.ts 使用同一键名。
// 单独成文件而不是内联在 index.html 里：这样内容安全策略可以禁止一切内联脚本。
(function () {
  var stored = null;
  try {
    stored = window.localStorage.getItem('ferry.theme');
  } catch (error) {
    stored = null;
  }
  var dark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  var resolved = stored === 'dark' || stored === 'light' ? stored : dark ? 'dark' : 'light';
  document.documentElement.dataset.theme = resolved;
})();
