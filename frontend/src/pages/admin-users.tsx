import { useState } from 'react'
import { Plus, Trash2, Users } from 'lucide-react'
import { api, type AuthUser, type UserRole } from '@/lib/api'
import { formatRelative } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { useAuth } from '@/hooks/use-auth'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,  toSelectItems,
} from '@/components/ui/select'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { PageHeader } from '@/components/admin-page-header'
import { EmptyState } from '@/components/empty-state'
import { ConfirmDialog } from '@/components/confirm-dialog'

const ROLES: { value: UserRole; label: string }[] = [
  { value: 'admin', label: '管理员' },
  { value: 'editor', label: '编辑' },
  { value: 'author', label: '作者' },
  { value: 'contributor', label: '贡献者' },
  { value: 'subscriber', label: '订阅者' },
]

export default function AdminUsers() {
  const { user: me } = useAuth()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [search, setSearch] = useState('')
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<AuthUser | null>(null)
  const [pendingDelete, setPendingDelete] = useState<AuthUser | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  const { data, error, loading } = useAsync(
    () => api.listUsers({ page, pageSize: 20, q: search || undefined }),
    [page, search, reloadKey],
  )

  const users = data?.items ?? []
  const refresh = () => setReloadKey((k) => k + 1)

  const toggleActive = async (u: AuthUser) => {
    try {
      await api.updateUser(u.id, { active: !u.active })
      toast.success(u.active ? '已停用' : '已启用')
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deleteUser(pendingDelete.id)
      toast.success('已删除')
      setPendingDelete(null)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader
        title="用户"
        count={data?.total}
        actions={
          <Button size="sm" onClick={() => setCreating(true)}>
            <Plus className="size-4" />
            新建
          </Button>
        }
      />

      <form
        className="mt-4 flex gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          setPage(1)
          setSearch(keyword.trim())
        }}
      >
        <input
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索用户名"
          aria-label="搜索用户"
          className="h-8 flex-1 rounded-md border border-input bg-transparent px-2 text-sm"
        />
        <Button type="submit" size="sm" variant="outline">
          搜索
        </Button>
      </form>

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {users.length === 0 && !loading && (
        <EmptyState icon={Users} message="没有匹配的用户" />
      )}

      {users.length > 0 && (
        <ul className="mt-4 overflow-hidden rounded-md border border-border">
          {users.map((u, i) => (
            <li
              key={u.id}
              className={`flex items-center gap-2 px-3 py-2.5 ${
                i > 0 ? 'border-t border-border' : ''
              }`}
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm">
                  {u.display_name}
                  {u.id === me?.id && (
                    <span className="ml-1.5 text-xs text-muted-foreground">（我）</span>
                  )}
                </p>
                <p className="truncate text-xs text-muted-foreground">
                  @{u.username}
                  {u.email ? ` · ${u.email}` : ''}
                  {u.last_login_at ? ` · ${formatRelative(u.last_login_at)}登录` : ''}
                </p>
              </div>
              <Badge variant="outline" className="shrink-0">
                {ROLES.find((r) => r.value === u.role)?.label ?? u.role}
              </Badge>
              <div className="flex shrink-0 items-center">
                <div className="flex items-center gap-1.5">
                  <Switch
                    checked={u.active !== false}
                    onCheckedChange={() => void toggleActive(u)}
                    aria-label={`${u.active !== false ? '停用' : '启用'} ${u.username}`}
                  />
                </div>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setEditing(u)}
                  aria-label={`编辑 ${u.username}`}
                >
                  编辑
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-7 text-destructive"
                  onClick={() => setPendingDelete(u)}
                  aria-label={`删除 ${u.username}`}
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}

      {data && data.totalPages > 1 && (
        <div className="mt-4 flex items-center justify-between text-sm">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
            上一页
          </Button>
          <span className="text-xs text-muted-foreground tabular-nums">
            {page} / {data.totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= data.totalPages}
            onClick={() => setPage(page + 1)}
          >
            下一页
          </Button>
        </div>
      )}

      <UserDialog
        open={creating}
        onOpenChange={setCreating}
        onSaved={() => {
          setCreating(false)
          refresh()
        }}
      />

      <UserDialog
        open={!!editing}
        user={editing}
        onOpenChange={(open) => !open && setEditing(null)}
        onSaved={() => {
          setEditing(null)
          refresh()
        }}
      />

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="删除用户"
        description={`删除后该用户的文章将变为无作者，确定删除「${pendingDelete?.display_name}」？`}
        onConfirm={remove}
      />
    </div>
  )
}

function UserDialog({
  open,
  user,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  user?: AuthUser | null
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const isEdit = Boolean(user)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [role, setRole] = useState<UserRole>('author')
  const [busy, setBusy] = useState(false)
  const [loadedId, setLoadedId] = useState<number | null>(null)

  if (user && user.id !== loadedId) {
    setLoadedId(user.id)
    setUsername(user.username)
    setEmail(user.email ?? '')
    setDisplayName(user.display_name)
    setRole(user.role)
    setPassword('')
  }
  if (!user && open && loadedId !== null) {
    setLoadedId(null)
    setUsername('')
    setEmail('')
    setDisplayName('')
    setRole('author')
    setPassword('')
  }

  const save = async () => {
    setBusy(true)
    try {
      if (isEdit && user) {
        await api.updateUser(user.id, {
          email: email.trim(),
          display_name: displayName.trim(),
          role,
          ...(password ? { password } : {}),
        })
        toast.success('已保存')
      } else {
        await api.createUser({
          username: username.trim(),
          password,
          email: email.trim(),
          display_name: displayName.trim(),
          role,
        })
        toast.success('已创建')
      }
      onSaved()
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="shadow-none sm:max-w-96">
        <DialogHeader>
          <DialogTitle>{isEdit ? '编辑用户' : '新建用户'}</DialogTitle>
          <DialogDescription>
            {isEdit ? '留空密码则不修改密码。' : '密码至少 8 位。'}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          {!isEdit && (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="u-username">用户名</Label>
              <Input
                id="u-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="字母、数字、点、下划线、连字符"
                required
              />
            </div>
          )}
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="u-password">{isEdit ? '新密码（选填）' : '密码'}</Label>
            <Input
              id="u-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="u-display">显示名</Label>
            <Input
              id="u-display"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              maxLength={64}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="u-email">邮箱</Label>
            <Input
              id="u-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>角色</Label>
            <Select
              items={toSelectItems(ROLES)}
              value={role}
              onValueChange={(v) => setRole(v as UserRole)}
            >
              <SelectTrigger className="w-full" aria-label="角色">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLES.map((r) => (
                  <SelectItem key={r.value} value={r.value}>
                    {r.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" size="sm" />}>取消</DialogClose>
          <Button
            size="sm"
            onClick={() => void save()}
            disabled={busy || (!isEdit && (!username.trim() || password.length < 8))}
          >
            {busy ? '保存中' : '保存'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
