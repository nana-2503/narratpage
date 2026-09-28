import { useRef, useState } from 'react'
import { ImagePlus, Link2, Upload } from 'lucide-react'
import { api } from '@/lib/api'
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
import { toast } from 'sonner'

interface EditorImageDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** 确认插入：src 为可访问地址，alt 为替代文本 */
  onInsert: (src: string, alt: string) => void
}

type Mode = 'upload' | 'url'

const ACCEPT = 'image/png,image/jpeg,image/webp,image/gif'

export function EditorImageDialog({ open, onOpenChange, onInsert }: EditorImageDialogProps) {
  const [mode, setMode] = useState<Mode>('upload')
  const [url, setUrl] = useState('')
  const [alt, setAlt] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const reset = () => {
    setUrl('')
    setAlt('')
    setFile(null)
    setUploading(false)
  }

  const close = () => {
    reset()
    onOpenChange(false)
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (mode === 'url') {
      const src = url.trim()
      if (!src) {
        toast.error('请填写图片地址')
        return
      }
      onInsert(src, alt.trim())
      close()
      return
    }
    if (!file) {
      toast.error('请先选择图片')
      return
    }
    setUploading(true)
    try {
      const { url: src } = await api.uploadImage(file)
      onInsert(src, alt.trim())
      close()
    } catch (err) {
      toast.error((err as Error).message)
    } finally {
      setUploading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && close()}>
      <DialogContent className="shadow-none sm:max-w-96">
        <DialogHeader>
          <DialogTitle>插入图片</DialogTitle>
          <DialogDescription>上传图片或粘贴图片链接</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <div className="flex gap-1">
            <Button
              type="button"
              size="sm"
              variant={mode === 'upload' ? 'secondary' : 'ghost'}
              onClick={() => setMode('upload')}
            >
              <Upload className="size-4" /> 上传
            </Button>
            <Button
              type="button"
              size="sm"
              variant={mode === 'url' ? 'secondary' : 'ghost'}
              onClick={() => setMode('url')}
            >
              <Link2 className="size-4" /> 链接
            </Button>
          </div>

          {mode === 'upload' ? (
            <div className="flex flex-col gap-1.5">
              <input
                ref={fileRef}
                type="file"
                accept={ACCEPT}
                className="hidden"
                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              />
              <Button type="button" variant="outline" onClick={() => fileRef.current?.click()}>
                <ImagePlus className="size-4" /> {file ? '重新选择' : '选择图片'}
              </Button>
              <p className="text-xs text-muted-foreground">
                {file
                  ? `${file.name}（${(file.size / 1024).toFixed(0)} KB）`
                  : '支持 PNG / JPEG / WebP / GIF，最大 5MB'}
              </p>
            </div>
          ) : (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="img-url">图片地址</Label>
              <Input
                id="img-url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://..."
              />
            </div>
          )}

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="img-alt">替代文本（可选）</Label>
            <Input
              id="img-alt"
              value={alt}
              onChange={(e) => setAlt(e.target.value)}
              placeholder="图片说明"
              maxLength={100}
            />
          </div>

          <DialogFooter>
            <DialogClose render={<Button variant="outline" size="sm" />}>取消</DialogClose>
            <Button type="submit" size="sm" disabled={uploading}>
              {uploading ? '上传中' : '插入'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
