import { useEffect, useState } from 'react'
import { api, type SiteOptions } from '@/lib/api'

/**
 * 站点设置的全局读取。
 *
 * 站点标题、导航、每页条数等被多个页面使用，逐页请求会造成
 * 重复请求与闪烁，故在顶层取一次并通过 context 分发。
 */
interface SiteState {
  site: SiteOptions
  loading: boolean
  reload: () => void
}

const DEFAULTS: SiteOptions = {
  title: '叙页博客系统',
  tagline: '',
  description: '',
  url: '',
  locale: 'zh-CN',
  timezone: 'Asia/Shanghai',
  date_format: 'YYYY-MM-DD',
  links: [],
  comments_enabled: true,
  comment_moderation: true,
  registration_open: false,
  posts_per_page: 10,
  show_author: true,
  show_date: true,
  show_reading_time: true,
  show_tags: true,
  show_cover: true,
  post_count: 0,
  page_count: 0,
  comment_count: 0,
  tag_count: 0,
}

let cache: SiteOptions | null = null
let inflight: Promise<SiteOptions> | null = null

/** 读取站点设置。首次调用会发起请求，之后走内存缓存。 */
export function useSite(): SiteState {
  const [site, setSite] = useState<SiteOptions>(cache ?? DEFAULTS)
  const [loading, setLoading] = useState(!cache)
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    let alive = true
    if (!inflight) {
      inflight = api
        .site()
        .then((s) => {
          cache = s
          return s
        })
        .catch(() => DEFAULTS)
        .finally(() => {
          inflight = null
        })
    }
    setLoading(true)
    void inflight.then((s) => {
      if (!alive) return
      setSite(s)
      setLoading(false)
    })
    return () => {
      alive = false
    }
  }, [nonce])

  return { site, loading, reload: () => setNonce((n) => n + 1) }
}

/** 站点设置变更后（后台保存）清缓存，让下次读取取到新值 */
export function invalidateSiteCache() {
  cache = null
}
