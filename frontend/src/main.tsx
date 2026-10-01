import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ThemeProvider } from 'next-themes'
import App from './App'
import { AuthProvider } from '@/hooks/use-auth'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Toaster } from '@/components/ui/sonner'
import { initAppearance } from '@/lib/theme-settings'
import './index.css'

// 先于首帧应用已保存的圆角：CSS 里的默认值会覆盖它，
// 若等到组件挂载后再设，会看到一次圆角跳变。
initAppearance()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      <BrowserRouter>
        <AuthProvider>
          <TooltipProvider delayDuration={200}>
            <App />
            <Toaster position="bottom-right" richColors closeButton />
          </TooltipProvider>
        </AuthProvider>
      </BrowserRouter>
    </ThemeProvider>
  </StrictMode>,
)
