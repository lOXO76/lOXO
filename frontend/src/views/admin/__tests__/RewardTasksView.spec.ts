import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import RewardTasksView from '../RewardTasksView.vue'
import type { AdminRewardTask, AdminRewardTaskPartner } from '@/api/admin/rewardTasks'

const mocks = vi.hoisted(() => ({
  getStats: vi.fn(),
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  listTasks: vi.fn(),
  createTask: vi.fn(),
  updateTask: vi.fn(),
  deleteTask: vi.fn(),
  listPartners: vi.fn(),
  createPartner: vi.fn(),
  updatePartner: vi.fn(),
  resetPartnerSecret: vi.fn(),
  deletePartner: vi.fn(),
  listRecords: vi.fn(),
  listEvents: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/rewardTasks', () => {
  const api = {
    getStats: mocks.getStats,
    getSettings: mocks.getSettings,
    updateSettings: mocks.updateSettings,
    listTasks: mocks.listTasks,
    createTask: mocks.createTask,
    updateTask: mocks.updateTask,
    deleteTask: mocks.deleteTask,
    listPartners: mocks.listPartners,
    createPartner: mocks.createPartner,
    updatePartner: mocks.updatePartner,
    resetPartnerSecret: mocks.resetPartnerSecret,
    deletePartner: mocks.deletePartner,
    listRecords: mocks.listRecords,
    listEvents: mocks.listEvents,
  }
  return { rewardTasksAdminAPI: api, default: api }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    rewardTasks: {
      getStats: mocks.getStats,
      getSettings: mocks.getSettings,
      updateSettings: mocks.updateSettings,
      listTasks: mocks.listTasks,
      createTask: mocks.createTask,
      updateTask: mocks.updateTask,
      deleteTask: mocks.deleteTask,
      listPartners: mocks.listPartners,
      createPartner: mocks.createPartner,
      updatePartner: mocks.updatePartner,
      resetPartnerSecret: mocks.resetPartnerSecret,
      deletePartner: mocks.deletePartner,
      listRecords: mocks.listRecords,
      listEvents: mocks.listEvents,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: mocks.showSuccess, showError: mocks.showError }),
}))

vi.mock('@/utils/format', () => ({ formatDateTime: (value: string | null) => `dt:${value ?? ''}` }))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}(${Object.values(params).join(',')})` : key,
    }),
  }
})

function makeTask(overrides: Partial<AdminRewardTask> = {}): AdminRewardTask {
  return {
    id: 1,
    title: '浏览 XX 官网 30 秒',
    description: '打开官网停留 30 秒',
    task_type: 'browse',
    verify_mode: 'timer',
    partner_id: null,
    partner_name: '',
    target_url: 'https://example.com/',
    icon_url: '',
    reward_amount: 0.5,
    period: 'daily',
    browse_seconds: 30,
    idle_timeout_seconds: 15,
    min_recharge_amount: 0,
    max_completions: 0,
    completed_count: 0,
    in_progress_count: 0,
    total_reward: 0,
    starts_at: null,
    ends_at: null,
    enabled: true,
    sort_order: 0,
    created_at: '2026-11-01T00:00:00Z',
    updated_at: '2026-11-01T00:00:00Z',
    ...overrides,
  }
}

function makePartner(overrides: Partial<AdminRewardTaskPartner> = {}): AdminRewardTaskPartner {
  return {
    id: 7,
    name: 'XX 官网',
    app_key: 'rk_0123456789abcdef01234567',
    secret_hint: '9f3c',
    allowed_domains: ['example.com'],
    contact: 'partner@example.com',
    notes: '',
    enabled: true,
    task_count: 1,
    last_event_at: null,
    created_at: '2026-11-01T00:00:00Z',
    updated_at: '2026-11-01T00:00:00Z',
    ...overrides,
  }
}

const stats = {
  total_tasks: 3,
  enabled_tasks: 2,
  completed_today: 5,
  reward_today: 1.5,
  completed_total: 42,
  reward_total: 21,
  partners: 1,
}

function pageOf<T>(items: T[], total = items.length) {
  return { items, total, page: 1, page_size: 20, pages: 1 }
}

function mountView() {
  return mount(RewardTasksView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true,
        RouterLink: RouterLinkStub,
        EmptyState: {
          props: ['title', 'description', 'actionText'],
          template: '<div class="empty-state-stub">{{ title }}</div>',
        },
        // 弹窗内容内联渲染，便于断言表单与一次性密钥
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show" class="dialog-stub"><slot /><slot name="footer" /></div>',
        },
        DataTable: {
          props: ['data', 'columns'],
          template: `<div class="table-stub">
            <span v-for="column in columns" :key="column.key" class="column-label">{{ column.label }}</span>
            <div v-if="!data || data.length === 0" class="table-empty"><slot name="empty" /></div>
            <div v-for="row in data" :key="row.id" class="data-row">
              <slot name="cell-title" :row="row" />
              <slot name="cell-name" :row="row" />
              <slot name="cell-user" :row="row" />
              <slot name="cell-type" :row="row" />
              <slot name="cell-quota" :row="row" />
              <slot name="cell-progress" :row="row" />
              <slot name="cell-reward" :row="row" />
              <slot name="cell-actions" :row="row" />
            </div>
          </div>`,
        },
      },
    },
  })
}

// 打开「新建任务」弹窗并切到 browse + sdk，补齐必填项，返回已就绪的 wrapper
async function openSdkTaskForm() {
  const wrapper = mountView()
  await flushPromises()
  await wrapper.find('[data-testid="task-create"]').trigger('click')
  await flushPromises()

  const findSelect = (id: string) =>
    wrapper.findAllComponents({ name: 'Select' }).find((component) => component.props('id') === id)
  findSelect('reward-task-verify-mode')!.vm.$emit('update:modelValue', 'sdk')
  findSelect('reward-task-verify-mode')!.vm.$emit('change', 'sdk')
  await flushPromises()

  await wrapper.find('[data-testid="reward-task-title"]').setValue('SDK 浏览任务')
  await wrapper.find('[data-testid="reward-task-target-url"]').setValue('https://example.com/')
  findSelect('reward-task-partner')!.vm.$emit('update:modelValue', '7')
  await flushPromises()
  return wrapper
}

describe('RewardTasksView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mocks.getStats.mockResolvedValue(stats)
    mocks.getSettings.mockResolvedValue({ enabled: true })
    mocks.updateSettings.mockImplementation(async (enabled: boolean) => ({ enabled }))
    mocks.listTasks.mockResolvedValue(
      pageOf([
        makeTask({ id: 1, title: '浏览 XX 官网 30 秒', completed_count: 3, max_completions: 10, in_progress_count: 2 }),
        makeTask({ id: 2, title: '充值满 100 元', task_type: 'recharge', verify_mode: 'callback', period: 'monthly', reward_amount: 5 }),
      ]),
    )
    mocks.listPartners.mockResolvedValue([makePartner()])
    mocks.listRecords.mockResolvedValue(pageOf([]))
    mocks.listEvents.mockResolvedValue(pageOf([]))
    mocks.createPartner.mockResolvedValue(makePartner({ id: 8, name: '新站点', app_secret: 'rs_0123456789abcdef0123456789abcdef0123456789abcdef' }))
    mocks.createTask.mockResolvedValue(makeTask({ id: 9 }))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the master switch state, overview and the task list', async () => {
    const wrapper = mountView()
    await flushPromises()

    // 总开关状态与概览
    expect(wrapper.find('[data-testid="reward-tasks-state"]').text()).toBe('admin.rewardTasks.state.enabled')
    expect(wrapper.text()).toContain('admin.rewardTasks.stats.totalTasks')
    expect(wrapper.text()).toContain('admin.rewardTasks.stats.enabledHint(2)')
    expect(wrapper.text()).toContain('admin.rewardTasks.stats.todayRewardHint($1.50)')
    expect(wrapper.text()).toContain('admin.rewardTasks.stats.rewardTotalHint($21.00)')

    // 任务列表：标题、类型、名额（completed/max）
    expect(mocks.listTasks).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(wrapper.text()).toContain('浏览 XX 官网 30 秒')
    expect(wrapper.text()).toContain('admin.rewardTasks.taskTypes.browse')
    expect(wrapper.text()).toContain('admin.rewardTasks.taskTypes.recharge')
    expect(wrapper.text()).toContain('admin.rewardTasks.tasks.quotaValue(3,10)')
    expect(wrapper.text()).toContain('admin.rewardTasks.tasks.quotaUnlimited')
    expect(wrapper.text()).toContain('+$0.50')
    expect(wrapper.text()).toContain('+$5.00')
  })

  it('links form fields to the selected task type', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="task-create"]').trigger('click')
    await flushPromises()

    const typeTabs = wrapper.findAll('[data-testid="reward-task-type-tab"]')
    expect(typeTabs).toHaveLength(3)

    // browse + timer：无需站点，显示计时模式提示与浏览秒数
    expect(wrapper.find('[data-testid="reward-task-timer-warning"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="reward-task-browse-seconds"]').exists()).toBe(true)
    expect(wrapper.find('#reward-task-partner').exists()).toBe(false)
    expect(wrapper.find('[data-testid="reward-task-min-recharge"]').exists()).toBe(false)

    // register：校验方式固定回调、必须选站点、无浏览秒数
    await typeTabs[1].trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="reward-task-timer-warning"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="reward-task-browse-seconds"]').exists()).toBe(false)
    expect(wrapper.find('#reward-task-partner').exists()).toBe(true)
    expect(wrapper.find('[data-testid="reward-task-min-recharge"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.rewardTasks.tasks.form.verifyModeFixed(admin.rewardTasks.verifyModes.callback)')

    // recharge：必须选站点，出现最低累计充值金额
    await typeTabs[2].trigger('click')
    await flushPromises()
    expect(wrapper.find('#reward-task-partner').exists()).toBe(true)
    expect(wrapper.find('[data-testid="reward-task-min-recharge"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="reward-task-browse-seconds"]').exists()).toBe(false)

    // browse + sdk：切换校验方式后必须选站点
    await typeTabs[0].trigger('click')
    await flushPromises()
    expect(wrapper.find('#reward-task-partner').exists()).toBe(false)
  })

  it('shows the one-time app_secret right after creating a partner', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="reward-tasks-tab-partners"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('XX 官网')

    await wrapper.find('[data-testid="partner-create"]').trigger('click')
    await flushPromises()

    const nameInput = wrapper.find('[data-testid="reward-partner-name"]')
    await nameInput.setValue('新站点')
    await wrapper.find('[data-testid="reward-partner-domains"]').setValue('example.com\n*.example.com')
    await wrapper.find('#reward-partner-form').trigger('submit')
    await flushPromises()

    expect(mocks.createPartner).toHaveBeenCalledWith({
      name: '新站点',
      allowed_domains: ['example.com', '*.example.com'],
      contact: '',
      notes: '',
      enabled: true,
    })
    // 一次性明文密钥只在此刻可见
    expect(wrapper.find('[data-testid="partner-secret-value"]').text()).toBe(
      'rs_0123456789abcdef0123456789abcdef0123456789abcdef',
    )
    expect(wrapper.find('[data-testid="partner-secret-app-key"]').text()).toBe('rk_0123456789abcdef01234567')
    expect(mocks.showSuccess).toHaveBeenCalledWith('admin.rewardTasks.partners.createSuccess')
  })

  it('shows the inactivity threshold only for SDK browse tasks and rejects invalid values', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="task-create"]').trigger('click')
    await flushPromises()

    // timer 模式不显示挂机阈值
    expect(wrapper.find('[data-testid="reward-task-idle-timeout"]').exists()).toBe(false)

    // 切到 sdk：出现阈值输入框，新建默认 15
    const verifySelect = wrapper
      .findAllComponents({ name: 'Select' })
      .find((component) => component.props('id') === 'reward-task-verify-mode')
    expect(verifySelect).toBeTruthy()
    verifySelect!.vm.$emit('update:modelValue', 'sdk')
    verifySelect!.vm.$emit('change', 'sdk')
    await flushPromises()

    const idleInput = wrapper.find('[data-testid="reward-task-idle-timeout"]')
    expect(idleInput.exists()).toBe(true)
    expect((idleInput.element as HTMLInputElement).value).toBe('15')

    // 补齐必填项（sdk 必须绑定接入站点）
    await wrapper.find('[data-testid="reward-task-title"]').setValue('SDK 浏览任务')
    await wrapper.find('[data-testid="reward-task-target-url"]').setValue('https://example.com/?t={ticket}')
    const partnerSelect = wrapper
      .findAllComponents({ name: 'Select' })
      .find((component) => component.props('id') === 'reward-task-partner')
    partnerSelect!.vm.$emit('update:modelValue', '7')
    await flushPromises()

    // 非法值（3 秒）被前端拦截，不会调用接口
    await idleInput.setValue('3')
    await wrapper.find('#reward-task-form').trigger('submit')
    await flushPromises()
    expect(mocks.createTask).not.toHaveBeenCalled()
    expect(mocks.showError).toHaveBeenCalledWith('admin.rewardTasks.tasks.form.invalidIdleTimeout')

    // 0 = 不检测，合法并写入 payload
    await idleInput.setValue('0')
    await wrapper.find('#reward-task-form').trigger('submit')
    await flushPromises()
    expect(mocks.createTask).toHaveBeenCalledWith(
      expect.objectContaining({
        task_type: 'browse',
        verify_mode: 'sdk',
        partner_id: 7,
        idle_timeout_seconds: 0,
      }),
    )
  })

  it('blocks submission when the SDK inactivity threshold is cleared', async () => {
    const wrapper = await openSdkTaskForm()
    const idleInput = wrapper.find('[data-testid="reward-task-idle-timeout"]')
    expect((idleInput.element as HTMLInputElement).value).toBe('15')

    // 清空后不能静默变成 0（关闭检测）或 15，必须先被校验拦下
    await idleInput.setValue('')
    await wrapper.find('#reward-task-form').trigger('submit')
    await flushPromises()
    expect(mocks.createTask).not.toHaveBeenCalled()
    expect(mocks.showError).toHaveBeenCalledWith('admin.rewardTasks.tasks.form.invalidIdleTimeout')

    // 只有明确填 0 才表示不检测
    await idleInput.setValue('0')
    await wrapper.find('#reward-task-form').trigger('submit')
    await flushPromises()
    expect(mocks.createTask).toHaveBeenCalledWith(expect.objectContaining({ idle_timeout_seconds: 0 }))
  })

  it('always sends idle_timeout_seconds, defaulting to 15 for non-SDK tasks', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="task-create"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="reward-task-title"]').setValue('计时浏览任务')
    await wrapper.find('[data-testid="reward-task-target-url"]').setValue('https://example.com/')
    await wrapper.find('#reward-task-form').trigger('submit')
    await flushPromises()

    expect(mocks.createTask).toHaveBeenCalledWith(
      expect.objectContaining({
        task_type: 'browse',
        verify_mode: 'timer',
        partner_id: null,
        idle_timeout_seconds: 15,
      }),
    )
  })

  it('confirms before toggling the master switch', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="reward-tasks-toggle"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.rewardTasks.state.confirmDisableMessage')

    const confirmButton = wrapper.findAll('button').find((button) => button.text() === 'common.confirm')
    expect(confirmButton).toBeTruthy()
    await confirmButton!.trigger('click')
    await flushPromises()

    expect(mocks.updateSettings).toHaveBeenCalledWith(false)
    expect(wrapper.find('[data-testid="reward-tasks-state"]').text()).toBe('admin.rewardTasks.state.disabled')
  })

  it('switches to the completions tab', async () => {
    mocks.listRecords.mockResolvedValue(
      pageOf([
        {
          id: 11,
          task_id: 1,
          task_title: '浏览 XX 官网 30 秒',
          task_type: 'browse',
          user_id: 5,
          user_email: 'user@example.com',
          username: 'alice',
          period_key: '2026-11-05',
          status: 'completed',
          progress_seconds: 30,
          recharge_total: 0,
          external_user_id: '',
          started_at: '2026-11-05T10:00:00Z',
          ticket_expires_at: '2026-11-06T10:00:00Z',
          completed_at: '2026-11-05T10:00:30Z',
          reward_amount: 0.5,
        },
      ]),
    )
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="reward-tasks-tab-records"]').trigger('click')
    await flushPromises()

    expect(mocks.listRecords).toHaveBeenCalled()
    expect(wrapper.text()).toContain('user@example.com')
    expect(wrapper.text()).toContain('admin.rewardTasks.records.progressSeconds(30)')
    expect(wrapper.text()).toContain('+$0.50')
  })
})
