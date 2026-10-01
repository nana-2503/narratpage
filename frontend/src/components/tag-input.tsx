import { useState } from 'react'
import { X } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'

/**
 * 标签输入：回车或逗号添加，退格删除末项。
 * 标签名交由后端归一化（同名自动合并），前端不做存在性校验。
 */
export function TagInput({
  value,
  onChange,
  placeholder = '输入标签后回车',
  suggestions = [],
}: {
  value: string[]
  onChange: (v: string[]) => void
  placeholder?: string
  suggestions?: string[]
}) {
  const [input, setInput] = useState('')

  const add = (raw: string) => {
    const name = raw.trim().replace(/,$/, '').trim()
    if (!name) return
    if (value.includes(name)) {
      setInput('')
      return
    }
    onChange([...value, name])
    setInput('')
  }

  const remove = (name: string) => {
    onChange(value.filter((v) => v !== name))
  }

  // 未添加过的建议标签
  const available = suggestions.filter((s) => !value.includes(s)).slice(0, 8)

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-1.5">
        {value.map((t) => (
          <Badge key={t} variant="secondary" className="gap-1 font-normal">
            {t}
            <button
              type="button"
              onClick={() => remove(t)}
              aria-label={`移除标签 ${t}`}
              className="text-muted-foreground hover:text-foreground"
            >
              <X className="size-3" />
            </button>
          </Badge>
        ))}
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ',') {
              e.preventDefault()
              add(input)
            } else if (e.key === 'Backspace' && input === '' && value.length > 0) {
              // 退格删末项：连续输入时的常用习惯
              const last = value[value.length - 1]
              if (last !== undefined) remove(last)
            }
          }}
          onBlur={() => add(input)}
          placeholder={value.length === 0 ? placeholder : ''}
          aria-label="添加标签"
          className="h-7 w-40 flex-1"
        />
      </div>

      {available.length > 0 && (
        <div className="flex flex-wrap items-center gap-1">
          <span className="text-xs text-muted-foreground">已有：</span>
          {available.map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => add(s)}
              className="rounded-sm border border-border px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground"
            >
              {s}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
