import type { CustomMenuItem } from '@/types'
import { sanitizeUrl } from './url'

export function headerLinkUrl(value: string): string {
  if (!value || value.length > 2048 || /[\s\\]/.test(value)) return ''
  return sanitizeUrl(value, { allowRelative: true })
}

export function customMenuNewTabUrl(item: CustomMenuItem): string | undefined {
  if (!item.open_in_new_tab) return undefined
  if (item.url.startsWith('md:')) return `/custom/${encodeURIComponent(item.id)}`
  return sanitizeUrl(item.url) || undefined
}
