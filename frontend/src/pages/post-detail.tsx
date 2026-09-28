import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, isNotFound } from '@/lib/api'
import { formatDate, formatDateTime, renderMarkdown } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { SiteHeader } from '@/components/site-header'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { toast } from 'sonner'

export default function PostDetail() {
  const { slug = '' } = useParams()

  const { data: post, error: postError } = useAsync(() => api.getPost(slug), [slug])
  const notFound = isNotFound(postError)

  const { data: commentsData } = useAsync(
    () => (post ? api.listComments({ postId: post.id }) : Promise.resolve({ items: [] })),
    [post?.id],
  )
  const comments = commentsData?.items ?? []

  const [author, setAuthor] = useState('')
  const [content, setContent] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!post) return
    setSubmitting(true)
    try {
      await api.createComment(post.id, author.trim(), content.trim())
      setAuthor('')
      setContent('')
      toast.success('评论已提交，等待审核')
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  if (notFound) {
    return (
      <div className="min-h-screen bg-background">
        <SiteHeader />
        <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">
          文章不存在或未发布
        </main>
      </div>
    )
  }

  if (!post) {
    return (
      <div className="min-h-screen bg-background">
        <SiteHeader />
        <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">
          {postError ? `加载失败：${postError.message}` : '加载中'}
        </main>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-3xl px-4 pb-16 pt-6">
        <Link to="/" className="text-xs text-muted-foreground hover:text-foreground">
          ← 返回
        </Link>

        <article className="mt-3">
          <h1 className="text-xl font-semibold tracking-tight">{post.title}</h1>
          <div className="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
            {post.category_name && <Badge variant="outline">{post.category_name}</Badge>}
            <span className="tabular-nums">{formatDate(post.published_at || post.created_at)}</span>
            <span aria-hidden>·</span>
            <span className="tabular-nums">{post.views} 阅读</span>
          </div>
          <Separator className="my-5" />
          <div
            className="md-body text-sm leading-7"
            dangerouslySetInnerHTML={{ __html: renderMarkdown(post.content || '') }}
          />
        </article>

        <Separator className="my-8" />

        <section>
          <h2 className="text-sm font-semibold">评论 {comments.length}</h2>
          <ul className="mt-3 border border-border">
            {comments.length === 0 && (
              <li className="px-3 py-6 text-center text-sm text-muted-foreground">暂无评论</li>
            )}
            {comments.map((c) => (
              <li key={c.id} className="border-t border-border px-3 py-3 first:border-t-0">
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span className="font-medium text-foreground">{c.author}</span>
                  <span className="tabular-nums">{formatDateTime(c.created_at)}</span>
                </div>
                <p className="mt-1 text-sm break-words whitespace-pre-wrap">{c.content}</p>
              </li>
            ))}
          </ul>

          <form onSubmit={submit} className="mt-4 flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="author">昵称</Label>
              <Input
                id="author"
                value={author}
                onChange={(e) => setAuthor(e.target.value)}
                maxLength={30}
                required
                className="sm:max-w-64"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="content">评论内容</Label>
              <textarea
                id="content"
                value={content}
                onChange={(e) => setContent(e.target.value)}
                maxLength={1000}
                required
                rows={3}
                className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring"
              />
            </div>
            <Button type="submit" disabled={submitting} className="w-fit">
              {submitting ? '提交中' : '提交评论'}
            </Button>
          </form>
        </section>
      </main>
    </div>
  )
}
