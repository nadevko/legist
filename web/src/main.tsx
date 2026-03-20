import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/globals.css'
import App from './App'

createRoot(document.getElementById('root')!).render(
  // Отключаем StrictMode в development для избежания двойной анимации
  // В production StrictMode не включается
  import.meta.env.DEV ? (
    <App/>
  ) : (
    <StrictMode>
      <App/>
    </StrictMode>
  )
)
