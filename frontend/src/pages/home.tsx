import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Search } from 'lucide-react'
import { api } from '@/lib/api'
import { formatDate } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { SiteHeader } from '@/components/site-header'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'

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

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-3xl px-4 pb-16 pt-6">
        <form
          className="flex flex-col gap-2 sm:flex-row"
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
              className="pl-8"
              aria-label="搜索文章"
            />
          </div>
          <Button type="submit" variant="outline" className="sm:w-20">
            搜索
          </Button>
        </form>

        <div className="mt-3 flex flex-wrap gap-1">
          <Button
            size="sm"
            variant={category ? 'ghost' : 'secondary'}
            onClick={() => updateParams({ category: null })}
          >
            全部
          </Button>
          {categories.map((c) => (
            <Button
              key={c.id}
              size="sm"
              variant={category === c.slug ? 'secondary' : 'ghost'}
              onClick={() => updateParams({ category: c.slug })}
            >
              {c.name}
            </Button>
          ))}
        </div>

        <Separator className="my-4" />

        {error && <p className="py-8 text-center text-sm text-destructive">{error.message}</p>}

        {loading && !data && (
          <p className="py-8 text-center text-sm text-muted-foreground">加载中</p>
        )}

        {!error && data && data.items.length === 0 && (
          <p className="py-8 text-center text-sm text-muted-foreground">暂无文章</p>
        )}

        {data && data.items.length > 0 && (
          <ul className="overflow-hidden rounded-md border border-border">
            {data.items.map((post, i) => (
              <li key={post.id} className={i > 0 ? 'border-t border-border' : undefined}>
                <Link
                  to={`/post/${post.slug}`}
                  className="flex items-center gap-3 px-3 py-3 hover:bg-muted/50"
                >
                  <span className="w-6 shrink-0 text-right text-xs text-muted-foreground tabular-nums">
                    {(page - 1) * PAGE_SIZE + i + 1}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-sm font-medium" title={post.title}>
                    {post.title}
                  </span>
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
