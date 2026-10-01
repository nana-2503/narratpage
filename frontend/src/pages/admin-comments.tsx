import { useState } from 'react'
import { MessageSquare, Trash2 } from 'lucide-react'
import { api, type Comment, type CommentStatus } from '@/lib/api'
import { formatDateTime } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { useSelection } from '@/hooks/use-selection'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { TableCheckbox } from '@/components/admin-table'
import { PageHeader } from '@/components/admin-page-header'
import { EmptyState } from '@/components/empty-state'
import { ConfirmDialog } from '@/components/confirm-dialog'

const TABS: { value: CommentStatus | 'all'; label: string }[] = [
  { value: 'pending', label: '待审核' },
  { value: 'approved', label: '已通过' },
  { value: 'spam', label: '垃圾' },
  { value: 'rejected', label: '已拒绝' },
  { value: 'all', label: '全部' },
]

export default function AdminComments() {
  const [tab, setTab] = useState<CommentStatus | 'all'>('pending')
  const [keyword, setKeyword] = useState('')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [pendingDelete, setPendingDelete] = useState<Comment | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  const { data: counts } = useAsync(() => api.commentCounts().catch(() => null), [reloadKey])
  const { data, error, loading } = useAsync(
    () =>
      api.listComments({
        status: tab === 'all' ? undefined : tab,
        q: search || undefined,
        page,
      }),
    [tab, search, page, reloadKey],
  )

  const comments = data?.items ?? []
  const selection = useSelection(comments, (c) => c.id)
  const refresh = () => setReloadKey((k) => k + 1)

  const moderate = async (id: number, status: CommentStatus) => {
    try {
      await api.moderateComment(id, status)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const runBatch = async (action: string, label: string) => {
    const ids = selection.selectedIds()
    if (ids.length === 0) {
      toast.error('未选择评论')
      return
    }
    try {
      const res = await api.batchComments(ids, action)
      toast.success(`${label} ${res.affected} 条`)
      selection.clear()
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deleteComment(pendingDelete.id)
      toast.success('已删除')
      setPendingDelete(null)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader
        title="评论"
        count={counts ? counts[tab as CommentStatus] ?? undefined : undefined}
      />

      <form
        className="mt-4 flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          setPage(1)
          setSearch(keyword.trim())
        }}
      >
        <div className="flex flex-wrap gap-1">
          {TABS.map((t) => (
            <Button
              key={t.value}
              size="sm"
              variant={tab === t.value ? 'secondary' : 'ghost'}
              aria-pressed={tab === t.value}
              onClick={() => {
                setTab(t.value)
                setPage(1)
              }}
            >
              {t.label}
              {counts && t.value !== 'all' && (
                <span className="ml-1 text-xs text-muted-foreground tabular-nums">
                  {counts[t.value] ?? 0}
                </span>
              )}
            </Button>
          ))}
        </div>
        <div className="ml-auto flex gap-2">
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="搜索昵称或内容"
            aria-label="搜索评论"
            className="h-8 w-44 rounded-md border border-input bg-transparent px-2 text-sm"
          />
          <Button type="submit" size="sm" variant="outline">
            搜索
          </Button>
        </div>
      </form>

      {selection.selected.size > 0 && (
        <div className="mt-3 flex flex-wrap items-center gap-1.5 rounded-md border border-border px-3 py-2 text-xs">
          <span className="text-muted-foreground tabular-nums">已选 {selection.selected.size}</span>
          <Button size="sm" variant="ghost" onClick={() => void runBatch('approve', '已通过')}>
            通过
          </Button>
          <Button size="sm" variant="ghost" onClick={() => void runBatch('reject', '已拒绝')}>
            拒绝
          </Button>
          <Button size="sm" variant="ghost" onClick={() => void runBatch('spam', '标记为垃圾')}>
            垃圾
          </Button>
          <Button size="sm" variant="ghost" onClick={() => void runBatch('delete', '已删除')}>
            删除
          </Button>
          <Button size="sm" variant="ghost" onClick={selection.clear}>
            取消选择
          </Button>
        </div>
      )}

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {comments.length === 0 && !loading && (
        <EmptyState icon={MessageSquare} message="暂无评论" />
      )}

      {comments.length > 0 && (
        <ul className="mt-4 overflow-hidden rounded-md border border-border">
          {comments.map((c, i) => (
            <li
              key={c.id}
              className={`px-3 py-3 ${i > 0 ? 'border-t border-border' : ''}`}
            >
              <div className="flex items-start gap-2">
                <TableCheckbox
                  checked={selection.isSelected(c.id)}
                  onChange={() => selection.toggle(c.id)}
                  label={`选择 ${c.author} 的评论`}
                />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    <span className="font-medium text-foreground">{c.author}</span>
                    <span className="tabular-nums">{formatDateTime(c.created_at)}</span>
                    {c.status !== 'approved' && (
                      <Badge variant={statusVariant(c.status)} className="shrink-0">
                        {statusLabel(c.status)}
                      </Badge>
                    )}
                    {c.post_title && (
                      <span className="truncate">于《{c.post_title}》</span>
                    )}
                  </div>
                  <p className="mt-1 whitespace-pre-wrap break-words text-sm">{c.content}</p>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  {c.status !== 'approved' && (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => void moderate(c.id, 'approved')}
                      aria-label="通过该评论"
                    >
                      通过
                    </Button>
                  )}
                  {c.status !== 'spam' && (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => void moderate(c.id, 'spam')}
                      aria-label="标记为垃圾"
                    >
                      垃圾
                    </Button>
                  )}
                  <Button
                    size="icon"
                    variant="ghost"
                    className="size-7 text-destructive"
                    onClick={() => setPendingDelete(c)}
                    aria-label="删除该评论"
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {data && data.totalPages > 1 && (
        <div className="mt-4 flex items-center justify-between text-sm">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
            上一页
          </Button>
          <span className="text-xs text-muted-foreground tabular-nums">
            {page} / {data.totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= data.totalPages}
            onClick={() => setPage(page + 1)}
          >
            下一页
          </Button>
        </div>
      )}

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="删除评论"
        description="删除后无法恢复。"
        onConfirm={remove}
      />
    </div>
  )
}

function statusLabel(status: string): string {
  switch (status) {
    case 'approved':
      return '已通过'
    case 'rejected':
      return '已拒绝'
    case 'spam':
      return '垃圾'
    default:
      return '待审核'
  }
}

function statusVariant(status: string): 'default' | 'secondary' | 'outline' | 'destructive' {
  switch (status) {
    case 'approved':
      return 'default'
    case 'spam':
      return 'destructive'
    case 'rejected':
      return 'secondary'
    default:
      return 'outline'
  }
}
