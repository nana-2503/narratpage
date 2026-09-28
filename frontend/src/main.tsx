import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ThemeProvider } from 'next-themes'
import './index.css'
import App from './App.tsx'
import { Toaster } from '@/components/ui/sonner'
import { applyRadius, loadRadius } from '@/lib/theme-settings'

// 首屏渲染前应用已保存的圆角，避免刷新闪烁
applyRadius(loadRadius())

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
        <App />
        <Toaster position="top-center" />
      </ThemeProvider>
    </BrowserRouter>
  </StrictMode>,
)
