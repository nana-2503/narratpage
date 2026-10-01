import { useEffect, useState } from 'react'
import { useTheme } from 'next-themes'
import { Check, Monitor, Moon, RotateCcw, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  DEFAULT_RADIUS,
  RADIUS_PRESETS,
  applyRadius,
  loadRadius,
  parseRadius,
  radiusRange,
  saveRadius,
} from '@/lib/theme-settings'

const THEME_MODES = [
  { value: 'system', label: '跟随系统', icon: Monitor },
  { value: 'light', label: '浅色', icon: Sun },
  { value: 'dark', label: '深色', icon: Moon },
] as const

/**
 * 主题面板：明暗模式 + 边框圆角。
 *
 * 用 Popover 而非 DropdownMenu：面板内含滑块与按钮等任意控件，
 * 而 Base UI 的 Menu 只允许 Menu.* 部件作为子节点。
 *
 * 两者都是「本设备上的显示偏好」，不属于账号资料；站点页与后台
 * 共用同一份 localStorage，因此收敛到这一个入口。
 */
export function ThemeMenu({ align = 'end' }: { align?: 'start' | 'center' | 'end' }) {
  const { theme, setTheme, resolvedTheme } = useTheme()
  const [radius, setRadius] = useState(loadRadius)

  // 圆角变化即时生效并持久化
  useEffect(() => {
    applyRadius(radius)
    saveRadius(radius)
  }, [radius])

  const isDark = resolvedTheme === 'dark'
  const radiusRem = parseRadius(radius)

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="主题设置"
            title="主题设置"
          />
        }
      >
        {isDark ? <Moon className="size-4" /> : <Sun className="size-4" />}
      </PopoverTrigger>

      <PopoverContent align={align} className="w-64">
        <div className="flex flex-col gap-4">
          <section>
            <h3 className="mb-1.5 text-xs font-medium text-muted-foreground">外观模式</h3>
            <div className="grid grid-cols-3 gap-1" role="radiogroup" aria-label="外观模式">
              {THEME_MODES.map(({ value, label, icon: Icon }) => {
                const active = (theme ?? 'system') === value
                return (
                  <Button
                    key={value}
                    variant={active ? 'secondary' : 'ghost'}
                    role="radio"
                    aria-checked={active}
                    aria-label={label}
                    title={label}
                    // 图标在上、标签在下，需要 h-auto 让内容撑开：
                    // 固定高度（size 默认的 h-8）会把文字裁掉 3px。
                    className="h-auto flex-col gap-1 px-1 py-2 text-xs"
                    onClick={() => setTheme(value)}
                  >
                    <Icon className="size-4" />
                    {label}
                  </Button>
                )
              })}
            </div>
          </section>

          <section>
            <div className="mb-1.5 flex items-center justify-between">
              <h3 className="text-xs font-medium text-muted-foreground">边框圆角</h3>
            </div>

            {/* 6 个预设按 3 列排布，避免换行造成的参差 */}
            <div className="grid grid-cols-3 gap-1">
              {RADIUS_PRESETS.map((p) => (
                <Button
                  key={p.value}
                  size="sm"
                  variant={radius === p.value ? 'secondary' : 'ghost'}
                  aria-pressed={radius === p.value}
                  className="h-8 px-1 text-xs sm:h-7"
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
              onValueChange={(v) =>
                setRadius(`${(Array.isArray(v) ? v[0] : v) ?? radiusRem}rem`)
              }
              aria-label="圆角大小"
              className="mt-3"
            />

            {/* 实时预览：方块与按钮同步反映当前圆角 */}
            <div className="mt-1 flex items-center gap-2">
              <div
                className="size-6 shrink-0 border border-border bg-muted"
                style={{ borderRadius: radius }}
                aria-hidden
              />
              <Button size="sm" variant="outline" className="h-6 px-2 text-xs" disabled>
                按钮
              </Button>
              <span className="truncate text-xs text-muted-foreground tabular-nums">
                {radius}
              </span>
              <Button
                size="sm"
                variant="ghost"
                className="ml-auto h-8 shrink-0 px-1.5 text-xs sm:h-6"
                disabled={radius === DEFAULT_RADIUS}
                onClick={() => setRadius(DEFAULT_RADIUS)}
              >
                {radius === DEFAULT_RADIUS ? (
                  <Check className="size-3" />
                ) : (
                  <RotateCcw className="size-3" />
                )}
                默认
              </Button>
            </div>
          </section>
        </div>
      </PopoverContent>
    </Popover>
  )
}
