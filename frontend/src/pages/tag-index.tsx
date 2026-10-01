import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import { useAsync } from '@/hooks/use-async'
import { SiteHeader } from '@/components/site-header'
import { SiteFooter } from '@/components/site-footer'
import { Badge } from '@/components/ui/badge'

/** 标签总览：按使用次数排序 */
export default function TagIndex() {
  const { data, error, loading } = useAsync(() => api.listTags(), [])
  const tags = data?.items ?? []

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-3xl flex-1 px-4 pb-16 pt-6">
        <Link to="/" className="text-xs text-muted-foreground hover:text-foreground">
          ← 返回
        </Link>
        <h1 className="mt-3 text-xl font-semibold tracking-tight">
          标签 {tags.length > 0 && <span className="text-muted-foreground">{tags.length}</span>}
        </h1>

        {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}
        {loading && !data && <p className="mt-4 text-sm text-muted-foreground">加载中</p>}

        {data && tags.length === 0 && (
          <p className="mt-4 text-sm text-muted-foreground">暂无标签</p>
        )}

        {tags.length > 0 && (
          <ul className="mt-4 flex flex-wrap gap-1.5">
            {tags.map((t) => (
              <li key={t.id}>
                <Link to={`/tag/${t.slug}`}>
                  <Badge variant="outline" className="font-normal tabular-nums">
                    {t.name} · {t.post_count}
                  </Badge>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </main>
      <SiteFooter />
    </div>
  )
}
