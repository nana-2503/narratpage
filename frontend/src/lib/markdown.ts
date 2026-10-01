import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({ breaks: true, gfm: true })

/** 估算中文阅读时长（CJK 400 字/分钟 + 英文 220 词/分钟，至少 1 分钟） */
export function estimateReadingTime(content: string): string {
  const text = content.replace(/```[\s\S]*?```/g, ' ').replace(/\s+/g, ' ').trim()
  const cjk = (text.match(/[\u4e00-\u9fff]/g) || []).length
  const words = (text.replace(/[\u4e00-\u9fff]/g, ' ').match(/[A-Za-z0-9]+/g) || []).length
  const minutes = Math.max(1, Math.ceil(cjk / 400 + words / 220))
  return `约 ${minutes} 分钟`
}

/**
 * 解析后端时间字符串：
 * - 带时区的 ISO（"2026-09-28T10:00:00.000Z"）直接解析
 * - SQLite datetime('now') 输出的 "YYYY-MM-DD HH:MM:SS"（UTC）按 UTC 解析，避免被当成本地时区
 */
function parseDate(iso: string | null): Date | null {
  if (!iso) return null
  const d = new Date(iso.includes('T') ? iso : `${iso.replace(' ', 'T')}Z`)
  return Number.isNaN(d.getTime()) ? null : d
}

const pad = (n: number) => String(n).padStart(2, '0')

export function formatDate(iso: string | null): string {
  const d = parseDate(iso)
  if (!d) return iso || '—'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function formatDateTime(iso: string | null): string {
  const d = parseDate(iso)
  if (!d) return iso || '—'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 相对时间：刚刚 / N 分钟前 / N 小时前 / N 天前 / 具体日期 */
export function formatRelative(iso: string | null): string {
  const d = parseDate(iso)
  if (!d) return '—'
  const diff = Date.now() - d.getTime()
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  if (diff < minute) return '刚刚'
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`
  if (diff < 30 * day) return `${Math.floor(diff / day)} 天前`
  return formatDate(iso)
}

export function renderMarkdown(src: string): string {
  const html = marked.parse(src || '', { async: false }) as string
  return DOMPurify.sanitize(html)
}

/** 从 Markdown 提取纯文本，用于摘要与 SEO description */
export function stripMarkdown(src: string): string {
  return (src || '')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/[*_`>#-]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

/** 文件大小格式化 */
export function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)))
  return `${(bytes / 1024 ** i).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}
