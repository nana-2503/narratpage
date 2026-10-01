import { Link } from 'react-router-dom'
import { SiteHeader } from '@/components/site-header'
import { SiteFooter } from '@/components/site-footer'

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto w-full max-w-3xl flex-1 px-4 py-16">
        <p className="text-xs text-muted-foreground tabular-nums">404</p>
        <h1 className="mt-1 text-lg font-semibold tracking-tight">页面不存在</h1>
        <div className="mt-4 flex flex-wrap gap-4 text-sm">
          <Link to="/" className="text-muted-foreground underline hover:text-foreground">
            返回首页
          </Link>
          <Link to="/archive" className="text-muted-foreground underline hover:text-foreground">
            浏览归档
          </Link>
          <Link to="/search" className="text-muted-foreground underline hover:text-foreground">
            搜索
          </Link>
        </div>
      </main>
      <SiteFooter />
    </div>
  )
}
