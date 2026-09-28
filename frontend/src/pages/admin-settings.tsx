import { useEffect, useState } from 'react'
import { useTheme } from 'next-themes'
import { Monitor, Moon, RotateCcw, Sun, Database } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Slider } from '@/components/ui/slider'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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

export default function AdminSettings() {
  const { theme, setTheme } = useTheme()
  const [radius, setRadius] = useState(loadRadius)
  const radiusRem = parseRadius(radius)
  const [dbType, setDbType] = useState('sqlite')
  const [redisEnabled, setRedisEnabled] = useState(false)
  const [siteUrl, setSiteUrl] = useState('')

  // 圆角变化即时生效（CSS 变量）并持久化
  useEffect(() => {
    applyRadius(radius)
    saveRadius(radius)
  }, [radius])

  useEffect(() => {
    // 加载安装时保存的配置
    try {
      const saved = localStorage.getItem('blog_config')
      if (saved) {
        const config = JSON.parse(saved)
        setDbType(config.DB_TYPE || 'sqlite')
        setRedisEnabled(config.REDIS_ENABLED === 'true')
        setSiteUrl(config.SITE_URL || '')
      }
    } catch {
      // ignore
    }
  }, [])

  return (
    <div className="flex max-w-2xl flex-col gap-8">
      <h1 className="text-sm font-semibold">设置</h1>

      {/* 系统信息 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            <Database className="size-4" /> 系统信息
          </CardTitle>
          <CardDescription>当前运行环境与存储配置</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">数据库</span>
              <p className="mt-0.5 font-medium capitalize">{dbType}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Redis</span>
              <p className="mt-0.5 font-medium">{redisEnabled ? '已启用' : '未启用'}</p>
            </div>
            <div className="col-span-2">
              <span className="text-muted-foreground">站点地址</span>
              <p className="mt-0.5 font-medium truncate">{siteUrl || '-'}</p>
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            如需修改数据库或 Redis 配置，请更新环境变量后重启服务。
          </p>
        </CardContent>
      </Card>

      {/* 外观设置 */}
      <section className="flex flex-col gap-3">
        <h2 className="text-xs font-medium text-muted-foreground">外观模式</h2>
        <div className="flex gap-1">
          {themeModes.map(({ value, label, icon: Icon }) => (
            <Button
              key={value}
              size="sm"
              variant={theme === value ? 'secondary' : 'ghost'}
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
          <Input placeholder="输入框" className="w-40" readOnly />
          <div className="size-12 rounded-md border border-border bg-muted" />
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
  )
}
