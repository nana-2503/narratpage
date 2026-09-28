const SITE_SUFFIX = '叙页博客系统';

function upsertMeta(selector: string, attr: 'name' | 'property', key: string, content: string) {
  let tag = document.querySelector<HTMLMetaElement>(selector);
  if (!tag) {
    tag = document.createElement('meta');
    tag.setAttribute(attr, key);
    document.head.appendChild(tag);
  }
  tag.setAttribute('content', content);
}

/** 设置详情页的 title / description / OG 标签（SPA 内切换路由时同步） */
export function setPageMeta(title: string, description?: string) {
  document.title = description ? `${title} · ${SITE_SUFFIX}` : title || SITE_SUFFIX;
  if (description) {
    upsertMeta('meta[name="description"]', 'name', 'description', description.slice(0, 200));
    upsertMeta('meta[property="og:title"]', 'property', 'og:title', title);
    upsertMeta('meta[property="og:description"]', 'property', 'og:description', description.slice(0, 200));
  }
}
