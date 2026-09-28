import { Link, useNavigate } from 'react-router-dom'
import { Menu } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

const links = [
  { to: '/', label: '文章' },
  { to: '/admin', label: '后台' },
]

export function SiteHeader() {
  const navigate = useNavigate()
  return (
    <header className="sticky top-0 z-30 border-b border-border bg-background">
      <div className="mx-auto flex h-12 max-w-3xl items-center gap-3 px-4">
        <Link to="/" className="text-sm font-semibold tracking-tight">
          BLOG
        </Link>
        <nav className="ml-auto hidden items-center gap-1 md:flex">
          {links.map((l) => (
            <Button key={l.to} variant="ghost" size="sm" onClick={() => navigate(l.to)}>
              {l.label}
            </Button>
          ))}
        </nav>
        <div className="ml-auto md:hidden">
          <Sheet>
            <SheetTrigger
              render={
                <Button variant="outline" size="icon" className="size-9" aria-label="菜单" />
              }
            >
              <Menu className="size-4" />
            </SheetTrigger>
            <SheetContent side="right" className="w-56">
              <SheetHeader>
                <SheetTitle className="text-sm">导航</SheetTitle>
              </SheetHeader>
              <nav className="flex flex-col gap-1 px-4">
                {links.map((l) => (
                  <Button
                    key={l.to}
                    variant="ghost"
                    className="justify-start"
                    onClick={() => navigate(l.to)}
                  >
                    {l.label}
                  </Button>
                ))}
              </nav>
            </SheetContent>
          </Sheet>
        </div>
      </div>
    </header>
  )
}
