import { useState } from 'react'
import { Check, Trash2, X } from 'lucide-react'
import { api, type Comment } from '@/lib/api'
import { formatDateTime } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { toast } from 'sonner'

const filters = [
  { value: 'pending', label: '待审核' },
  { value: 'approved', label: '已通过' },
  { value: 'rejected', label: '已拒绝' },
] as const

const statusLabel: Record<Comment['status'], string> = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已拒绝',
}

export default function AdminComments() {
  const [status, setStatus] = useState<Comment['status']>('pending')

  const { data, error, loading, reload } = useAsync(() => api.listComments({ status }), [status])
  const comments = data?.items ?? []

  const act = async (fn: () => Promise<unknown>, msg: string) => {
    try {
      await fn()
      toast.success(msg)
      reload()
    } catch (e) {
      toast.error((e as Error).message)
    }
  }

  return (
    <div>
      <h1 className="text-sm font-semibold">评论审核</h1>
      <div className="mt-3 flex gap-1">
        {filters.map((f) => (
          <Button
            key={f.value}
            size="sm"
            variant={status === f.value ? 'secondary' : 'ghost'}
            onClick={() => setStatus(f.value)}
          >
            {f.label}
          </Button>
        ))}
      </div>

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      <ul className="mt-4 border border-border">
        {comments.length === 0 && (
          <li className="px-3 py-8 text-center text-sm text-muted-foreground">
            {loading ? '加载中' : '暂无评论'}
          </li>
        )}
        {comments.map((c, i) => (
          <li key={c.id} className={`px-3 py-3 ${i > 0 ? 'border-t border-border' : ''}`}>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="font-medium text-foreground">{c.author}</span>
              <span className="min-w-0 truncate" title={c.post_title || `文章 #${c.post_id}`}>
                {c.post_title || `文章 #${c.post_id}`}
              </span>
              <span className="ml-auto shrink-0 tabular-nums">{formatDateTime(c.created_at)}</span>
            </div>
            <p className="mt-1 text-sm whitespace-pre-wrap">{c.content}</p>
            <div className="mt-2 flex items-center gap-1">
              <Badge variant={c.status === 'approved' ? 'default' : 'secondary'}>
                {statusLabel[c.status]}
              </Badge>
              <span className="ml-auto flex gap-1">
                {c.status !== 'approved' && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => act(() => api.moderateComment(c.id, 'approved'), '已通过')}
                  >
                    <Check className="size-4" /> 通过
                  </Button>
                )}
                {c.status !== 'rejected' && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => act(() => api.moderateComment(c.id, 'rejected'), '已拒绝')}
                  >
                    <X className="size-4" /> 拒绝
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-destructive"
                  aria-label="删除评论"
                  onClick={() => act(() => api.deleteComment(c.id), '已删除')}
                >
                  <Trash2 className="size-4" />
                </Button>
              </span>
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}
