# LLM SEO 优化器使用指南（`seo.llm`）

> 实验性功能，默认关闭。用大模型改进页面 SEO 文案，减轻人工编写成本。
> 本文如实区分**当前已在渲染链路生效**、与**已实现但未接入请求链路**的能力（以便你不被误导）。
> 代码见 `internal/seo/llm_optimizer.go` 与注入点 `internal/prerender/seo_injector.go`。

---

## ⚠️ 先看：当前真正生效的只有「标题优化」

LLM 优化器实现了四类能力，但**渲染链路（`SEOInjector.InjectSEOTags`）当前只调用 `OptimizeTitle`**：
启用 `seo.llm` 后，LLM 只把 `<title>` 优化掉；其余三类是已实现、可给二次开发调用，但暂未接入。

| 能力 | 函数 | 链路状态 |
|------|------|---------|
| 标题优化 | `OptimizeTitle` | ✅ 已接入（`seo_injector.go:36`） |
| 描述优化 | `OptimizeDescription` | 🔬 已实现，未接入 |
| 关键词提取 | `ExtractKeywords` | 🔬 已实现，未接入 |
| 结构化数据 | `GenerateStructuredData` | 🔬 已实现，未接入（另有逻辑内置 WebPage JSON-LD，见下） |

> 除 LLM 外，注入器还**总有**规则式元数据优化：补全 `<title>`/`<meta name="description">`/Open Graph/Twitter Card、注入 `WebPage` JSON-LD 与 `<link rel="canonical">`。这些**不依赖** `seo.llm`，启用渲染即可。

---

## 接入各家模型

使用 **OpenAI 兼容的 `/v1/chat/completions`** 协议。`provider` 只是预设，实际端点由 `api_url` 决定。

| provider | 默认 `api_url` | 必填项 |
|---------|---------------|--------|
| `openai` | `https://api.openai.com/v1/chat/completions` | `api_key` |
| `zhipu` | 需手动填 | `api_url` + `api_key` |
| `deepseek` | 需手动填 | `api_url` + `api_key` |
| `ollama` | 需手动填（如 `http://localhost:11434/v1/chat/completions`） | `api_url`（本地可无 key） |

> `zhipu`/`deepseek`/`ollama` **必须**设置 `api_url`，否则请求打到默认 openai 地址必然失败。

### 配置示例（以 DeepSeek 为例）

```yaml
seo:
  llm:
    enabled: true
    provider: "deepseek"
    api_url: "https://api.deepseek.com/v1/chat/completions"
    api_key: "sk-xxxx"
    model: "deepseek-chat"
    max_tokens: 500
    temperature: 0.3
    timeout: "10s"
    max_retries: 2
```

### Ollama 本地示例（零外部费用）

```yaml
seo:
  llm:
    enabled: true
    provider: "ollama"
    api_url: "http://localhost:11434/v1/chat/completions"
    api_key: ""
    model: "llama3.1:8b"
```

---

## 配置键

| 键 | 默认 | 说明 |
|----|------|------|
| `enabled` | `false` | 总开关（决定是否创建含 LLM 的注入器） |
| `provider` | `openai` | `openai` / `zhipu` / `deepseek` / `ollama` |
| `api_key` | `""` | API 密钥（本地 Ollama 可留空） |
| `api_url` | `""`（openai 有默认） | 非 openai 提供商必填 |
| `model` | `gpt-4o-mini` | 模型名 |
| `max_tokens` | `500` | 单次输出上限 |
| `temperature` | `0.3` | 创造性；越低越稳定 |
| `timeout` | `10s` | 单次请求超时 |
| `max_retries` | `2` | 失败重试（线性退避） |
| `prompts.*` | 内置默认 | 四类任务提示词模板，`%s` 为占位符（`title_optimization` / `description_optimization` / `keyword_extraction` / `structured_data`） |

---

## 验证

1. **连通性**：启动后看日志有无 `LLM title optimization failed`——有则为 api_url/api_key/网络问题。
2. **标题变化**：`curl -A "Googlebot" http://你的域名/`，对比 `<title>` 是否被改写为 LLM 生成的新标题（长度 30–60 字、含关键词）。
3. **规则式元数据**：确认 `<meta name="description">`、`og:*`、`twitter:*`、`<link rel="canonical">` 及 `WebPage` JSON-LD 已注入（这些无需 LLM）。

---

## 注意点

- **成本**：`OptimizeTitle` 每个渲染页面调用一次 LLM；高流量留意额度与延迟。
- **失败不阻断**：LLM 异常会回退原始 `<title>`，不影响页面。
- **只优化标题**：若你希望描述/关键词/结构化数据也走 LLM，当前需在 `seo_injector.go` 中补调用 `OptimizeDescription` / `ExtractKeywords` / `GenerateStructuredData`（或在控制台/API 侧自行调用对应函数）。
- **可能覆盖原始标签**：生效后会替换页面 `<title>`，务必先在测试站点验证输出质量。
