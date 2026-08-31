# 功能文档索引

`docs/features/` 下分为两类文档：

- **使用者指南（how-to）**：讲怎么开、怎么配、怎么验证。适合用户/运维。
- **内部实现（internals）**：讲代码结构、数据流。适合开发者/二次开发。

## 使用者指南

| 文档 | 主题 |
|------|------|
| [advanced-waf-guide.md](advanced-waf-guide.md) | CC 攻击防护、威胁情报订阅、爬虫真实性验证、GeoIP 兜底链的使用 |
| [acme-ssl-guide.md](acme-ssl-guide.md) | Let's Encrypt 证书：HTTP-01 / DNS-01 通配符 / 手动导入、自动续期 |
| [seo-llm-guide.md](seo-llm-guide.md) | LLM SEO 优化器：接 OpenAI/智谱/DeepSeek/Ollama，优化标题/描述/关键词/结构化数据 |
| [aeo-guide.md](aeo-guide.md) | AEO（AI 搜索引擎优化）：识别 AI 爬虫、供给纯净答案、category_policy 策略 |

## 内部实现

| 文档 | 主题 |
|------|------|
| [prerender-engine.md](prerender-engine.md) | 预渲染引擎架构与数据流 |
| [waf-firewall.md](waf-firewall.md) | WAF 检测器分层、规则引擎、动作处理 |
| [seo-aeo.md](seo-aeo.md) | SEO/AEO 优化器内部结构（Meta/结构化数据/AI 爬虫） |

> 更全面的配置键位与字段级说明见 [CONFIG_REFERENCE.md](../CONFIG_REFERENCE.md)。
