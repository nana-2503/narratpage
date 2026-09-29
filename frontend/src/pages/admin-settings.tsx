import { useEffect, useState } from 'react'
import { useTheme } from 'next-themes'
import { Monitor, Moon, RotateCcw, Sun } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Slider } from '@/components/ui/slider'
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

const themeModes = [
  { value: 'system', label: '跟随系统', icon: Monitor },
  { value: 'light', label: '浅色', icon: Sun },
  { value: 'dark', label: '深色', icon: Moon },
] as const

/** 读取安装向导写入的运行配置（惰性初始化，避免 effect 内 setState） */
function loadRunConfig() {
  const fallback = { dbType: 'sqlite', redisEnabled: false, siteUrl: '' }
  try {
    const saved = localStorage.getItem('blog_config')
    if (!saved) return fallback
    const config = JSON.parse(saved)
    return {
      dbType: config.DB_TYPE || 'sqlite',
      redisEnabled: config.REDIS_ENABLED === 'true',
      siteUrl: config.SITE_URL || '',
    }
  } catch {
    return fallback
  }
}

export default function AdminSettings() {
  const { theme, setTheme } = useTheme()
  const [radius, setRadius] = useState(loadRadius)
  const radiusRem = parseRadius(radius)
  const [runConfig] = useState(loadRunConfig)
  const { dbType, redisEnabled, siteUrl } = runConfig

  // 圆角变化即时生效（CSS 变量）并持久化
  useEffect(() => {
    applyRadius(radius)
    saveRadius(radius)
  }, [radius])

  return (
    <div className="flex flex-col gap-8">
      <h1 className="text-sm font-semibold">设置</h1>

      <section className="flex flex-col gap-3">
        <h2 className="text-xs font-medium text-muted-foreground">运行环境</h2>
        <dl className="grid grid-cols-2 gap-x-4 gap-y-3 rounded-md border border-border p-4 text-sm sm:grid-cols-3">
          <div className="min-w-0">
            <dt className="text-xs text-muted-foreground">数据库</dt>
            <dd className="mt-0.5 truncate font-medium">{dbType}</dd>
          </div>
          <div className="min-w-0">
            <dt className="text-xs text-muted-foreground">Redis</dt>
            <dd className="mt-0.5 truncate font-medium">{redisEnabled ? '已启用' : '未启用'}</dd>
          </div>
          <div className="col-span-2 min-w-0 sm:col-span-1">
            <dt className="text-xs text-muted-foreground">站点地址</dt>
            <dd className="mt-0.5 truncate font-medium" title={siteUrl || undefined}>
              {siteUrl || '—'}
            </dd>
          </div>
        </dl>
      </section>

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
    </div>
  )
}