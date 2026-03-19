export type Risk = 'red' | 'orange' | 'green'

export interface Act {
  id: number
  type: 'ЛНА' | 'НПА'
  title: string
  org: string
  versions: number
  date: string
  updatedBy: string
}

export interface Version {
  num: number
  date: string
  author: string
  changes: number
  size: string
  status: 'Актуальная' | 'Архив'
  checked: boolean
}

export interface Change {
  n: number
  s: string
  old: string
  nw: string
  type: string
  risk: Risk
  law: string
  rec: string
}

export interface ChainVersion {
  ver: string
  date: string
  author: string
  risk: Risk
  changes: number
  red: number
  org: number
  grn: number
  title: string
}

export interface User {
  name: string
  email: string
}

export interface FileInfo {
  name: string
  size: number
}
