# AEO · AI 搜索引擎优化指南

> AEO = **Answer Engine Optimization**（答案引擎优化）。当用户问 ChatGPT / Perplexity / 豆包等 AI 助手时，
> 它们会抓取你的页面来回答。AEO 的目标是让 **AI 爬虫拿到可直接引用的干净答案**，从而在 AI 回答中被引用、提升品牌可见度。
> 本文面向**使用者**，如实区分「当前已生效」与「已实现但尚未接入请求链路」的能力。内部实现见 [seo-aeo.md](seo-aeo.md)。

---

## 为什么需要 AEO

- AI 助手（GPTBot / ClaudeBot / PerplexityBot / Google-Extended / Bytespider 等）会爬取你的页面。
- 如果返回的是带导航、广告、`script` 噪声的复杂 HTML，AI 很难提取「能直接引用的一句话答案」，你的内容就用不上。
- 传统 SEO 面向结果列表；AEO 面向**直接答案**（featured snippet / AI 摘要）。

---

## ✅ 已生效的能力：爬虫分类 + 渲染策略（`prerender.category_policy`）

Prerender Shield 会把请求爬虫分成 `search` / `social` / `ai` / `generic` 四类，并对每类**独立决定渲染策略**。这是当前已接入请求链路（`internal/site-handler/handler.go`）的核心 AEO 杠杆。

### `ai` 分类识别哪些爬虫

| 爬虫 | 公司 | 用途 |
|------|------|------|
| GPTBot / ClaudeBot / Claude-Web | OpenAI / Anthropic | 训练 / 搜索 |
| PerplexityBot / Perplexity-User | Perplexity AI | 搜索 |
| Google-Extended | Google (Gemini) | 训练 |
| Cohere-AI | Cohere | 训练 |
| FacebookBot | Meta AI | 训练 |
| AppleBot | Apple | 搜索 |
| Bytespider | ByteDance | 训练 |

> 若想整体拦掉某类 AI **训练**爬虫（如 GPTBot），请在 `robots.txt` 里 `Disallow`（Prerender Shield 可用 `seo.robots` 生成）；面向 AI 的策略这里主要做**友好供给**，而非拦截。

### 每种策略值

| 策略 | 行为 | 何时用 |
|------|------|--------|
| `render` | 该分类爬虫总是触发**实时渲染**，拿最新 HTML | AI 爬虫（要新鲜内容） |
| `cache_only` | 只回缓存，不触发渲染 | 社交/低价值爬虫（省算力） |
| `passthrough` | 透传上游，不预渲染 | 不希望被烧 Chromium 的类 |

### 配置示例

```yaml
sites:
  - id: "site1"
    prerender:
      category_policy:
        ai: "render"          # AI 爬虫拿最新渲染
        search: "render"      # 搜索引擎爬虫也拿最新
        social: "render"
        generic: "render"
```

> **默认策略**（`internal/site-handler/handler.go` 的 `defaultCategoryPolicies`）为：
> `search: render`、`social: render`、`ai: cache_only`、`generic: render`。
> 即 **AI 爬虫默认只回缓存、不触发新渲染**——若要让 AI 爬虫拿到最新内容，**务必显式设置 `ai: render`**。
> 未配置的类别返回 `render`（未命中默认表时兜底 `render`）。分类解析见 `prerenderPolicyFor`。

---

## 🔬 已实现但尚未接入请求链路的能力

以下内容在 `internal/seo/aeo.go` 中**已实现、单元测试通过，但当前未在渲染/响应链路中实际调用**，配置暂不生效——列为演进方向，供二次开发接入：

| 能力 | 函数 | 现状 |
|------|------|------|
| AI 爬虫识别号 | `IsAICrawler` / `KnownAICrawlers` | 已实现，未在请求路径调用 |
| 纯净答案提取 | `ExtractAnswer`（剥 nav/广告/script，输出 summary/bullet/qa） | 已实现，未接入 |
| AEO 结果结构 | `AEResult{ IsAICrawler, ExtractedAnswer, StructuredData }` | 已实现 |
| 结构化数据增强 | `EnableStructuredData` | 见 `structured_data.go`（Meta 注入管线已启用其生成） |

> 若你需要这些能力真正生效（对 AI 爬虫返回去噪后的纯净正文），建议在 `internal/prerender/seo_injector.go` 的响应出口注入 `ExtractAnswer`，或关注后期版本。

---

## 为什么「预渲染 + 正确策略」本身就已提升 AEO

即使答案提取未接线，`ai: render` 已带来实际收益：

1. **SSR 而非 SPA 空壳**：AI 爬虫通常不执行 JS，实时渲染保证它拿到完整 HTML（而 `passthrough` 会给它原始 SPA 空壳，AI 无法理解）。
2. **结构化数据（JSON-LD）**：`seo.llm` / `structured_data.go` 可生成 Schema.org JSON-LD，明确告知 AI 页面是 FAQ / Product / Article，机器可直接消费，AEO 收益显著。
3. **新鲜度**：搜索结果可能引用过期快照；`ai: render` 保证 AI 爬虫每次拿最新内容。

---

## 验证

```bash
# 用某 AI 爬虫 UA 请求，观察是否触发"最新渲染"（而非 passthrough 的空壳）
curl -A "GPTBot" http://你的域名/some-page         # 应返回完整 HTML
curl -A "Mozilla/5.0 ..." http://你的域名/some-page # 普通请求原样透传/走缓存

# 确认 AI 爬虫分类与渲染次数
curl "http://127.0.0.1:9598/api/v1/crawler/url-stats?site=site1"
```

- 控制台「爬虫日志」能看到 `ai` 分类的爬虫访问记录。
- 对比 `render` vs `passthrough`：`ai: render` 命中时返回完整渲染 HTML；`passthrough` 返回源站内容。

---

## 常见疑问

**Q：开启 AEO 影响普通用户吗？**
不影响。策略仅作用于爬虫分类；普通 UA 走原样透传/缓存，不会被改动或烧渲染。

**Q：AI 爬虫识别号和答案提取何时能用？**
目前分类+策略可用；`ExtractAnswer` 去噪答案尚未接入，属演进项（见上表）。

**Q：想拦 AI 训练爬虫但保留 AI 搜索爬虫怎么配？**
两者不冲突：在 `seo.robots` 对训练型 UA（如 GPTBot）`disallow`；面向搜索型 AI（如 Perplexity）继续 `category_policy.ai = render`。Prerender Shield 会按 UA 分别处理。
