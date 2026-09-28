import type { Editor } from '@tiptap/react'
import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import Placeholder from '@tiptap/extension-placeholder'
import { Markdown, type MarkdownStorage } from 'tiptap-markdown'
import {
  Bold,
  Code,
  Heading2,
  Heading3,
  Italic,
  Link2,
  List,
  ListOrdered,
  Minus,
  Quote,
  Strikethrough,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface ToolbarItem {
  icon: typeof Bold
  label: string
  isActive: (editor: Editor) => boolean
  toggle: (editor: Editor) => void
}

function setLink(editor: Editor) {
  const previous = editor.getAttributes('link').href as string | undefined
  const url = window.prompt('链接地址（留空取消）', previous ?? 'https://')
  if (url === null) return
  if (url === '') {
    editor.chain().focus().extendMarkRange('link').unsetLink().run()
    return
  }
  editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
}

const toolbarItems: ToolbarItem[] = [
  { icon: Bold, label: '加粗', isActive: (e) => e.isActive('bold'), toggle: (e) => e.chain().focus().toggleBold().run() },
  { icon: Italic, label: '斜体', isActive: (e) => e.isActive('italic'), toggle: (e) => e.chain().focus().toggleItalic().run() },
  { icon: Strikethrough, label: '删除线', isActive: (e) => e.isActive('strike'), toggle: (e) => e.chain().focus().toggleStrike().run() },
  { icon: Heading2, label: '二级标题', isActive: (e) => e.isActive('heading', { level: 2 }), toggle: (e) => e.chain().focus().toggleHeading({ level: 2 }).run() },
  { icon: Heading3, label: '三级标题', isActive: (e) => e.isActive('heading', { level: 3 }), toggle: (e) => e.chain().focus().toggleHeading({ level: 3 }).run() },
  { icon: List, label: '无序列表', isActive: (e) => e.isActive('bulletList'), toggle: (e) => e.chain().focus().toggleBulletList().run() },
  { icon: ListOrdered, label: '有序列表', isActive: (e) => e.isActive('orderedList'), toggle: (e) => e.chain().focus().toggleOrderedList().run() },
  { icon: Quote, label: '引用', isActive: (e) => e.isActive('blockquote'), toggle: (e) => e.chain().focus().toggleBlockquote().run() },
  { icon: Code, label: '代码块', isActive: (e) => e.isActive('codeBlock'), toggle: (e) => e.chain().focus().toggleCodeBlock().run() },
  { icon: Link2, label: '链接', isActive: (e) => e.isActive('link'), toggle: setLink },
  { icon: Minus, label: '分割线', isActive: () => false, toggle: (e) => e.chain().focus().setHorizontalRule().run() },
]

interface MarkdownEditorProps {
  /** 初始 Markdown（组件挂载时进入编辑器；之后以上下文为准，不回写） */
  initialValue: string
  /** 内容变化时回调最新 Markdown */
  onChange: (markdown: string) => void
  placeholder?: string
}

/** 富文本编辑器：Tiptap WYSIWYG，内部与数据库均保持 Markdown 格式 */
export function MarkdownEditor({ initialValue, onChange, placeholder = '开始写作…' }: MarkdownEditorProps) {
  const editor = useEditor({
    extensions: [
      StarterKit,
      Link.configure({ openOnClick: false, autolink: true }),
      Placeholder.configure({ placeholder }),
      Markdown,
    ],
    content: initialValue,
    onUpdate: ({ editor }) => {
      // tiptap-markdown 通过 storage 暴露序列化能力（TS 需显式声明）
      const { getMarkdown } = (editor.storage as unknown as { markdown: MarkdownStorage }).markdown
      onChange(getMarkdown())
    },
  })

  return (
    <div className="md-editor rounded-md border border-input text-sm">
      <div className="flex flex-wrap items-center gap-0.5 border-b border-input p-1">
        {toolbarItems.map(({ icon: Icon, label, isActive, toggle }) => (
          <Button
            key={label}
            type="button"
            variant="ghost"
            size="icon"
            aria-label={label}
            title={label}
            disabled={!editor}
            className={cn('size-8', editor && isActive(editor) && 'bg-muted text-foreground')}
            onClick={() => editor && toggle(editor)}
          >
            <Icon className="size-4" />
          </Button>
        ))}
      </div>
      <EditorContent editor={editor} className="md-body" />
    </div>
  )
}
