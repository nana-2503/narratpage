import { useCallback, useMemo, useState } from 'react'

/**
 * 表格多选状态。
 *
 * 后台列表（文章、评论）都需要批量操作，
 * 抽出选择逻辑避免每页重复，且统一「全选/反选/清空」语义。
 */
export function useSelection<T>(items: T[], getKey: (item: T) => number | string) {
  const [selected, setSelected] = useState<Set<string | number>>(() => new Set())

  const allKeys = useMemo(() => items.map(getKey), [items, getKey])
  const allSelected = allKeys.length > 0 && allKeys.every((k) => selected.has(k))

  const toggle = useCallback((key: number | string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }, [])

  const toggleAll = useCallback(() => {
    setSelected((prev) => {
      if (allKeys.length > 0 && allKeys.every((k) => prev.has(k))) {
        // 全部已选时取消全选
        return new Set<string | number>()
      }
      return new Set(allKeys)
    })
  }, [allKeys])

  const clear = useCallback(() => setSelected(new Set()), [])

  const isSelected = useCallback((key: number | string) => selected.has(key), [selected])

  /** 返回数字 ID 列表，便于直接提交给批量接口 */
  const selectedIds = useCallback(
    () =>
      [...selected]
        .map((k) => Number(k))
        .filter((n) => !Number.isNaN(n)),
    [selected],
  )

  return { selected, selectedIds, isSelected, toggle, toggleAll, clear, allSelected }
}
