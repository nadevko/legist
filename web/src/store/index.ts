import { create } from 'zustand'
import type { User } from '../types'
import { TOAST_MS } from '../utils/helpers'

// ── AUTH STORE ──────────────────────────────────────────────
interface AuthStore {
  isAuthenticated: boolean
  currentUser: User
  login: (email: string, password: string, name?: string) => Promise<void>
  logout: () => void
}

const savedUser = (() => {
  try {
    const raw = localStorage.getItem('legist_user')
    if (raw) { const u = JSON.parse(raw); return { isAuthenticated: true, currentUser: u as User } }
  } catch { /* ignore */ }
  return { isAuthenticated: false, currentUser: { name: '', email: '' } }
})()

export const useAuthStore = create<AuthStore>((set, get) => ({
  isAuthenticated: savedUser.isAuthenticated,
  currentUser: savedUser.currentUser,

  login: async (email, password, name) => {
    const isRegister = !!name
    const endpoint = isRegister ? '/api/users' : '/api/sessions'
    
    const res = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': crypto.randomUUID(),
      },
      body: JSON.stringify({ email, password, name }),
    })

    const data = await res.json()
    if (!res.ok) {
      throw new Error(data.error?.message || 'Ошибка аутентификации')
    }

    // Если это была регистрация, нужно еще войти
    if (isRegister) {
      return get().login(email, password)
    }

    // Сохраняем токены
    if (data.access_token) {
      localStorage.setItem('legist_token', data.access_token)
      if (data.refresh_token) {
        localStorage.setItem('legist_refresh_token', data.refresh_token)
      }
      
      const userRes = await fetch('/api/me', {
        headers: {
          'Authorization': `Bearer ${data.access_token}`
        }
      })
      
      let user = { name: data.email?.split('@')[0] || 'Пользователь', email: data.email || '' }
      if (userRes.ok) {
        const userData = await userRes.json()
        user = { name: userData.email.split('@')[0], email: userData.email }
      }
      
      localStorage.setItem('legist_user', JSON.stringify(user))
      set({ isAuthenticated: true, currentUser: user })
      return // Успешное завершение после установки стейта
    }
  },

  logout: () => {
    localStorage.removeItem('legist_user')
    localStorage.removeItem('legist_token')
    localStorage.removeItem('legist_refresh_token')
    set({ isAuthenticated: false, currentUser: { name: '', email: '' } })
  },
}))

// ── UI STORE ────────────────────────────────────────────────
interface UIStore {
  toast: { visible: boolean; msg: string }
  modalDel: boolean
  modalDelText: string
  modalNtsi: boolean
  modalShortcuts: boolean
  showToast: (msg: string) => void
  setModalDel: (open: boolean, text?: string) => void
  setModalNtsi: (open: boolean) => void
  setModalShortcuts: (open: boolean) => void
}

let toastTimer: ReturnType<typeof setTimeout> | null = null

export const useUIStore = create<UIStore>((set) => ({
  toast: { visible: false, msg: '' },
  modalDel: false,
  modalDelText: 'Это действие необратимо.',
  modalNtsi: false,
  modalShortcuts: false,

  showToast: (msg) => {
    if (toastTimer) clearTimeout(toastTimer)
    set({ toast: { visible: true, msg } })
    toastTimer = setTimeout(() => set({ toast: { visible: false, msg: '' } }), TOAST_MS)
  },
  setModalDel: (open, text) =>
    set((s) => ({ modalDel: open, modalDelText: text ?? s.modalDelText })),
  setModalNtsi: (open) => set({ modalNtsi: open }),
  setModalShortcuts: (open) => set({ modalShortcuts: open }),
}))

// ── COMPARE STORE ───────────────────────────────────────────
interface CompareStore {
  title: string
  sub: string
  setCompare: (title: string, sub: string) => void
}

export const useCompareStore = create<CompareStore>((set) => ({
  title: 'Анализ изменений',
  sub: 'Правила внутреннего трудового распорядка · 10 изменений',
  setCompare: (title, sub) => set({ title, sub }),
}))
