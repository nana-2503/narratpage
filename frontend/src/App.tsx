import { lazy, Suspense, useEffect, useState } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import Home from '@/pages/home'
import PostDetail from '@/pages/post-detail'
import AdminLogin from '@/pages/admin-login'
import NotFound from '@/pages/not-found'
import Install from '@/pages/install'
import { api, getInstalled } from '@/lib/api'

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
  const [checking, setChecking] = useState(true)
  const [installed, setInstalled] = useState<boolean | null>(null)
  const location = useLocation()

  useEffect(() => {
    // 如果已明确标记为已安装，直接放行
    if (getInstalled()) {
      setInstalled(true)
      setChecking(false)
      return
    }
    api.installStatus()
      .then((status) => {
        const isInstalled = status.installed
        setInstalled(isInstalled)
        if (isInstalled) {
          // 缓存安装状态
          localStorage.setItem('blog_installed', 'true')
        }
      })
      .catch(() => {
        // 网络错误时保守处理：不强制跳转，允许用户尝试访问
        setInstalled(true)
      })
      .finally(() => setChecking(false))
  }, [location.pathname])

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
        <Route
          path="/admin"
          element={
            <InstallRedirect>
              <AdminLayout />
            </InstallRedirect>
          }
        >
          <Route index element={<Navigate to="/admin/posts" replace />} />
          <Route path="login" element={<AdminLogin />} />
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
