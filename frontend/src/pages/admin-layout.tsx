import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate, Outlet } from 'react-router-dom'
import { ExternalLink, Home, Menu, LogOut } from 'lucide-react'
import { getToken, setToken } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
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

export default function AdminLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const { user, ready, logout } = useAuth()
  const [menuOpen, setMenuOpen] = useState(false)

  // 未登录时跳转登录页。ready 为 false 表示尚未校验完 token，
  // 此时不能判定为未登录，否则刷新会先闪一下再跳走。
  useEffect(() => {
    if (!ready) return
    if (!getToken() || !user) {
      navigate('/admin/login', { replace: true, state: { from: location.pathname } })
    }
  }, [ready, user, navigate, location.pathname])

  useEffect(() => {
    setMenuOpen(false)
  }, [location.pathname])

  if (!ready || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        加载中
      </div>
    )
  }

  const navItems = buildNav(user.role)

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-30 border-b border-border bg-background/95 backdrop-blur-sm">
        <div className="mx-auto flex h-12 max-w-6xl items-center gap-1 px-4">
          <Link to="/admin/dashboard" className="mr-1 shrink-0 text-sm font-semibold tracking-tight">
            后台
          </Link>

          <nav className="hidden items-center gap-1 md:flex">
            {navItems.map((item) => (
              <Button
                key={item.to}
                size="sm"
                variant={isActive(location.pathname, item.to) ? 'secondary' : 'ghost'}
                aria-current={isActive(location.pathname, item.to) ? 'page' : undefined}
                onClick={() => navigate(item.to)}
              >
                {item.label}
              </Button>
            ))}
          </nav>

          <div className="ml-auto flex items-center gap-1">
            <span className="hidden max-w-32 truncate text-xs text-muted-foreground lg:inline">
              {user.display_name}
            </span>
            <Button
              size="sm"
              variant="ghost"
              className="hidden md:inline-flex"
              onClick={() => navigate('/')}
            >
              <ExternalLink className="size-4" />
              站点
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="hidden md:inline-flex"
              onClick={() => void logout()}
            >
              <LogOut className="size-4" />
              退出
            </Button>
            <ThemeMenu />

            <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
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
                  <SheetTitle className="truncate text-sm">
                    {user.display_name}
                  </SheetTitle>
                </SheetHeader>
                <nav className="flex flex-col gap-1 px-3">
                  {navItems.map((item) => (
                    <SheetClose
                      key={item.to}
                      render={
                        <Button
                          size="lg"
                          variant="ghost"
                          className="justify-start"
                          aria-label={item.label}
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
                    render={
                      <Button size="lg" variant="ghost" className="justify-start" />
                    }
                    onClick={() => navigate('/')}
                  >
                    <Home className="size-4" />
                    返回站点
                  </SheetClose>
                  <SheetClose
                    render={<Button size="lg" variant="outline" className="justify-start" />}
                    onClick={() => {
                      // 本地状态先清，避免登出请求失败时卡在后台
                      setToken(null)
                      void logout()
                    }}
                  >
                    <LogOut className="size-4" />
                    退出登录
                  </SheetClose>
                </div>
              </SheetContent>
            </Sheet>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}

interface NavItem {
  to: string
  label: string
}

/** 按角色裁剪导航项：避免展示点进去就 403 的入口 */
function buildNav(role: string): NavItem[] {
  const isAdmin = role === 'admin'
  const canWrite = isAdmin || role === 'editor' || role === 'author' || role === 'contributor'
  const canModerate = isAdmin || role === 'editor'

  const items: NavItem[] = [{ to: '/admin/dashboard', label: '概览' }]
  if (canWrite) {
    items.push(
      { to: '/admin/posts', label: '文章' },
      { to: '/admin/pages', label: '页面' },
    )
  }
  if (canModerate) {
    items.push({ to: '/admin/comments', label: '评论' })
  }
  if (isAdmin) {
    items.push(
      { to: '/admin/categories', label: '分类' },
      { to: '/admin/tags', label: '标签' },
      { to: '/admin/media', label: '媒体' },
      { to: '/admin/users', label: '用户' },
    )
  }
  if (canWrite) {
    items.push({ to: '/admin/trash', label: '回收站' })
  }
  if (isAdmin) {
    items.push(
      { to: '/admin/redirects', label: '重定向' },
      { to: '/admin/settings', label: '设置' },
    )
  }
  items.push({ to: '/admin/account', label: '账号' })
  return items
}

function isActive(pathname: string, to: string): boolean {
  if (to === '/admin/dashboard') return pathname === to
  return pathname === to || pathname.startsWith(`${to}/`)
}
