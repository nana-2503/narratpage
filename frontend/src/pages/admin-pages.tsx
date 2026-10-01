import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { FileText, Plus, Trash2 } from 'lucide-react'
import { api, type Post } from '@/lib/api'
import { formatDate } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { useAuth } from '@/hooks/use-auth'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { PageHeader } from '@/components/admin-page-header'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { statusLabel, statusVariant } from '@/pages/admin-posts'

/** 页面管理：与文章同表不同 type，列表按菜单顺序展示 */
export default function AdminPages() {
  const navigate = useNavigate()
  const { can } = useAuth()
  const [pendingDelete, setPendingDelete] = useState<Post | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  const { data, error, loading } = useAsync(
    () => api.listPages({ page: 1, pageSize: 100, status: 'all', order: 'meta' }),
    [reloadKey],
  )

  const pages = data?.items ?? []
  const refresh = () => setReloadKey((k) => k + 1)

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deletePost(pendingDelete.id)
      toast.success('已删除')
      setPendingDelete(null)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const move = async (page: Post, up: boolean) => {
    try {
      await api.updatePost(page.id, {
        menu_order: page.menu_order + (up ? -1 : 1),
      } as never)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader
        title="页面"
        count={pages.length}
        actions={
          can('page.manage') && (
            <Button size="sm" onClick={() => navigate('/admin/pages/new')}>
              <Plus className="size-4" />
              新建
            </Button>
          )
        }
      />

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {pages.length === 0 && !loading && (
        <EmptyState icon={FileText} message="暂无页面" />
      )}

      {pages.length > 0 && (
        <ul className="mt-4 overflow-hidden rounded-md border border-border">
          {pages.map((p, i) => (
            <li
              key={p.id}
              className={`flex items-center gap-3 px-3 py-2.5 ${
                i > 0 ? 'border-t border-border' : ''
              }`}
            >
              <span className="w-6 shrink-0 text-right text-xs text-muted-foreground tabular-nums">
                {i + 1}
              </span>
              <button
                onClick={() => navigate(`/admin/pages/${p.id}`)}
                className="min-w-0 flex-1 truncate text-left text-sm font-medium hover:underline"
                title={p.title}
              >
                {p.title}
              </button>
              <Badge variant={statusVariant(p.status)} className="shrink-0">
                {statusLabel(p.status)}
              </Badge>
              <span className="hidden shrink-0 text-xs text-muted-foreground tabular-nums sm:inline">
                {formatDate(p.updated_at)}
              </span>
              <span className="flex shrink-0 items-center">
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-7"
                  disabled={i === 0}
                  onClick={() => void move(p, true)}
                  aria-label={`上移 ${p.title}`}
                >
                  ↑
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-7"
                  disabled={i === pages.length - 1}
                  onClick={() => void move(p, false)}
                  aria-label={`下移 ${p.title}`}
                >
                  ↓
                </Button>
                {can('page.manage') && (
                  <Button
                    size="icon"
                    variant="ghost"
                    className="size-7 text-destructive"
                    onClick={() => setPendingDelete(p)}
                    aria-label={`删除 ${p.title}`}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                )}
              </span>
            </li>
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="删除页面"
        description={`删除后不可恢复，确定删除「${pendingDelete?.title}」？`}
        onConfirm={remove}
      />
    </div>
  )
}
