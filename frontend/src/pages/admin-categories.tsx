import { useMemo, useState } from 'react'
import { FolderTree, Plus, Trash2 } from 'lucide-react'
import { api, type Category } from '@/lib/api'
import { useAsync } from '@/hooks/use-async'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { PageHeader } from '@/components/admin-page-header'
import { EmptyState } from '@/components/empty-state'
import { ConfirmDialog } from '@/components/confirm-dialog'

export default function AdminCategories() {
  const [name, setName] = useState('')
  const [pendingDelete, setPendingDelete] = useState<Category | null>(null)
  const [editing, setEditing] = useState<Category | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  const { data, error, loading } = useAsync(() => api.listCategories(), [reloadKey])
  const categories = useMemo(() => data?.items ?? [], [data?.items])
  const refresh = () => setReloadKey((k) => k + 1)

  const add = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) return
    try {
      await api.createCategory(trimmed)
      setName('')
      toast.success('已添加')
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deleteCategory(pendingDelete.id)
      toast.success('已删除')
      setPendingDelete(null)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const move = async (cat: Category, up: boolean) => {
    const ids = categories.map((c) => c.id)
    const idx = ids.indexOf(cat.id)
    if (idx < 0) return
    const target = up ? idx - 1 : idx + 1
    if (target < 0 || target >= ids.length) return
    ;[ids[idx], ids[target]] = [ids[target] as number, ids[idx] as number]
    try {
      await api.reorderCategories(ids)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader title="分类" count={categories.length} />

      <form onSubmit={add} className="mt-4 flex gap-2">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="新分类名称"
          maxLength={50}
          aria-label="新分类名称"
        />
        <Button type="submit" variant="outline" className="shrink-0">
          <Plus className="size-4" />
          添加
        </Button>
      </form>

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {categories.length === 0 && !loading && (
        <EmptyState icon={FolderTree} message="暂无分类" />
      )}

      {categories.length > 0 && (
        <ul className="mt-4 overflow-hidden rounded-md border border-border">
          {categories.map((c, i) => (
            <li
              key={c.id}
              className={`flex items-center gap-2 px-3 py-2.5 ${
                i > 0 ? 'border-t border-border' : ''
              }`}
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm" title={c.name}>
                  {c.name}
                </p>
                <p className="truncate text-xs text-muted-foreground">
                  /{c.slug} · {c.post_count ?? 0} 篇
                </p>
              </div>
              <div className="flex shrink-0 items-center">
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-7"
                  disabled={i === 0}
                  onClick={() => void move(c, true)}
                  aria-label={`上移 ${c.name}`}
                >
                  ↑
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-7"
                  disabled={i === categories.length - 1}
                  onClick={() => void move(c, false)}
                  aria-label={`下移 ${c.name}`}
                >
                  ↓
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setEditing(c)}
                  aria-label={`编辑 ${c.name}`}
                >
                  编辑
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="size-7 text-destructive"
                  onClick={() => setPendingDelete(c)}
                  aria-label={`删除 ${c.name}`}
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <EditCategoryDialog
        category={editing}
        onOpenChange={(open) => !open && setEditing(null)}
        onSaved={() => {
          setEditing(null)
          refresh()
        }}
      />

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="删除分类"
        description={`该分类下的文章将变为未分类，确定删除「${pendingDelete?.name}」？`}
        onConfirm={remove}
      />
    </div>
  )
}

function EditCategoryDialog({
  category,
  onOpenChange,
  onSaved,
}: {
  category: Category | null
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [loadedId, setLoadedId] = useState<number | null>(null)

  // 打开时初始化表单：仅在切换到新条目时重置，避免每次渲染覆盖输入
  if (category && category.id !== loadedId) {
    setLoadedId(category.id)
    setName(category.name)
    setSlug(category.slug)
    setDescription(category.description ?? '')
  }
  if (!category && loadedId !== null) {
    setLoadedId(null)
  }

  const save = async () => {
    if (!category || !name.trim()) return
    setBusy(true)
    try {
      await api.updateCategory(category.id, {
        name: name.trim(),
        slug: slug.trim() || undefined,
        description: description.trim(),
      })
      toast.success('已保存')
      onSaved()
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={!!category} onOpenChange={onOpenChange}>
      <DialogContent className="shadow-none sm:max-w-96">
        <DialogHeader>
          <DialogTitle>编辑分类</DialogTitle>
          <DialogDescription>slug 留空则保持不变，既有链接不会失效。</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="名称"
            maxLength={50}
            aria-label="名称"
          />
          <Input
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            placeholder="slug"
            aria-label="slug"
          />
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="描述（选填）"
            aria-label="描述"
          />
        </div>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" size="sm" />}>取消</DialogClose>
          <Button size="sm" onClick={() => void save()} disabled={busy || !name.trim()}>
            {busy ? '保存中' : '保存'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
