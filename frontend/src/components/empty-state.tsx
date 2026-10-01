import { cn } from '@/lib/utils'

/**
 * 空状态占位。
 * 只渲染数据态（图标 + 提示），不写解释性长文案。
 */
export function EmptyState({
  icon: Icon,
  message,
  action,
  className,
}: {
  icon?: React.ComponentType<{ className?: string }>
  message: string
  action?: React.ReactNode
  className?: string
}) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-2 px-3 py-10 text-center',
        className,
      )}
    >
      {Icon && <Icon className="size-5 text-muted-foreground" />}
      <p className="text-sm text-muted-foreground">{message}</p>
      {action}
    </div>
  )
}
