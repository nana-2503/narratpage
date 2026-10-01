import { useRef, useState } from 'react'
import { Copy, Image as ImageIcon, Trash2, Upload } from 'lucide-react'
import { api, type MediaItem } from '@/lib/api'
import { formatBytes, formatDate } from '@/lib/markdown'
import { useAsync } from '@/hooks/use-async'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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

const FILTERS = [
  { value: '', label: '全部' },
  { value: 'image', label: '图片' },
  { value: 'pdf', label: 'PDF' },
]

export default function AdminMedia() {
  const fileRef = useRef<HTMLInputElement>(null)
  const [page, setPage] = useState(1)
  const [mime, setMime] = useState('')
  const [keyword, setKeyword] = useState('')
  const [search, setSearch] = useState('')
  const [uploading, setUploading] = useState(false)
  const [editing, setEditing] = useState<MediaItem | null>(null)
  const [pendingDelete, setPendingDelete] = useState<MediaItem | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  const { data, error, loading } = useAsync(
    () => api.listMedia({ page, pageSize: 24, mime: mime || undefined, q: search || undefined }),
    [page, mime, search, reloadKey],
  )

  const items = data?.items ?? []
  const refresh = () => setReloadKey((k) => k + 1)

  const upload = async (files: FileList | null) => {
    if (!files || files.length === 0) return
    setUploading(true)
    let ok = 0
    try {
      for (const file of Array.from(files)) {
        try {
          await api.uploadImage(file)
          ok++
        } catch (err) {
          toast.error(`${file.name}：${(err as Error).message}`)
        }
      }
      if (ok > 0) {
        toast.success(`已上传 ${ok} 个文件`)
        refresh()
      }
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    try {
      const res = await api.deleteMedia(pendingDelete.id)
      toast.success(
        res.referenced > 0 ? '已删除记录（文件仍被文章引用，保留在磁盘）' : '已删除',
      )
      setPendingDelete(null)
      refresh()
    } catch (err) {
      toast.error((err as Error).message)
    }
  }

  const copyUrl = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url)
      toast.success('已复制地址')
    } catch {
      toast.error('复制失败')
    }
  }

  return (
    <div>
      <PageHeader
        title="媒体"
        count={data?.count}
        actions={
          <>
            <input
              ref={fileRef}
              type="file"
              multiple
              accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml,application/pdf"
              className="hidden"
              onChange={(e) => void upload(e.target.files)}
            />
            <Button
              size="sm"
              onClick={() => fileRef.current?.click()}
              disabled={uploading}
            >
              <Upload className="size-4" />
              {uploading ? '上传中' : '上传'}
            </Button>
          </>
        }
      />

      {data && data.totalSize > 0 && (
        <p className="mt-1 text-xs text-muted-foreground tabular-nums">
          共 {data.count} 个文件 · {formatBytes(data.totalSize)}
        </p>
      )}

      <form
        className="mt-4 flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          setPage(1)
          setSearch(keyword.trim())
        }}
      >
        <div className="flex flex-wrap gap-1">
          {FILTERS.map((f) => (
            <Button
              key={f.value}
              size="sm"
              variant={mime === f.value ? 'secondary' : 'ghost'}
              aria-pressed={mime === f.value}
              onClick={() => {
                setMime(f.value)
                setPage(1)
              }}
            >
              {f.label}
            </Button>
          ))}
        </div>
        <div className="ml-auto flex gap-2">
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="搜索文件名"
            aria-label="搜索媒体"
            className="h-8 w-40 rounded-md border border-input bg-transparent px-2 text-sm"
          />
          <Button type="submit" size="sm" variant="outline">
            搜索
          </Button>
        </div>
      </form>

      {error && <p className="mt-4 text-sm text-destructive">{error.message}</p>}

      {items.length === 0 && !loading && (
        <EmptyState icon={ImageIcon} message={search ? '没有匹配的文件' : '暂无文件'} />
      )}

      {items.length > 0 && (
        <ul className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {items.map((m) => (
            <li key={m.id} className="overflow-hidden rounded-md border border-border">
              <div className="flex h-28 items-center justify-center bg-muted/40">
                {m.mime.startsWith('image/') ? (
                  <img
                    src={m.url}
                    alt={m.alt_text || m.title || m.filename}
                    className="size-full object-cover"
                    loading="lazy"
                  />
                ) : (
                  <span className="truncate px-2 text-xs text-muted-foreground">
                    {m.mime || '文件'}
                  </span>
                )}
              </div>
              <div className="flex flex-col gap-1 p-2">
                <p className="truncate text-xs" title={m.title || m.filename}>
                  {m.title || m.filename}
                </p>
                <p className="truncate text-[10px] text-muted-foreground tabular-nums">
                  {formatBytes(m.size)} · {formatDate(m.created_at)}
                </p>
                <div className="flex items-center gap-1">
                  <Button
                    size="icon"
                    variant="ghost"
                    className="size-6"
                    onClick={() => void copyUrl(m.url)}
                    aria-label="复制地址"
                    title="复制地址"
                  >
                    <Copy className="size-3" />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-6 px-1.5 text-xs"
                    onClick={() => setEditing(m)}
                  >
                    编辑
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="ml-auto size-6 text-destructive"
                    onClick={() => setPendingDelete(m)}
                    aria-label="删除文件"
                  >
                    <Trash2 className="size-3" />
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {data && data.totalPages > 1 && (
        <div className="mt-4 flex items-center justify-between text-sm">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
            上一页
          </Button>
          <span className="text-xs text-muted-foreground tabular-nums">
            {page} / {data.totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= data.totalPages}
            onClick={() => setPage(page + 1)}
          >
            下一页
          </Button>
        </div>
      )}

      <EditMediaDialog
        item={editing}
        onOpenChange={(open) => !open && setEditing(null)}
        onSaved={() => {
          setEditing(null)
          refresh()
        }}
      />

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title="删除文件"
        description="仅删除媒体库记录；被文章引用的文件会保留在磁盘上。"
        onConfirm={remove}
      />
    </div>
  )
}

function EditMediaDialog({
  item,
  onOpenChange,
  onSaved,
}: {
  item: MediaItem | null
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const [title, setTitle] = useState('')
  const [alt, setAlt] = useState('')
  const [caption, setCaption] = useState('')
  const [busy, setBusy] = useState(false)
  const [loadedId, setLoadedId] = useState<number | null>(null)

  if (item && item.id !== loadedId) {
    setLoadedId(item.id)
    setTitle(item.title)
    setAlt(item.alt_text)
    setCaption(item.caption)
  }
  if (!item && loadedId !== null) setLoadedId(null)

  const save = async () => {
    if (!item) return
    setBusy(true)
    try {
      await api.updateMedia(item.id, { title, alt_text: alt, caption })
      toast.success('已保存')
      onSaved()
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={!!item} onOpenChange={onOpenChange}>
      <DialogContent className="shadow-none sm:max-w-96">
        <DialogHeader>
          <DialogTitle>编辑文件信息</DialogTitle>
          <DialogDescription className="truncate">{item?.filename}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="m-title">标题</Label>
            <Input
              id="m-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={200}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="m-alt">替代文本</Label>
            <Input
              id="m-alt"
              value={alt}
              onChange={(e) => setAlt(e.target.value)}
              maxLength={255}
              placeholder="供读屏软件使用"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="m-caption">说明</Label>
            <Input id="m-caption" value={caption} onChange={(e) => setCaption(e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" size="sm" />}>取消</DialogClose>
          <Button size="sm" onClick={() => void save()} disabled={busy}>
            {busy ? '保存中' : '保存'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
