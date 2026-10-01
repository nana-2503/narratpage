import { Link } from 'react-router-dom'
import { useSite } from '@/hooks/use-site'

/** 页脚：版权与 footer 位置的站点链接 */
export function SiteFooter() {
  const { site } = useSite()
  const links = site.links.filter((l) => l.position === 'footer')
  const year = new Date().getFullYear()

  return (
    <footer className="border-t border-border">
      <div className="mx-auto flex max-w-3xl flex-col gap-2 px-4 py-6 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <span className="tabular-nums">© {year}</span>
          <span className="truncate">{site.title}</span>
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          {links.map((l) => (
            /^https?:\/\//.test(l.url) ? (
              <a
                key={l.url}
                href={l.url}
                target={l.target || '_blank'}
                rel="noreferrer noopener"
                className="truncate hover:text-foreground"
              >
                {l.label}
              </a>
            ) : (
              <Link key={l.url} to={l.url} className="truncate hover:text-foreground">
                {l.label}
              </Link>
            )
          ))}
          <a href="/api/feed.xml" className="hover:text-foreground">
            RSS
          </a>
        </div>
      </div>
    </footer>
  )
}
