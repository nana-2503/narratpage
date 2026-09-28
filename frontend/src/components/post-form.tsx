import { useState } from 'react'
import type { Category, Post, PostInput } from '@/lib/api'
import { renderMarkdown } from '@/lib/markdown'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { toast } from 'sonner'

interface PostFormProps {
  /** null 表示新建 */
  post: Post | null
  categories: Category[]
  saving: boolean
  onSubmit: (body: PostInput) => void
  onCancel: () => void
}

/** 文章编辑表单：字段状态自包含，提交时输出规范化的 PostInput。
 *  页面通过 key={post?.id ?? 'new'} 控制在数据到达后重新挂载以初始化字段。 */
export function PostForm({ post, categories, saving, onSubmit, onCancel }: PostFormProps) {
  const [title, setTitle] = useState(post?.title ?? '')
  const [slug, setSlug] = useState(post?.slug ?? '')
  const [summary, setSummary] = useState(post?.summary ?? '')
  const [coverUrl, setCoverUrl] = useState(post?.cover_url ?? '')
  const [content, setContent] = useState(post?.content ?? '')
  const [categoryId, setCategoryId] = useState(post?.category_id ? String(post.category_id) : '')
  const [published, setPublished] = useState(post?.status === 'published')
  const [preview, setPreview] = useState(false)

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) {
      toast.error('标题必填')
      return
    }
    onSubmit({
      title: title.trim(),
      slug: slug.trim(),
      summary: summary.trim(),
      content,
      cover_url: coverUrl.trim(),
      category_id: categoryId ? Number(categoryId) : null,
      status: published ? 'published' : 'draft',
    })
  }

  return (
    <form onSubmit={submit} className="mt-4 flex max-w-2xl flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="title">标题</Label>
        <Input
          id="title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={200}
          required
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="slug">Slug（留空自动生成）</Label>
          <Input
            id="slug"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            placeholder="post-url-slug"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label>分类</Label>
          <Select
            value={categoryId || 'none'}
            onValueChange={(v) => setCategoryId(!v || v === 'none' ? '' : v)}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder="选择分类" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">未分类</SelectItem>
              {categories.map((c) => (
                <SelectItem key={c.id} value={String(c.id)}>
                  {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="summary">摘要</Label>
        <Input
          id="summary"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          maxLength={500}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="cover">封面图 URL</Label>
        <Input
          id="cover"
          value={coverUrl}
          onChange={(e) => setCoverUrl(e.target.value)}
          placeholder="https://..."
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <div className="flex items-center justify-between">
          <Label htmlFor="content">正文（Markdown）</Label>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setPreview((v) => !v)}
            aria-label={preview ? '切换到编辑' : '切换到预览'}
          >
            {preview ? '编辑' : '预览'}
          </Button>
        </div>
        {preview ? (
          <div
            className="md-body min-h-64 rounded-md border border-input px-3 py-2 text-sm"
            dangerouslySetInnerHTML={{ __html: renderMarkdown(content) }}
          />
        ) : (
          <textarea
            id="content"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            rows={16}
            className="w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-sm outline-none focus-visible:border-ring"
          />
        )}
      </div>

      <div className="flex items-center gap-2">
        <Switch id="published" checked={published} onCheckedChange={setPublished} />
        <Label htmlFor="published">发布</Label>
      </div>

      <div className="flex gap-2">
        <Button type="submit" disabled={saving}>
          {saving ? '保存中' : '保存'}
        </Button>
        <Button type="button" variant="outline" onClick={onCancel}>
          取消
        </Button>
      </div>
    </form>
  )
}
