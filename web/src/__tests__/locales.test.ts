/**
 * 后台 i18n locale 完整性守护测试。
 *
 * 背景: 控制台支持 zh/en/ja/ko 四语言。翻译 key 在语言间漂移不会导致
 * 构建失败，只会在切换语言时渲染 raw key。此测试确保四份 locale 的
 * key 结构完全对齐。
 */
import { describe, expect, it } from 'vitest'
import { readFileSync, readdirSync, existsSync } from 'node:fs'
import { join } from 'node:path'

const LOCALES_DIR = join(__dirname, '..', 'locales')
const PAGES_DIR = join(LOCALES_DIR, 'pages')
const LANGS = ['zh', 'en', 'ja', 'ko'] as const

function load(lang: string): Record<string, unknown> {
  return JSON.parse(readFileSync(join(LOCALES_DIR, `${lang}.json`), 'utf-8'))
}

/** 加载页面级命名空间 locale（pages/<ns>.<lang>.json），按命名空间分组 */
function loadPageNamespaces(lang: string): Record<string, Record<string, unknown>> {
  const out: Record<string, Record<string, unknown>> = {}
  for (const f of readdirSync(PAGES_DIR)) {
    const m = f.match(/^(.+)\.(zh|en|ja|ko)\.json$/)
    if (!m) continue
    const [, ns, fileLang] = m
    if (fileLang !== lang) continue
    out[ns] = JSON.parse(readFileSync(join(PAGES_DIR, f), 'utf-8'))
  }
  return out
}

/** 收集嵌套对象全部叶子路径 */
function leafPaths(obj: unknown, prefix = ''): string[] {
  if (obj === null || typeof obj !== 'object') return [prefix]
  if (Array.isArray(obj)) {
    return obj.flatMap((v, i) => leafPaths(v, `${prefix}[${i}]`))
  }
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) =>
    leafPaths(v, prefix ? `${prefix}.${k}` : k)
  )
}

describe('admin locales integrity', () => {
  const refPaths = new Set(leafPaths(load('zh')))

  it.each(LANGS.filter((l) => l !== 'zh'))('locale "%s" keys match zh exactly', (lang) => {
    const paths = new Set(leafPaths(load(lang)))
    const missing = [...refPaths].filter((p) => !paths.has(p))
    const extra = [...paths].filter((p) => !refPaths.has(p))
    expect(missing, `${lang} 缺少的 key`).toEqual([])
    expect(extra, `${lang} 多出的 key`).toEqual([])
  })

  it('no untranslated empty strings in any locale', () => {
    for (const lang of LANGS) {
      const empties = leafPaths(load(lang)).length // sanity
      expect(empties).toBeGreaterThan(0)
      const data = load(lang)
      const check = (node: unknown, path: string) => {
        if (typeof node === 'string') {
          expect(node.trim() !== '', `${lang}:${path} 为空翻译`).toBe(true)
        } else if (Array.isArray(node)) {
          node.forEach((v, i) => check(v, `${path}[${i}]`))
        } else if (node && typeof node === 'object') {
          for (const [k, v] of Object.entries(node)) check(v, path ? `${path}.${k}` : k)
        }
      }
      check(data, '')
    }
  })
})

describe('admin page-level locales integrity (pages/<ns>.<lang>.json)', () => {
  it('every namespace has all four languages with identical key structure', () => {
    if (!existsSync(PAGES_DIR)) return
    // 以 zh 为基准收集命名空间清单
    const zhNamespaces = loadPageNamespaces('zh')
    expect(Object.keys(zhNamespaces).length).toBeGreaterThan(0)

    for (const lang of LANGS.filter((l) => l !== 'zh')) {
      const langNamespaces = loadPageNamespaces(lang)
      for (const [ns, zhData] of Object.entries(zhNamespaces)) {
        const langData = langNamespaces[ns]
        expect(langData, `${lang}/${ns}.json 缺失`).toBeDefined()
        const refPaths = new Set(leafPaths(zhData))
        const paths = new Set(leafPaths(langData))
        const missing = [...refPaths].filter((p) => !paths.has(p))
        const extra = [...paths].filter((p) => !refPaths.has(p))
        expect(missing, `${lang}/${ns} 缺少的 key`).toEqual([])
        expect(extra, `${lang}/${ns} 多出的 key`).toEqual([])
      }
      for (const ns of Object.keys(langNamespaces)) {
        expect(zhNamespaces[ns], `${lang}/${ns}.json 在 zh 中不存在（多余文件）`).toBeDefined()
      }
    }
  })
})
