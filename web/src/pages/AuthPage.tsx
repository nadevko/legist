import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store'

type Mode = 'login' | 'register'

export default function AuthPage() {
  const [mode, setMode] = useState<Mode>('login')
  const [showPw, setShowPw] = useState(false)
  const [form, setForm] = useState({ name: '', email: '', password: '', confirm: '' })
  const [errors, setErrors] = useState({ name: '', email: '', password: '', confirm: '' })
  const [loading, setLoading] = useState(false)
  const login = useAuthStore(s => s.login)
  const navigate = useNavigate()

  const setField = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => {
    setForm(f => ({ ...f, [k]: e.target.value }))
    setErrors(er => ({ ...er, [k]: '' }))
  }

  const validate = () => {
    const errs = { name: '', email: '', password: '', confirm: '' }
    const re = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
    if (mode === 'register' && !form.name.trim()) errs.name = 'Введите имя'
    if (!form.email) errs.email = 'Введите email'
    else if (!re.test(form.email)) errs.email = 'Некорректный email'
    if (!form.password) errs.password = 'Введите пароль'
    if (mode === 'register' && form.password !== form.confirm) errs.confirm = 'Пароли не совпадают'
    setErrors(errs)
    return !Object.values(errs).some(Boolean)
  }

  const submit = async () => {
    if (loading) return
    if (!validate()) return
    setLoading(true)
    try {
      await login(form.email, form.password, mode === 'register' ? form.name : undefined)
      navigate('/')
    } catch (err) {
      setErrors(e => ({ ...e, email: 'Ошибка входа' }))
    } finally { setLoading(false) }
  }

  const handleKey = (e: React.KeyboardEvent) => { if (e.key === 'Enter') submit() }

  const switchMode = (m: Mode) => {
    setMode(m); setErrors({ name: '', email: '', password: '', confirm: '' }); setShowPw(false)
  }

  const EyeIcon = () => showPw
    ? <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
    : <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>

  return (
    <div className="auth-split">
      {/* ── LEFT PANEL ── */}
      <div className="auth-left">
        <div className="al-top">легист.бел</div>

        <div className="al-mid">
          <h1 className="al-h1">
            Легист.бел&nbsp;—<br />
            ИИ-помощник юриста РБ
          </h1>
          <p className="al-sub">
            Работа с нормативными актами и документами
            <br />в юрисдикции Республики Беларусь
          </p>
        </div>

        <div className="al-bot">
          <div className="al-bird" style={{ width: 44, height: 'auto', mixBlendMode: 'multiply' }}>
            <img src="/stork.png" alt="Легист Аист" style={{ width: '100%', height: 'auto', display: 'block' }} />
          </div>
          <div className="al-cpy">
            Copyright (c) 2026 Baradzin Aliaksei, Naurasiuk Matvei, Sarachuk Daniil, Lisovitskiy Bogdan, Sikorski Artsiom
          </div>
        </div>
      </div>

      {/* ── RIGHT PANEL (image + card) ── */}
      <div className="auth-right">
        <div className="ar-card">
          <h2 className="ar-title">
            {mode === 'login' ? 'Добро пожаловать' : 'Регистрация'}
          </h2>
          <p className="ar-sub">
            {mode === 'login' ? 'Войдите в свой аккаунт' : 'Создайте новый аккаунт'}
          </p>

          {mode === 'register' && (
            <div className="ar-field">
              <label className="ar-label">Имя</label>
              <div className="ar-input-wrap">
                <input
                  className={`ar-input${errors.name ? ' has-error' : ''}`}
                  type="text"
                  placeholder="Иванов Иван"
                  value={form.name}
                  onChange={setField('name')}
                  onKeyDown={handleKey}
                />
              </div>
              {errors.name && <div className="ar-err-msg">{errors.name}</div>}
            </div>
          )}

          <div className="ar-field">
            <label className="ar-label">Email</label>
            <div className="ar-input-wrap">
              <input
                className={`ar-input${errors.email ? ' has-error' : ''}`}
                type="email"
                placeholder="name@example.com"
                value={form.email}
                onChange={setField('email')}
                onKeyDown={handleKey}
              />
            </div>
            {errors.email && <div className="ar-err-msg">{errors.email}</div>}
          </div>

          <div className="ar-field">
            <label className="ar-label">Пароль</label>
            <div className="ar-input-wrap">
              <input
                className={`ar-input${errors.password ? ' has-error' : ''}`}
                type={showPw ? 'text' : 'password'}
                placeholder="Введите пароль"
                value={form.password}
                onChange={setField('password')}
                onKeyDown={handleKey}
                style={{ paddingRight: 44 }}
              />
              <button 
                type="button" 
                className="ar-input-btn" 
                onClick={() => setShowPw(!showPw)}
                tabIndex={-1}
              >
                <EyeIcon />
              </button>
            </div>
            {errors.password && <div className="ar-err-msg">{errors.password}</div>}
          </div>

          {mode === 'register' && (
            <div className="ar-field">
              <label className="ar-label">Повторите пароль</label>
              <div className="ar-input-wrap">
                <input
                  className={`ar-input${errors.confirm ? ' has-error' : ''}`}
                  type={showPw ? 'text' : 'password'}
                  placeholder="Введите пароль"
                  value={form.confirm}
                  onChange={setField('confirm')}
                  onKeyDown={handleKey}
                  style={{ paddingRight: 44 }}
                />
              </div>
              {errors.confirm && <div className="ar-err-msg">{errors.confirm}</div>}
            </div>
          )}

          <button className="ar-btn" disabled={loading} onClick={submit}>
            {loading
              ? 'Загрузка...'
              : mode === 'login'
                ? 'Войти в систему'
                : 'Зарегистрироваться'}
          </button>

          <div className="ar-switch">
            {mode === 'login' ? 'Нет аккаунта?' : 'Уже есть аккаунт?'}
            <button
              className="ar-switch-btn"
              onClick={() => switchMode(mode === 'login' ? 'register' : 'login')}
            >
              {mode === 'login' ? 'Зарегистрироваться' : 'Войти'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}