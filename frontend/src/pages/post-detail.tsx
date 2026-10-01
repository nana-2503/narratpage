import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Eye, Lock, MessageSquare, Send } from 'lucide-react'
import { api, isNotFound, type Comment } from '@/lib/api'
import {
  estimateReadingTime,
  formatDate,
  formatDateTime,
  renderMarkdown,
} from '@/lib/markdown'
import { clearPostMeta, setJsonLd, setPageMeta } from '@/lib/meta'
import { useAsync } from '@/hooks/use-async'
import { useSite } from '@/hooks/use-site'
import { useAuth } from '@/hooks/use-auth'
import { SiteHeader } from '@/components/site-header'
import { SiteFooter } from '@/components/site-footer'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import { toast } from 'sonner'

const UNLOCK_KEY = 'blog_unlocked_posts'

/** 已解锁的文章 id：刷新后仍可查看正文 */
function readUnlocked(): number[] {
  try {
    const raw = localStorage.getItem(UNLOCK_KEY)
    return raw ? (JSON.parse(raw) as number[]) : []
  } catch {
    return []
  }
}

function markUnlocked(id: number) {
  const list = readUnlocked()
  if (!list.includes(id)) {
    localStorage.setItem(UNLOCK_KEY, JSON.stringify([...list, id]))
  }
}

export default function PostDetail() {
  const { slug = '' } = useParams()
  const { site } = useSite()
  const { user } = useAuth()

  const { data: post, error: postError } = useAsync(() => api.getPost(slug), [slug])
  const notFound = isNotFound(postError)

  const {
    data: commentsData,
    reload: reloadComments,
  } = useAsync(
    () =>
      post
        ? api.listComments({ postId: post.id, nested: true })
        : Promise.resolve({ items: [] as Comment[] }),
    [post?.id],
  )
  const comments = commentsData?.items ?? []

  const { data: neighbors } = useAsync(
    () =>
      post
        ? api.getPostNeighbors(slug)
        : Promise.resolve({ prev: null, next: null }),
    [post?.id, slug],
  )

  const [unlocked, setUnlocked] = useState(false)
  const [password, setPassword] = useState('')
  const [unlocking, setUnlocking] = useState(false)

  // 访问密码：已解锁过的文章直接放行
  useEffect(() => {
    if (post?.has_password && post.id) {
      setUnlocked(readUnlocked().includes(post.id))
    }
  }, [post?.id, post?.has_password])

  // SEO：标题、描述、OG、结构化数据
  useEffect(() => {
    if (!post) return
    const url = `${site.url || window.location.origin}/post/${post.slug}`
    setPageMeta(post.title, post.summary, {
      image: post.cover_url || undefined,
      type: 'article',
      url,
    })
    setJsonLd({
      '@context': 'https://schema.org',
      '@type': 'BlogPosting',
      headline: post.title,
      description: post.summary,
      datePublished: post.published_at,
      dateModified: post.updated_at,
      author: post.author ? { '@type': 'Person', name: post.author.display_name } : undefined,
      keywords: post.tags?.map((t) => t.name).join(', '),
    })
    return () => {
      clearPostMeta()
      setJsonLd(null)
    }
  }, [post, site.url])

  const locked = Boolean(post?.has_password) && !unlocked

  const unlock = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!post) return
    setUnlocking(true)
    try {
      // 后端以「已解锁」标记放行正文，密码本身不下发到前端存储
      const full = await api.getPost(post.slug)
      if (full.content) {
        markUnlocked(post.id)
        setUnlocked(true)
        window.location.reload()
      } else {
        toast.error('访问密码不正确')
      }
    } catch {
      toast.error('验证失败，请稍后再试')
    } finally {
      setUnlocking(false)
    }
  }

  if (notFound) {
    return (
      <div className="min-h-screen bg-background">
        <SiteHeader />
        <main className="mx-auto w-full max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">
          文章不存在或未发布
        </main>
      </div>
    )
  }

  if (!post) {
    return (
      <div className="min-h-screen bg-background">
        <SiteHeader />
        <main className="mx-auto w-full max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">
          {postError ? `加载失败：${postError.message}` : '加载中'}
        </main>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto w-full max-w-3xl flex-1 px-4 pb-16 pt-6">
        <Link to="/" className="text-xs text-muted-foreground hover:text-foreground">
          ← 返回
        </Link>

        <article className="mt-3">
          <h1 className="text-xl font-semibold tracking-tight text-balance">{post.title}</h1>
          <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
            {post.category_name && (
              <Link to={`/category/${post.category_slug}`}>
                <Badge variant="outline">{post.category_name}</Badge>
              </Link>
            )}
            {site.show_date && (
              <>
                <span className="tabular-nums">
                  {formatDate(post.published_at || post.created_at)}
                </span>
                <span aria-hidden>·</span>
              </>
            )}
            {site.show_reading_time && (
              <>
                <span className="tabular-nums">{estimateReadingTime(post.content || '')}</span>
                <span aria-hidden>·</span>
              </>
            )}
            <span className="inline-flex items-center gap-1 tabular-nums">
              <Eye className="size-3" />
              {post.views}
            </span>
            {site.show_author && post.author && (
              <>
                <span aria-hidden>·</span>
                <Link to={`/author/${post.author.id}`} className="hover:text-foreground">
                  {post.author.display_name}
                </Link>
              </>
            )}
          </div>

          {post.tags && post.tags.length > 0 && site.show_tags && (
            <div className="mt-2 flex flex-wrap gap-1">
              {post.tags.map((t) => (
                <Link key={t.id} to={`/tag/${t.slug}`}>
                  <Badge variant="secondary" className="text-xs font-normal">
                    {t.name}
                  </Badge>
                </Link>
              ))}
            </div>
          )}

          <Separator className="my-5" />

          {post.cover_url && !locked && (
            <img
              src={post.cover_url}
              alt={post.title}
              className="mb-5 w-full rounded-md border border-border object-cover"
              loading="lazy"
            />
          )}

          {locked ? (
            <LockedPanel
              password={password}
              setPassword={setPassword}
              onSubmit={unlock}
              loading={unlocking}
            />
          ) : (
            <div
              className="md-body text-sm leading-7"
              dangerouslySetInnerHTML={{ __html: renderMarkdown(post.content || '') }}
            />
          )}
        </article>

        {(neighbors?.prev || neighbors?.next) && (
          <nav className="mt-6 flex items-center justify-between gap-3 text-sm">
            {neighbors.prev ? (
              <Link
                to={`/post/${neighbors.prev.slug}`}
                className="min-w-0 flex-1 truncate text-muted-foreground hover:text-foreground"
                title={neighbors.prev.title}
              >
                ← {neighbors.prev.title}
              </Link>
            ) : (
              <span />
            )}
            {neighbors.next ? (
              <Link
                to={`/post/${neighbors.next.slug}`}
                className="min-w-0 flex-1 truncate text-right text-muted-foreground hover:text-foreground"
                title={neighbors.next.title}
              >
                {neighbors.next.title} →
              </Link>
            ) : (
              <span />
            )}
          </nav>
        )}

        <Separator className="my-8" />

        {site.comments_enabled && (
          <section>
            <h2 className="flex items-center gap-1.5 text-sm font-semibold">
              <MessageSquare className="size-4" />
              评论 {comments.length}
            </h2>
            <CommentList comments={comments} />

            {site.comment_moderation && (
              <p className="mt-3 text-xs text-muted-foreground">评论提交后需审核通过才会显示。</p>
            )}

            <CommentForm
              postId={post.id}
              defaultAuthor={user?.display_name || user?.username || ''}
              onSubmitted={reloadComments}
            />
          </section>
        )}
      </main>
      <SiteFooter />
    </div>
  )
}

function LockedPanel({
  password,
  setPassword,
  onSubmit,
  loading,
}: {
  password: string
  setPassword: (v: string) => void
  onSubmit: (e: React.FormEvent) => void
  loading: boolean
}) {
  return (
    <form onSubmit={onSubmit} className="rounded-md border border-border p-6">
      <div className="flex items-center gap-2 text-sm font-medium">
        <Lock className="size-4" />
        受密码保护
      </div>
      <div className="mt-3 flex gap-2">
        <Input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="访问密码"
          aria-label="访问密码"
          required
        />
        <Button type="submit" disabled={loading || !password} className="shrink-0">
          {loading ? '验证中' : '解锁'}
        </Button>
      </div>
    </form>
  )
}

function CommentList({ comments }: { comments: Comment[] }) {
  if (comments.length === 0) {
    return (
      <ul className="mt-3 overflow-hidden rounded-md border border-border">
        <li className="px-3 py-6 text-center text-sm text-muted-foreground">暂无评论</li>
      </ul>
    )
  }
  return (
    <ul className="mt-3 overflow-hidden rounded-md border border-border">
      {comments.map((c) => (
        <li key={c.id} className="border-t border-border px-3 py-3 first:border-t-0">
          <CommentRow comment={c} />
          {c.replies && c.replies.length > 0 && (
            <ul className="mt-3 space-y-3 border-l border-border pl-3">
              {c.replies.map((r) => (
                <li key={r.id}>
                  <CommentRow comment={r} />
                </li>
              ))}
            </ul>
          )}
        </li>
      ))}
    </ul>
  )
}

function CommentRow({ comment }: { comment: Comment }) {
  return (
    <div>
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <span className="font-medium text-foreground">{comment.author}</span>
        <span className="tabular-nums">{formatDateTime(comment.created_at)}</span>
      </div>
      <p className="mt-1 whitespace-pre-wrap break-words text-sm">{comment.content}</p>
    </div>
  )
}

function CommentForm({
  postId,
  defaultAuthor,
  onSubmitted,
}: {
  postId: number
  defaultAuthor: string
  onSubmitted: () => void
}) {
  const [author, setAuthor] = useState(defaultAuthor)
  const [email, setEmail] = useState('')
  const [content, setContent] = useState('')
  const [honeypot, setHoneypot] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // 登录用户在昵称输入框上直接回车即可提交
  useEffect(() => {
    if (defaultAuthor) setAuthor(defaultAuthor)
  }, [defaultAuthor])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!author.trim() || !content.trim()) return
    setSubmitting(true)
    try {
      const res = await api.createComment({
        postId,
        author: author.trim(),
        email: email.trim() || undefined,
        content: content.trim(),
        website_confirm: honeypot || undefined,
      })
      setContent('')
      setHoneypot('')
      toast.success(
        res.status === 'pending' ? '评论已提交，等待审核' : '评论已发布',
      )
      // 已通过审核的评论立即出现在列表中
      if (res.status !== 'pending') onSubmitted()
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={submit} className="mt-4 flex flex-col gap-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="author">昵称</Label>
          <Input
            id="author"
            value={author}
            onChange={(e) => setAuthor(e.target.value)}
            maxLength={64}
            required
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="email">邮箱（选填，不公开）</Label>
          <Input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            maxLength={255}
          />
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="content">评论内容</Label>
        <Textarea
          id="content"
          value={content}
          onChange={(e) => setContent(e.target.value)}
          maxLength={3000}
          required
          rows={4}
        />
      </div>

      {/* 蜜罐字段：正常用户不可见，机器人往往会填 */}
      <div className="hidden" aria-hidden>
        <Label htmlFor="website_confirm">请留空</Label>
        <Input
          id="website_confirm"
          tabIndex={-1}
          autoComplete="off"
          value={honeypot}
          onChange={(e) => setHoneypot(e.target.value)}
        />
      </div>

      <Button type="submit" disabled={submitting} className="w-fit">
        <Send className="size-4" />
        {submitting ? '提交中' : '提交评论'}
      </Button>
    </form>
  )
}
