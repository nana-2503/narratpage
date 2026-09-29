import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate, Outlet } from 'react-router-dom'
import { Menu } from 'lucide-react'
import { api, getToken, setToken } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { ThemeToggle } from '@/components/theme-toggle'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

const navItems = [
  { to: '/admin/posts', label: '文章' },
  { to: '/admin/comments', label: '评论' },
  { to: '/admin/categories', label: '分类' },
  { to: '/admin/account', label: '账号' },
  { to: '/admin/settings', label: '设置' },
]

export default function AdminLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const [checking, setChecking] = useState(true)

  useEffect(() => {
    const token = getToken()
    if (!token) {
      navigate('/admin/login', { replace: true, state: { from: location.pathname } })
      return
    }
    api
      .me()
      .then(() => setChecking(false))
      .catch(() => {
        setToken(null)
        navigate('/admin/login', { replace: true, state: { from: location.pathname } })
      })
  }, [navigate, location.pathname])

  const logout = () => {
    setToken(null)
    navigate('/admin/login', { replace: true })
  }

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        加载中
      </div>
    )
  }

  const isActive = (to: string) => location.pathname.startsWith(to)

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-30 border-b border-border bg-background/95 backdrop-blur-sm">
        <div className="mx-auto flex h-12 max-w-5xl items-center gap-1 px-4">
          <Link to="/admin/posts" className="mr-1 shrink-0 text-sm font-semibold tracking-tight">
            后台
          </Link>

          <nav className="hidden items-center gap-1 md:flex">
            {navItems.map((item) => (
              <Button
                key={item.to}
                size="sm"
                variant={isActive(item.to) ? 'secondary' : 'ghost'}
                aria-current={isActive(item.to) ? 'page' : undefined}
                onClick={() => navigate(item.to)}
              >
                {item.label}
              </Button>
            ))}
          </nav>

          <div className="ml-auto flex items-center gap-1">
            <Button
              size="sm"
              variant="ghost"
              className="hidden md:inline-flex"
              onClick={() => navigate('/')}
            >
              站点
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="hidden md:inline-flex"
              onClick={logout}
            >
              退出
            </Button>
            <ThemeToggle />

            <Sheet>
              <SheetTrigger
                render={
                  <Button
                    variant="outline"
                    size="icon"
                    className="size-9 md:hidden"
                    aria-label="打开菜单"
                  />
                }
              >
                <Menu className="size-4" />
              </SheetTrigger>
              <SheetContent side="right" className="w-60 gap-0">
                <SheetHeader>
                  <SheetTitle className="text-sm">后台菜单</SheetTitle>
                </SheetHeader>
                <nav className="flex flex-col gap-1 px-3">
                  {navItems.map((item) => (
                    <SheetClose
                      key={item.to}
                      render={
                        <Button
                          size="lg"
                          variant={isActive(item.to) ? 'secondary' : 'ghost'}
                          className="justify-start"
                          aria-current={isActive(item.to) ? 'page' : undefined}
                        />
                      }
                      onClick={() => navigate(item.to)}
                    >
                      {item.label}
                    </SheetClose>
                  ))}
                </nav>
                <div className="px-3 py-3">
                  <Separator />
                </div>
                <div className="flex flex-col gap-1 px-3">
                  <SheetClose
                    render={<Button size="lg" variant="ghost" className="justify-start" />}
                    onClick={() => navigate('/')}
                  >
                    返回站点
                  </SheetClose>
                  <SheetClose
                    render={<Button size="lg" variant="outline" className="justify-start" />}
                    onClick={logout}
                  >
                    退出登录
                  </SheetClose>
                </div>
              </SheetContent>
            </Sheet>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}