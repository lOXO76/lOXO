import type { DocsCategory } from './types'
import zhChat from './chat/zh'
import enChat from './chat/en'
import zhImage from './image/zh'
import enImage from './image/en'
import zhVideo from './video/zh'
import enVideo from './video/en'

/** Categories in catalog order; the first one opens by default. */
export const zhCategories: DocsCategory[] = [zhChat, zhImage, zhVideo]
export const enCategories: DocsCategory[] = [enChat, enImage, enVideo]
