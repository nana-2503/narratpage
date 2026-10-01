import { useEffect } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, isNotFound } from '@/lib/api'
import { formatDate, renderMarkdown } from '@/lib/markdown'
import { clearPostMeta, setPageMeta } from '@/lib/meta'
import { useAsync } from '@/hooks/use-async'
import { SiteHeader } from '@/components/site-header'
import { SiteFooter } from '@/components/site-footer'

/** 独立页面视图：与文章分离，URL 形态为 /page/:slug */
export default function PageView() {
  const { slug = '' } = useParams()
  const { data: page, error } = useAsync(() => api.getPost(slug), [slug])
  const notFound = isNotFound(error)

  useEffect(() => {
    if (!page) return
    setPageMeta(page.title, page.summary)
    return () => clearPostMeta()
  }, [page])

  if (notFound) {
    return (
      <div className="min-h-screen bg-background">
        <SiteHeader />
        <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">
          页面不存在
        </main>
      </div>
    )
  }

  if (!page) {
    return (
      <div className="min-h-screen bg-background">
        <SiteHeader />
        <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">
          {error ? `加载失败：${error.message}` : '加载中'}
        </main>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-3xl flex-1 px-4 pb-16 pt-6">
        <Link to="/" className="text-xs text-muted-foreground hover:text-foreground">
          ← 返回
        </Link>
        <article className="mt-3">
          <h1 className="text-xl font-semibold tracking-tight text-balance">{page.title}</h1>
          <p className="mt-1 text-xs text-muted-foreground tabular-nums">
            更新于 {formatDate(page.updated_at)}
          </p>
          <div
            className="md-body mt-5 text-sm leading-7"
            dangerouslySetInnerHTML={{ __html: renderMarkdown(page.content || '') }}
          />
        </article>
      </main>
      <SiteFooter />
    </div>
  )
}
