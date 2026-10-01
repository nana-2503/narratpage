import { lazy, Suspense, useEffect, useState } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import Home from '@/pages/home'
import PostDetail from '@/pages/post-detail'
import PageView from '@/pages/page-view'
import Archive from '@/pages/archive'
import TagIndex from '@/pages/tag-index'
import NotFound from '@/pages/not-found'
import Install from '@/pages/install'
import { api, getInstalled, setInstalled as persistInstalled } from '@/lib/api'
import { useSite } from '@/hooks/use-site'
import { setSiteMeta } from '@/lib/meta'

const AdminLayout = lazy(() => import('@/pages/admin-layout'))
const AdminDashboard = lazy(() => import('@/pages/admin-dashboard'))
const AdminPosts = lazy(() => import('@/pages/admin-posts'))
const AdminPostEditor = lazy(() => import('@/pages/admin-post-editor'))
const AdminPages = lazy(() => import('@/pages/admin-pages'))
const AdminComments = lazy(() => import('@/pages/admin-comments'))
const AdminCategories = lazy(() => import('@/pages/admin-categories'))
const AdminTags = lazy(() => import('@/pages/admin-tags'))
const AdminMedia = lazy(() => import('@/pages/admin-media'))
const AdminUsers = lazy(() => import('@/pages/admin-users'))
const AdminAccount = lazy(() => import('@/pages/admin-account'))
const AdminSettings = lazy(() => import('@/pages/admin-settings'))
const AdminRedirects = lazy(() => import('@/pages/admin-redirects'))
const AdminTrash = lazy(() => import('@/pages/admin-trash'))
const AdminLogin = lazy(() => import('@/pages/admin-login'))

function Loading() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
      加载中
    </div>
  )
}

/** 安装检测：未安装时引导至安装向导 */
function InstallGuard({ children }: { children: React.ReactNode }) {
  // 本地已标记完成时直接放行，避免首屏多一次请求与闪烁
  const cached = getInstalled()
  const [installed, setInstalled] = useState<boolean | null>(cached ? true : null)
  const [checking, setChecking] = useState(!cached)
  const location = useLocation()

  useEffect(() => {
    if (cached) return
    let ignore = false
    api
      .installStatus()
      .then((status) => {
        if (ignore) return
        setInstalled(status.installed)
        if (status.installed) persistInstalled(true)
      })
      .catch(() => {
        // 网络错误时保守放行，避免把可访问的站点锁在安装页
        if (!ignore) setInstalled(true)
      })
      .finally(() => {
        if (!ignore) setChecking(false)
      })
    return () => {
      ignore = true
    }
  }, [cached, location.pathname])

  if (checking) return <Loading />
  if (installed === false) {
    return <Navigate to="/install" state={{ from: location.pathname }} replace />
  }
  return <>{children}</>
}

/** 站点级 meta 同步 */
function SiteMeta() {
  const { site } = useSite()
  useEffect(() => {
    if (site.title) setSiteMeta(site)
  }, [site.title, site.description, site.url])
  return null
}

export default function App() {
  return (
    <Suspense fallback={<Loading />}>
      <SiteMeta />
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/post/:slug" element={<PostDetail />} />
        <Route path="/page/:slug" element={<PageView />} />
        <Route path="/category/:slug" element={<Home />} />
        <Route path="/tag/:slug" element={<Home />} />
        <Route path="/author/:id" element={<Home />} />
        <Route path="/archive" element={<Archive />} />
        <Route path="/tags" element={<TagIndex />} />
        <Route path="/search" element={<Home />} />
        <Route path="/install" element={<Install />} />

        {/* 登录页必须独立于 /admin：AdminLayout 自带登录守卫，
            若把 login 嵌在其下，未登录时会永远卡在“加载中” */}
        <Route path="/admin/login" element={<AdminLogin />} />
        <Route
          path="/admin"
          element={
            <InstallGuard>
              <AdminLayout />
            </InstallGuard>
          }
        >
          <Route index element={<Navigate to="/admin/dashboard" replace />} />
          <Route path="dashboard" element={<AdminDashboard />} />
          <Route path="posts" element={<AdminPosts />} />
          <Route path="posts/new" element={<AdminPostEditor />} />
          <Route path="posts/:id" element={<AdminPostEditor />} />
          <Route path="pages" element={<AdminPages />} />
          <Route path="pages/new" element={<AdminPostEditor />} />
          <Route path="pages/:id" element={<AdminPostEditor />} />
          <Route path="comments" element={<AdminComments />} />
          <Route path="categories" element={<AdminCategories />} />
          <Route path="tags" element={<AdminTags />} />
          <Route path="media" element={<AdminMedia />} />
          <Route path="trash" element={<AdminTrash />} />
          <Route path="redirects" element={<AdminRedirects />} />
          <Route path="users" element={<AdminUsers />} />
          <Route path="account" element={<AdminAccount />} />
          <Route path="settings" element={<AdminSettings />} />
        </Route>
        <Route path="*" element={<NotFound />} />
      </Routes>
    </Suspense>
  )
}
