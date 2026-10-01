import { useState } from 'react'
import { Link, NavLink } from 'react-router-dom'
import { Menu, Rss } from 'lucide-react'
import { useSite } from '@/hooks/use-site'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { ThemeMenu } from '@/components/theme-menu'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

export function SiteHeader() {
  const { site } = useSite()
  const [menuOpen, setMenuOpen] = useState(false)

  // 导航链接取自后台「站点设置」，支持站内路径与外链
  const links = site.links.filter((l) => l.position === 'header')

  const navItems = links.length > 0
    ? links
    : [
        { label: '归档', url: '/archive', position: 'header' as const },
        { label: '标签', url: '/tags', position: 'header' as const },
      ]

  return (
    <header className="sticky top-0 z-30 border-b border-border bg-background/95 backdrop-blur-sm">
      <div className="mx-auto flex h-12 max-w-3xl items-center gap-2 px-4">
        <Link
          to="/"
          className="min-w-0 shrink-0 truncate text-sm font-semibold tracking-tight"
          title={site.title}
        >
          {site.title}
        </Link>

        <nav className="ml-2 hidden items-center gap-1 sm:flex">
          {navItems.map((l) => (
            <HeaderLink key={l.url} href={l.url} label={l.label} onNavigate={() => setMenuOpen(false)} />
          ))}
        </nav>

        <div className="ml-auto flex items-center gap-1">
          <a
            href="/api/feed.xml"
            className="hidden rounded-sm p-1.5 text-muted-foreground transition-colors hover:text-foreground sm:inline-flex"
            aria-label="RSS 订阅"
            title="RSS 订阅"
          >
            <Rss className="size-4" />
          </a>
          <ThemeMenu />

          <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
            <SheetTrigger
              render={
                <Button
                  variant="outline"
                  size="icon"
                  className="size-9 sm:hidden"
                  aria-label="打开菜单"
                />
              }
            >
              <Menu className="size-4" />
            </SheetTrigger>
            <SheetContent side="right" className="w-60 gap-0">
              <SheetHeader>
                <SheetTitle className="truncate text-sm">{site.title}</SheetTitle>
              </SheetHeader>
              <nav className="flex flex-col gap-1 px-3">
                {navItems.map((l) => (
                  <SheetClose
                    key={l.url}
                    render={
                      <Button
                        size="lg"
                        variant="ghost"
                        className="justify-start"
                        aria-label={l.label}
                      />
                    }
                    onClick={() => setMenuOpen(false)}
                  >
                    <HeaderLink href={l.url} label={l.label} plain />
                  </SheetClose>
                ))}
              </nav>
              {site.links.some((l) => l.position === 'social') && (
                <>
                  <div className="px-3 py-3">
                    <Separator />
                  </div>
                  <div className="flex flex-col gap-1 px-3">
                    {site.links
                      .filter((l) => l.position === 'social')
                      .map((l) => (
                        <a
                          key={l.url}
                          href={l.url}
                          target="_blank"
                          rel="noreferrer noopener"
                          className="truncate rounded-sm px-2 py-1.5 text-sm text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                        >
                          {l.label}
                        </a>
                      ))}
                  </div>
                </>
              )}
            </SheetContent>
          </Sheet>
        </div>
      </div>
    </header>
  )
}

/** 站内路径用 NavLink（可高亮），外链用普通 a 标签 */
function HeaderLink({
  href,
  label,
  onNavigate,
  plain,
}: {
  href: string
  label: string
  onNavigate?: () => void
  plain?: boolean
}) {
  const isExternal = /^https?:\/\//.test(href)
  if (isExternal) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noreferrer noopener"
        className="truncate rounded-sm px-2 py-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
        title={label}
      >
        {label}
      </a>
    )
  }
  if (plain) return <span className="truncate">{label}</span>
  return (
    <NavLink
      to={href}
      onClick={onNavigate}
      className={({ isActive }) =>
        cn(
          'truncate rounded-sm px-2 py-1 text-sm transition-colors',
          isActive ? 'text-foreground' : 'text-muted-foreground hover:text-foreground',
        )
      }
    >
      {label}
    </NavLink>
  )
}
