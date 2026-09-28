// 与后端 API 的契约类型。后端 SQL / 校验规则变更时需同步更新此处。
const TOKEN_KEY = 'blog_admin_token';
const INSTALLED_KEY = 'blog_installed';

// 同源部署用默认 '/api'；前后端分离部署时构建时指定 VITE_API_BASE
const API_BASE = (import.meta.env.VITE_API_BASE || '/api').replace(/\/+$/, '');

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

export function getInstalled(): boolean {
  return localStorage.getItem(INSTALLED_KEY) === 'true';
}

export function setInstalled(value: boolean) {
  if (value) localStorage.setItem(INSTALLED_KEY, 'true');
  else localStorage.removeItem(INSTALLED_KEY);
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

/** 判断加载错误是否为 404（供 not-found 展示分支使用） */
export function isNotFound(err: Error | null): boolean {
  return err instanceof ApiError && err.status === 404;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    ...(typeof options.body === 'string' ? { 'content-type': 'application/json' } : {}),
    ...(token ? { authorization: `Bearer ${token}` } : {}),
  };
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 401) setToken(null);
    throw new ApiError(res.status, (data as { error?: string }).error || `请求失败 (${res.status})`);
  }
  return data as T;
}

export interface Category {
  id: number;
  name: string;
  slug: string;
  post_count?: number;
}

export interface Post {
  id: number;
  title: string;
  slug: string;
  summary: string;
  cover_url: string;
  content?: string;
  status: 'draft' | 'published';
  views: number;
  created_at: string;
  updated_at: string;
  published_at: string | null;
  category_id: number | null;
  category_name: string | null;
  category_slug: string | null;
}

export interface Comment {
  id: number;
  post_id: number;
  author: string;
  content: string;
  status: 'pending' | 'approved' | 'rejected';
  created_at: string;
  /** 后台审核列表由后端联表返回；前台不返回 */
  post_title?: string;
}

export interface PostListResult {
  items: Post[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

/** 详情页上下篇（prev = 更新的文章，next = 更早的文章） */
export interface PostNeighbors {
  prev: { slug: string; title: string } | null;
  next: { slug: string; title: string } | null;
}

/** 文章表单提交体；slug 为空串表示由后端自动生成 */
export interface PostInput {
  title: string;
  slug: string;
  summary: string;
  content: string;
  cover_url: string;
  category_id: number | null;
  status: 'draft' | 'published';
}

export interface InstallStatus {
  installed: boolean;
  dbType: string;
  redisEnabled: boolean;
}

export interface InstallResponse {
  ok: boolean;
  config: Record<string, string>;
}

export const api = {
  // 安装
  installStatus: () => request<InstallStatus>('/install/status'),
  install: (body: {
    dbType: string;
    dbDsn: string;
    adminUsername: string;
    adminPassword: string;
    siteUrl: string;
    redisEnabled: boolean;
    redisUrl: string;
  }) =>
    request<InstallResponse>('/install', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  // 认证
  login: (username: string, password: string) =>
    request<{ token: string; username: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  me: () => request<{ username: string }>('/auth/me'),
  uploadImage: (file: File) => {
    const form = new FormData();
    form.append('file', file);
    return request<{ url: string }>('/uploads', { method: 'POST', body: form });
  },
  changePassword: (oldPassword: string, newPassword: string) =>
    request<{ ok: true }>('/auth/password', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    }),

  // 文章
  listPosts: (params: { page?: number; pageSize?: number; category?: string; q?: string; all?: boolean } = {}) => {
    const query = new URLSearchParams();
    if (params.page) query.set('page', String(params.page));
    if (params.pageSize) query.set('pageSize', String(params.pageSize));
    if (params.category) query.set('category', params.category);
    if (params.q) query.set('q', params.q);
    if (params.all) query.set('status', 'all');
    return request<PostListResult>(`/posts?${query.toString()}`);
  },
  getPost: (slug: string) => request<Post>(`/posts/${encodeURIComponent(slug)}`),
  getPostNeighbors: (slug: string) =>
    request<PostNeighbors>(`/posts/neighbors/${encodeURIComponent(slug)}`),
  getPostById: (id: number) => request<Post>(`/posts/id/${id}`),
  createPost: (body: Partial<Post> & { content: string }) =>
    request<{ id: number; slug: string }>('/posts', { method: 'POST', body: JSON.stringify(body) }),
  updatePost: (id: number, body: Partial<Post>) =>
    request<{ id: number }>(`/posts/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deletePost: (id: number) => request<{ ok: true }>(`/posts/${id}`, { method: 'DELETE' }),

  // 分类
  listCategories: () => request<{ items: Category[] }>('/categories'),
  createCategory: (name: string) =>
    request<Category>('/categories', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteCategory: (id: number) => request<{ ok: true }>(`/categories/${id}`, { method: 'DELETE' }),

  // 评论
  listComments: (params: { postId?: number; status?: string } = {}) => {
    const query = new URLSearchParams();
    if (params.postId) query.set('postId', String(params.postId));
    if (params.status) query.set('status', params.status);
    return request<{ items: Comment[] }>(`/comments?${query.toString()}`);
  },
  createComment: (postId: number, author: string, content: string) =>
    request<{ id: number; status: string }>('/comments', {
      method: 'POST',
      body: JSON.stringify({ postId, author, content }),
    }),
  moderateComment: (id: number, status: Comment['status']) =>
    request<{ id: number }>(`/comments/${id}`, { method: 'PUT', body: JSON.stringify({ status }) }),
  deleteComment: (id: number) => request<{ ok: true }>(`/comments/${id}`, { method: 'DELETE' }),
};
