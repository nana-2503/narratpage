import { createContext, useContext, useRef, useState } from 'react'
import { cn } from '@/lib/utils'

interface TooltipContextValue {
  delayDuration: number
}

const TooltipContext = createContext<TooltipContextValue | null>(null)

function useTooltip(): TooltipContextValue {
  const ctx = useContext(TooltipContext)
  if (!ctx) throw new Error('Tooltip 组件必须在 TooltipProvider 内使用')
  return ctx
}

export function TooltipProvider({
  children,
  delayDuration = 200,
}: {
  children: React.ReactNode
  delayDuration?: number
}) {
  return <TooltipContext.Provider value={{ delayDuration }}>{children}</TooltipContext.Provider>
}

/**
 * 轻量 Tooltip：悬停或聚焦后延迟显示，用于展示被截断的完整值。
 *
 * 需求仅为「显示一行文本」，故自实现而不引入 Radix：
 * 避免额外定位依赖，并能精确控制延迟。
 */
export function Tooltip({
  children,
  content,
  side = 'top',
}: {
  children: React.ReactNode
  content: React.ReactNode
  side?: 'top' | 'bottom'
}) {
  const { delayDuration } = useTooltip()
  const [open, setOpen] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const show = () => {
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => setOpen(true), delayDuration)
  }
  const hide = () => {
    if (timer.current) clearTimeout(timer.current)
    setOpen(false)
  }

  if (!content) return <>{children}</>

  return (
    <span
      className="relative inline-flex"
      onMouseEnter={show}
      onMouseLeave={hide}
      onFocus={show}
      onBlur={hide}
    >
      {children}
      {open && (
        <span
          role="tooltip"
          className={cn(
            'pointer-events-none absolute left-1/2 z-50 w-max max-w-64 -translate-x-1/2 rounded-sm border border-border bg-popover px-2 py-1 text-xs text-popover-foreground',
            side === 'top' ? 'bottom-full mb-1' : 'top-full mt-1',
          )}
        >
          {content}
        </span>
      )}
    </span>
  )
}

/** 始终带 title 的截断文本：满足「单行不换行 + 悬停可见完整值」 */
export function Truncate({ children, className }: { children: string; className?: string }) {
  return (
    <span className={cn('block truncate', className)} title={children}>
      {children}
    </span>
  )
}
