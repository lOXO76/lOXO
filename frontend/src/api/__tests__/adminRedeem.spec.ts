import { beforeEach, describe, expect, it, vi } from 'vitest'
import { generate } from '../admin/redeem'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { post } }))

describe('admin redeem code generation payload', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    post.mockResolvedValue({ data: [] })
  })

  it('sends the benefit type and limits for a multi-use code', async () => {
    await generate(2, 'multi_use', 5, undefined, undefined, undefined, {
      benefitType: 'balance',
      maxUsers: 100,
      maxUsesPerUser: 3
    })

    expect(post).toHaveBeenCalledWith('/admin/redeem-codes/generate', {
      count: 2,
      type: 'multi_use',
      value: 5,
      benefit_type: 'balance',
      max_users: 100,
      max_uses_per_user: 3
    })
  })

  it('sends the group and validity for a multi-use subscription benefit', async () => {
    await generate(1, 'multi_use', 0, 7, 30, 3, {
      benefitType: 'subscription',
      maxUsers: 10,
      maxUsesPerUser: 1
    })

    expect(post).toHaveBeenCalledWith('/admin/redeem-codes/generate', {
      count: 1,
      type: 'multi_use',
      value: 0,
      benefit_type: 'subscription',
      max_users: 10,
      max_uses_per_user: 1,
      group_id: 7,
      validity_days: 30,
      expires_in_days: 3
    })
  })

  it('does not send subscription fields for a multi-use balance benefit', async () => {
    await generate(1, 'multi_use', 5, 7, 30, undefined, {
      benefitType: 'balance',
      maxUsers: 10,
      maxUsesPerUser: 1
    })

    const payload = post.mock.calls[0][1]
    expect(payload).not.toHaveProperty('group_id')
    expect(payload).not.toHaveProperty('validity_days')
  })

  it('keeps single-use payloads unchanged', async () => {
    await generate(1, 'balance', 10)
    expect(post).toHaveBeenLastCalledWith('/admin/redeem-codes/generate', {
      count: 1,
      type: 'balance',
      value: 10
    })

    await generate(1, 'subscription', 0, 7, 30)
    expect(post).toHaveBeenLastCalledWith('/admin/redeem-codes/generate', {
      count: 1,
      type: 'subscription',
      value: 0,
      group_id: 7,
      validity_days: 30
    })

    for (const [, payload] of post.mock.calls) {
      expect(payload).not.toHaveProperty('benefit_type')
      expect(payload).not.toHaveProperty('max_users')
      expect(payload).not.toHaveProperty('max_uses_per_user')
    }
  })
})
