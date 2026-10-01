import { cn } from '@/lib/utils'

/**
 * 后台页面统一的标题区。
 * 标题 + 计数 + 右侧操作，避免每页重复写 flex 布局。
 */
export function PageHeader({
  title,
  count,
  actions,
  className,
}: {
  title: string
  count?: number
  actions?: React.ReactNode
  className?: string
}) {
  return (
    <div className={cn('flex flex-wrap items-center justify-between gap-2', className)}>
      <h1 className="flex items-baseline gap-1.5 text-sm font-semibold">
        {title}
        {typeof count === 'number' && (
          <span className="text-xs font-normal text-muted-foreground tabular-nums">{count}</span>
        )}
      </h1>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}
