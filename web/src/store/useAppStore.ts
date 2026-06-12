import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// 全局状态类型
interface AppState {
  // 主题
  theme: 'light' | 'dark'
  setTheme: (theme: 'light' | 'dark') => void
  
  // 语言
  language: string
  setLanguage: (lang: string) => void
  
  // 侧边栏
  sidebarCollapsed: boolean
  toggleSidebar: () => void
  
  // 标签页
  tabs: Array<{ key: string; title: string; closable: boolean }>
  activeTab: string
  addTab: (tab: { key: string; title: string; closable: boolean }) => void
  removeTab: (key: string) => void
  setActiveTab: (key: string) => void
  
  // 用户信息
  username: string | null
  token: string | null
  setUser: (username: string, token: string) => void
  logout: () => void
  
  // 加载状态
  loading: boolean
  setLoading: (loading: boolean) => void
  
  // 通知
  notifications: Array<{ id: string; type: string; message: string; time: number }>
  addNotification: (notification: { type: string; message: string }) => void
  removeNotification: (id: string) => void
  clearNotifications: () => void
}

// 创建全局状态
export const useAppStore = create<AppState>()(
  persist(
    (set) => ({
      // 主题
      theme: 'light',
      setTheme: (theme) => set({ theme }),
      
      // 语言
      language: 'zh',
      setLanguage: (language) => set({ language }),
      
      // 侧边栏
      sidebarCollapsed: false,
      toggleSidebar: () => set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
      
      // 标签页
      tabs: [{ key: '/', title: '概览', closable: false }],
      activeTab: '/',
      addTab: (tab) => set((state) => {
        if (state.tabs.find(t => t.key === tab.key)) {
          return { activeTab: tab.key }
        }
        return { tabs: [...state.tabs, tab], activeTab: tab.key }
      }),
      removeTab: (key) => set((state) => {
        const newTabs = state.tabs.filter(t => t.key !== key)
        const newActiveTab = state.activeTab === key 
          ? newTabs[newTabs.length - 1]?.key || '/'
          : state.activeTab
        return { tabs: newTabs, activeTab: newActiveTab }
      }),
      setActiveTab: (activeTab) => set({ activeTab }),
      
      // 用户信息
      username: null,
      token: null,
      setUser: (username, token) => set({ username, token }),
      logout: () => set({ username: null, token: null }),
      
      // 加载状态
      loading: false,
      setLoading: (loading) => set({ loading }),
      
      // 通知
      notifications: [],
      addNotification: (notification) => set((state) => ({
        notifications: [
          { ...notification, id: Date.now().toString(), time: Date.now() },
          ...state.notifications,
        ].slice(0, 50),
      })),
      removeNotification: (id) => set((state) => ({
        notifications: state.notifications.filter(n => n.id !== id),
      })),
      clearNotifications: () => set({ notifications: [] }),
    }),
    {
      name: 'app-storage',
      partialize: (state) => ({
        theme: state.theme,
        language: state.language,
        sidebarCollapsed: state.sidebarCollapsed,
      }),
    }
  )
)

export default useAppStore
