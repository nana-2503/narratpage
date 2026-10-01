import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import {
  api,
  can,
  getCachedUser,
  getToken,
  setCachedUser,
  setToken,
  type AuthUser,
  type UserRole,
} from '@/lib/api'

interface AuthState {
  user: AuthUser | null
  /** 首次校验是否完成：未完成前不应展示「未登录」相关 UI */
  ready: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
  /** 是否具备某能力点 */
  can: (cap: string) => boolean
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  // 先用缓存值渲染，避免刷新后首帧闪一下「未登录」
  const [user, setUser] = useState<AuthUser | null>(getCachedUser)
  const [ready, setReady] = useState(false)

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setUser(null)
      setCachedUser(null)
      setReady(true)
      return
    }
    try {
      const me = await api.me()
      setUser(me)
      setCachedUser(me)
    } catch {
      // token 失效：清理本地状态
      setToken(null)
      setCachedUser(null)
      setUser(null)
    } finally {
      setReady(true)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.login(username, password)
    setToken(res.token)
    setCachedUser(res.user)
    setUser(res.user)
    setReady(true)
  }, [])

  const logout = useCallback(async () => {
    try {
      await api.logout()
    } catch {
      // 登出失败也要清本地状态，避免卡在「看似已登录」
    }
    setToken(null)
    setCachedUser(null)
    setUser(null)
  }, [])

  const value = useMemo<AuthState>(
    () => ({
      user,
      ready,
      login,
      logout,
      refresh,
      can: (cap: string) => can(user?.role as UserRole | undefined, cap),
    }),
    [user, ready, login, logout, refresh],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth 必须在 AuthProvider 内使用')
  return ctx
}
