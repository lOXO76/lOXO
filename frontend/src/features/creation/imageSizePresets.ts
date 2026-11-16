/**
 * 生图「分辨率 × 画面比例」→ 请求参数的厂商映射表（写死，集中维护）。
 *
 * 表单只负责画胶囊、把用户选的组合交给这里，由这里翻译成各厂商真正认的字段：
 *
 * - `openai`：发 `size`（像素尺寸，如 `2048x1152`）。分辨率档只是给用户看的分组，
 *   后端按像素长边分计费档（长边 ≤1024 → 1K、≤2048 → 2K、更大 → 4K），所以表里
 *   每一格都是具体尺寸；`auto` 交给后端按提示词自选尺寸，此时没有比例可选。
 * - `gemini`：发 `aspectRatio` + `imageSize`（1K / 2K / 4K），所有组合都可用。
 * - `grok`：发 `aspectRatio` + `imageSize`（网关层把 imageSize 作为 resolution 发送），
 *   同样所有组合都可用，但只有 1K / 2K 两档。
 *
 * openai 的像素尺寸取值规则（改表时照这个来）：
 * - 宽高都是 16 的倍数，长边 ≤ 3840，宽高比 ≤ 3:1；
 * - 总像素必须在 655,360 与 8,294,400 之间（gpt-image-2 的尺寸约束）；
 * - 长边决定计费档：≤1024 → 1K、≤2048 → 2K、更大 → 4K，每格都要落在自己那一档里；
 * - 每格优先取精确比例，其次取该档内像素最大、比例误差 ≤ 1% 的尺寸。
 *   所以 1K 少了 9:16 / 16:9 / 1:2 / 2:1 / 9:21 / 21:9：长边锁在 1024 时这些宽幅比例最高只能到
 *   1024x576 / 576x1024（589,824 像素，低于 655,360 下限）；想凑够像素就得把长边拉过 1024，
 *   计费会跳到 2K 档，和「1K」对不上，因此这几档在 1K 下不存在。
 *
 * 新增厂商的步骤：
 * 1. 把厂商名加进 `CreationImageVendor`；
 * 2. 在 `VENDOR_PRESETS` 里补 `resolutions` / `ratios`（数组顺序即显示顺序）；
 * 3. 若它发的是 `size` 而不是 `aspectRatio`，再加一张「分辨率 → 比例 → 尺寸」的表，
 *    并在 `imageRatiosFor` / `resolveImageSize` 里接上；发比例的厂商走默认分支即可。
 *
 * 本文件是纯数据 + 纯函数，不依赖 Vue，方便单测与以后单独搬运。
 */

/** 生图参数风格：openai 走 size，gemini / grok 走 aspectRatio + imageSize。 */
export type CreationImageVendor = 'openai' | 'gemini' | 'grok'

export interface CreationImageSizeVendorPreset {
  /** 分辨率档位，按显示顺序。 */
  resolutions: string[]
  /** 画面比例全集，按显示顺序（openai 会再按当前分辨率过滤）。 */
  ratios: string[]
}

/** openai：分辨率 → 比例 → 实际请求的 size（取值规则见文件头）。`auto` 不进这张表（由后端自选）。 */
const OPENAI_SIZES: Readonly<Record<string, Readonly<Record<string, string>>>> = {
  '1K': {
    '1:1': '1024x1024',
    '3:4': '768x1024',
    '4:3': '1024x768',
    '2:3': '672x1008',
    '3:2': '1008x672',
    '4:5': '816x1024',
    '5:4': '1024x816',
  },
  '2K': {
    '1:1': '2048x2048',
    '3:4': '1536x2048',
    '4:3': '2048x1536',
    '2:3': '1344x2016',
    '3:2': '2016x1344',
    '4:5': '1648x2048',
    '5:4': '2048x1648',
    '9:16': '1152x2048',
    '16:9': '2048x1152',
    '1:2': '1024x2048',
    '2:1': '2048x1024',
    '9:21': '864x2016',
    '21:9': '2016x864',
  },
  '4K': {
    '1:1': '2880x2880',
    '3:4': '2448x3264',
    '4:3': '3264x2448',
    '2:3': '2336x3504',
    '3:2': '3504x2336',
    '4:5': '2560x3200',
    '5:4': '3200x2560',
    '9:16': '2160x3840',
    '16:9': '3840x2160',
    '1:2': '1920x3840',
    '2:1': '3840x1920',
    '9:21': '1632x3808',
    '21:9': '3808x1632',
  },
}

/** openai 的「自动」档：显示为 i18n 的「自动」，请求里 size=auto。 */
const OPENAI_AUTO = 'auto'

const VENDOR_PRESETS: Readonly<Record<CreationImageVendor, CreationImageSizeVendorPreset>> = {
  openai: {
    resolutions: ['1K', '2K', '4K', OPENAI_AUTO],
    ratios: ['1:1', '3:4', '4:3', '2:3', '3:2', '4:5', '5:4', '9:16', '16:9', '1:2', '2:1', '9:21', '21:9'],
  },
  gemini: {
    resolutions: ['1K', '2K', '4K'],
    ratios: ['1:1', '3:4', '4:3', '2:3', '3:2', '4:5', '5:4', '9:16', '16:9', '21:9', 'auto'],
  },
  grok: {
    resolutions: ['1K', '2K'],
    ratios: ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3', '2:1', '1:2', '19.5:9', '9:19.5', '20:9', '9:20'],
  },
}

/** 一组「分辨率 + 比例」翻译出的请求参数。 */
export type CreationImageResolvedSize =
  /** openai：只发 size。 */
  | { kind: 'size'; size: string }
  /** gemini / grok：发 aspectRatio + imageSize。 */
  | { kind: 'aspect'; aspectRatio: string; imageSize: string }

/** 某厂商的分辨率 / 比例全集（返回副本，调用方改不动内置表）。 */
export function imageSizePreset(vendor: CreationImageVendor): CreationImageSizeVendorPreset {
  const preset = VENDOR_PRESETS[vendor]
  return { resolutions: [...preset.resolutions], ratios: [...preset.ratios] }
}

/**
 * 该分辨率下可选的比例（保持 `ratios` 的显示顺序）。
 * openai 只有部分组合，`auto` 档一个比例都没有（表单据此隐藏比例组）；其余厂商返回全集。
 */
export function imageRatiosFor(vendor: CreationImageVendor, resolution: string): string[] {
  const preset = VENDOR_PRESETS[vendor]
  if (vendor !== 'openai') {
    return preset.resolutions.includes(resolution) ? [...preset.ratios] : []
  }
  const sizes = OPENAI_SIZES[resolution]
  return sizes ? preset.ratios.filter(ratio => ratio in sizes) : []
}

/**
 * 把「分辨率 + 比例」翻译成请求参数。
 * 组合不被支持（例如 openai 的 1K + 16:9、grok 的 4K）时返回 null，调用方据此禁用提交或回退。
 * openai 的 `auto` 忽略比例，直接返回 size=auto。
 */
export function resolveImageSize(
  vendor: CreationImageVendor,
  resolution: string,
  ratio: string,
): CreationImageResolvedSize | null {
  if (vendor === 'openai') {
    if (resolution === OPENAI_AUTO) return { kind: 'size', size: OPENAI_AUTO }
    const size = OPENAI_SIZES[resolution]?.[ratio]
    return size ? { kind: 'size', size } : null
  }
  const preset = VENDOR_PRESETS[vendor]
  if (!preset.resolutions.includes(resolution) || !preset.ratios.includes(ratio)) return null
  return { kind: 'aspect', aspectRatio: ratio, imageSize: resolution }
}

/** `2048x1152` → `2048×1152`，给「输出尺寸」提示用；`auto` 等没有像素值的返回空串。 */
export function imageSizePixels(size: string): string {
  const match = /^\s*(\d+)\s*x\s*(\d+)\s*$/i.exec(size)
  return match ? `${match[1]}×${match[2]}` : ''
}
