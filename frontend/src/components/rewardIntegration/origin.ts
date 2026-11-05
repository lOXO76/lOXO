/** 文档示例里的本站地址：始终取当前站点 origin，避免文档与部署环境不一致。 */
export function currentOrigin(): string {
  return window.location.origin.replace(/\/+$/, '')
}
