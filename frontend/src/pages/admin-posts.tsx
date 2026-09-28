import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Trash2 } from 'lucide-react'
import { api, type Post } from '@/lib/api'
import { formatDate } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
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
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'

export default function AdminPosts() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [pendingDelete, setPendingDelete] = useState<Post | null>(null)

  const { data, error, loading, reload } = useAsync(
    () => api.listPosts({ page, pageSize: 20, all: true }),
    [page],
  )
  const posts = data?.items ?? []
  const totalPages = data?.totalPages ?? 1
  const total = data?.total ?? 0

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deletePost(pendingDelete.id)
      toast.success('已删除')
      setPendingDelete(null)
      reload()
    } catch (e) {
      toast.error((e as Error).message)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between">
        <h1 className="text-sm font-semibold">文章 {total}</h1>
        <Button size="sm" onClick={() => navigate('/admin/posts/new')}>
          <Plus className="size-4" /> 新建
        </Button>
      </div>

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {/* 桌面端：表格，单元格单行不换行 */}
      <div className="mt-4 hidden overflow-hidden rounded-md border border-border md:block">
        <Table>
          <TableHeader>
            <TableRow className="text-xs text-muted-foreground hover:bg-transparent">
              <TableHead className="font-medium">标题</TableHead>
              <TableHead className="font-medium">分类</TableHead>
              <TableHead className="font-medium">状态</TableHead>
              <TableHead className="text-right font-medium">阅读</TableHead>
              <TableHead className="font-medium">发布时间</TableHead>
              <TableHead className="text-right font-medium">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {posts.length === 0 && (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={6} className="h-16 text-center text-muted-foreground">
                  {loading ? '加载中' : '暂无文章'}
                </TableCell>
              </TableRow>
            )}
            {posts.map((p) => (
              <TableRow key={p.id}>
                <TableCell className="max-w-64 truncate font-medium" title={p.title}>
                  {p.title}
                </TableCell>
                <TableCell className="text-muted-foreground">{p.category_name || '—'}</TableCell>
                <TableCell>
                  <Badge variant={p.status === 'published' ? 'default' : 'secondary'}>
                    {p.status === 'published' ? '已发布' : '草稿'}
                  </Badge>
                </TableCell>
                <TableCell className="text-right tabular-nums text-muted-foreground">
                  {p.views}
                </TableCell>
                <TableCell className="tabular-nums text-muted-foreground">
                  {formatDate(p.published_at || p.created_at)}
                </TableCell>
                <TableCell className="text-right">
                  <Button size="sm" variant="ghost" onClick={() => navigate(`/admin/posts/${p.id}`)}>
                    编辑
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
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* 移动端：堆叠列表，单层边框 */}
      <ul className="mt-4 overflow-hidden rounded-md border border-border md:hidden">
        {posts.length === 0 && (
          <li className="px-3 py-8 text-center text-sm text-muted-foreground">
            {loading ? '加载中' : '暂无文章'}
          </li>
        )}
        {posts.map((p, i) => (
          <li key={p.id} className={`px-3 py-3 ${i > 0 ? 'border-t border-border' : ''}`}>
            <div className="flex items-center gap-2">
              <span className="min-w-0 flex-1 truncate text-sm font-medium" title={p.title}>
                {p.title}
              </span>
              <Badge variant={p.status === 'published' ? 'default' : 'secondary'}>
                {p.status === 'published' ? '已发布' : '草稿'}
              </Badge>
            </div>
            <div className="mt-1 flex items-center justify-between text-xs text-muted-foreground">
              <span className="truncate">
                {p.category_name || '未分类'} · {formatDate(p.published_at || p.created_at)} · {p.views} 阅读
              </span>
              <span className="flex shrink-0 gap-1">
                <Button size="sm" variant="ghost" onClick={() => navigate(`/admin/posts/${p.id}`)}>
                  编辑
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
              </span>
            </div>
          </li>
        ))}
      </ul>

      {totalPages > 1 && (
        <div className="mt-4 flex items-center justify-between text-sm">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
            上一页
          </Button>
          <span className="text-xs text-muted-foreground tabular-nums">
            {page} / {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage(page + 1)}
          >
            下一页
          </Button>
        </div>
      )}

      <Dialog open={!!pendingDelete} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <DialogContent className="shadow-none sm:max-w-80">
          <DialogHeader>
            <DialogTitle>删除文章</DialogTitle>
            <DialogDescription>
              删除后不可恢复，确定删除「{pendingDelete?.title}」？
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" size="sm" />}>取消</DialogClose>
            <Button variant="destructive" size="sm" onClick={remove}>
              删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
