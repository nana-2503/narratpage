import { useEffect, useRef, useState } from 'react'

export interface AsyncState<T> {
  data: T | null
  error: Error | null
  loading: boolean
}

/**
 * 通用异步数据加载 hook：自动管理 loading/error、请求竞态与重新加载。
 *
 * - fetcher 经 ref 读取，调用方无需 useCallback 包住；
 * - deps 为依赖数组，长度须保持稳定（同 useEffect 规则）；
 * - deps 变化或组件卸载后返回的旧响应会被丢弃（竞态保护）；
 * - deps 变化重取时继续展示旧数据（stale-while-revalidate），
 *   loading 仅表示首次加载与 reload() 期间；
 * - reload() 供事件处理（删除、审核等）后重新拉取。
 */
export function useAsync<T>(
  fetcher: () => Promise<T>,
  deps: unknown[],
): AsyncState<T> & { reload: () => void } {
  const fetcherRef = useRef(fetcher)
  const [attempt, setAttempt] = useState(0)
  const [state, setState] = useState<AsyncState<T>>({
    data: null,
    error: null,
    loading: true,
  })

  // 每次渲染后把最新 fetcher 同步进 ref（避免渲染期写 ref）；
  // 必须声明在下面的数据 effect 之前，保证其先执行
  useEffect(() => {
    fetcherRef.current = fetcher
  })

  useEffect(() => {
    let ignore = false
    fetcherRef.current()
      .then((data) => {
        if (!ignore) setState({ data, error: null, loading: false })
      })
      .catch((error: unknown) => {
        if (!ignore) setState({ data: null, error: error as Error, loading: false })
      })
    return () => {
      ignore = true
    }
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- deps 由调用方显式提供（转发型 hook 的标准写法）
  }, [...deps, attempt])

  const reload = () => {
    setState((s) => ({ ...s, loading: true }))
    setAttempt((n) => n + 1)
  }

  return { ...state, reload }
}
