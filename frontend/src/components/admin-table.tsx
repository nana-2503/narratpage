import { cn } from '@/lib/utils'

/** 表格复选框（受控） */
export function TableCheckbox({
  checked,
  onChange,
  label,
  indeterminate,
}: {
  checked: boolean
  onChange: () => void
  label: string
  indeterminate?: boolean
}) {
  return (
    <input
      type="checkbox"
      checked={checked}
      onChange={onChange}
      aria-label={label}
      ref={(el) => {
        if (el) el.indeterminate = Boolean(indeterminate) && !checked
      }}
      className={cn('size-3.5 cursor-pointer accent-foreground')}
    />
  )
}

/** 表格容器：统一边框与横向滚动 */
export function TableWrap({ children }: { children: React.ReactNode }) {
  return (
    <div className="mt-4 hidden overflow-hidden rounded-md border border-border md:block">
      {children}
    </div>
  )
}

/** 移动端列表容器 */
export function ListWrap({ children }: { children: React.ReactNode }) {
  return <ul className="mt-4 overflow-hidden rounded-md border border-border md:hidden">{children}</ul>
}
