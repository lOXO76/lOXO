import { describe, expect, it } from 'vitest'
import {
  countTicketChars,
  isBlankTicketText,
  isTicketTextWithinLimit,
  truncateTicketText,
} from '../ticketText'

/** 结果里不得出现孤立代理对（即不能把一个 emoji 切成两半）。 */
function hasLoneSurrogate(value: string): boolean {
  for (const char of Array.from(value)) {
    const code = char.codePointAt(0) ?? 0
    if (code >= 0xd800 && code <= 0xdfff && char.length !== 2) return true
  }
  // 逐 code unit 检查是否有未配对的高/低代理
  for (let i = 0; i < value.length; i += 1) {
    const unit = value.charCodeAt(i)
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(i + 1)
      if (Math.abs(i + 1 - i) !== 1 || !(next >= 0xdc00 && next <= 0xdfff)) return true
      i += 1
    } else if (unit >= 0xdc00 && unit <= 0xdfff) {
      return true
    }
  }
  return false
}

describe('countTicketChars', () => {
  it('counts ASCII by character', () => {
    expect(countTicketChars('hello')).toBe(5)
    expect(countTicketChars('')).toBe(0)
    expect(countTicketChars(null)).toBe(0)
    expect(countTicketChars(undefined)).toBe(0)
  })

  it('counts emoji as one code point (not two UTF-16 units)', () => {
    expect('😀'.length).toBe(2)
    expect(countTicketChars('😀')).toBe(1)
    expect(countTicketChars('a😀b')).toBe(3)
    // 199 个 ASCII + 1 个 emoji = 200 code point，正好卡在上限
    expect(countTicketChars(`${'a'.repeat(199)}😀`)).toBe(200)
  })

  it('counts multi-codepoint grapheme clusters per code point', () => {
    // 家庭 emoji 由多个 code point 组成，按后端 rune 语义逐个计数
    expect(countTicketChars('👨‍👩‍👧')).toBe(5)
  })
})

describe('truncateTicketText', () => {
  it('keeps text that already fits', () => {
    expect(truncateTicketText('short', 10)).toBe('short')
    expect(truncateTicketText(`${'a'.repeat(199)}😀`, 200)).toBe(`${'a'.repeat(199)}😀`)
  })

  it('never splits a surrogate pair', () => {
    const truncated = truncateTicketText('😀😀😀', 2)
    expect(truncated).toBe('😀😀')
    expect(countTicketChars(truncated)).toBe(2)
    expect(hasLoneSurrogate(truncated)).toBe(false)
    expect(truncated).not.toContain('\ufffd')
  })

  it('truncates the 199-ASCII + emoji boundary at exactly max code points', () => {
    const source = `${'a'.repeat(199)}😀b`
    const truncated = truncateTicketText(source, 200)
    expect(countTicketChars(truncated)).toBe(200)
    expect(truncated.endsWith('😀')).toBe(true)
    expect(hasLoneSurrogate(truncated)).toBe(false)
    // 旧实现 `slice(0, 200)` 会切出半个代理对
    expect(source.slice(0, 200)).not.toBe(truncated)
    expect(hasLoneSurrogate(source.slice(0, 200))).toBe(true)
  })

  it('handles degenerate limits and empty input', () => {
    expect(truncateTicketText('😀', 0)).toBe('')
    expect(truncateTicketText('😀', -3)).toBe('')
    expect(truncateTicketText('', 5)).toBe('')
    expect(truncateTicketText(null, 5)).toBe('')
    expect(truncateTicketText(undefined, 5)).toBe('')
  })
})

describe('isTicketTextWithinLimit', () => {
  it('uses code point length', () => {
    expect(isTicketTextWithinLimit(`${'a'.repeat(199)}😀`, 200)).toBe(true)
    expect(isTicketTextWithinLimit(`${'a'.repeat(200)}😀`, 200)).toBe(false)
    expect(isTicketTextWithinLimit('😀😀', 2)).toBe(true)
  })
})

describe('isBlankTicketText', () => {
  it('treats whitespace-only input as blank', () => {
    expect(isBlankTicketText('')).toBe(true)
    expect(isBlankTicketText('   \n\t')).toBe(true)
    expect(isBlankTicketText(null)).toBe(true)
    expect(isBlankTicketText('😀')).toBe(false)
  })
})
