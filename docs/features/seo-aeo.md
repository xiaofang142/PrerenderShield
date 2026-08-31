# 功能详细文档 — SEO/AEO 优化体系

> 本文为**内部实现**（结构/数据流/代码）。使用者请转 [AEO · AI 搜索引擎优化指南](aeo-guide.md) 与 [LLM SEO 优化器使用指南](seo-llm-guide.md)，配置键位见 [../CONFIG_REFERENCE.md](../CONFIG_REFERENCE.md)。

---

## 1. Meta 标签优化

### 结构

```
internal/seo/
├── meta_tags.go           # Meta 标签优化器
│   ├── MetaTagsOptimizer
│   ├── NewMetaTagsOptimizer(config)
│   ├── OptimizeMetaTags(html, keywords) → Result
│   ├── BuildOptimizedHTML(html, result) → string
│   └── GenerateKeywords(html) → []string
├── meta_tags_test.go
├── structured_data.go     # 结构化数据
├── structured_data_test.go
└── aeo.go                 # AEO AI 爬虫优化
```

### 数据流

```
渲染完成 → HTML
  → 提取现有 Title/Description
  → 分析缺失标签
  → 生成优化后的 Meta 标签
  │   ├── <title> 优化
  │   ├── <meta name="description">
  │   ├── <meta name="keywords">
  │   └── <meta name="robots">
  → 注入 Open Graph 标签
  │   ├── og:title
  │   ├── og:description
  │   ├── og:image
  │   └── og:url
  → 注入 Twitter Card 标签
  │   ├── twitter:card
  │   ├── twitter:title
  │   └── twitter:description
  → 注入 Canonical URL
  → 输出优化后的 HTML
```

### 配置

```go
MetaTagsConfig{
  TitleMinLength:       30,
  TitleMaxLength:       60,
  DescriptionMinLength: 120,
  DescriptionMaxLength: 160,
  MaxKeywords:          10,
  MinKeywordLength:     3,
  AutoGenerateKeywords: true,
  EnableOpenGraph:      true,
  EnableTwitterCard:    true,
}
```

---

## 2. 结构化数据 (Schema.org)

### 结构

```
internal/seo/structured_data.go
├── StructuredDataOptimizer
├── GenerateJSONLD(schema) → JSON string
├── BuildStructuredDataHTML(schema) → HTML string
├── InjectStructuredData(html, schema) → string
└── 支持类型:
    ├── Article
    ├── Product
    ├── Organization
    ├── LocalBusiness
    ├── FAQPage
    └── BreadcrumbList
```

### 数据流

```
渲染完成 → HTML
  → 构建 Schema 数据 (JSON-LD)
  │   ├── @context: https://schema.org
  │   ├── @type: WebPage / Article / Product 等
  │   ├── name / description / url
  │   └── 类型特定字段
  → 注入到 <head> 中 </head> 前
  → 输出 HTML
```

### 输出示例

```json
{
  "@context": "https://schema.org",
  "@type": "WebPage",
  "name": "页面标题",
  "description": "页面描述",
  "url": "https://example.com/page"
}
```

---

## 3. AEO — AI 爬虫引擎优化

### 结构

```
internal/seo/aeo.go
├── AEOConfig
├── IsAICrawler(ua) → *AICrawlerInfo
├── KnownAICrawlers  (10种)
└── ExtractAnswer(html) → text
```

### 数据流

```
请求 → AEO 检测
  ├── IsAICrawler(User-Agent)?
  │    ├── GPTBot → OpenAI (训练用途)
  │    ├── ClaudeBot → Anthropic (训练用途)
  │    ├── PerplexityBot → Perplexity (搜索)
  │    ├── Google-Extended → Google/Gemini
  │    ├── Cohere-AI → Cohere
  │    ├── FacebookBot → Meta
  │    ├── AppleBot → Apple
  │    └── Bytespider → ByteDance
  │
  ├── 是 AI 爬虫?
  │    ├── 提取纯净正文 (去除非内容元素)
  │    ├── 注入结构化数据 (JSON-LD)
  │    └── 返回处理过的 HTML
  │
  └── 不是 → 传统 SEO 处理
```

### AI 爬虫清单

| Bot Token | 公司 | 用途 | 处理策略 |
|-----------|------|------|---------|
| `gptbot` | OpenAI | 训练 | 纯净内容+结构化 |
| `claudebot` | Anthropic | 训练 | 同上 |
| `claude-web` | Anthropic | 搜索 | 同上 |
| `perplexitybot` | Perplexity | 搜索 | 同上 |
| `perplexity-user` | Perplexity | 搜索 | 同上 |
| `google-extended` | Google | 训练 | 同上 |
| `cohere-ai` | Cohere | 训练 | 同上 |
| `facebookbot` | Meta | 训练 | 同上 |
| `applebot` | Apple | 搜索 | 同上 |
| `bytespider` | ByteDance | 训练 | 同上 |

---

## 4. SEO 注入器 (集成到渲染流水线)

### 结构

```
internal/prerender/seo_injector.go
├── SEOInjector
│   ├── metaOpt: *MetaTagsOptimizer
│   └── structOpt: *StructuredDataOptimizer
├── NewSEOInjector(metaConfig, structConfig)
└── InjectSEOTags(html, url) → []byte
```

### 数据流

```
Chromium 渲染完成 → HTML
  → InjectSEOTags(html, url)
  │   ├── OptimizeMetaTags: 分析并优化 Meta
  │   ├── BuildOptimizedHTML: 注入 Meta/OG/Twitter
  │   ├── BuildStructuredDataHTML: 生成 JSON-LD
  │   ├── 注入 </head> 前
  │   └── 注入 Canonical URL
  → 缓存此 HTML
  → 返回给爬虫
```

### 业务流

```
渲染流水线:
  爬虫请求 → 缓存? → Chromium → SEO 注入 → 缓存 → 返回
                              ↑
                    这是新增的步骤
                    注入约 15-20 行 HTML 标签
                    耗时 < 5ms
                    零影响渲染速度
```
