/**
 * SEO 与社交分享卡片的 meta 管理。
 *
 * SPA 内切换路由不会触发浏览器默认的 head 更新，故需在每次
 * 页面数据到达后显式同步。
 */

const SITE_SUFFIX_FALLBACK = '叙页博客系统'

function upsertMeta(selector: string, attr: 'name' | 'property', key: string, content: string) {
  let tag = document.querySelector<HTMLMetaElement>(selector)
  if (!tag) {
    tag = document.createElement('meta')
    tag.setAttribute(attr, key)
    document.head.appendChild(tag)
  }
  tag.setAttribute('content', content)
}

function removeMeta(selector: string) {
  document.querySelector(selector)?.remove()
}

function upsertLink(rel: string, href: string) {
  let link = document.querySelector<HTMLLinkElement>(`link[rel="${rel}"]`)
  if (!link) {
    const created = document.createElement('link')
    created.setAttribute('rel', rel)
    document.head.appendChild(created)
    link = created
  }
  link.setAttribute('href', href)
}

/** 设置详情页的 title / description / OG 标签 */
export function setPageMeta(
  title: string,
  description?: string,
  options?: { image?: string; type?: string; url?: string },
) {
  document.title = description ? `${title} · ${SITE_SUFFIX_FALLBACK}` : title || SITE_SUFFIX_FALLBACK
  if (description) {
    const desc = description.slice(0, 200)
    upsertMeta('meta[name="description"]', 'name', 'description', desc)
    upsertMeta('meta[property="og:description"]', 'property', 'og:description', desc)
    upsertMeta('meta[name="twitter:description"]', 'name', 'twitter:description', desc)
  }
  upsertMeta('meta[property="og:title"]', 'property', 'og:title', title)
  upsertMeta('meta[name="twitter:title"]', 'name', 'twitter:title', title)
  if (options?.type) upsertMeta('meta[property="og:type"]', 'property', 'og:type', options.type)
  if (options?.url) upsertMeta('meta[property="og:url"]', 'property', 'og:url', options.url)
  if (options?.image) {
    upsertMeta('meta[property="og:image"]', 'property', 'og:image', options.image)
    upsertMeta('meta[name="twitter:image"]', 'name', 'twitter:image', options.image)
  }
}

/** 清除详情页专属标签，避免上一篇的内容残留 */
export function clearPostMeta() {
  for (const sel of [
    'meta[property="og:type"]',
    'meta[property="og:url"]',
    'meta[property="og:image"]',
    'meta[name="twitter:image"]',
  ]) {
    removeMeta(sel)
  }
}

/** 设置站点级标签（首页与列表页） */
export function setSiteMeta(site: { title: string; description: string; url: string }) {
  document.title = site.title
  const desc = site.description || site.title
  upsertMeta('meta[name="description"]', 'name', 'description', desc.slice(0, 200))
  upsertMeta('meta[property="og:site_name"]', 'property', 'og:site_name', site.title)
  upsertMeta('meta[property="og:type"]', 'property', 'og:type', 'website')
  if (site.url) {
    upsertMeta('meta[property="og:url"]', 'property', 'og:url', site.url)
    upsertLink('canonical', site.url)
  }
}

/** 注入 JSON-LD 结构化数据（文章详情页用） */
export function setJsonLd(data: Record<string, unknown> | null) {
  const existing = document.getElementById('json-ld')
  if (!data) {
    existing?.remove()
    return
  }
  const script = (existing as HTMLScriptElement | null) ?? document.createElement('script')
  script.id = 'json-ld'
  script.type = 'application/ld+json'
  script.textContent = JSON.stringify(data)
  if (!existing) document.head.appendChild(script)
}
