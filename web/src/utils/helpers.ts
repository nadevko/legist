import type { Risk } from '../types'

export function rBdg(r: Risk): string {
  return { red: 'bdg-r', orange: 'bdg-o', green: 'bdg-g' }[r] || 'bdg-g'
}

export function rFull(r: Risk): string {
  return { red: 'Противоречие', orange: 'Проверить', green: 'Безопасно' }[r] || 'Безопасно'
}

export function rLbl(r: Risk): string {
  return { red: 'Проверить', orange: 'Проверить', green: 'Безопасно' }[r] || 'Безопасно'
}

export function pl(n: number, f1: string, f2: string, f5: string): string {
  const a = Math.abs(n) % 100, b = a % 10
  if (a > 10 && a < 20) return f5
  if (b > 1 && b < 5) return f2
  if (b === 1) return f1
  return f5
}

export function esc(s: string): string {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/\n/g, '<br>')
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
}

export const PROGRESS_STEPS = ['Парсинг', 'Структура', 'Семантика', 'НПА РБ', 'Отчёт']
export const PROGRESS_STEP_LABELS = [
  'Парсинг документов...',
  'Структурный анализ...',
  'Семантический анализ...',
  'Проверка законодательства РБ...',
  'Формирование отчёта...',
]
export const TOAST_MS = 3000
