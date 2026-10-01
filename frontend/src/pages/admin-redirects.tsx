import { useState } from 'react'
import { ArrowRight, Plus, Trash2 } from 'lucide-react'
import { api, type Redirect } from '@/lib/api'
import { formatDate } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageHeader } from '@/components/admin-page-header'
import { EmptyState } from '@/components/empty-state'
import { ConfirmDialog } from '@/components/confirm-dialog'

/** 重定向规则：改 slug 或迁移路径时保住外链 */
export default function AdminRedirects() {
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [pendingDelete, setPendingDelete] = useState<Redirect | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  const { data, error, loading } = useAsync(() => api.listRedirects(), [reloadKey])
  const items = data?.items ?? []
  const refresh = () => setReloadKey((k) => k + 1)

  const add = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!from.trim() || !to.trim()) return
    try {
      await api.createRedirect(from.trim(), to.trim())
      setFrom('')
      setTo('')
      toast.success('已添加')
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deleteRedirect(pendingDelete.id)
      toast.success('已删除')
      setPendingDelete(null)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader title="重定向" count={items.length} />

      <form onSubmit={add} className="mt-4 flex flex-wrap items-end gap-2">
        <div className="flex flex-col gap-1.5">
          <label htmlFor="r-from" className="text-xs text-muted-foreground">
            来源路径
          </label>
          <Input
            id="r-from"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            placeholder="/old-path"
            className="w-44"
          />
        </div>
        <ArrowRight className="mb-2.5 size-4 shrink-0 text-muted-foreground" />
        <div className="flex flex-col gap-1.5">
          <label htmlFor="r-to" className="text-xs text-muted-foreground">
            目标路径
          </label>
          <Input
            id="r-to"
            value={to}
            onChange={(e) => setTo(e.target.value)}
            placeholder="/post/new-path"
            className="w-44"
          />
        </div>
        <Button type="submit" variant="outline">
          <Plus className="size-4" />
          添加
        </Button>
      </form>

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {items.length === 0 && !loading && (
        <EmptyState icon={ArrowRight} message="暂无重定向规则" />
      )}

      {items.length > 0 && (
        <ul className="mt-4 overflow-hidden rounded-md border border-border">
          {items.map((r, i) => (
            <li
              key={r.id}
              className={`flex items-center gap-2 px-3 py-2.5 ${
                i > 0 ? 'border-t border-border' : ''
              }`}
            >
              <code className="min-w-0 flex-1 truncate text-xs" title={r.from_path}>
                {r.from_path}
              </code>
              <ArrowRight className="size-3 shrink-0 text-muted-foreground" />
              <code className="min-w-0 flex-1 truncate text-xs" title={r.to_path}>
                {r.to_path}
              </code>
              <span className="hidden shrink-0 text-xs text-muted-foreground tabular-nums sm:inline">
                {r.hits} 次 · {formatDate(r.created_at)}
              </span>
              <Button
                size="icon"
                variant="ghost"
                className="size-7 text-destructive"
                onClick={() => setPendingDelete(r)}
                aria-label={`删除 ${r.from_path} 的重定向`}
              >
                <Trash2 className="size-4" />
              </Button>
            </li>
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="删除重定向"
        description={`删除后 ${pendingDelete?.from_path} 将不再跳转。`}
        onConfirm={remove}
      />
    </div>
  )
}
