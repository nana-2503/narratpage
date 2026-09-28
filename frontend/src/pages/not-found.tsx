import { Link } from 'react-router-dom'
import { SiteHeader } from '@/components/site-header'

export default function NotFound() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-3xl px-4 py-16 text-center">
        <h1 className="text-lg font-semibold tracking-tight">页面不存在</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          你访问的地址没有对应的内容，可能已被移除或链接有误。
        </p>
        <Link
          to="/"
          className="mt-4 inline-block text-sm text-muted-foreground underline hover:text-foreground"
        >
          返回首页
        </Link>
      </main>
    </div>
  )
}
