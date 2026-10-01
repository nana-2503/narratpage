import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { LogIn } from 'lucide-react'
import { getToken } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ThemeToggle } from '@/components/theme-toggle'

export default function AdminLogin() {
  const navigate = useNavigate()
  const location = useLocation()
  const { login, user, ready } = useAuth()

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  // 已登录时直接进入后台
  if (ready && user && getToken()) {
    const from = (location.state as { from?: string })?.from
    navigate(from && from !== '/admin/login' ? from : '/admin/dashboard', { replace: true })
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await login(username.trim(), password)
      const from = (location.state as { from?: string })?.from
      navigate(from && from !== '/admin/login' ? from : '/admin/dashboard', { replace: true })
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <div className="flex justify-end p-3">
        <ThemeToggle />
      </div>

      <div className="flex flex-1 items-center justify-center px-4 pb-16">
        <form
          onSubmit={submit}
          className="flex w-full max-w-xs flex-col gap-4"
        >
          <div>
            <h1 className="text-sm font-semibold">登录后台</h1>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="l-user">用户名</Label>
            <Input
              id="l-user"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
              required
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="l-pass">密码</Label>
            <Input
              id="l-pass"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
          </div>

          {error && <p className="text-xs text-destructive">{error}</p>}

          <Button type="submit" disabled={submitting || !username || !password}>
            <LogIn className="size-4" />
            {submitting ? '登录中' : '登录'}
          </Button>

          <Link
            to="/"
            className="text-center text-xs text-muted-foreground hover:text-foreground"
          >
            返回站点
          </Link>
        </form>
      </div>
    </div>
  )
}
