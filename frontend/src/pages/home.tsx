import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Search, X } from 'lucide-react'
import { api } from '@/lib/api'
import { formatDate } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { cn } from '@/lib/utils'
import { SiteHeader } from '@/components/site-header'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

const PAGE_SIZE = 10

export default function Home() {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1)
  const category = searchParams.get('category') || ''
  const q = searchParams.get('q') || ''

  const [keyword, setKeyword] = useState(q)

  const { data: categoriesData } = useAsync(() => api.listCategories(), [])
  const categories = categoriesData?.items ?? []

  const { data, error, loading } = useAsync(
    () => api.listPosts({ page, pageSize: PAGE_SIZE, category, q }),
    [page, category, q],
  )

  const updateParams = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams)
    for (const [k, v] of Object.entries(patch)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    if (!('page' in patch)) next.delete('page')
    setSearchParams(next)
  }

  const filtered = Boolean(q || category)

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-3xl px-4 pb-16">
        {/* 粘性筛选栏：搜索 + 分类，滚动时始终可达 */}
        <div className="sticky top-12 z-20 -mx-4 border-b border-border bg-background/95 px-4 py-3 backdrop-blur-sm">
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault()
              updateParams({ q: keyword.trim() || null })
            }}
          >
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                placeholder="搜索标题"
                className={cn('pl-8', keyword && 'pr-8')}
                aria-label="搜索文章"
              />
              {keyword && (
                <button
                  type="button"
                  aria-label="清除搜索"
                  className="absolute top-1/2 right-1.5 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground transition-colors hover:text-foreground"
                  onClick={() => {
                    setKeyword('')
                    updateParams({ q: null })
                  }}
                >
                  <X className="size-4" />
                </button>
              )}
            </div>
            <Button type="submit" variant="outline" className="shrink-0 sm:w-20">
              搜索
            </Button>
          </form>

          <div className="mt-2 flex flex-wrap gap-1">
            <Button
              size="sm"
              variant={category ? 'ghost' : 'secondary'}
              aria-pressed={!category}
              onClick={() => updateParams({ category: null })}
            >
              全部
            </Button>
            {categories.map((c) => (
              <Button
                key={c.id}
                size="sm"
                variant={category === c.slug ? 'secondary' : 'ghost'}
                aria-pressed={category === c.slug}
                onClick={() => updateParams({ category: c.slug })}
              >
                {c.name}
              </Button>
            ))}
          </div>
        </div>

        {error && <p className="py-8 text-center text-sm text-destructive">{error.message}</p>}

        {loading && !data && (
          <p className="py-8 text-center text-sm text-muted-foreground">加载中</p>
        )}

        {data && !error && (
          <p className="pt-4 text-xs text-muted-foreground tabular-nums">
            {filtered ? `${data.total} 条结果` : `共 ${data.total} 篇`}
          </p>
        )}

        {!error && data && data.items.length === 0 && (
          <p className="py-8 text-center text-sm text-muted-foreground">
            {filtered ? '没有匹配的文章' : '暂无文章'}
          </p>
        )}

        {data && data.items.length > 0 && (
          <ul className="mt-2 overflow-hidden rounded-md border border-border">
            {data.items.map((post, i) => (
              <li key={post.id} className={i > 0 ? 'border-t border-border' : undefined}>
                <Link
                  to={`/post/${post.slug}`}
                  className="flex items-center gap-3 px-3 py-3 transition-colors hover:bg-muted/50"
                  title={post.title}
                >
                  <span className="w-6 shrink-0 text-right text-xs text-muted-foreground tabular-nums">
                    {(page - 1) * PAGE_SIZE + i + 1}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">{post.title}</span>
                  {post.category_name && (
                    <Badge variant="outline" className="hidden shrink-0 sm:inline-flex">
                      {post.category_name}
                    </Badge>
                  )}
                  <span className="hidden shrink-0 text-xs text-muted-foreground tabular-nums sm:inline">
                    {formatDate(post.published_at || post.created_at)}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}

        {data && data.totalPages > 1 && (
          <div className="mt-4 flex items-center justify-between text-sm">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => updateParams({ page: String(page - 1) })}
            >
              上一页
            </Button>
            <span className="text-xs text-muted-foreground tabular-nums">
              {page} / {data.totalPages} 页 · 共 {data.total} 篇
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= data.totalPages}
              onClick={() => updateParams({ page: String(page + 1) })}
            >
              下一页
            </Button>
          </div>
        )}
      </main>
    </div>
  )
}