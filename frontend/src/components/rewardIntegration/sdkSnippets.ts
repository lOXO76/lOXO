// 浏览器 SDK 的接入片段（放在 .ts 里，避免 SFC 解析器把片段里的 </script> 当成块结束）

export function buildSdkInstallSnippet(origin: string): string {
  const base = origin.replace(/\/+$/, '')
  return [
    '<!-- 在需要计时的页面引入一次；src 必须指向本站 -->',
    `<script src="${base}/reward-task-sdk.js" data-app-key="rk_0123456789abcdef01234567" async></script>`,
    '',
    '<!-- 可选属性：关闭右下角进度角标、指定角标语言（zh / en，默认跟随浏览器语言） -->',
    `<!-- <script src="${base}/reward-task-sdk.js" data-app-key="rk_0123456789abcdef01234567" data-badge="false" data-lang="zh" async></script> -->`,
  ].join('\n')
}

export function buildSdkCustomUiSnippet(origin: string): string {
  const base = origin.replace(/\/+$/, '')
  return [
    '// 1) 引入 SDK 时用 data-badge="false" 关闭默认角标',
    `// <script src="${base}/reward-task-sdk.js" data-app-key="rk_0123456789abcdef01234567" data-badge="false" async></script>`,
    '',
    "const hint = document.getElementById('reward-hint')",
    '',
    'window.RewardTaskSDK.onChange(function (state) {',
    '  if (state.completed) {',
    "    hint.textContent = '任务完成，奖励已发放到你的账户 ✓'",
    '    return',
    '  }',
    '',
    "  if (state.status === 'paused') {",
    '    // paused_reason: "idle"（无操作）或 "hidden"（页面不可见）',
    "    hint.textContent = state.paused_reason === 'idle'",
    "      ? '检测到你没有在浏览，计时已暂停（' + state.progress_seconds + ' / ' + state.browse_seconds + ' 秒）。滚动页面或移动鼠标即可继续。'",
    "      : '页面不可见，计时已暂停'",
    '    return',
    '  }',
    '',
    "  hint.textContent = '浏览任务进行中 ' + state.progress_seconds + ' / ' + state.browse_seconds + ' 秒'",
    '})',
    '',
    '// 也可以随时主动读取当前状态',
    'const current = window.RewardTaskSDK.getState()',
  ].join('\n')
}
