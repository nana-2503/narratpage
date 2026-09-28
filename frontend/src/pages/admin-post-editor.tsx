import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type PostInput } from '@/lib/api'
import { useAsync } from '@/hooks/use-async'
import { PostForm } from '@/components/post-form'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'

export default function AdminPostEditor() {
  const { id } = useParams()
  const isNew = !id
  const navigate = useNavigate()
  const [saving, setSaving] = useState(false)

  const { data: categoriesData } = useAsync(() => api.listCategories(), [])
  const categories = categoriesData?.items ?? []

  const { data: post, error, loading } = useAsync(
    () => (isNew ? Promise.resolve(null) : api.getPostById(Number(id))),
    [id, isNew],
  )

  const save = async (body: PostInput) => {
    setSaving(true)
    try {
      if (isNew) {
        const { id: newId } = await api.createPost(body)
        toast.success('已创建')
        navigate(`/admin/posts/${newId}`, { replace: true })
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

  if (loading) return <p className="text-sm text-muted-foreground">加载中</p>

  if (error) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p className="text-sm text-destructive">{error.message}</p>
        <Button variant="outline" size="sm" onClick={() => navigate('/admin/posts')}>
          返回列表
        </Button>
      </div>
    )
  }

  return (
    <div>
      <h1 className="text-sm font-semibold">{isNew ? '新建文章' : '编辑文章'}</h1>
      <PostForm
        key={post?.id ?? 'new'}
        post={post}
        categories={categories}
        saving={saving}
        onSubmit={save}
        onCancel={() => navigate('/admin/posts')}
      />
    </div>
  )
}
