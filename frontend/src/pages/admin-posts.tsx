import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { FileText, Plus, Trash2 } from 'lucide-react'
import { api, type Post, type PostStatus } from '@/lib/api'
import { formatDate } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { useAuth } from '@/hooks/use-auth'
import { useSelection } from '@/hooks/use-selection'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ListWrap, TableCheckbox, TableWrap } from '@/components/admin-table'
import { PageHeader } from '@/components/admin-page-header'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'

type Filter = 'all' | 'draft' | 'published' | 'pending' | 'private'

const FILTERS: { value: Filter; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'published', label: '已发布' },
  { value: 'draft', label: '草稿' },
  { value: 'pending', label: '待审核' },
  { value: 'private', label: '私密' },
]

export default function AdminPosts() {
  const navigate = useNavigate()
  const { can } = useAuth()
  const [page, setPage] = useState(1)
  const [filter, setFilter] = useState<Filter>('all')
  const [keyword, setKeyword] = useState('')
  const [search, setSearch] = useState('')
  const [pendingDelete, setPendingDelete] = useState<Post | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  const { data, error, loading } = useAsync(
    () =>
      api.listPosts({
        page,
        pageSize: 20,
        status: filter === 'all' ? 'all' : filter,
        q: search || undefined,
      }),
    [page, filter, search, reloadKey],
  )

  const posts = data?.items ?? []
  const selection = useSelection(posts, (p) => p.id)

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

  const runBatch = async (action: string, label: string) => {
    const ids = selection.selectedIds()
    if (ids.length === 0) {
      toast.error('未选择内容')
      return
    }
    try {
      const res = await api.batchPosts(ids, action)
      toast.success(`${label} ${res.ok} 条${res.skipped ? `，跳过 ${res.skipped} 条` : ''}`)
      selection.clear()
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const moveToTrash = async (post: Post) => {
    try {
      await api.trashPost(post.id)
      toast.success('已移入回收站')
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader
        title="文章"
        count={data?.total}
        actions={
          can('post.create') && (
            <Button size="sm" onClick={() => navigate('/admin/posts/new')}>
              <Plus className="size-4" />
              新建
            </Button>
          )
        }
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
          {FILTERS.map((f) => (
            <Button
              key={f.value}
              size="sm"
              variant={filter === f.value ? 'secondary' : 'ghost'}
              aria-pressed={filter === f.value}
              onClick={() => {
                setFilter(f.value)
                setPage(1)
              }}
            >
              {f.label}
            </Button>
          ))}
        </div>
        <div className="ml-auto flex gap-2">
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="搜索标题"
            aria-label="搜索文章"
            className="h-8 w-40 rounded-md border border-input bg-transparent px-2 text-sm"
          />
          <Button type="submit" size="sm" variant="outline">
            搜索
          </Button>
        </div>
      </form>

      {selection.selected.size > 0 && (
        <div className="mt-3 flex flex-wrap items-center gap-1.5 rounded-md border border-border px-3 py-2 text-xs">
          <span className="text-muted-foreground tabular-nums">已选 {selection.selected.size}</span>
          {can('post.publish') && (
            <Button size="sm" variant="ghost" onClick={() => void runBatch('publish', '已发布')}>
              发布
            </Button>
          )}
          <Button size="sm" variant="ghost" onClick={() => void runBatch('draft', '已转为草稿')}>
            转草稿
          </Button>
          <Button size="sm" variant="ghost" onClick={() => void runBatch('trash', '已移入回收站')}>
            移入回收站
          </Button>
          <Button size="sm" variant="ghost" onClick={selection.clear}>
            取消选择
          </Button>
        </div>
      )}

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {posts.length === 0 && !loading && (
        <EmptyState
          icon={FileText}
          message={search || filter !== 'all' ? '没有匹配的文章' : '暂无文章'}
        />
      )}

      {posts.length > 0 && (
        <>
          <TableWrap>
            <Table>
              <TableHeader>
                <TableRow className="text-xs text-muted-foreground hover:bg-transparent">
                  <TableHead className="w-8">
                    <TableCheckbox
                      checked={selection.allSelected}
                      onChange={selection.toggleAll}
                      label="全选"
                      indeterminate={selection.selected.size > 0}
                    />
                  </TableHead>
                  <TableHead className="font-medium">标题</TableHead>
                  <TableHead className="font-medium">分类</TableHead>
                  <TableHead className="font-medium">状态</TableHead>
                  <TableHead className="text-right font-medium">阅读</TableHead>
                  <TableHead className="font-medium">发布时间</TableHead>
                  <TableHead className="text-right font-medium">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {posts.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell>
                      <TableCheckbox
                        checked={selection.isSelected(p.id)}
                        onChange={() => selection.toggle(p.id)}
                        label={`选择 ${p.title}`}
                      />
                    </TableCell>
                    <TableCell className="max-w-64 truncate font-medium" title={p.title}>
                      <button
                        onClick={() => navigate(`/admin/posts/${p.id}`)}
                        className="max-w-full truncate text-left hover:underline"
                      >
                        {p.title}
                      </button>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {p.category_name || '—'}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Badge variant={statusVariant(p.status)}>{statusLabel(p.status)}</Badge>
                        {p.sticky && (
                          <span className="text-[10px] text-muted-foreground">置顶</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-right tabular-nums text-muted-foreground">
                      {p.views}
                    </TableCell>
                    <TableCell className="tabular-nums text-muted-foreground">
                      {formatDate(p.published_at || p.created_at)}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => navigate(`/admin/posts/${p.id}`)}
                      >
                        编辑
                      </Button>
                      {can('post.delete') && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-destructive"
                          onClick={() => setPendingDelete(p)}
                          aria-label={`删除 ${p.title}`}
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableWrap>

          <ListWrap>
            {posts.map((p, i) => (
              <li key={p.id} className={`px-3 py-3 ${i > 0 ? 'border-t border-border' : ''}`}>
                <div className="flex items-center gap-2">
                  <TableCheckbox
                    checked={selection.isSelected(p.id)}
                    onChange={() => selection.toggle(p.id)}
                    label={`选择 ${p.title}`}
                  />
                  <button
                    onClick={() => navigate(`/admin/posts/${p.id}`)}
                    className="min-w-0 flex-1 truncate text-left text-sm font-medium hover:underline"
                    title={p.title}
                  >
                    {p.title}
                  </button>
                  <Badge variant={statusVariant(p.status)} className="shrink-0">
                    {statusLabel(p.status)}
                  </Badge>
                </div>
                <div className="mt-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                  <span className="min-w-0 truncate">
                    {p.category_name || '未分类'} ·{' '}
                    {formatDate(p.published_at || p.created_at)} · {p.views} 阅读
                  </span>
                  <span className="flex shrink-0 gap-1">
                    <Button size="sm" variant="ghost" onClick={() => navigate(`/admin/posts/${p.id}`)}>
                      编辑
                    </Button>
                    {can('post.delete') && (
                      <>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => void moveToTrash(p)}
                          aria-label={`移入回收站 ${p.title}`}
                        >
                          回收站
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-destructive"
                          onClick={() => setPendingDelete(p)}
                          aria-label={`删除 ${p.title}`}
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      </>
                    )}
                  </span>
                </div>
              </li>
            ))}
          </ListWrap>
        </>
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
        title="删除文章"
        description={`删除后不可恢复，确定删除「${pendingDelete?.title}」？`}
        onConfirm={remove}
      />
    </div>
  )
}

export function statusLabel(status: PostStatus | string): string {
  switch (status) {
    case 'published':
      return '已发布'
    case 'pending':
      return '待审核'
    case 'private':
      return '私密'
    case 'trash':
      return '回收站'
    default:
      return '草稿'
  }
}

export function statusVariant(
  status: string,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  switch (status) {
    case 'published':
      return 'default'
    case 'pending':
      return 'outline'
    case 'trash':
      return 'destructive'
    default:
      return 'secondary'
  }
}
