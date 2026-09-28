import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import { api, type Category } from '@/lib/api'
import { useAsync } from '@/hooks/use-async'
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
import { toast } from 'sonner'

export default function AdminCategories() {
  const [name, setName] = useState('')
  const [pendingDelete, setPendingDelete] = useState<Category | null>(null)

  const { data, error, reload } = useAsync(() => api.listCategories(), [])
  const categories = data?.items ?? []

  const add = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) return
    try {
      await api.createCategory(trimmed)
      setName('')
      toast.success('已添加')
      reload()
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
      reload()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div className="max-w-md">
      <h1 className="text-sm font-semibold">分类 {categories.length}</h1>

      <form onSubmit={add} className="mt-4 flex gap-2">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="新分类名称"
          maxLength={50}
          aria-label="新分类名称"
        />
        <Button type="submit" variant="outline" className="shrink-0">
          添加
        </Button>
      </form>

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      <ul className="mt-4 overflow-hidden rounded-md border border-border">
        {categories.length === 0 && (
          <li className="px-3 py-8 text-center text-sm text-muted-foreground">暂无分类</li>
        )}
        {categories.map((c, i) => (
          <li
            key={c.id}
            className={`flex items-center gap-3 px-3 py-2.5 ${i > 0 ? 'border-t border-border' : ''}`}
          >
            <span className="min-w-0 flex-1 truncate text-sm" title={c.name}>
              {c.name}
            </span>
            <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
              {c.post_count ?? 0} 篇
            </span>
            <Button
              size="sm"
              variant="ghost"
              className="shrink-0 text-destructive"
              aria-label={`删除 ${c.name}`}
              onClick={() => setPendingDelete(c)}
            >
              <Trash2 className="size-4" />
            </Button>
          </li>
        ))}
      </ul>
      <p className="mt-3 text-xs text-muted-foreground">删除分类后，该分类下的文章将变为未分类。</p>

      <Dialog open={!!pendingDelete} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <DialogContent className="shadow-none sm:max-w-80">
          <DialogHeader>
            <DialogTitle>删除分类</DialogTitle>
            <DialogDescription>
              该分类下的文章将变为未分类，确定删除「{pendingDelete?.name}」？
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" size="sm" />}>取消</DialogClose>
            <Button variant="destructive" size="sm" onClick={remove}>
              删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
