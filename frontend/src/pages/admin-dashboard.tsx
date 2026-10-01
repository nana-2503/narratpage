import { Link } from 'react-router-dom'
import {
  Eye,
  FileText,
  Image as ImageIcon,
  MessageSquare,
  PenLine,
  Plus,
  Users,
} from 'lucide-react'
import { api } from '@/lib/api'
import { formatBytes, formatRelative } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { useAuth } from '@/hooks/use-auth'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'

export default function AdminDashboard() {
  const { user, can } = useAuth()
  const { data: stats, loading } = useAsync(() => api.stats(), [])
  const { data: recent } = useAsync(
    () => api.listPosts({ page: 1, pageSize: 5, status: 'all' }),
    [],
  )
  const { data: counts } = useAsync(() => api.commentCounts().catch(() => null), [])

  if (loading && !stats) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-5 w-24" />
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-20" />
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-sm font-semibold">概览</h1>
        {can('post.create') && (
          <div className="flex gap-2">
            <Button size="sm" onClick={() => window.location.assign('/admin/posts/new')}>
              <Plus className="size-4" />
              写文章
            </Button>
            <Button size="sm" variant="outline" onClick={() => window.location.assign('/admin/pages/new')}>
              <FileText className="size-4" />
              新页面
            </Button>
          </div>
        )}
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="文章" value={stats?.posts ?? 0} icon={PenLine} to="/admin/posts" />
        <StatCard label="页面" value={stats?.pages ?? 0} icon={FileText} to="/admin/pages" />
        <StatCard
          label="评论"
          value={stats?.comments ?? 0}
          icon={MessageSquare}
          to="/admin/comments"
          badge={counts?.pending ? `${counts.pending} 待审` : undefined}
        />
        <StatCard label="总阅读" value={stats?.views ?? 0} icon={Eye} />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="标签" value={stats?.tags ?? 0} icon={PenLine} to="/admin/tags" />
        <StatCard
          label="媒体"
          value={stats?.media.count ?? 0}
          icon={ImageIcon}
          to="/admin/media"
          hint={formatBytes(stats?.media.size ?? 0)}
        />
        <StatCard
          label="用户"
          value={Object.values(stats?.users ?? {}).reduce((a, b) => a + b, 0)}
          icon={Users}
          to="/admin/users"
        />
        <StatCard
          label="我的角色"
          value={roleLabel(user?.role)}
          icon={Users}
          to="/admin/account"
        />
      </div>

      <section>
        <h2 className="text-xs font-medium text-muted-foreground">最近编辑</h2>
        <ul className="mt-2 overflow-hidden rounded-md border border-border">
          {!recent || recent.items.length === 0 ? (
            <li className="px-3 py-6 text-center text-sm text-muted-foreground">暂无内容</li>
          ) : (
            recent.items.map((p, i) => (
              <li
                key={p.id}
                className={`flex items-center gap-3 px-3 py-2.5 ${
                  i > 0 ? 'border-t border-border' : ''
                }`}
              >
                <Link
                  to={`/admin/${p.type === 'page' ? 'pages' : 'posts'}/${p.id}`}
                  className="min-w-0 flex-1 truncate text-sm hover:underline"
                  title={p.title}
                >
                  {p.title}
                </Link>
                <Badge variant={statusVariant(p.status)} className="shrink-0">
                  {statusLabel(p.status)}
                </Badge>
                <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">
                  {formatRelative(p.updated_at)}
                </span>
              </li>
            ))
          )}
        </ul>
      </section>
    </div>
  )
}

function StatCard({
  label,
  value,
  icon: Icon,
  to,
  hint,
  badge,
}: {
  label: string
  value: number | string
  icon: typeof Eye
  to?: string
  hint?: string
  badge?: string
}) {
  const inner = (
    <div className="rounded-md border border-border p-3 transition-colors hover:bg-muted/40">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-xs text-muted-foreground">{label}</span>
        <Icon className="size-3.5 shrink-0 text-muted-foreground" />
      </div>
      <div className="mt-1 flex items-baseline gap-2">
        <span className="text-lg font-semibold tabular-nums">{value}</span>
        {hint && <span className="truncate text-xs text-muted-foreground">{hint}</span>}
      </div>
      {badge && (
        <span className="mt-1 inline-block text-xs text-destructive">{badge}</span>
      )}
    </div>
  )
  return to ? <Link to={to}>{inner}</Link> : inner
}

function statusLabel(status: string): string {
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

function statusVariant(status: string): 'default' | 'secondary' | 'outline' | 'destructive' {
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

function roleLabel(role?: string): string {
  switch (role) {
    case 'admin':
      return '管理员'
    case 'editor':
      return '编辑'
    case 'author':
      return '作者'
    case 'contributor':
      return '贡献者'
    case 'subscriber':
      return '订阅者'
    default:
      return '—'
  }
}
