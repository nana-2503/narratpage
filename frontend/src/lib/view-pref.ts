// 博客列表的视图偏好（列表 / 卡片）。
//
// 属于「本设备上的显示偏好」，与主题外观同源，因此也存 localStorage。
// 只在挂载时读一次，之后由组件内 state 维护，避免 SSR/首帧闪烁。

const KEY = 'narrat-post-view'

export type PostView = 'list' | 'card'

export const DEFAULT_VIEW: PostView = 'list'

export function loadView(): PostView {
  try {
    return localStorage.getItem(KEY) === 'card' ? 'card' : DEFAULT_VIEW
  } catch {
    return DEFAULT_VIEW // 隐私模式等 localStorage 不可用场景
  }
}

export function saveView(view: PostView) {
  try {
    localStorage.setItem(KEY, view)
  } catch {
    /* 持久化失败不影响本次生效 */
  }
}