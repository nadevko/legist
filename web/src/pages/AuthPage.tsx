import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store'

type Mode = 'login' | 'register'

export default function AuthPage() {
  const [mode, setMode] = useState<Mode>('login')
  const [shaking, setShaking] = useState(false)
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
    else if (form.password.length < 8) errs.password = 'Минимум 8 символов'
    if (mode === 'register' && form.password !== form.confirm) errs.confirm = 'Пароли не совпадают'
    setErrors(errs)
    return !Object.values(errs).some(Boolean)
  }

  const pwStrength = (() => {
    const p = form.password; if (!p) return 0
    let s = 0
    if (p.length >= 8) s++
    if (/[A-Z]/.test(p) || /[0-9]/.test(p)) s++
    if (/[^A-Za-z0-9]/.test(p) && p.length >= 10) s++
    return Math.max(p.length > 0 ? 1 : 0, s)
  })()
  const pwClass = ['', 'str-weak', 'str-mid', 'str-strong'][pwStrength] || 'str-weak'

  const submit = async () => {
    if (loading) return
    if (!validate()) { setShaking(true); return }
    setLoading(true)
    try {
      await login(form.email, form.password, mode === 'register' ? form.name : undefined)
      navigate('/')
    } catch {
      setErrors(e => ({ ...e, email: 'Ошибка входа' }))
      setShaking(true)
    } finally { setLoading(false) }
  }

  const handleKey = (e: React.KeyboardEvent) => { if (e.key === 'Enter') submit() }

  const switchMode = (m: Mode) => {
    setMode(m); setErrors({ name:'',email:'',password:'',confirm:'' }); setShowPw(false)
  }

  const EyeIcon = () => showPw
    ? <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
    : <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>

  const ErrMsg = ({ msg }: { msg: string }) => msg ? (
    <div className="auth-err-msg">
      <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
      {msg}
    </div>
  ) : null

  return (
    <div className="auth-page">
      <div className="auth-wrap">
        {/* Logo */}
        <div className="auth-logo-row">
          <div className="auth-logo-icon">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
              <polyline points="14 2 14 8 20 8"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="12" y2="17"/>
            </svg>
          </div>
          <div>
            <div className="auth-logo-name">legist</div>
            <div className="auth-logo-tier">Enterprise</div>
          </div>
        </div>

        {/* Card */}
        <div className={`auth-card${shaking ? ' is-shaking' : ''}`} onAnimationEnd={() => setShaking(false)}>
          <div className="auth-heading">{mode === 'login' ? 'Добро пожаловать' : 'Создать аккаунт'}</div>
          <div className="auth-subheading">{mode === 'login' ? 'Войдите в свой аккаунт legist' : 'Зарегистрируйтесь для доступа к платформе'}</div>

          {/* Tabs */}
          <div className="auth-tabs">
            <button className={`auth-tab${mode === 'login' ? ' active' : ''}`} onClick={() => switchMode('login')}>Войти</button>
            <button className={`auth-tab${mode === 'register' ? ' active' : ''}`} onClick={() => switchMode('register')}>Регистрация</button>
          </div>

          {mode === 'login' ? (
            <div>
              <div className="auth-field">
                <label className="auth-label" htmlFor="a-email">Email</label>
                <div className="auth-input-wrap">
                  <input id="a-email" className={`auth-input${errors.email ? ' has-error' : ''}`} type="email" placeholder="you@example.com" value={form.email} onChange={setField('email')} onKeyDown={handleKey}/>
                  <span className="auth-input-ico"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M2 7l10 7 10-7"/></svg></span>
                </div>
                <ErrMsg msg={errors.email}/>
              </div>
              <div className="auth-field">
                <label className="auth-label" htmlFor="a-pw">Пароль</label>
                <div className="auth-input-wrap">
                  <input id="a-pw" className={`auth-input${errors.password ? ' has-error' : ''}`} type={showPw ? 'text' : 'password'} placeholder="Введите пароль" value={form.password} onChange={setField('password')} onKeyDown={handleKey}/>
                  <button className="auth-input-btn" type="button" onClick={() => setShowPw(!showPw)}><EyeIcon/></button>
                </div>
                <ErrMsg msg={errors.password}/>
              </div>
              <button className="auth-btn" disabled={loading} onClick={submit}>
                {loading && <span className="auth-spinner"/>}
                <span>{loading ? 'Входим...' : 'Войти в систему'}</span>
              </button>
              <div className="auth-switch">
                Забыли пароль? <button className="auth-switch-btn" onClick={() => switchMode('register')}>Восстановить</button>
              </div>
            </div>
          ) : (
            <div>
              <div className="auth-field">
                <label className="auth-label" htmlFor="a-name">Имя</label>
                <div className="auth-input-wrap">
                  <input id="a-name" className={`auth-input${errors.name ? ' has-error' : ''}`} type="text" placeholder="Иванов Иван" value={form.name} onChange={setField('name')} onKeyDown={handleKey}/>
                  <span className="auth-input-ico"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="12" cy="8" r="4"/><path d="M4 20c0-3.31 3.58-6 8-6s8 2.69 8 6"/></svg></span>
                </div>
                <ErrMsg msg={errors.name}/>
              </div>
              <div className="auth-field">
                <label className="auth-label" htmlFor="a-reg-email">Email</label>
                <div className="auth-input-wrap">
                  <input id="a-reg-email" className={`auth-input${errors.email ? ' has-error' : ''}`} type="email" placeholder="you@company.by" value={form.email} onChange={setField('email')} onKeyDown={handleKey}/>
                  <span className="auth-input-ico"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M2 7l10 7 10-7"/></svg></span>
                </div>
                <ErrMsg msg={errors.email}/>
              </div>
              <div className="auth-field">
                <label className="auth-label" htmlFor="a-reg-pw">Пароль</label>
                <div className="auth-input-wrap">
                  <input id="a-reg-pw" className={`auth-input${errors.password ? ' has-error' : ''}`} type={showPw ? 'text' : 'password'} placeholder="Минимум 8 символов" value={form.password} onChange={setField('password')} onKeyDown={handleKey}/>
                  <button className="auth-input-btn" type="button" onClick={() => setShowPw(!showPw)}><EyeIcon/></button>
                </div>
                {form.password.length > 0 && (
                  <div className="auth-strength">
                    {[1,2,3].map(i => <div key={i} className={`auth-str-seg${pwStrength >= i ? ' '+pwClass : ''}`}/>)}
                  </div>
                )}
                <ErrMsg msg={errors.password}/>
              </div>
              <div className="auth-field">
                <label className="auth-label" htmlFor="a-confirm">Повторите пароль</label>
                <div className="auth-input-wrap">
                  <input id="a-confirm" className={`auth-input${errors.confirm ? ' has-error' : ''}`} type={showPw ? 'text' : 'password'} placeholder="Повторите пароль" value={form.confirm} onChange={setField('confirm')} onKeyDown={handleKey}/>
                </div>
                <ErrMsg msg={errors.confirm}/>
              </div>
              <button className="auth-btn" disabled={loading} onClick={submit}>
                {loading && <span className="auth-spinner"/>}
                <span>{loading ? 'Создаём аккаунт...' : 'Создать аккаунт'}</span>
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
