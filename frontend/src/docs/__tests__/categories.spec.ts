import { describe, expect, it } from 'vitest'
import { enCategories, zhCategories } from '..'
import type { DocBlock, DocsCategory, DocSection } from '../types'

function allSections(category: DocsCategory): DocSection[] {
  return [
    ...category.beforeProtocols,
    ...category.protocols.flatMap(protocol => protocol.sections),
    ...category.afterProtocols
  ]
}

function allBlocks(category: DocsCategory): DocBlock[] {
  return allSections(category).flatMap(section => section.blocks)
}

function examples(category: DocsCategory) {
  return allBlocks(category).filter((block): block is Extract<DocBlock, { type: 'example' }> => block.type === 'example')
}

/** Every deep-link token the view can resolve, as #api-docs-{token}. */
function anchorTokens(category: DocsCategory): string[] {
  return [
    ...category.beforeProtocols.map(section => section.id),
    category.protocolsId,
    ...category.protocols.flatMap(protocol => [
      protocol.id,
      ...protocol.sections.map(section => `${protocol.id}-${section.id}`),
      ...protocol.sections.flatMap(section => section.blocks.flatMap(block => block.type === 'example' ? [`${protocol.id}-${block.id}`] : []))
    ]),
    ...category.afterProtocols.map(section => section.id)
  ]
}

describe('api docs categories', () => {
  it('lists text chat, image and video categories in the same order for both locales', () => {
    expect(zhCategories.map(category => category.id)).toEqual(['chat', 'image', 'video'])
    expect(enCategories.map(category => category.id)).toEqual(zhCategories.map(category => category.id))
  })

  it('keeps every anchor token unique across all categories', () => {
    for (const categories of [zhCategories, enCategories]) {
      const tokens = categories.flatMap(anchorTokens)
      expect(new Set(tokens).size).toBe(tokens.length)
    }
  })

  it('resolves each token to exactly one category by id or protocol prefix', () => {
    for (const categories of [zhCategories, enCategories]) {
      for (const category of categories) {
        for (const token of anchorTokens(category)) {
          const owners = categories.filter(candidate =>
            token === candidate.protocolsId
            || [...candidate.beforeProtocols, ...candidate.afterProtocols].some(section => section.id === token)
            || candidate.protocols.some(protocol => token === protocol.id || token.startsWith(`${protocol.id}-`))
          )
          expect(owners.map(owner => owner.id)).toEqual([category.id])
        }
      }
    }
  })

  it.each(['chat', 'image'])('mirrors the %s category structure between locales', id => {
    const zh = zhCategories.find(category => category.id === id)!
    const en = enCategories.find(category => category.id === id)!
    expect(en.protocols.map(protocol => protocol.id)).toEqual(zh.protocols.map(protocol => protocol.id))
    expect(allSections(en).map(section => section.id)).toEqual(allSections(zh).map(section => section.id))
    for (const [index, zhSection] of allSections(zh).entries()) {
      const enSection = allSections(en)[index]!
      expect(enSection.blocks.map(block => block.type)).toEqual(zhSection.blocks.map(block => block.type))
    }
    const zhBlocks = allBlocks(zh)
    const enBlocks = allBlocks(en)
    for (const [index, zhBlock] of zhBlocks.entries()) {
      const enBlock = enBlocks[index]!
      if (zhBlock.type === 'table' && enBlock.type === 'table') {
        expect(enBlock.headers).toHaveLength(zhBlock.headers.length)
        expect(enBlock.rows.map(row => row.length)).toEqual(zhBlock.rows.map(row => row.length))
      }
      if (zhBlock.type === 'list' && enBlock.type === 'list') {
        expect(enBlock.items).toHaveLength(zhBlock.items.length)
      }
      if (zhBlock.type === 'example' && enBlock.type === 'example') {
        expect(enBlock.id).toBe(zhBlock.id)
        expect(enBlock.responses.map(response => response.status)).toEqual(zhBlock.responses.map(response => response.status))
      }
    }
  })

  it.each(['chat', 'image'])('gives every %s example a curl request and a copyable response', id => {
    for (const categories of [zhCategories, enCategories]) {
      const category = categories.find(item => item.id === id)!
      expect(examples(category).length).toBeGreaterThan(0)
      for (const example of examples(category)) {
        expect(example.title).toBeTruthy()
        expect(example.description).toBeTruthy()
        expect(example.request.trim()).toMatch(/^curl\s+/)
        expect(example.request).toContain('{{BASE_URL}}')
        expect(example.request).toContain('{{API_KEY}}')
        expect(example.responses.length).toBeGreaterThan(0)
        for (const response of example.responses) {
          expect(response.title).toBeTruthy()
          expect(response.body).toBeTruthy()
        }
      }
    }
  })

  it('keeps JSON response examples valid so the view can pretty-print them', () => {
    for (const categories of [zhCategories, enCategories]) {
      for (const category of categories.filter(item => item.id !== 'video')) {
        for (const response of examples(category).flatMap(example => example.responses)) {
          const body = response.body!.trim()
          if (body.startsWith('{')) expect(() => JSON.parse(body)).not.toThrow()
        }
      }
    }
  })

  it('documents both sync and async image generation', () => {
    for (const categories of [zhCategories, enCategories]) {
      const image = categories.find(category => category.id === 'image')!
      const requests = examples(image).map(example => example.request).join('\n')
      expect(requests).toContain('/v1/images/generations\'')
      expect(requests).toContain('/v1/images/generations/async')
      expect(requests).toContain('/v1/images/tasks/')
      expect(requests).toContain(':generateContent')
      const asyncQuery = image.protocols.find(protocol => protocol.id === 'image-async')!
        .sections.flatMap(section => section.blocks)
        .find((block): block is Extract<DocBlock, { type: 'example' }> => block.type === 'example' && block.id === 'query')!
      expect(asyncQuery.responses.map(response => JSON.parse(response.body!).status)).toEqual(['processing', 'completed', 'failed'])
    }
  })

  it('documents the OpenAI, Anthropic and Gemini chat entry points', () => {
    for (const categories of [zhCategories, enCategories]) {
      const chat = categories.find(category => category.id === 'chat')!
      const requests = examples(chat).map(example => example.request).join('\n')
      for (const path of ['/v1/chat/completions', '/v1/responses', '/v1/messages', ':generateContent', ':streamGenerateContent']) {
        expect(requests).toContain(path)
      }
    }
  })
})
