import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, Eye, History, Save, Trash2 } from 'lucide-react'
import { api, type Post, type PostInput, type PostStatus } from '@/lib/api'
import { formatRelative, renderMarkdown } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { useAuth } from '@/hooks/use-auth'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { MarkdownEditor } from '@/components/markdown-editor'
import { EditorImageDialog } from '@/components/editor-image-dialog'
import { PageHeader } from '@/components/admin-page-header'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { RevisionPanel } from '@/components/revision-panel'
import { TagInput } from '@/components/tag-input'
import { MediaPicker } from '@/components/media-picker'
import { statusLabel, statusVariant } from '@/pages/admin-posts'

export default function AdminPostEditor() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const isNew = !id
  const isPage = window.location.pathname.startsWith('/admin/pages')

  const [post, setPost] = useState<Post | null>(null)
  const [loading, setLoading] = useState(!isNew)
  const [saving, setSaving] = useState(false)
  const [preview, setPreview] = useState(false)
  const [pendingDelete, setPendingDelete] = useState(false)
  const [imageOpen, setImageOpen] = useState(false)
  // 图片插入的目标位置：正文 / 封面
  const [imageTarget, setImageTarget] = useState<'content' | 'cover'>('content')
  const [tab, setTab] = useState('write')

  // 表单字段：新建时用默认值，编辑时由 post 初始化
  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')
  const [summary, setSummary] = useState('')
  const [coverUrl, setCoverUrl] = useState('')
  const [content, setContent] = useState('')
  const [categoryId, setCategoryId] = useState('')
  const [tags, setTags] = useState<string[]>([])
  const [status, setStatus] = useState<PostStatus>(isPage ? 'published' : 'draft')
  const [sticky, setSticky] = useState(false)
  const [password, setPassword] = useState('')
  const [meta, setMeta] = useState<Record<string, string>>({})

  const { data: categoriesData } = useAsync(() => api.listCategories(), [])
  const categories = categoriesData?.items ?? []

  // 载入已有内容
  useEffect(() => {
    if (isNew) return
    let alive = true
    setLoading(true)
    api
      .getPost(Number(id))
      .then((p) => {
        if (!alive) return
        setPost(p)
        setTitle(p.title)
        setSlug(p.slug)
        setSummary(p.summary)
        setCoverUrl(p.cover_url)
        setContent(p.content)
        setCategoryId(p.category_id ? String(p.category_id) : '')
        setTags((p.tags ?? []).map((t) => t.name))
        setStatus(p.status)
        setSticky(p.sticky)
        setMeta({})
      })
      .catch((err) => toast.error((err as Error).message))
      .finally(() => alive && setLoading(false))
    return () => {
      alive = false
    }
  }, [id, isNew])

  // 编辑已有内容时读取元数据
  useEffect(() => {
    if (isNew || !post) return
    void api
      .getPostMeta(post.id)
      .then((res) => setMeta(res.items ?? {}))
      .catch(() => {})
  }, [post?.id, isNew])

  const canPublish = can('post.publish')

  const save = async (overrideStatus?: PostStatus) => {
    if (!title.trim()) {
      toast.error('标题必填')
      setTab('write')
      return
    }
    setSaving(true)
    const body: Partial<PostInput> = {
      title: title.trim(),
      slug: slug.trim(),
      summary: summary.trim(),
      content,
      cover_url: coverUrl.trim(),
      category_id: categoryId ? Number(categoryId) : null,
      status: overrideStatus ?? status,
      type: isPage ? 'page' : 'post',
      sticky,
      password: password.trim() || undefined,
      tags: tags.length > 0 ? tags : [],
      ...(Object.keys(meta).length > 0 ? { meta } : {}),
    }
    try {
      if (isNew) {
        const res = await api.createPost(body)
        toast.success('已创建')
        navigate(isPage ? `/admin/pages/${res.id}` : `/admin/posts/${res.id}`, { replace: true })
      } else {
        await api.updatePost(Number(id), body)
        toast.success('已保存')
      }
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    try {
      await api.deletePost(Number(id))
      toast.success('已删除')
      navigate(isPage ? '/admin/pages' : '/admin/posts')
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  if (loading) {
    return <p className="text-sm text-muted-foreground">加载中</p>
  }

  const backTo = isPage ? '/admin/pages' : '/admin/posts'

  return (
    <div>
      <PageHeader
        title={isNew ? (isPage ? '新建页面' : '新建文章') : '编辑'}
        actions={
          <>
            <Button size="sm" variant="ghost" onClick={() => navigate(backTo)}>
              <ArrowLeft className="size-4" />
              返回
            </Button>
            {!isNew && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => void api.saveRevision(Number(id)).then(() => toast.success('已保存快照'))}
              >
                <History className="size-4" />
                存快照
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={() => void save()} disabled={saving}>
              <Save className="size-4" />
              {saving ? '保存中' : '保存'}
            </Button>
          </>
        }
      />

      {post && (
        <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <Badge variant={statusVariant(post.status)}>{statusLabel(post.status)}</Badge>
          <span className="tabular-nums">{post.views} 阅读</span>
          <span>更新于 {formatRelative(post.updated_at)}</span>
          {post.author && <span>作者 {post.author.display_name}</span>}
          {can('post.delete') && (
            <Button
              size="sm"
              variant="ghost"
              className="text-destructive"
              onClick={() => setPendingDelete(true)}
            >
              <Trash2 className="size-4" />
              删除
            </Button>
          )}
        </div>
      )}

      <Tabs value={tab} onValueChange={setTab} className="mt-4">
        <TabsList>
          <TabsTrigger value="write">写作</TabsTrigger>
          <TabsTrigger value="meta">元数据</TabsTrigger>
          {!isNew && <TabsTrigger value="history">历史</TabsTrigger>}
        </TabsList>

        <TabsContent value="write" className="mt-4">
          <div className="flex flex-col gap-4">
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

            <div className="grid gap-4 sm:grid-cols-2">
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
                <MediaPicker value={coverUrl} onChange={setCoverUrl} />
              </div>
            </div>

            {!isPage && (
              <div className="flex flex-col gap-1.5">
                <Label>标签</Label>
                <TagInput value={tags} onChange={setTags} />
              </div>
            )}

            <div className="flex flex-col gap-1.5">
              <div className="flex items-center justify-between">
                <Label>正文（Markdown）</Label>
                <div className="flex gap-1">
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => setPreview(!preview)}
                    aria-label={preview ? '切换到编辑' : '切换到预览'}
                  >
                    <Eye className="size-4" />
                    {preview ? '编辑' : '预览'}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      setImageTarget('cover')
                      setImageOpen(true)
                    }}
                  >
                    设为封面
                  </Button>
                </div>
              </div>
              {preview ? (
                <MarkdownPreview content={content} />
              ) : (
                <MarkdownEditor
                  initialValue={content}
                  onChange={setContent}
                  placeholder="开始写作，支持 Markdown 语法与工具栏快捷操作…"
                  onRequestImage={() => {
                    setImageTarget('content')
                    setImageOpen(true)
                  }}
                />
              )}
            </div>

            <div className="flex flex-wrap items-center gap-4">
              <div className="flex items-center gap-2">
                <Label htmlFor="status">状态</Label>
                <Select
                  value={status}
                  onValueChange={(v) => setStatus(v as PostStatus)}
                >
                  <SelectTrigger className="w-32" id="status">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="draft">草稿</SelectItem>
                    {canPublish && <SelectItem value="published">已发布</SelectItem>}
                    {canPublish && <SelectItem value="pending">待审核</SelectItem>}
                    {canPublish && <SelectItem value="private">私密</SelectItem>}
                  </SelectContent>
                </Select>
              </div>

              {!isPage && canPublish && (
                <div className="flex items-center gap-2">
                  <Switch id="sticky" checked={sticky} onCheckedChange={setSticky} />
                  <Label htmlFor="sticky">置顶</Label>
                </div>
              )}

              {!isPage && (
                <div className="flex items-center gap-2">
                  <Switch
                    id="protect"
                    checked={password !== ''}
                    onCheckedChange={(v) => setPassword(v ? ' ' : '')}
                  />
                  <Label htmlFor="protect">密码保护</Label>
                </div>
              )}
            </div>

            {!isPage && password !== '' && (
              <div className="flex flex-col gap-1.5 sm:max-w-64">
                <Label htmlFor="password">访问密码</Label>
                <Input
                  id="password"
                  type="text"
                  value={password.trim()}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="留空则不保护"
                />
              </div>
            )}

            {canPublish && status !== 'published' && !isNew && (
              <div>
                <Button size="sm" variant="outline" onClick={() => void save('published')} disabled={saving}>
                  直接发布
                </Button>
              </div>
            )}
          </div>
        </TabsContent>

        <TabsContent value="meta" className="mt-4">
          <MetaEditor
            value={meta}
            onChange={setMeta}
            disabled={isNew}
            hint={isNew ? '保存后可编辑元数据' : undefined}
          />
        </TabsContent>

        {!isNew && (
          <TabsContent value="history" className="mt-4">
            <RevisionPanel
              postId={Number(id)}
              onRestored={() => {
                void api.getPost(Number(id)).then((p) => {
                  setTitle(p.title)
                  setContent(p.content)
                  setSummary(p.summary)
                })
                toast.success('已回滚到所选版本')
              }}
            />
          </TabsContent>
        )}
      </Tabs>

      <EditorImageDialog
        open={imageOpen}
        onOpenChange={setImageOpen}
        onInsert={(src, alt) => {
          if (imageTarget === 'cover') {
            setCoverUrl(src)
            toast.success('已设为封面')
            return
          }
          // 正文：追加 Markdown 图片语法
          const md = alt ? `![${alt}](${src})` : `![](${src})`
          setContent((prev) => (prev ? `${prev}\n\n${md}` : md))
        }}
      />

      <ConfirmDialog
        open={pendingDelete}
        onOpenChange={setPendingDelete}
        title="删除内容"
        description="删除后不可恢复，确定删除？"
        onConfirm={remove}
      />
    </div>
  )
}

function MarkdownPreview({ content }: { content: string }) {
  return (
    <div
      className="md-body min-h-64 rounded-md border border-input px-3 py-2 text-sm"
      dangerouslySetInnerHTML={{ __html: renderMarkdown(content) }}
    />
  )
}

function MetaEditor({
  value,
  onChange,
  disabled,
  hint,
}: {
  value: Record<string, string>
  onChange: (v: Record<string, string>) => void
  disabled?: boolean
  hint?: string
}) {
  const entries = Object.entries(value)

  if (disabled) {
    return <p className="text-sm text-muted-foreground">{hint || '保存后可编辑'}</p>
  }

  return (
    <div className="flex flex-col gap-3">
      {entries.length === 0 && (
        <p className="text-sm text-muted-foreground">暂无自定义元数据</p>
      )}
      {entries.map(([k]) => (
        <div key={k} className="grid gap-2 sm:grid-cols-[10rem_1fr]">
          <Input value={k} readOnly aria-label={`键 ${k}`} className="font-mono text-xs" />
          <Textarea
            value={value[k] ?? ''}
            onChange={(e) => onChange({ ...value, [k]: e.target.value })}
            rows={2}
            aria-label={`值 ${k}`}
          />
        </div>
      ))}
    </div>
  )
}
