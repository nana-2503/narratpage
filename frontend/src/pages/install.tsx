import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Check, Database, Lock, Server } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'
import { api, getInstalled, setInstalled } from '@/lib/api'

type Step = 'database' | 'account'

const steps: { value: Step; label: string }[] = [
  { value: 'database', label: '数据库' },
  { value: 'account', label: '管理员' },
]

const dbTypes = [
  { value: 'sqlite', label: 'SQLite', hint: '单文件，零运维' },
  { value: 'mysql', label: 'MySQL', hint: '已有 MySQL 环境' },
  { value: 'pgsql', label: 'PostgreSQL', hint: '已有 PostgreSQL 环境' },
] as const

export default function Install() {
  const navigate = useNavigate()
  const [checking, setChecking] = useState(true)
  const [step, setStep] = useState<Step>('database')
  const [submitting, setSubmitting] = useState(false)

  const [dbType, setDbType] = useState('sqlite')
  const [dbDsn, setDbDsn] = useState('')
  const [adminUsername, setAdminUsername] = useState('admin')
  const [adminPassword, setAdminPassword] = useState('')
  const [siteUrl, setSiteUrl] = useState('')
  const [redisEnabled, setRedisEnabled] = useState(false)
  const [redisUrl, setRedisUrl] = useState('')

  useEffect(() => {
    // 前端已标记完成则直接放行，避免重复检查
    if (getInstalled()) {
      navigate('/', { replace: true })
      return
    }
    api
      .installStatus()
      .then((status) => {
        if (status.installed) {
          setInstalled(true)
          navigate('/', { replace: true })
          return
        }
        setSiteUrl(window.location.origin)
      })
      .catch(() => toast.error('无法连接服务器，请检查后端是否启动'))
      .finally(() => setChecking(false))
  }, [navigate])

  const submit = async () => {
    if (adminPassword.length < 6) {
      toast.error('密码长度至少 6 位')
      return
    }
    setSubmitting(true)
    try {
      const res = await api.install({
        dbType,
        dbDsn: dbType === 'sqlite' ? '' : dbDsn,
        adminUsername,
        adminPassword,
        siteUrl: siteUrl || window.location.origin,
        redisEnabled,
        redisUrl,
      })
      if (res.config) localStorage.setItem('blog_config', JSON.stringify(res.config))
      setInstalled(true)
      toast.success('安装完成')
      navigate('/', { replace: true })
    } catch (e) {
      toast.error(e instanceof Error ? e.message : '安装失败')
    } finally {
      setSubmitting(false)
    }
  }

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        检查安装状态…
      </div>
    )
  }

  const stepIndex = steps.findIndex((s) => s.value === step)

  return (
    <div className="flex min-h-screen flex-col items-center bg-background px-4 py-10 text-foreground sm:justify-center sm:py-16">
      <div className="w-full max-w-md">
        <header className="mb-6 text-center">
          <span className="inline-flex size-9 items-center justify-center rounded-md border border-border text-sm font-semibold">
            叙
          </span>
          <h1 className="mt-3 text-lg font-semibold tracking-tight">安装向导</h1>
        </header>

        {/* 步骤指示 */}
        <ol className="mb-5 flex items-center gap-2">
          {steps.map((s, i) => {
            const done = i < stepIndex
            const active = i === stepIndex
            return (
              <li key={s.value} className="flex flex-1 items-center gap-2">
                <span
                  className={cn(
                    "flex size-6 shrink-0 items-center justify-center rounded-full border text-xs tabular-nums",
                    active
                      ? "border-primary bg-primary text-primary-foreground"
                      : done
                        ? "border-primary text-primary"
                        : "border-border text-muted-foreground",
                  )}
                >
                  {done ? <Check className="size-3.5" /> : i + 1}
                </span>
                <span
                  className={cn(
                    "shrink-0 text-xs",
                    active || done ? "text-foreground" : "text-muted-foreground",
                  )}
                >
                  {s.label}
                </span>
                {i < steps.length - 1 && <span className="h-px flex-1 bg-border" />}
              </li>
            )
          })}
        </ol>

        <div className="rounded-md border border-border p-4">
          {step === 'database' ? (
            <div className="flex flex-col gap-5">
              <h2 className="flex items-center gap-2 text-sm font-medium">
                <Database className="size-4 text-muted-foreground" /> 选择数据库
              </h2>

              <RadioGroup
                name="db-type"
                value={dbType}
                onValueChange={setDbType}
                className="gap-2"
              >
                {dbTypes.map((opt) => (
                  <label
                    key={opt.value}
                    className={cn(
                      "flex cursor-pointer items-center gap-3 rounded-md border p-3 transition-colors",
                      dbType === opt.value
                        ? "border-foreground/30 bg-muted/60"
                        : "border-border hover:bg-muted/40",
                    )}
                  >
                    <RadioGroupItem value={opt.value} />
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm font-medium">{opt.label}</span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {opt.hint}
                      </span>
                    </span>
                  </label>
                ))}
              </RadioGroup>

              {dbType !== 'sqlite' && (
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="db-dsn">连接字符串</Label>
                  <Input
                    id="db-dsn"
                    spellCheck={false}
                    autoComplete="off"
                    placeholder={
                      dbType === 'mysql'
                        ? 'user:pass@tcp(127.0.0.1:3306)/blog?charset=utf8mb4'
                        : 'postgres://user:pass@127.0.0.1:5432/blog?sslmode=disable'
                    }
                    value={dbDsn}
                    onChange={(e) => setDbDsn(e.target.value)}
                  />
                </div>
              )}

              <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
                <span className="flex min-w-0 items-center gap-2">
                  <Server className="size-4 shrink-0 text-muted-foreground" />
                  <span className="min-w-0">
                    <span className="block text-sm font-medium">Redis</span>
                    <span className="block truncate text-xs text-muted-foreground">
                      登录限流与多实例共享
                    </span>
                  </span>
                </span>
                <Switch
                  checked={redisEnabled}
                  onCheckedChange={setRedisEnabled}
                  aria-label="启用 Redis"
                />
              </div>

              {redisEnabled && (
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="redis-url">Redis 地址</Label>
                  <Input
                    id="redis-url"
                    spellCheck={false}
                    autoComplete="off"
                    placeholder="redis://127.0.0.1:6379"
                    value={redisUrl}
                    onChange={(e) => setRedisUrl(e.target.value)}
                  />
                </div>
              )}

              <div className="flex justify-end pt-1">
                <Button onClick={() => setStep('account')}>下一步</Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-5">
              <h2 className="flex items-center gap-2 text-sm font-medium">
                <Lock className="size-4 text-muted-foreground" /> 管理员账号
              </h2>

              <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="admin-username">用户名</Label>
                  <Input
                    id="admin-username"
                    autoComplete="username"
                    value={adminUsername}
                    onChange={(e) => setAdminUsername(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="admin-password">密码（至少 6 位）</Label>
                  <Input
                    id="admin-password"
                    type="password"
                    autoComplete="new-password"
                    value={adminPassword}
                    onChange={(e) => setAdminPassword(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="site-url">站点地址</Label>
                  <Input
                    id="site-url"
                    inputMode="url"
                    spellCheck={false}
                    autoComplete="off"
                    placeholder="https://example.com"
                    value={siteUrl}
                    onChange={(e) => setSiteUrl(e.target.value)}
                  />
                </div>
              </div>

              <div className="flex justify-between pt-1">
                <Button variant="outline" onClick={() => setStep('database')}>
                  上一步
                </Button>
                <Button onClick={submit} disabled={submitting}>
                  {submitting ? '安装中…' : '完成安装'}
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}