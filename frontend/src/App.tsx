import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import Home from '@/pages/home'
import PostDetail from '@/pages/post-detail'
import AdminLogin from '@/pages/admin-login'
import NotFound from '@/pages/not-found'

const AdminLayout = lazy(() => import('@/pages/admin-layout'))
const AdminPosts = lazy(() => import('@/pages/admin-posts'))
const AdminPostEditor = lazy(() => import('@/pages/admin-post-editor'))
const AdminComments = lazy(() => import('@/pages/admin-comments'))
const AdminCategories = lazy(() => import('@/pages/admin-categories'))

function AdminFallback() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
      加载中
    </div>
  )
}

export default function App() {
  return (
    <Suspense fallback={<AdminFallback />}>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/post/:slug" element={<PostDetail />} />
        <Route path="/admin/login" element={<AdminLogin />} />
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<Navigate to="/admin/posts" replace />} />
          <Route path="posts" element={<AdminPosts />} />
          <Route path="posts/new" element={<AdminPostEditor />} />
          <Route path="posts/:id" element={<AdminPostEditor />} />
          <Route path="comments" element={<AdminComments />} />
          <Route path="categories" element={<AdminCategories />} />
        </Route>
        <Route path="*" element={<NotFound />} />
      </Routes>
    </Suspense>
  )
}
