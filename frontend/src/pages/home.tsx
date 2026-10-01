import { useEffect, useState } from 'react'
import { Link, useLocation, useSearchParams } from 'react-router-dom'
import { LayoutGrid, List, Search, X } from 'lucide-react'
import { api, type Post } from '@/lib/api'
import { formatDate, stripMarkdown } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { useSite } from '@/hooks/use-site'
import { cn } from '@/lib/utils'
import { DEFAULT_VIEW, loadView, saveView, type PostView } from '@/lib/view-pref'
import { SiteHeader } from '@/components/site-header'
import { SiteFooter } from '@/components/site-footer'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

export default function Home() {
  const location = useLocation()
  const [searchParams, setSearchParams] = useSearchParams()
  const { site } = useSite()

  // 路径段 /category/:slug、/tag/:slug 归一为查询条件，
  // 这样分类/标签页与首页共用同一份渲染逻辑，筛选状态可分享。
  const pathSegment = decodeURIComponent(location.pathname.split('/')[2] || '')
  const pathKind = location.pathname.split('/')[1]
  const fromPath =
    pathKind === 'category' ? 'category' : pathKind === 'tag' ? 'tag' : ''

  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1)
  const category = searchParams.get('category') || (fromPath === 'category' ? pathSegment : '')
  const tag = searchParams.get('tag') || (fromPath === 'tag' ? pathSegment : '')
  const q = searchParams.get('q') || ''
  const author = searchParams.get('author') || ''
  const year = searchParams.get('year') || ''
  const month = searchParams.get('month') || ''
  const pageSize = site.posts_per_page || 10

  const [keyword, setKeyword] = useState(q)
  // URL 中的 q 被外部改变（如点击标签）时，同步回输入框
  useEffect(() => setKeyword(q), [q])

  const [view, setView] = useState<PostView>(loadView)
  const switchView = (v: PostView) => {
    setView(v)
    saveView(v)
  }

  const { data: categoriesData } = useAsync(() => api.listCategories(), [])
  const categories = categoriesData?.items ?? []

  const { data, error, loading } = useAsync(
    () =>
      api.listPosts({
        page,
        pageSize,
        category: category || undefined,
        tag: tag || undefined,
        q: q || undefined,
        author: author ? Number(author) : undefined,
        year: year ? Number(year) : undefined,
        month: month ? Number(month) : undefined,
      }),
    [page, category, tag, q, author, year, month, pageSize],
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

  const filtered = Boolean(q || category || tag || author || year)

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto w-full max-w-3xl flex-1 px-4 pb-16">
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
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                placeholder="搜索标题与正文"
                className={cn('pl-8', keyword && 'pr-8')}
                aria-label="搜索文章"
              />
              {keyword && (
                <button
                  type="button"
                  aria-label="清除搜索"
                  className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground transition-colors hover:text-foreground"
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
              variant={!category && !tag ? 'secondary' : 'ghost'}
              aria-pressed={!category && !tag}
              onClick={() => setSearchParams(new URLSearchParams())}
            >
              全部
            </Button>
            {categories.map((c) => (
              <Button
                key={c.id}
                size="sm"
                variant={category === c.slug ? 'secondary' : 'ghost'}
                aria-pressed={category === c.slug}
                onClick={() => updateParams({ category: c.slug, tag: null })}
              >
                {c.name}
                {typeof c.post_count === 'number' && c.post_count > 0 && (
                  <span className="ml-1 text-xs text-muted-foreground tabular-nums">
                    {c.post_count}
                  </span>
                )}
              </Button>
            ))}
          </div>
        </div>

        {/* 当前生效的筛选条件，可单独清除 */}
        {(category || tag || q || year || author) && (
          <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs">
            {tag && (
              <FilterChip label={`标签：${tag}`} onClear={() => updateParams({ tag: null })} />
            )}
            {year && (
              <FilterChip
                label={`时间：${year}${month ? `-${month}` : ''}`}
                onClear={() => updateParams({ year: null, month: null })}
              />
            )}
            {author && (
              <FilterChip label="作者筛选" onClear={() => updateParams({ author: null })} />
            )}
            {q && <FilterChip label={`搜索：${q}`} onClear={() => updateParams({ q: null })} />}
          </div>
        )}

        {error && <p className="py-8 text-center text-sm text-destructive">{error.message}</p>}

        {loading && !data && (
          <p className="py-8 text-center text-sm text-muted-foreground">加载中</p>
        )}

        {data && !error && (
          <div className="flex items-center justify-between gap-2 pt-4">
            <p className="text-xs text-muted-foreground tabular-nums">
              {filtered ? `${data.total} 条结果` : `共 ${data.total} 篇`}
            </p>
            <ViewToggle value={view} onChange={switchView} />
          </div>
        )}

        {!error && data && data.items.length === 0 && (
          <p className="py-8 text-center text-sm text-muted-foreground">
            {filtered ? '没有匹配的文章' : '暂无文章'}
          </p>
        )}

        {data && data.items.length > 0 && view === 'list' && (
          <ul className="mt-2 overflow-hidden rounded-md border border-border">
            {data.items.map((post, i) => (
              <li key={post.id} className={i > 0 ? 'border-t border-border' : undefined}>
                <Link
                  to={`/post/${post.slug}`}
                  className="flex items-center gap-3 px-3 py-3 transition-colors hover:bg-muted/50"
                  title={post.title}
                >
                  <span className="w-6 shrink-0 text-right text-xs text-muted-foreground tabular-nums">
                    {(page - 1) * pageSize + i + 1}
                  </span>
                  {post.sticky && (
                    <span className="shrink-0 text-[10px] text-muted-foreground">置顶</span>
                  )}
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

        {data && data.items.length > 0 && view === 'card' && (
          <ul className="mt-2 grid gap-3 sm:grid-cols-2">
            {data.items.map((post) => (
              <li key={post.id} className="flex">
                <PostCard post={post} />
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

        <div className="mt-10 flex flex-wrap justify-center gap-4 text-xs text-muted-foreground">
          <Link to="/archive" className="hover:text-foreground">
            归档
          </Link>
          <Link to="/tags" className="hover:text-foreground">
            标签
          </Link>
          <a href="/api/feed.xml" className="hover:text-foreground">
            RSS
          </a>
        </div>
      </main>
      <SiteFooter />
    </div>
  )
}

function ViewToggle({ value, onChange }: { value: PostView; onChange: (v: PostView) => void }) {
  return (
    <div
      role="radiogroup"
      aria-label="列表视图"
      className="flex shrink-0 rounded-md border border-border p-0.5"
    >
      {(
        [
          { value: DEFAULT_VIEW, label: '列表', icon: List },
          { value: 'card' as const, label: '卡片', icon: LayoutGrid },
        ] as const
      ).map(({ value: v, label, icon: Icon }) => (
        <button
          key={v}
          type="button"
          role="radio"
          aria-checked={value === v}
          aria-label={label}
          title={label}
          onClick={() => onChange(v)}
          className={cn(
            'flex size-7 items-center justify-center rounded-sm transition-colors',
            'focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none',
            value === v
              ? 'bg-secondary text-secondary-foreground'
              : 'text-muted-foreground hover:text-foreground',
          )}
        >
          <Icon className="size-4" />
        </button>
      ))}
    </div>
  )
}

/** 卡片视图条目：封面图 + 标题 + 摘要 + 元信息 */
function PostCard({ post }: { post: Post }) {
  // 无摘要时从正文截取，避免卡片只有一行标题而高度参差
  const excerpt = post.summary?.trim() || stripMarkdown(post.content || '').slice(0, 90)
  return (
    <Link
      to={`/post/${post.slug}`}
      className="flex w-full flex-col overflow-hidden rounded-md border border-border transition-colors hover:border-foreground/40 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
    >
      {post.cover_url && (
        <img
          src={post.cover_url}
          alt=""
          loading="lazy"
          className="aspect-video w-full shrink-0 object-cover"
        />
      )}
      <div className="flex flex-1 flex-col gap-1.5 p-3">
        <h2 className="line-clamp-2 text-sm font-medium leading-snug" title={post.title}>
          {post.title}
        </h2>
        {excerpt && <p className="line-clamp-2 text-xs text-muted-foreground">{excerpt}</p>}
        <div className="mt-auto flex items-center gap-2 pt-1 text-xs text-muted-foreground">
          {post.sticky && <span className="shrink-0">置顶</span>}
          {post.category_name && (
            <span className="min-w-0 truncate" title={post.category_name}>
              {post.category_name}
            </span>
          )}
          <span className="ml-auto shrink-0 tabular-nums">
            {formatDate(post.published_at || post.created_at)}
          </span>
        </div>
      </div>
    </Link>
  )
}

function FilterChip({ label, onClear }: { label: string; onClear: () => void }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-sm border border-border px-1.5 py-0.5">
      <span className="truncate">{label}</span>
      <button
        type="button"
        onClick={onClear}
        aria-label={`清除筛选 ${label}`}
        className="text-muted-foreground hover:text-foreground"
      >
        <X className="size-3" />
      </button>
    </span>
  )
}
