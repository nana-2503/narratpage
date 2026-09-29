import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Menu } from 'lucide-react'
import { Button, buttonVariants } from '@/components/ui/button'
import { ThemeToggle } from '@/components/theme-toggle'
import { cn } from '@/lib/utils'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

const links = [
  { to: '/', label: '文章', exact: true },
  { to: '/admin', label: '后台', exact: false },
]

export function SiteHeader() {
  const navigate = useNavigate()
  const { pathname } = useLocation()

  const isActive = (to: string, exact: boolean) =>
    exact ? pathname === to : pathname.startsWith(to)

  return (
    <header className="sticky top-0 z-30 border-b border-border bg-background/95 backdrop-blur-sm">
      <div className="mx-auto flex h-12 max-w-3xl items-center gap-1 px-4">
        <Link to="/" className="mr-1 shrink-0 text-sm font-semibold tracking-tight">
          叙页博客
        </Link>

        <nav className="ml-auto hidden items-center gap-1 md:flex">
          {links.map((l) => (
            <Button
              key={l.to}
              variant={isActive(l.to, l.exact) ? 'secondary' : 'ghost'}
              size="sm"
              aria-current={isActive(l.to, l.exact) ? 'page' : undefined}
              onClick={() => navigate(l.to)}
            >
              {l.label}
            </Button>
          ))}
          <a
            href="/api/rss.xml"
            className={buttonVariants({ variant: 'ghost', size: 'sm' })}
            aria-label="RSS 订阅"
          >
            RSS
          </a>
          <ThemeToggle />
        </nav>

        <div className="ml-auto flex items-center gap-1 md:hidden">
          <ThemeToggle />
          <Sheet>
            <SheetTrigger
              render={
                <Button variant="outline" size="icon" className="size-9" aria-label="打开菜单" />
              }
            >
              <Menu className="size-4" />
            </SheetTrigger>
            <SheetContent side="right" className="w-60 gap-0">
              <SheetHeader>
                <SheetTitle className="text-sm">导航</SheetTitle>
              </SheetHeader>
              <nav className="flex flex-col gap-1 px-3">
                {links.map((l) => (
                  <SheetClose
                    key={l.to}
                    render={
                      <Button
                        size="lg"
                        variant={isActive(l.to, l.exact) ? 'secondary' : 'ghost'}
                        className="justify-start"
                        aria-current={isActive(l.to, l.exact) ? 'page' : undefined}
                      />
                    }
                    onClick={() => navigate(l.to)}
                  >
                    {l.label}
                  </SheetClose>
                ))}
                <a
                  href="/api/rss.xml"
                  className={cn(buttonVariants({ variant: 'ghost', size: 'lg' }), 'justify-start')}
                  aria-label="RSS 订阅"
                >
                  RSS
                </a>
              </nav>
            </SheetContent>
          </Sheet>
        </div>
      </div>
    </header>
  )
}