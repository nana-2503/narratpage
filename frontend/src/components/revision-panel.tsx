import { useState } from 'react'
import { History, RotateCcw, Trash2 } from 'lucide-react'
import { api, type Revision } from '@/lib/api'
import { formatDateTime, stripMarkdown } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/empty-state'
import { ConfirmDialog } from '@/components/confirm-dialog'

/**
 * 编辑历史面板。
 *
 * 每次内容变更由后端自动存档，此处只负责展示与回滚。
 * 回滚本身也会先为当前内容存档，因此不会丢失正在编辑的版本。
 */
export function RevisionPanel({
  postId,
  onRestored,
}: {
  postId: number
  onRestored: () => void
}) {
  const { data, error, loading, reload } = useAsync(
    () => api.listRevisions(postId),
    [postId],
  )
  const [pendingRestore, setPendingRestore] = useState<Revision | null>(null)
  const [pendingDelete, setPendingDelete] = useState<Revision | null>(null)

  const items = data?.items ?? []

  const restore = async () => {
    if (!pendingRestore) return
    try {
      await api.restoreRevision(postId, pendingRestore.id)
      setPendingRestore(null)
      reload()
      onRestored()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deleteRevision(postId, pendingDelete.id)
      toast.success('已删除该版本')
      setPendingDelete(null)
      reload()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  if (loading) return <p className="text-sm text-muted-foreground">加载中</p>
  if (error) return <p className="text-sm text-destructive">{error.message}</p>
  if (items.length === 0) {
    return <EmptyState icon={History} message="暂无历史版本" />
  }

  return (
    <>
      <ul className="overflow-hidden rounded-md border border-border">
        {items.map((rev, i) => (
          <li
            key={rev.id}
            className={`flex items-center gap-3 px-3 py-2.5 ${
              i > 0 ? 'border-t border-border' : ''
            }`}
          >
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm" title={rev.title || '（无标题）'}>
                {rev.title || '（无标题）'}
              </p>
              <p className="truncate text-xs text-muted-foreground">
                {formatDateTime(rev.created_at)} · {previewOf(rev.content)}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-1">
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setPendingRestore(rev)}
                aria-label={`回滚到 ${formatDateTime(rev.created_at)} 的版本`}
              >
                <RotateCcw className="size-4" />
                回滚
              </Button>
              <Button
                size="icon"
                variant="ghost"
                className="size-7 text-destructive"
                onClick={() => setPendingDelete(rev)}
                aria-label="删除该版本"
              >
                <Trash2 className="size-4" />
              </Button>
            </div>
          </li>
        ))}
      </ul>

      <ConfirmDialog
        open={!!pendingRestore}
        onOpenChange={(open) => !open && setPendingRestore(null)}
        title="回滚版本"
        description="当前内容会先存为新版本，然后回滚到所选版本。"
        confirmText="回滚"
        onConfirm={restore}
      />

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="删除版本"
        description="删除后无法恢复该历史版本。"
        onConfirm={remove}
      />
    </>
  )
}

function previewOf(content: string): string {
  const text = stripMarkdown(content)
  return text.length > 60 ? `${text.slice(0, 60)}…` : text
}
