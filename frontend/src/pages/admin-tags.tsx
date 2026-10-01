import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Tags, Trash2 } from 'lucide-react'
import { api, type Tag } from '@/lib/api'
import { useAsync } from '@/hooks/use-async'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { PageHeader } from '@/components/admin-page-header'
import { EmptyState } from '@/components/empty-state'
import { ConfirmDialog } from '@/components/confirm-dialog'

export default function AdminTags() {
  const [keyword, setKeyword] = useState('')
  const [search, setSearch] = useState('')
  const [newTag, setNewTag] = useState('')
  const [pendingDelete, setPendingDelete] = useState<Tag | null>(null)
  const [mergeSource, setMergeSource] = useState<string>('')
  const [mergeTarget, setMergeTarget] = useState<string>('')
  const [reloadKey, setReloadKey] = useState(0)

  const { data, error, loading } = useAsync(() => api.listTags(search), [search, reloadKey])
  const tags = data?.items ?? []
  const refresh = () => setReloadKey((k) => k + 1)

  const add = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = newTag.trim()
    if (!trimmed) return
    try {
      await api.createTag(trimmed)
      setNewTag('')
      toast.success('已添加')
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    try {
      await api.deleteTag(pendingDelete.id)
      toast.success('已删除')
      setPendingDelete(null)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const runMerge = async () => {
    if (!mergeSource || !mergeTarget) {
      toast.error('请选择来源与目标标签')
      return
    }
    try {
      const res = await api.mergeTags(Number(mergeSource), Number(mergeTarget))
      toast.success(`已合并，迁移 ${res.moved} 篇文章`)
      setMergeSource('')
      setMergeTarget('')
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  return (
    <div>
      <PageHeader title="标签" count={tags.length} />

      <form onSubmit={add} className="mt-4 flex gap-2">
        <Input
          value={newTag}
          onChange={(e) => setNewTag(e.target.value)}
          placeholder="新标签名称"
          maxLength={50}
          aria-label="新标签名称"
        />
        <Button type="submit" variant="outline" className="shrink-0">
          添加
        </Button>
      </form>

      <form
        className="mt-3 flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          setSearch(keyword.trim())
        }}
      >
        <input
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索标签"
          aria-label="搜索标签"
          className="h-8 w-40 rounded-md border border-input bg-transparent px-2 text-sm"
        />
        <Button type="submit" size="sm" variant="outline">
          搜索
        </Button>
      </form>

      {tags.length > 1 && (
        <div className="mt-4 flex flex-wrap items-end gap-2 rounded-md border border-border p-3">
          <div className="flex flex-col gap-1.5">
            <span className="text-xs text-muted-foreground">合并标签</span>
            <Select
              value={mergeSource}
              onValueChange={(v) => setMergeSource(v ?? '')}
            >
              <SelectTrigger className="w-36" aria-label="来源标签">
                <SelectValue placeholder="来源" />
              </SelectTrigger>
              <SelectContent>
                {tags.map((t) => (
                  <SelectItem key={t.id} value={String(t.id)}>
                    {t.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <span className="pb-2 text-xs text-muted-foreground">并入</span>
          <div className="flex flex-col gap-1.5">
            <Select
              value={mergeTarget}
              onValueChange={(v) => setMergeTarget(v ?? '')}
            >
              <SelectTrigger className="w-36" aria-label="目标标签">
                <SelectValue placeholder="目标" />
              </SelectTrigger>
              <SelectContent>
                {tags
                  .filter((t) => String(t.id) !== mergeSource)
                  .map((t) => (
                    <SelectItem key={t.id} value={String(t.id)}>
                      {t.name}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>
          <Button size="sm" variant="outline" onClick={() => void runMerge()}>
            合并
          </Button>
        </div>
      )}

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {tags.length === 0 && !loading && (
        <EmptyState icon={Tags} message={search ? '没有匹配的标签' : '暂无标签'} />
      )}

      {tags.length > 0 && (
        <ul className="mt-4 overflow-hidden rounded-md border border-border">
          {tags.map((t, i) => (
            <li
              key={t.id}
              className={`flex items-center gap-3 px-3 py-2.5 ${
                i > 0 ? 'border-t border-border' : ''
              }`}
            >
              <Link
                to={`/tag/${t.slug}`}
                className="min-w-0 flex-1 truncate text-sm hover:underline"
                title={t.name}
              >
                {t.name}
              </Link>
              <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                {t.post_count} 篇
              </span>
              <Button
                size="icon"
                variant="ghost"
                className="size-7 text-destructive"
                onClick={() => setPendingDelete(t)}
                aria-label={`删除 ${t.name}`}
              >
                <Trash2 className="size-4" />
              </Button>
            </li>
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="删除标签"
        description={`该标签会从 ${pendingDelete?.post_count ?? 0} 篇文章上移除，确定删除？`}
        onConfirm={remove}
      />
    </div>
  )
}
