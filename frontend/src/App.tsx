import { lazy, Suspense, useEffect, useState } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import Home from '@/pages/home'
import PostDetail from '@/pages/post-detail'
import AdminLogin from '@/pages/admin-login'
import NotFound from '@/pages/not-found'
import Install from '@/pages/install'
import { api, getInstalled, setInstalled as persistInstalled } from '@/lib/api'

const AdminLayout = lazy(() => import('@/pages/admin-layout'))
const AdminPosts = lazy(() => import('@/pages/admin-posts'))
const AdminPostEditor = lazy(() => import('@/pages/admin-post-editor'))
const AdminComments = lazy(() => import('@/pages/admin-comments'))
const AdminCategories = lazy(() => import('@/pages/admin-categories'))
const AdminAccount = lazy(() => import('@/pages/admin-account'))
const AdminSettings = lazy(() => import('@/pages/admin-settings'))

function AdminFallback() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
      加载中
    </div>
  )
}

function InstallRedirect({ children }: { children: React.ReactNode }) {
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

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        加载中
      </div>
    )
  }

  if (installed === false) {
    return <Navigate to="/install" state={{ from: location.pathname }} replace />
  }

  return <>{children}</>
}

export default function App() {
  return (
    <Suspense fallback={<AdminFallback />}>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/post/:slug" element={<PostDetail />} />
        <Route path="/install" element={<Install />} />
        {/* 登录页必须独立于 /admin：AdminLayout 自带登录守卫，
            若把 login 嵌在其下，未登录时会永远卡在“加载中” */}
        <Route path="/admin/login" element={<AdminLogin />} />
        <Route
          path="/admin"
          element={
            <InstallRedirect>
              <AdminLayout />
            </InstallRedirect>
          }
        >
          <Route index element={<Navigate to="/admin/posts" replace />} />
          <Route path="posts" element={<AdminPosts />} />
          <Route path="posts/new" element={<AdminPostEditor />} />
          <Route path="posts/:id" element={<AdminPostEditor />} />
          <Route path="comments" element={<AdminComments />} />
          <Route path="categories" element={<AdminCategories />} />
          <Route path="account" element={<AdminAccount />} />
          <Route path="settings" element={<AdminSettings />} />
        </Route>
        <Route path="*" element={<NotFound />} />
      </Routes>
    </Suspense>
  )
}
