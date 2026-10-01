import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { RotateCcw, Trash2, TrashIcon } from 'lucide-react'
import { api, type Post } from '@/lib/api'
import { formatRelative } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { PageHeader } from '@/components/admin-page-header'
import { EmptyState } from '@/components/empty-state'
import { ConfirmDialog } from '@/components/confirm-dialog'

/** 回收站：误删内容可在此恢复或彻底清除 */
export default function AdminTrash() {
  const navigate = useNavigate()
  const [pendingPurge, setPendingPurge] = useState(false)
  const [pendingDelete, setPendingDelete] = useState<Post | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  const { data, error, loading } = useAsync(
    () => api.listPosts({ status: 'trash', pageSize: 100 }),
    [reloadKey],
  )

  const items = data?.items ?? []
  const refresh = () => setReloadKey((k) => k + 1)

  const restore = async (post: Post) => {
    try {
      await api.restorePost(post.id)
      toast.success('已恢复为草稿')
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deletePost(pendingDelete.id)
      toast.success('已彻底删除')
      setPendingDelete(null)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const purge = async () => {
    try {
      const res = await api.purgeTrash()
      toast.success(`已清空 ${res.deleted} 条`)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader
        title="回收站"
        count={items.length}
        actions={
          items.length > 0 && (
            <Button size="sm" variant="outline" onClick={() => setPendingPurge(true)}>
              <TrashIcon className="size-4" />
              清空
            </Button>
          )
        }
      />

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {items.length === 0 && !loading && <EmptyState icon={TrashIcon} message="回收站是空的" />}

      {items.length > 0 && (
        <ul className="mt-4 overflow-hidden rounded-md border border-border">
          {items.map((p, i) => (
            <li
              key={p.id}
              className={`flex items-center gap-2 px-3 py-2.5 ${
                i > 0 ? 'border-t border-border' : ''
              }`}
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm" title={p.title}>
                  {p.title}
                </p>
                <p className="truncate text-xs text-muted-foreground">
                  {p.type === 'page' ? '页面' : '文章'} · 删除于{' '}
                  {formatRelative(p.deleted_at)}
                </p>
              </div>
              <Badge variant="secondary" className="shrink-0">
                {p.type === 'page' ? '页面' : '文章'}
              </Badge>
              <div className="flex shrink-0 items-center">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => void restore(p)}
                  aria-label={`恢复 ${p.title}`}
                >
                  <RotateCcw className="size-4" />
                  恢复
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => navigate(`/admin/${p.type === 'page' ? 'pages' : 'posts'}/${p.id}`)}
                  aria-label={`编辑 ${p.title}`}
                >
                  编辑
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-7 text-destructive"
                  onClick={() => setPendingDelete(p)}
                  aria-label={`彻底删除 ${p.title}`}
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="彻底删除"
        description={`此操作不可恢复，确定删除「${pendingDelete?.title}」？`}
        onConfirm={remove}
      />

      <ConfirmDialog
        open={pendingPurge}
        onOpenChange={setPendingPurge}
        title="清空回收站"
        description={`将彻底删除其中全部 ${items.length} 条内容，此操作不可恢复。`}
        confirmText="清空"
        onConfirm={purge}
      />
    </div>
  )
}
