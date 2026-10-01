// 与后端 API 的契约类型。
//
// 后端 SQL / 校验规则变更时需同步更新此处。
// 约定：本文件不依赖 React，可被任意模块引入。

const TOKEN_KEY = 'blog_admin_token'
const INSTALLED_KEY = 'blog_installed'
const USER_KEY = 'blog_admin_user'

// 同源部署用默认 '/api'；前后端分离部署时构建时指定 VITE_API_BASE
const API_BASE = (import.meta.env.VITE_API_BASE || '/api').replace(/\/+$/, '')

// ---------- 令牌与身份 ----------

export interface AuthUser {
  id: number
  username: string
  display_name: string
  role: UserRole
  email?: string
  avatar_url?: string
  bio?: string
  active?: boolean
  created_at?: string
  last_login_at?: string | null
}

export type UserRole = 'admin' | 'editor' | 'author' | 'contributor' | 'subscriber'

/** 角色对应的能力点，与后端 auth.roleCaps 保持一致 */
export const ROLE_CAPS: Record<UserRole, string[]> = {
  admin: ['*'],
  editor: [
    'post.create', 'post.edit_any', 'post.edit_own', 'post.delete', 'post.publish',
    'page.manage', 'media.manage', 'comment.manage', 'taxonomy.manage',
  ],
  author: ['post.create', 'post.edit_own', 'post.publish', 'media.manage'],
  contributor: ['post.create', 'post.edit_own'],
  subscriber: [],
}

export function can(role: UserRole | undefined, cap: string): boolean {
  if (!role) return false
  const caps = ROLE_CAPS[role] || []
  return caps.includes('*') || caps.includes(cap)
}

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

/** 读取缓存的登录身份，避免首屏闪一下未登录态 */
export function getCachedUser(): AuthUser | null {
  try {
    const raw = localStorage.getItem(USER_KEY)
    return raw ? (JSON.parse(raw) as AuthUser) : null
  } catch {
    return null
  }
}

export function setCachedUser(user: AuthUser | null) {
  if (user) localStorage.setItem(USER_KEY, JSON.stringify(user))
  else localStorage.removeItem(USER_KEY)
}

export function getInstalled(): boolean {
  return localStorage.getItem(INSTALLED_KEY) === 'true'
}

export function setInstalled(value: boolean) {
  if (value) localStorage.setItem(INSTALLED_KEY, 'true')
  else localStorage.removeItem(INSTALLED_KEY)
}

// ---------- 错误 ----------

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/** 判断加载错误是否为 404（供 not-found 展示分支使用） */
export function isNotFound(err: Error | null): boolean {
  return err instanceof ApiError && err.status === 404
}

/** 判断是否为未认证/登录过期 */
export function isUnauthorized(err: Error | null): boolean {
  return err instanceof ApiError && err.status === 401
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    ...(typeof options.body === 'string' ? { 'content-type': 'application/json' } : {}),
    ...(token ? { authorization: `Bearer ${token}` } : {}),
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    if (res.status === 401) {
      setToken(null)
      setCachedUser(null)
    }
    throw new ApiError(res.status, (data as { error?: string }).error || `请求失败 (${res.status})`)
  }
  return data as T
}

// ---------- 内容模型 ----------

export interface Category {
  id: number
  name: string
  slug: string
  description?: string
  is_page?: number
  position?: number
  post_count?: number
}

export interface Tag {
  id: number
  name: string
  slug: string
  description?: string
  post_count?: number
}

export interface Author {
  id: number
  username: string
  display_name: string
  avatar_url?: string
}

export type PostStatus = 'draft' | 'published' | 'pending' | 'private' | 'trash'
export type PostType = 'post' | 'page' | 'attachment'

export interface Post {
  id: number
  title: string
  slug: string
  summary: string
  content: string
  cover_url: string
  category_id: number | null
  category_name: string | null
  category_slug: string | null
  status: PostStatus
  type: PostType
  sticky: boolean
  has_password: boolean
  menu_order: number
  author_id: number | null
  author?: Author | null
  views: number
  template?: string
  tags?: Tag[]
  created_at: string
  updated_at: string
  published_at: string | null
  scheduled_at: string | null
  deleted_at: string | null
}

export interface Comment {
  id: number
  post_id: number
  parent_id: number | null
  author_id: number | null
  author: string
  website?: string
  content: string
  status: CommentStatus
  is_pingback: boolean
  created_at: string
  post_title?: string
  replies?: Comment[]
}

export type CommentStatus = 'pending' | 'approved' | 'rejected' | 'spam'

export interface MediaItem {
  id: number
  filename: string
  url: string
  mime: string
  size: number
  title: string
  alt_text: string
  caption: string
  width: number
  height: number
  uploaded_by: number | null
  created_at: string
}

export interface Revision {
  id: number
  post_id: number
  author_id: number | null
  title: string
  content: string
  excerpt: string
  created_at: string
}

export interface Redirect {
  id: number
  from_path: string
  to_path: string
  hits: number
  enabled: boolean
  created_at: string
}

export interface PostListResult {
  items: Post[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

export interface PostNeighbors {
  prev: { slug: string; title: string } | null
  next: { slug: string; title: string } | null
}

/** 文章表单提交体；slug 为空串表示由后端自动生成 */
export interface PostInput {
  title: string
  slug?: string
  summary: string
  content: string
  cover_url: string
  category_id: number | null
  status: PostStatus
  type?: PostType
  sticky?: boolean
  password?: string
  menu_order?: number
  tags?: string[]
  tag_ids?: number[]
  meta?: Record<string, string>
}

export interface SiteLink {
  id?: number
  label: string
  url: string
  position: 'header' | 'footer' | 'social'
  target?: string
  order?: number
}

export interface SiteOptions {
  title: string
  tagline: string
  description: string
  url: string
  locale: string
  timezone: string
  date_format: string
  links: SiteLink[]
  comments_enabled: boolean
  comment_moderation: boolean
  registration_open: boolean
  posts_per_page: number
  show_author: boolean
  show_date: boolean
  show_reading_time: boolean
  show_tags: boolean
  /** 是否显示封面图 */
  show_cover: boolean
  post_count: number
  page_count: number
  comment_count: number
  tag_count: number
}

export interface SiteStats {
  posts: number
  pages: number
  comments: number
  tags: number
  views: number
  users: Record<string, number>
  media: { count: number; size: number }
  pendingComments: number
}

export interface ArchiveData {
  total: number
  years: { slug: string; count: number }[]
  months: { month: string; count: number }[]
  categories: Category[]
  tags: Tag[]
  authors: {
    id: number
    username: string
    display_name: string
    avatar_url: string
    post_count: number
  }[]
}

export interface SearchResult {
  results: Post[]
  tags: Tag[]
  categories: Category[]
  total: number
}

export interface InstallStatus {
  installed: boolean
  dbType: string
  redisEnabled: boolean
}

export interface InstallResult {
  ok: boolean
  username: string
  siteTitle: string
  dbType: string
  env: Record<string, string>
  needRestart: boolean
}

export interface SessionInfo {
  id: number
  ip: string
  user_agent: string
  created_at: string
  last_seen_at: string
  current: boolean
}

/** 构造查询串，跳过空值 */
function qs(params: object): string {
  const search = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === '') continue
    search.set(k, String(v))
  }
  const s = search.toString()
  return s ? `?${s}` : ''
}

export interface ListParams {
  page?: number
  pageSize?: number
  category?: string
  tag?: string
  q?: string
  author?: number
  year?: number
  month?: number
  sticky?: boolean
  status?: string
  type?: PostType
  order?: string
}

export const api = {
  // ---------- 安装 ----------
  installStatus: () => request<InstallStatus>('/install/status'),
  install: (body: {
    dbType: string
    dbDsn: string
    adminUsername: string
    adminPassword: string
    adminEmail?: string
    siteUrl: string
    siteTitle?: string
    redisEnabled: boolean
    redisUrl: string
  }) =>
    request<InstallResult>('/install', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  installApply: (body: {
    adminUsername: string
    adminPassword: string
    adminEmail?: string
    siteUrl: string
    siteTitle?: string
  }) =>
    request<{ ok: boolean; username: string; seeded: boolean }>('/install/apply', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  // ---------- 认证 ----------
  login: (username: string, password: string) =>
    request<{ token: string; user: AuthUser }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  me: () => request<AuthUser>('/auth/me'),
  logout: () => request<{ ok: boolean }>('/auth/logout', { method: 'POST' }),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<{ ok: true }>('/auth/password', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    }),
  sessions: () => request<{ items: SessionInfo[] }>('/auth/sessions'),
  revokeOtherSessions: () =>
    request<{ ok: boolean; revoked: number }>('/auth/sessions/others', { method: 'DELETE' }),

  // ---------- 文章 ----------
  listPosts: (params: ListParams = {}) => request<PostListResult>(`/posts${qs(params)}`),
  listPages: (params: ListParams = {}) => request<PostListResult>(`/pages${qs(params)}`),
  getPost: (slugOrId: string | number) =>
    request<Post>(`/posts/${encodeURIComponent(String(slugOrId))}`),
  getPostNeighbors: (slug: string) =>
    request<PostNeighbors>(`/posts/${encodeURIComponent(slug)}/neighbors`),
  createPost: (body: Partial<PostInput>) =>
    request<{ id: number; slug: string }>('/posts', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  updatePost: (id: number, body: Partial<PostInput>) =>
    request<{ id: number }>(`/posts/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deletePost: (id: number) => request<{ ok: true }>(`/posts/${id}`, { method: 'DELETE' }),
  trashPost: (id: number) => request<{ ok: true }>(`/posts/${id}/trash`, { method: 'POST' }),
  restorePost: (id: number) => request<{ ok: true }>(`/posts/${id}/restore`, { method: 'POST' }),
  purgeTrash: () =>
    request<{ ok: true; deleted: number }>('/posts/trash/purge', { method: 'DELETE' }),
  batchPosts: (ids: number[], action: string) =>
    request<{ ok: number; skipped: number }>('/posts/batch', {
      method: 'POST',
      body: JSON.stringify({ ids, action }),
    }),

  // ---------- 标签关联 ----------
  addPostTag: (postId: number, tagId: number, tagName?: string) =>
    request<{ ok: true }>(`/posts/${postId}/tags`, {
      method: 'POST',
      body: JSON.stringify({ tag_id: tagId, tag_name: tagName }),
    }),
  removePostTag: (postId: number, tagId: number) =>
    request<{ ok: true }>(`/posts/${postId}/tags/${tagId}`, { method: 'DELETE' }),

  // ---------- 元数据 ----------
  getPostMeta: (id: number) =>
    request<{ items: Record<string, string> }>(`/posts/${id}/meta`),
  setPostMeta: (id: number, meta: Record<string, string>) =>
    request<{ ok: true }>(`/posts/${id}/meta`, {
      method: 'PUT',
      body: JSON.stringify(meta),
    }),
  deletePostMeta: (id: number, key: string) =>
    request<{ ok: true }>(`/posts/${id}/meta/${encodeURIComponent(key)}`, {
      method: 'DELETE',
    }),

  // ---------- 历史版本 ----------
  listRevisions: (id: number) => request<{ items: Revision[] }>(`/posts/${id}/revisions`),
  saveRevision: (id: number) => request<{ id: number }>(`/posts/${id}/revisions`, { method: 'POST' }),
  restoreRevision: (id: number, revId: number) =>
    request<{ ok: true }>(`/posts/${id}/revisions/${revId}/restore`, { method: 'POST' }),
  deleteRevision: (id: number, revId: number) =>
    request<{ ok: true }>(`/posts/${id}/revisions/${revId}`, { method: 'DELETE' }),

  // ---------- 分类 ----------
  listCategories: (isPage = false) =>
    request<{ items: Category[] }>(`/categories${qs({ page: isPage ? 1 : undefined })}`),
  createCategory: (name: string, description = '') =>
    request<Category>('/categories', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  updateCategory: (id: number, body: { name?: string; slug?: string; description?: string }) =>
    request<{ id: number; slug: string }>(`/categories/${id}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  deleteCategory: (id: number) => request<{ ok: true }>(`/categories/${id}`, { method: 'DELETE' }),
  reorderCategories: (ids: number[]) =>
    request<{ ok: true }>('/categories/reorder', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    }),

  // ---------- 标签 ----------
  listTags: (q = '') => request<{ items: Tag[] }>(`/tags${qs({ q })}`),
  createTag: (name: string) =>
    request<Tag>('/tags', { method: 'POST', body: JSON.stringify({ name }) }),
  updateTag: (id: number, body: { name?: string; slug?: string; description?: string }) =>
    request<{ id: number; slug: string }>(`/tags/${id}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  deleteTag: (id: number) => request<{ ok: true }>(`/tags/${id}`, { method: 'DELETE' }),
  mergeTags: (source: number, target: number) =>
    request<{ ok: true; moved: number }>('/tags/merge', {
      method: 'POST',
      body: JSON.stringify({ source, target }),
    }),

  // ---------- 评论 ----------
  listComments: (params: { postId?: number; status?: string; q?: string; nested?: boolean; page?: number } = {}) =>
    request<{ items: Comment[]; total: number; page: number; pageSize: number; totalPages: number }>(
      `/comments${qs(params)}`,
    ),
  createComment: (body: { postId?: number; slug?: string; parentId?: number | null; author: string; email?: string; website?: string; content: string; website_confirm?: string }) =>
    request<{ id: number; status: string }>('/comments', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  moderateComment: (id: number, status: CommentStatus) =>
    request<{ id: number; status: string }>(`/comments/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ status }),
    }),
  deleteComment: (id: number) => request<{ ok: true }>(`/comments/${id}`, { method: 'DELETE' }),
  batchComments: (ids: number[], action: string) =>
    request<{ ok: true; affected: number }>('/comments/batch', {
      method: 'POST',
      body: JSON.stringify({ ids, action }),
    }),
  commentCounts: () =>
    request<Record<CommentStatus, number>>('/comments/counts'),

  // ---------- 媒体 ----------
  uploadImage: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<{ id: number; url: string; filename: string; mime: string; size: number }>(
      '/uploads',
      { method: 'POST', body: form },
    )
  },
  listMedia: (params: { page?: number; pageSize?: number; mime?: string; q?: string } = {}) =>
    request<{
      items: MediaItem[]
      total: number
      page: number
      pageSize: number
      totalPages: number
      count: number
      totalSize: number
    }>(`/media${qs(params)}`),
  updateMedia: (id: number, body: { title?: string; alt_text?: string; caption?: string }) =>
    request<MediaItem>(`/media/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteMedia: (id: number) =>
    request<{ ok: true; referenced: number }>(`/media/${id}`, { method: 'DELETE' }),

  // ---------- 用户 ----------
  updateMe: (body: { display_name?: string; email?: string; bio?: string; avatar_url?: string }) =>
    request<AuthUser>('/users/me', { method: 'PUT', body: JSON.stringify(body) }),
  listUsers: (params: { page?: number; pageSize?: number; q?: string } = {}) =>
    request<{ items: AuthUser[]; total: number; page: number; pageSize: number; totalPages: number }>(
      `/users${qs(params)}`,
    ),
  createUser: (body: {
    username: string
    password: string
    email?: string
    display_name?: string
    role?: UserRole
    bio?: string
  }) => request<AuthUser>('/users', { method: 'POST', body: JSON.stringify(body) }),
  updateUser: (
    id: number,
    body: {
      email?: string
      display_name?: string
      role?: UserRole
      bio?: string
      active?: boolean
      password?: string
    },
  ) => request<AuthUser>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteUser: (id: number) => request<{ ok: true }>(`/users/${id}`, { method: 'DELETE' }),

  // ---------- 站点 ----------
  site: () => request<SiteOptions>('/site'),
  settings: () => request<SiteStats>('/settings'),
  updateSettings: (body: Partial<SiteOptions>) =>
    request<SiteStats>('/settings', { method: 'PUT', body: JSON.stringify(body) }),
  stats: () => request<SiteStats>('/stats'),
  archive: () => request<ArchiveData>('/archive'),
  search: (q: string) => request<SearchResult>(`/search${qs({ q })}`),

  // ---------- 重定向 ----------
  listRedirects: () => request<{ items: Redirect[] }>('/redirects'),
  createRedirect: (fromPath: string, toPath: string) =>
    request<{ id: number }>('/redirects', {
      method: 'POST',
      body: JSON.stringify({ from_path: fromPath, to_path: toPath }),
    }),
  deleteRedirect: (id: number) => request<{ ok: true }>(`/redirects/${id}`, { method: 'DELETE' }),
}
