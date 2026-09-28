import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Database, Lock, Globe, Rocket } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Switch } from '@/components/ui/switch'
import { toast } from 'sonner'
import { api, getInstalled, setInstalled } from '@/lib/api'

type Step = 'welcome' | 'database' | 'account' | 'finish'

const dbTypes = [
  { value: 'sqlite', label: 'SQLite（默认，单文件）', description: '零运维，适合个人博客' },
  { value: 'mysql', label: 'MySQL', description: '适合已有 MySQL 环境' },
  { value: 'pgsql', label: 'PostgreSQL', description: '适合已有 PostgreSQL 环境' },
] as const

export default function Install() {
  const navigate = useNavigate()
  const [step, setStep] = useState<Step>('welcome')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  // 表单数据
  const [dbType, setDbType] = useState('sqlite')
  const [dbDsn, setDbDsn] = useState('')
  const [adminUsername, setAdminUsername] = useState('admin')
  const [adminPassword, setAdminPassword] = useState('')
  const [siteUrl, setSiteUrl] = useState('')
  const [redisEnabled, setRedisEnabled] = useState(false)
  const [redisUrl, setRedisUrl] = useState('')

  useEffect(() => {
    // 如果前端已标记为已安装，直接跳转
    if (getInstalled()) {
      navigate('/', { replace: true })
      return
    }

    // 检查后端安装状态
    api.installStatus()
      .then((status) => {
        if (status.installed) {
          setInstalled(true)
          navigate('/', { replace: true })
          return
        }
        // 预填站点 URL
        if (typeof window !== 'undefined') {
          setSiteUrl(window.location.origin)
        }
        setStep('database')
      })
      .catch(() => {
        toast.error('无法连接服务器，请检查后端是否启动')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [navigate])

  const handleSubmit = async () => {
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
        siteUrl: siteUrl || (typeof window !== 'undefined' ? window.location.origin : ''),
        redisEnabled,
        redisUrl,
      })
      // 保存配置到 localStorage（后续可在设置页查看）
      if (res.config) {
        localStorage.setItem('blog_config', JSON.stringify(res.config))
      }
      setInstalled(true)
      toast.success('安装完成！')
      navigate('/', { replace: true })
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '安装失败'
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <p className="text-sm text-muted-foreground">检查安装状态…</p>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-background px-4">
      <div className="w-full max-w-lg">
        {/* Logo / 标题 */}
        <div className="mb-8 text-center">
          <h1 className="text-2xl font-bold tracking-tight">叙页博客</h1>
          <p className="mt-1 text-sm text-muted-foreground">首次使用，请完成安装向导</p>
        </div>

        {/* 步骤指示器 */}
        <div className="mb-8 flex items-center justify-center gap-2">
          {(['database', 'account', 'finish'] as const).map((s, i) => (
            <div key={s} className="flex items-center gap-2">
              <div
                className={`flex size-8 items-center justify-center rounded-full text-xs font-medium ${
                  step === s
                    ? 'bg-primary text-primary-foreground'
                    : i < ['database', 'account', 'finish'].indexOf(step)
                      ? 'bg-primary/20 text-primary'
                      : 'bg-muted text-muted-foreground'
                }`}
              >
                {i + 1}
              </div>
              {i < 2 && <div className="h-px w-8 bg-border" />}
            </div>
          ))}
        </div>

        <div className="rounded-lg border border-border bg-card p-6">
          {step === 'welcome' && (
            <div className="flex flex-col items-center gap-4 text-center">
              <Rocket className="size-12 text-primary" />
              <h2 className="text-lg font-semibold">欢迎使用叙页博客</h2>
              <p className="text-sm text-muted-foreground">
                本向导将帮助你配置数据库、创建管理员账号。
                <br />
                整个过程大约需要 1 分钟。
              </p>
              <Button onClick={() => setStep('database')} className="mt-2">
                开始安装
              </Button>
            </div>
          )}

          {step === 'database' && (
            <div className="flex flex-col gap-6">
              <div>
                <h2 className="flex items-center gap-2 text-base font-semibold">
                  <Database className="size-4" /> 选择数据库
                </h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  博客系统使用 SQLite 作为默认数据库，无需额外配置。
                  <br />
                  你也可以选择 MySQL 或 PostgreSQL。
                </p>
              </div>

              <RadioGroup value={dbType} onValueChange={setDbType}>
                {dbTypes.map((opt) => (
                  <div
                    key={opt.value}
                    className={`flex items-start gap-3 rounded-md border p-3 ${
                      dbType === opt.value ? 'border-primary bg-primary/5' : 'border-border'
                    }`}
                  >
                    <RadioGroupItem value={opt.value} id={opt.value} className="mt-0.5" />
                    <div className="flex-1">
                      <Label htmlFor={opt.value} className="text-sm font-medium">
                        {opt.label}
                      </Label>
                      <p className="text-xs text-muted-foreground">{opt.description}</p>
                    </div>
                  </div>
                ))}
              </RadioGroup>

              {dbType !== 'sqlite' && (
                <div className="flex flex-col gap-2">
                  <Label htmlFor="dbDsn" className="text-xs">数据库连接字符串 (DSN)</Label>
                  <Input
                    id="dbDsn"
                    placeholder={
                      dbType === 'mysql'
                        ? 'user:password@tcp(127.0.0.1:3306)/blog?charset=utf8mb4'
                        : 'postgres://user:password@127.0.0.1:5432/blog?sslmode=disable'
                    }
                    value={dbDsn}
                    onChange={(e) => setDbDsn(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    留空则使用默认连接参数，数据库需提前创建。
                  </p>
                </div>
              )}

              <div className="flex items-center justify-between rounded-md border border-border p-3">
                <div className="flex items-center gap-2">
                  <Globe className="size-4 text-muted-foreground" />
                  <div>
                    <Label className="text-sm font-medium">启用 Redis（可选）</Label>
                    <p className="text-xs text-muted-foreground">用于登录限流和多实例共享</p>
                  </div>
                </div>
                <Switch checked={redisEnabled} onCheckedChange={setRedisEnabled} />
              </div>

              {redisEnabled && (
                <div className="flex flex-col gap-2">
                  <Label htmlFor="redisUrl" className="text-xs">Redis 连接地址</Label>
                  <Input
                    id="redisUrl"
                    placeholder="redis://127.0.0.1:6379"
                    value={redisUrl}
                    onChange={(e) => setRedisUrl(e.target.value)}
                  />
                </div>
              )}

              <div className="flex justify-end">
                <Button onClick={() => setStep('account')}>下一步</Button>
              </div>
            </div>
          )}

          {step === 'account' && (
            <div className="flex flex-col gap-6">
              <div>
                <h2 className="flex items-center gap-2 text-base font-semibold">
                  <Lock className="size-4" /> 管理员账号
                </h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  设置后台管理账号，请妥善保管。
                </p>
              </div>

              <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="adminUsername" className="text-xs">用户名</Label>
                  <Input
                    id="adminUsername"
                    value={adminUsername}
                    onChange={(e) => setAdminUsername(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="adminPassword" className="text-xs">密码</Label>
                  <Input
                    id="adminPassword"
                    type="password"
                    value={adminPassword}
                    onChange={(e) => setAdminPassword(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">长度至少 6 位</p>
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="siteUrl" className="text-xs">站点地址</Label>
                  <Input
                    id="siteUrl"
                    placeholder="https://example.com"
                    value={siteUrl}
                    onChange={(e) => setSiteUrl(e.target.value)}
                  />
                </div>
              </div>

              <div className="flex justify-between">
                <Button variant="outline" onClick={() => setStep('database')}>
                  上一步
                </Button>
                <Button onClick={handleSubmit} disabled={submitting}>
                  {submitting ? '安装中…' : '完成安装'}
                </Button>
              </div>
            </div>
          )}

          {step === 'finish' && (
            <div className="flex flex-col items-center gap-4 text-center">
              <Rocket className="size-12 text-primary" />
              <h2 className="text-lg font-semibold">安装成功！</h2>
              <p className="text-sm text-muted-foreground">
                博客系统已就绪，即将进入管理后台。
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
