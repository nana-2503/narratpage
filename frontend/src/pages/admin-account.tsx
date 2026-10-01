import { useEffect, useState } from 'react'
import { KeyRound, LogOut, Monitor, Save, Sun, Moon, RotateCcw } from 'lucide-react'
import { useTheme } from 'next-themes'
import { api, type SessionInfo } from '@/lib/api'
import { formatDateTime } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { useAuth } from '@/hooks/use-auth'
import {
  DEFAULT_RADIUS,
  RADIUS_PRESETS,
  applyRadius,
  loadRadius,
  parseRadius,
  radiusRange,
  saveRadius,
} from '@/lib/theme-settings'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import { PageHeader } from '@/components/admin-page-header'
import { ConfirmDialog } from '@/components/confirm-dialog'

const themeModes = [
  { value: 'system', label: '跟随系统', icon: Monitor },
  { value: 'light', label: '浅色', icon: Sun },
  { value: 'dark', label: '深色', icon: Moon },
] as const

const ROLES: Record<string, string> = {
  admin: '管理员',
  editor: '编辑',
  author: '作者',
  contributor: '贡献者',
  subscriber: '订阅者',
}

export default function AdminAccount() {
  const { user, logout } = useAuth()
  const { theme, setTheme } = useTheme()

  const [displayName, setDisplayName] = useState(user?.display_name ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [bio, setBio] = useState(user?.bio ?? '')
  const [avatarUrl, setAvatarUrl] = useState(user?.avatar_url ?? '')
  const [savingProfile, setSavingProfile] = useState(false)

  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [savingPassword, setSavingPassword] = useState(false)
  const [pendingRevoke, setPendingRevoke] = useState(false)

  const [radius, setRadius] = useState(loadRadius)
  const radiusRem = parseRadius(radius)

  // 圆角变化即时生效（写入 CSS 变量）并持久化
  useEffect(() => {
    applyRadius(radius)
    saveRadius(radius)
  }, [radius])

  const { data: sessionsData, reload: reloadSessions } = useAsync(
    () => api.sessions(),
    [],
  )
  const sessions = sessionsData?.items ?? []
  const otherSessions = sessions.filter((s: SessionInfo) => !s.current)

  const saveProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    setSavingProfile(true)
    try {
      await api.updateMe({
        display_name: displayName.trim(),
        email: email.trim(),
        bio,
        avatar_url: avatarUrl.trim(),
      })
      toast.success('资料已更新')
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setSavingProfile(false)
    }
  }

  const savePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      toast.error('两次输入的新密码不一致')
      return
    }
    if (newPassword.length < 8) {
      toast.error('新密码至少 8 位')
      return
    }
    setSavingPassword(true)
    try {
      await api.changePassword(oldPassword, newPassword)
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
      toast.success('密码已修改，其它设备的登录已失效')
      reloadSessions()
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setSavingPassword(false)
    }
  }

  const revokeOthers = async () => {
    try {
      const res = await api.revokeOtherSessions()
      toast.success(`已登出 ${res.revoked} 个其它会话`)
      reloadSessions()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader
        title="账号"
        actions={
          <Button size="sm" variant="outline" onClick={() => void logout()}>
            <LogOut className="size-4" />
            退出登录
          </Button>
        }
      />

      {user && (
        <p className="mt-1 text-xs text-muted-foreground">
          @{user.username} · {ROLES[user.role] ?? user.role} · 注册于{' '}
          {formatDateTime(user.created_at ?? null)}
        </p>
      )}

      <form onSubmit={saveProfile} className="mt-6 flex max-w-xl flex-col gap-4">
        <h2 className="text-xs font-medium text-muted-foreground">个人资料</h2>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="a-display">显示名</Label>
            <Input
              id="a-display"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              maxLength={64}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="a-email">邮箱</Label>
            <Input
              id="a-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="a-avatar">头像地址</Label>
          <Input
            id="a-avatar"
            value={avatarUrl}
            onChange={(e) => setAvatarUrl(e.target.value)}
            placeholder="https://..."
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="a-bio">简介</Label>
          <Textarea
            id="a-bio"
            value={bio}
            onChange={(e) => setBio(e.target.value)}
            rows={3}
            maxLength={2000}
          />
        </div>
        <div>
          <Button type="submit" size="sm" disabled={savingProfile}>
            <Save className="size-4" />
            {savingProfile ? '保存中' : '保存资料'}
          </Button>
        </div>
      </form>

      <Separator className="my-8" />

      <form onSubmit={savePassword} className="flex max-w-xl flex-col gap-4">
        <h2 className="text-xs font-medium text-muted-foreground">修改密码</h2>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="p-old">当前密码</Label>
          <Input
            id="p-old"
            type="password"
            value={oldPassword}
            onChange={(e) => setOldPassword(e.target.value)}
            autoComplete="current-password"
          />
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="p-new">新密码</Label>
            <Input
              id="p-new"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              autoComplete="new-password"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="p-confirm">确认新密码</Label>
            <Input
              id="p-confirm"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              autoComplete="new-password"
            />
          </div>
        </div>
        <div>
          <Button
            type="submit"
            size="sm"
            variant="outline"
            disabled={savingPassword || !oldPassword || newPassword.length < 8}
          >
            <KeyRound className="size-4" />
            {savingPassword ? '提交中' : '修改密码'}
          </Button>
        </div>
      </form>

      <Separator className="my-8" />

      <section>
        <div className="flex items-center justify-between">
          <h2 className="text-xs font-medium text-muted-foreground">登录会话</h2>
          {otherSessions.length > 0 && (
            <Button size="sm" variant="ghost" onClick={() => setPendingRevoke(true)}>
              登出其它设备
            </Button>
          )}
        </div>
        <ul className="mt-2 overflow-hidden rounded-md border border-border">
          {sessions.length === 0 && (
            <li className="px-3 py-4 text-sm text-muted-foreground">暂无会话记录</li>
          )}
          {sessions.map((s, i) => (
            <li
              key={s.id}
              className={`px-3 py-2.5 ${i > 0 ? 'border-t border-border' : ''}`}
            >
              <div className="flex items-center gap-2">
                <span className="min-w-0 flex-1 truncate text-sm" title={s.user_agent}>
                  {s.ip}
                </span>
                {s.current && <Badge className="shrink-0">当前</Badge>}
                <span className="hidden shrink-0 text-xs text-muted-foreground tabular-nums sm:inline">
                  {formatDateTime(s.last_seen_at)}
                </span>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <Separator className="my-8" />

      <div className="grid items-start gap-8 lg:grid-cols-2">
        <section className="flex flex-col gap-3">
          <h2 className="text-xs font-medium text-muted-foreground">外观模式</h2>
          <div className="flex flex-wrap gap-1">
            {themeModes.map(({ value, label, icon: Icon }) => (
              <Button
                key={value}
                size="sm"
                variant={theme === value ? 'secondary' : 'ghost'}
                aria-pressed={theme === value}
                onClick={() => setTheme(value)}
              >
                <Icon className="size-4" /> {label}
              </Button>
            ))}
          </div>
        </section>

        <section className="flex flex-col gap-3">
          <div className="flex items-center justify-between">
            <h2 className="text-xs font-medium text-muted-foreground">边框圆角</h2>
            <span className="text-xs text-muted-foreground tabular-nums">{radius}</span>
          </div>

          <div className="flex flex-wrap gap-1">
            {RADIUS_PRESETS.map((p) => (
              <Button
                key={p.value}
                size="sm"
                variant={radius === p.value ? 'secondary' : 'ghost'}
                aria-pressed={radius === p.value}
                onClick={() => setRadius(p.value)}
              >
                {p.label}
              </Button>
            ))}
          </div>

          <Slider
            value={[radiusRem]}
            min={radiusRange.min}
            max={radiusRange.max}
            step={radiusRange.step}
            onValueChange={(v) => setRadius(`${(Array.isArray(v) ? v[0] : v) ?? radiusRem}rem`)}
            aria-label="圆角大小"
          />

          {/* 实时预览：直接反映当前圆角 */}
          <div className="flex flex-wrap items-center gap-3 rounded-md border border-border p-4">
            <Button size="sm">按钮</Button>
            <Badge variant="outline">标签</Badge>
            <Input placeholder="输入框" className="w-32" readOnly tabIndex={-1} />
            <div className="size-10 rounded-md border border-border bg-muted" />
          </div>

          <div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setRadius(DEFAULT_RADIUS)
                toast.success('已恢复默认圆角')
              }}
            >
              <RotateCcw className="size-4" /> 恢复默认
            </Button>
          </div>
        </section>
      </div>

      <ConfirmDialog
        open={pendingRevoke}
        onOpenChange={setPendingRevoke}
        title="登出其它设备"
        description={`将结束 ${otherSessions.length} 个其它登录会话，当前设备不受影响。`}
        confirmText="登出"
        onConfirm={revokeOthers}
      />
    </div>
  )
}
