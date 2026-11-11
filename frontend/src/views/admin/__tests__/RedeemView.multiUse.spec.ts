import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const {
  listRedeemCodes,
  generateRedeemCodes,
  batchDeleteRedeemCodes,
  getAllGroups,
  showSuccess,
  showError,
  showInfo
} = vi.hoisted(() => ({
    listRedeemCodes: vi.fn(),
    generateRedeemCodes: vi.fn(),
    batchDeleteRedeemCodes: vi.fn(),
    getAllGroups: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn(),
    showInfo: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    redeem: {
      list: listRedeemCodes,
      generate: generateRedeemCodes,
      delete: vi.fn(),
      batchDelete: batchDeleteRedeemCodes,
      batchUpdate: vi.fn(),
      exportCodes: vi.fn()
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
    showInfo
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

// 带参数的文案把参数一并输出，便于断言传给 i18n 的插值。
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <table>
      <tbody>
        <tr v-for="row in data" :key="row.id" :data-row="row.id">
          <td v-for="column in columns" :key="column.key">
            <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]">
              {{ row[column.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  `
}

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  setup(
    props: { options: Array<{ value: unknown; label: string }> },
    { emit }: { emit: (event: string, ...args: unknown[]) => void }
  ) {
    const onChange = (event: Event) => {
      const raw = (event.target as HTMLSelectElement).value
      const option = props.options.find((item) => String(item.value ?? '') === raw)
      const value = option ? option.value : raw
      emit('update:modelValue', value)
      emit('change', value, option ?? null)
    }
    return { onChange }
  },
  template: `
    <select v-bind="$attrs" :value="modelValue ?? ''" @change="onChange">
      <option v-for="option in options" :key="String(option.value ?? '')" :value="option.value ?? ''">
        {{ option.label }}
      </option>
    </select>
  `
}

const mountView = async () => {
  const wrapper = mount(RedeemView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        Pagination: true,
        ConfirmDialog: true,
        Select: SelectStub,
        GroupBadge: true,
        GroupOptionItem: true,
        Icon: true,
        Teleport: true
      }
    }
  })
  await flushPromises()
  return wrapper
}

type Wrapper = Awaited<ReturnType<typeof mountView>>

const openMultiUseForm = async (wrapper: Wrapper) => {
  await wrapper.get('[data-test="generate-open"]').trigger('click')
  await wrapper.get('[data-test="generate-type-select"]').setValue('multi_use')
}

const submitGenerateForm = async (wrapper: Wrapper) => {
  await wrapper.get('[data-test="generate-form"]').trigger('submit')
  await flushPromises()
}

describe('admin RedeemView multi-use codes', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
    vi.clearAllMocks()

    listRedeemCodes.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    generateRedeemCodes.mockResolvedValue([
      { id: 9, code: 'MULTI-1', type: 'multi_use', value: 5, status: 'unused', used_by: null, used_at: null, created_at: '2026-01-01T00:00:00Z' }
    ])
    getAllGroups.mockResolvedValue([])
  })

  it('offers the multi-use type and only shows its limits once selected', async () => {
    const wrapper = await mountView()
    await wrapper.get('[data-test="generate-open"]').trigger('click')

    const typeOptions = wrapper
      .get('[data-test="generate-type-select"]')
      .findAll('option')
      .map((option) => option.attributes('value'))
    expect(typeOptions).toEqual(['balance', 'concurrency', 'subscription', 'invitation', 'multi_use'])
    expect(wrapper.find('[data-test="generate-max-users-input"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="generate-benefit-type-select"]').exists()).toBe(false)

    await wrapper.get('[data-test="generate-type-select"]').setValue('multi_use')

    expect(wrapper.find('[data-test="generate-max-users-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="generate-max-uses-per-user-input"]').exists()).toBe(true)
    const benefitOptions = wrapper
      .get('[data-test="generate-benefit-type-select"]')
      .findAll('option')
      .map((option) => option.attributes('value'))
    expect(benefitOptions).toEqual(['balance', 'concurrency', 'subscription'])
    wrapper.unmount()
  })

  it('generates a multi-use code with the entered limits', async () => {
    const wrapper = await mountView()
    await openMultiUseForm(wrapper)

    await wrapper.get('[data-test="generate-value-input"]').setValue(5)
    await wrapper.get('[data-test="generate-max-users-input"]').setValue(50)
    await wrapper.get('[data-test="generate-max-uses-per-user-input"]').setValue(3)
    await submitGenerateForm(wrapper)

    expect(showError).not.toHaveBeenCalled()
    expect(generateRedeemCodes).toHaveBeenCalledTimes(1)
    expect(generateRedeemCodes).toHaveBeenCalledWith(1, 'multi_use', 5, undefined, undefined, undefined, {
      benefitType: 'balance',
      maxUsers: 50,
      maxUsesPerUser: 3
    })
    wrapper.unmount()
  })

  it('passes the selected benefit type and hides the amount for a subscription benefit', async () => {
    getAllGroups.mockResolvedValue([
      { id: 7, name: 'Pro', description: null, platform: 'openai', subscription_type: 'subscription', rate_multiplier: 1 }
    ])
    const wrapper = await mountView()
    await openMultiUseForm(wrapper)

    await wrapper.get('[data-test="generate-benefit-type-select"]').setValue('concurrency')
    await wrapper.get('[data-test="generate-value-input"]').setValue(2)
    await submitGenerateForm(wrapper)

    expect(generateRedeemCodes).toHaveBeenLastCalledWith(1, 'multi_use', 2, undefined, undefined, undefined, {
      benefitType: 'concurrency',
      maxUsers: 100,
      maxUsesPerUser: 1
    })

    // 订阅权益：不再显示面值输入，必须先选择分组。
    await openMultiUseForm(wrapper)
    await wrapper.get('[data-test="generate-benefit-type-select"]').setValue('subscription')
    expect(wrapper.find('[data-test="generate-value-input"]').exists()).toBe(false)
    await submitGenerateForm(wrapper)

    expect(showError).toHaveBeenCalledWith('admin.redeem.groupRequired')
    expect(generateRedeemCodes).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it.each([
    ['zero users', 0, 1],
    ['too many users', 1000001, 1],
    ['fractional users', 1.5, 1],
    ['zero uses per user', 10, 0],
    ['too many uses per user', 10, 1001]
  ])('rejects invalid limits: %s', async (_name, maxUsers, maxUsesPerUser) => {
    const wrapper = await mountView()
    await openMultiUseForm(wrapper)

    await wrapper.get('[data-test="generate-max-users-input"]').setValue(maxUsers)
    await wrapper.get('[data-test="generate-max-uses-per-user-input"]').setValue(maxUsesPerUser)
    await submitGenerateForm(wrapper)

    expect(showError).toHaveBeenCalledWith('admin.redeem.multiUseLimitsRequired')
    expect(generateRedeemCodes).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps single-use generation free of multi-use options', async () => {
    const wrapper = await mountView()
    await wrapper.get('[data-test="generate-open"]').trigger('click')
    await submitGenerateForm(wrapper)

    expect(generateRedeemCodes).toHaveBeenCalledWith(1, 'balance', 10, undefined, undefined, undefined, undefined)
    wrapper.unmount()
  })

  it('keeps partially redeemed multi-use codes when deleting all unused codes', async () => {
    const unused = { status: 'unused', used_by: null, used_at: null, created_at: '2026-01-01T00:00:00Z' }
    listRedeemCodes.mockResolvedValue({
      items: [
        { ...unused, id: 1, code: 'MULTI-ACTIVE', type: 'multi_use', benefit_type: 'balance', value: 5, max_users: 10, max_uses_per_user: 1, used_count: 2, used_user_count: 2 },
        { ...unused, id: 2, code: 'MULTI-FRESH', type: 'multi_use', benefit_type: 'balance', value: 5, max_users: 10, max_uses_per_user: 1, used_count: 0, used_user_count: 0 },
        { ...unused, id: 3, code: 'SINGLE-1', type: 'balance', value: 20 }
      ],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })
    batchDeleteRedeemCodes.mockResolvedValue({ deleted: 2, message: 'ok' })
    const wrapper = await mountView()

    // 第二个确认框是「删除全部未使用」。
    wrapper.findAllComponents(ConfirmDialog)[1].vm.$emit('confirm')
    await flushPromises()

    expect(batchDeleteRedeemCodes).toHaveBeenCalledTimes(1)
    expect(batchDeleteRedeemCodes).toHaveBeenCalledWith([2, 3])
    wrapper.unmount()
  })

  it('shows usage for a multi-use code and the source code for its redemptions', async () => {
    listRedeemCodes.mockResolvedValue({
      items: [
        {
          id: 1,
          code: 'MULTI-1',
          type: 'multi_use',
          benefit_type: 'balance',
          value: 5,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          max_users: 10,
          max_uses_per_user: 2,
          used_count: 3,
          used_user_count: 2
        },
        {
          id: 2,
          code: 'f3a9c1d2e4b5a6978877665544332211',
          type: 'balance',
          value: 5,
          status: 'used',
          used_by: 42,
          used_at: '2026-01-02T00:00:00Z',
          created_at: '2026-01-02T00:00:00Z',
          parent_id: 1,
          parent_code: 'MULTI-1'
        },
        {
          id: 3,
          code: 'SINGLE-1',
          type: 'balance',
          value: 20,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z'
        }
      ],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })
    const wrapper = await mountView()

    const multiUseRow = wrapper.get('[data-row="1"]')
    expect(multiUseRow.text()).toContain('admin.redeem.types.multi_use')
    expect(multiUseRow.text()).toContain('$5.00')
    expect(multiUseRow.text()).toContain('admin.redeem.perRedemption')
    const usage = multiUseRow.get('[data-test="multi-use-usage"]').text()
    expect(usage).toContain('admin.redeem.multiUseUsage')
    expect(usage).toContain('"usedCount":3')
    expect(usage).toContain('"usedUsers":2')
    expect(usage).toContain('"maxUsers":10')
    expect(usage).toContain('"perUser":2')
    expect(multiUseRow.find('[data-test="redeemed-from"]').exists()).toBe(false)
    // 部分兑换后仍可兑换，不能显示成「未使用」。
    expect(multiUseRow.text()).toContain('admin.redeem.multiUseStatus.active')
    expect(multiUseRow.text()).not.toContain('admin.redeem.status.unused')

    const recordRow = wrapper.get('[data-row="2"]')
    expect(recordRow.get('[data-test="redeemed-from"]').text()).toContain('"code":"MULTI-1"')
    expect(recordRow.find('[data-test="multi-use-usage"]').exists()).toBe(false)
    expect(recordRow.text()).not.toContain('admin.redeem.perRedemption')
    expect(recordRow.text()).toContain('admin.redeem.status.used')

    const singleRow = wrapper.get('[data-row="3"]')
    expect(singleRow.find('[data-test="multi-use-usage"]').exists()).toBe(false)
    expect(singleRow.find('[data-test="redeemed-from"]').exists()).toBe(false)
    expect(singleRow.text()).toContain('$20.00')
    expect(singleRow.text()).toContain('admin.redeem.status.unused')
    wrapper.unmount()
  })
})
