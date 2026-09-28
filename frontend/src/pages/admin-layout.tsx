import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate, Outlet } from 'react-router-dom'
import { api, getToken, setToken } from '@/lib/api'
import { Button } from '@/components/ui/button'

const navItems = [
  { to: '/admin/posts', label: '文章' },
  { to: '/admin/comments', label: '评论' },
  { to: '/admin/categories', label: '分类' },
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

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="border-b border-border">
        <div className="mx-auto flex h-12 max-w-5xl items-center gap-3 px-4">
          <Link to="/admin/posts" className="text-sm font-semibold tracking-tight">
            后台
          </Link>
          <nav className="ml-auto flex items-center gap-1 overflow-x-auto">
            {navItems.map((item) => (
              <Button
                key={item.to}
                size="sm"
                variant={location.pathname.startsWith(item.to) ? 'secondary' : 'ghost'}
                onClick={() => navigate(item.to)}
              >
                {item.label}
              </Button>
            ))}
            <Button size="sm" variant="outline" onClick={logout}>
              退出
            </Button>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
