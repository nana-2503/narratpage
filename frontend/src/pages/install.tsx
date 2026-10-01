import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Check, Database, Lock, Server } from 'lucide-react'
import { api, getInstalled, setInstalled } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Switch } from '@/components/ui/switch'
import { toast } from 'sonner'

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
  const [applied, setApplied] = useState(false)

  const [dbType, setDbType] = useState('sqlite')
  const [dbDsn, setDbDsn] = useState('')
  const [adminUsername, setAdminUsername] = useState('admin')
  const [adminPassword, setAdminPassword] = useState('')
  const [adminEmail, setAdminEmail] = useState('')
  const [siteUrl, setSiteUrl] = useState('')
  const [siteTitle, setSiteTitle] = useState('')
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
    if (adminPassword.length < 8) {
      toast.error('密码长度至少 8 位')
      return
    }
    setSubmitting(true)
    try {
      // 第一步：校验配置并试连数据库
      const res = await api.install({
        dbType,
        dbDsn: dbType === 'sqlite' ? '' : dbDsn,
        adminUsername: adminUsername.trim(),
        adminPassword,
        adminEmail: adminEmail.trim(),
        siteUrl: siteUrl || window.location.origin,
        siteTitle: siteTitle.trim(),
        redisEnabled,
        redisUrl,
      })

      // 第二步：若目标库就是当前库，可直接落库并完成安装。
      // 换库需要改环境变量并重启，此时只提示，不写当前库。
      const sameDB = res.dbType === (await api.installStatus()).dbType
      if (sameDB) {
        await api.installApply({
          adminUsername: adminUsername.trim(),
          adminPassword,
          adminEmail: adminEmail.trim(),
          siteUrl: siteUrl || window.location.origin,
          siteTitle: siteTitle.trim(),
        })
        setInstalled(true)
        toast.success('安装完成')
        navigate('/admin/login', { replace: true })
        return
      }

      // 跨库安装：展示 env 片段，由用户写入后重启
      setApplied(true)
    } catch (e) {
      toast.error((e as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        加载中
      </div>
    )
  }

  if (applied) {
    return (
      <EnvInstructions
        env={{
          DB_TYPE: dbType,
          DB_DSN: dbType === 'sqlite' ? '' : dbDsn,
          ADMIN_USERNAME: adminUsername,
          ADMIN_PASSWORD: adminPassword,
          SITE_URL: siteUrl,
        }}
        onDone={() => navigate('/admin/login', { replace: true })}
      />
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4 py-10 text-foreground">
      <div className="w-full max-w-md">
        <div className="flex items-center gap-2">
          {steps.map((s, i) => (
            <div key={s.value} className="flex items-center gap-2">
              {i > 0 && <span className="h-px w-6 bg-border" />}
              <button
                type="button"
                onClick={() => setStep(s.value)}
                className={`flex items-center gap-1.5 text-xs ${
                  step === s.value ? 'text-foreground' : 'text-muted-foreground'
                }`}
              >
                <span
                  className={`flex size-4 items-center justify-center rounded-full border text-[10px] ${
                    step === s.value ? 'border-foreground' : 'border-border'
                  }`}
                >
                  {steps.findIndex((x) => x.value === step) > i ? (
                    <Check className="size-2.5" />
                  ) : (
                    i + 1
                  )}
                </span>
                {s.label}
              </button>
            </div>
          ))}
        </div>

        {step === 'database' ? (
          <div className="mt-6 flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label>数据库类型</Label>
              <RadioGroup value={dbType} onValueChange={(v) => setDbType(v as string)}>
                {dbTypes.map((d) => (
                  <label
                    key={d.value}
                    className="flex cursor-pointer items-center gap-2.5 rounded-md border border-border px-3 py-2.5"
                  >
                    <RadioGroupItem value={d.value} id={`db-${d.value}`} />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm">{d.label}</p>
                      <p className="truncate text-xs text-muted-foreground">{d.hint}</p>
                    </div>
                  </label>
                ))}
              </RadioGroup>
            </div>

            {dbType !== 'sqlite' && (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="i-dsn">连接串</Label>
                <Input
                  id="i-dsn"
                  value={dbDsn}
                  onChange={(e) => setDbDsn(e.target.value)}
                  placeholder={
                    dbType === 'mysql'
                      ? 'user:pass@tcp(host:3306)/blog?charset=utf8mb4'
                      : 'postgres://user:pass@host:5432/blog?sslmode=disable'
                  }
                />
              </div>
            )}

            <div className="flex items-center justify-between rounded-md border border-border px-3 py-2.5">
              <div className="min-w-0">
                <p className="truncate text-sm">启用 Redis</p>
                <p className="truncate text-xs text-muted-foreground">
                  用于登录限流的多实例共享
                </p>
              </div>
              <Switch checked={redisEnabled} onCheckedChange={setRedisEnabled} />
            </div>

            {redisEnabled && (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="i-redis">Redis 地址</Label>
                <Input
                  id="i-redis"
                  value={redisUrl}
                  onChange={(e) => setRedisUrl(e.target.value)}
                  placeholder="redis://127.0.0.1:6379"
                />
              </div>
            )}

            <Button onClick={() => setStep('account')}>下一步</Button>
          </div>
        ) : (
          <div className="mt-6 flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="i-title">站点标题</Label>
              <Input
                id="i-title"
                value={siteTitle}
                onChange={(e) => setSiteTitle(e.target.value)}
                placeholder="叙页博客系统"
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="i-url">站点地址</Label>
              <Input
                id="i-url"
                value={siteUrl}
                onChange={(e) => setSiteUrl(e.target.value)}
                placeholder={window.location.origin}
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="i-user">管理员用户名</Label>
              <Input
                id="i-user"
                value={adminUsername}
                onChange={(e) => setAdminUsername(e.target.value)}
                autoComplete="username"
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="i-email">管理员邮箱（选填）</Label>
              <Input
                id="i-email"
                type="email"
                value={adminEmail}
                onChange={(e) => setAdminEmail(e.target.value)}
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="i-pass">密码</Label>
              <Input
                id="i-pass"
                type="password"
                value={adminPassword}
                onChange={(e) => setAdminPassword(e.target.value)}
                autoComplete="new-password"
              />
              <p className="text-xs text-muted-foreground">至少 8 位</p>
            </div>

            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setStep('database')}>
                上一步
              </Button>
              <Button
                onClick={() => void submit()}
                disabled={submitting || adminPassword.length < 8 || !adminUsername.trim()}
              >
                {submitting ? '安装中' : '完成安装'}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

/** 跨库安装：数据库类型需改环境变量并重启，给出可复制的配置片段 */
function EnvInstructions({
  env,
  onDone,
}: {
  env: Record<string, string>
  onDone: () => void
}) {
  const snippet = Object.entries(env)
    .filter(([, v]) => v !== '')
    .map(([k, v]) => `${k}=${v}`)
    .join('\n')

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4 py-10 text-foreground">
      <div className="w-full max-w-md">
        <div className="flex items-center gap-2">
          <Server className="size-4" />
          <h1 className="text-sm font-semibold">需要重启以切换数据库</h1>
        </div>

        <p className="mt-2 text-sm text-muted-foreground">
          把下面内容写入 .env，然后重启服务：
        </p>

        <pre className="mt-3 overflow-x-auto rounded-md border border-border p-3 text-xs">
          {snippet}
        </pre>

        <div className="mt-4 flex flex-col gap-2">
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Database className="size-3.5" />
            数据库类型与连接串在进程启动时读取，运行期无法切换
          </p>
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Lock className="size-3.5" />
            重启后用上面设置的用户名与密码登录
          </p>
        </div>

        <Button className="mt-4" onClick={onDone}>
          我已配置，去登录
        </Button>
      </div>
    </div>
  )
}
