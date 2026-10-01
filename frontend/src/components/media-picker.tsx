import { useRef, useState } from 'react'
import { Image as ImageIcon, Loader2, Trash2, Upload } from 'lucide-react'
import { api, type MediaItem } from '@/lib/api'
import { useAsync } from '@/hooks/use-async'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { EmptyState } from '@/components/empty-state'

/**
 * 封面图选择器：媒体库挑选 / 本地上传 / 直填 URL 三种来源。
 *
 * 媒体库已有完整的上传与管理页，这里只做「选一个」，
 * 避免作者为了张封面还要跳走再手动复制地址。
 */
export function MediaPicker({
  value,
  onChange,
  label = '封面图',
}: {
  value: string
  onChange: (url: string) => void
  label?: string
}) {
  const fileRef = useRef<HTMLInputElement>(null)
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [uploading, setUploading] = useState(false)

  const { data, loading } = useAsync(
    () =>
      open
        ? api.listMedia({ mime: 'image', q: search || undefined, pageSize: 60 })
        : Promise.resolve(null),
    [open, search],
  )
  const items = data?.items ?? []

  const upload = async (files: FileList | null) => {
    const file = files?.[0]
    if (!file) return
    setUploading(true)
    try {
      const res = await api.uploadImage(file)
      onChange(res.url)
      toast.success('已上传并设为封面')
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-sm font-medium">{label}</span>

      <div className="flex items-start gap-3">
        {/* 缩略图：同时充当「从媒体库选择」的点击区 */}
        <button
          type="button"
          onClick={() => setOpen(true)}
          aria-label="从媒体库选择封面图"
          title="从媒体库选择"
          className="relative flex size-24 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-muted transition-colors hover:border-foreground/40"
        >
          {value ? (
            <img src={value} alt="" className="size-full object-cover" />
          ) : (
            <ImageIcon className="size-5 text-muted-foreground" />
          )}
        </button>

        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <Input
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder="图片地址"
            aria-label={`${label}地址`}
            className="h-8 text-xs"
          />
          <div className="flex flex-wrap gap-1">
            <Button type="button" size="sm" variant="outline" onClick={() => setOpen(true)}>
              媒体库
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={uploading}
              onClick={() => fileRef.current?.click()}
            >
              {uploading ? <Loader2 className="size-3.5 animate-spin" /> : <Upload className="size-3.5" />}
              上传
            </Button>
            {value && (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() => onChange('')}
                aria-label="清除封面图"
              >
                <Trash2 className="size-3.5" />
                清除
              </Button>
            )}
          </div>
        </div>
      </div>

      <input
        ref={fileRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={(e) => void upload(e.target.files)}
      />

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>选择封面图</DialogTitle>
            <DialogDescription>媒体库</DialogDescription>
          </DialogHeader>

          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索文件名"
            aria-label="搜索媒体"
            className="h-8 text-sm"
          />

          {loading ? (
            <div className="flex justify-center py-10 text-muted-foreground">
              <Loader2 className="size-5 animate-spin" />
            </div>
          ) : items.length === 0 ? (
            <EmptyState
              icon={ImageIcon}
              message="没有图片"
              action={
                <Button type="button" size="sm" variant="outline" onClick={() => fileRef.current?.click()}>
                  <Upload className="size-4" />
                  上传
                </Button>
              }
            />
          ) : (
            <ul className="grid max-h-[60vh] grid-cols-3 gap-2 overflow-y-auto sm:grid-cols-4 md:grid-cols-5">
              {items.map((m) => (
                <li key={m.id}>
                  <button
                    type="button"
                    onClick={() => {
                      onChange(m.url)
                      setOpen(false)
                    }}
                    title={m.title || m.filename}
                    className="block w-full overflow-hidden rounded-md border border-border transition-colors hover:border-foreground/40 focus-visible:ring-3 focus-visible:ring-ring/50"
                  >
                    <img
                      src={m.url}
                      alt=""
                      loading="lazy"
                      className="aspect-square w-full object-cover"
                    />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}

/** 供其它位置复用：判断媒体是否为图片 */
export function isImage(m: MediaItem) {
  return m.mime.startsWith('image/')
}