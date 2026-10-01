import { useEffect, useState } from 'react'
import { Plus, Save, Trash2 } from 'lucide-react'
import { api, type SiteLink, type SiteOptions } from '@/lib/api'
import { invalidateSiteCache } from '@/hooks/use-site'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { PageHeader } from '@/components/admin-page-header'

const POSITIONS: { value: SiteLink['position']; label: string }[] = [
  { value: 'header', label: '顶部导航' },
  { value: 'footer', label: '页脚' },
  { value: 'social', label: '社交链接' },
]

export default function AdminSettings() {
  const [form, setForm] = useState<SiteOptions | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api
      .site()
      .then(setForm)
      .catch((err) => toast.error((err as Error).message))
  }, [])

  if (!form) return <p className="text-sm text-muted-foreground">加载中</p>

  const set = <K extends keyof SiteOptions>(key: K, value: SiteOptions[K]) =>
    setForm((prev) => (prev ? { ...prev, [key]: value } : prev))

  const setLink = (index: number, patch: Partial<SiteLink>) =>
    setForm((prev) => {
      if (!prev) return prev
      const links: SiteLink[] = prev.links.map((l) => ({ ...l }))
      const cur = links[index]
      if (cur) links[index] = { ...cur, ...patch }
      return { ...prev, links }
    })

  const addLink = () =>
    setForm((prev) => {
      if (!prev) return prev
      const links: SiteLink[] = [...prev.links]
      links.push({ label: '', url: '', position: 'footer' })
      return { ...prev, links }
    })

  const removeLink = (index: number) =>
    setForm((prev) =>
      prev ? { ...prev, links: prev.links.filter((_, i) => i !== index) } : prev,
    )

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      await api.updateSettings(form)
      // 站点设置存在后端，前端缓存需失效才能读到新值
      invalidateSiteCache()
      toast.success('设置已保存')
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <PageHeader title="设置" />

      <form onSubmit={save} className="mt-6 flex max-w-2xl flex-col gap-6">
        <section className="flex flex-col gap-4">
          <h2 className="text-xs font-medium text-muted-foreground">站点信息</h2>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="s-title">站点标题</Label>
            <Input
              id="s-title"
              value={form.title}
              onChange={(e) => set('title', e.target.value)}
              maxLength={100}
              required
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="s-tagline">副标题</Label>
            <Input
              id="s-tagline"
              value={form.tagline}
              onChange={(e) => set('tagline', e.target.value)}
              maxLength={200}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="s-desc">站点描述（SEO）</Label>
            <Textarea
              id="s-desc"
              value={form.description}
              onChange={(e) => set('description', e.target.value)}
              rows={2}
              maxLength={500}
            />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="s-url">站点地址</Label>
              <Input
                id="s-url"
                value={form.url}
                onChange={(e) => set('url', e.target.value)}
                placeholder="https://example.com"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="s-perpage">每页文章数</Label>
              <Input
                id="s-perpage"
                type="number"
                min={1}
                max={100}
                value={form.posts_per_page}
                onChange={(e) => set('posts_per_page', Number(e.target.value) || 10)}
              />
            </div>
          </div>
        </section>

        <Separator />

        <section className="flex flex-col gap-4">
          <h2 className="text-xs font-medium text-muted-foreground">评论</h2>
          <ToggleRow
            id="s-comments"
            label="开启评论"
            checked={form.comments_enabled}
            onChange={(v) => set('comments_enabled', v)}
          />
          <ToggleRow
            id="s-moderation"
            label="评论需审核"
            hint="开启后评论提交后先进入待审核"
            checked={form.comment_moderation}
            onChange={(v) => set('comment_moderation', v)}
            disabled={!form.comments_enabled}
          />
        </section>

        <Separator />

        <section className="flex flex-col gap-4">
          <h2 className="text-xs font-medium text-muted-foreground">文章显示</h2>
          <ToggleRow
            id="s-show-date"
            label="显示日期"
            checked={form.show_date}
            onChange={(v) => set('show_date', v)}
          />
          <ToggleRow
            id="s-show-reading"
            label="显示阅读时长"
            checked={form.show_reading_time}
            onChange={(v) => set('show_reading_time', v)}
          />
          <ToggleRow
            id="s-show-author"
            label="显示作者"
            checked={form.show_author}
            onChange={(v) => set('show_author', v)}
          />
          <ToggleRow
            id="s-show-tags"
            label="显示标签"
            checked={form.show_tags}
            onChange={(v) => set('show_tags', v)}
          />
        </section>

        <Separator />

        <section className="flex flex-col gap-3">
          <div className="flex items-center justify-between">
            <h2 className="text-xs font-medium text-muted-foreground">站点链接</h2>
            <Button type="button" size="sm" variant="outline" onClick={addLink}>
              <Plus className="size-4" />
              添加
            </Button>
          </div>

          {form.links.length === 0 && (
            <p className="text-sm text-muted-foreground">暂无链接</p>
          )}

          {form.links.map((link, i) => (
            <div
              key={`${link.label}-${i}`}
              className="grid items-end gap-2 rounded-md border border-border p-3 sm:grid-cols-[1fr_1.4fr_8rem_auto]"
            >
              <div className="flex flex-col gap-1.5">
                <Label htmlFor={`l-label-${i}`}>名称</Label>
                <Input
                  id={`l-label-${i}`}
                  value={link.label}
                  onChange={(e) => setLink(i, { label: e.target.value })}
                  maxLength={50}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor={`l-url-${i}`}>地址</Label>
                <Input
                  id={`l-url-${i}`}
                  value={link.url}
                  onChange={(e) => setLink(i, { url: e.target.value })}
                  placeholder="https:// 或 /path"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor={`l-pos-${i}`}>位置</Label>
                <Select
                  value={link.position}
                  onValueChange={(v) => setLink(i, { position: v as SiteLink['position'] })}
                >
                  <SelectTrigger id={`l-pos-${i}`} className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {POSITIONS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button
                type="button"
                size="icon"
                variant="ghost"
                className="size-8 text-destructive"
                onClick={() => removeLink(i)}
                aria-label={`删除链接 ${link.label || i + 1}`}
              >
                <Trash2 className="size-4" />
              </Button>
            </div>
          ))}
        </section>

        <div>
          <Button type="submit" size="sm" disabled={saving}>
            <Save className="size-4" />
            {saving ? '保存中' : '保存设置'}
          </Button>
        </div>
      </form>
    </div>
  )
}

function ToggleRow({
  id,
  label,
  hint,
  checked,
  onChange,
  disabled,
}: {
  id: string
  label: string
  hint?: string
  checked: boolean
  onChange: (v: boolean) => void
  disabled?: boolean
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <div className="min-w-0">
        <Label htmlFor={id}>{label}</Label>
        {hint && <p className="truncate text-xs text-muted-foreground">{hint}</p>}
      </div>
      <Switch
        id={id}
        checked={checked}
        onCheckedChange={onChange}
        disabled={disabled}
      />
    </div>
  )
}
