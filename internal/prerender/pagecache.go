package prerender

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// PageEnvelope 渲染缓存信封：除 HTML 外携带真实状态码与业务过期时间，
// 使 SPA 软 404 / 临时 5xx 不再以 200 形态读回污染收录。
// V=1 兼容读法：历史裸 HTML 字节无法解出信封时按 Status=200 处理。
type PageEnvelope struct {
	Status    int    `json:"s"`
	HTML      string `json:"h"`
	ExpiresAt int64  `json:"e"`
	CreatedAt int64  `json:"c"` // 渲染完成时间（Unix 秒）；存量信封缺省为 0（Last-Modified/304 链路自动降级）
	V         int    `json:"v"`
}

const pageEnvelopeVersion = 1

func marshalPageEnvelope(env PageEnvelope) []byte {
	env.V = pageEnvelopeVersion
	b, _ := json.Marshal(env)
	return b
}

// unmarshalPageEnvelope 解析信封；legacy 裸 HTML（非 JSON 信封）回退为 200。
func unmarshalPageEnvelope(raw []byte) (PageEnvelope, bool) {
	var env PageEnvelope
	s := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(s, "{") {
		return PageEnvelope{Status: 200, HTML: string(raw), V: 0}, true
	}
	if err := json.Unmarshal([]byte(s), &env); err != nil || env.HTML == "" && env.Status == 0 {
		return PageEnvelope{Status: 200, HTML: s, V: 0}, true
	}
	if env.Status == 0 {
		env.Status = 200
	}
	return env, true
}

// Fresh 判断信封是否仍在业务有效期内（软过期供数的判定依据）
func (e PageEnvelope) Fresh(now time.Time) bool {
	return e.ExpiresAt == 0 || now.Unix() < e.ExpiresAt
}

// staleTTL 计算软过期存储时长：业务 TTL 之后额外保留一段窗口用于降级供数，
// 窗口 = max(12h, 业务TTL)，总时长封顶 30 天防止体积膨胀。
func staleRetention(dataTTL time.Duration) time.Duration {
	if dataTTL <= 0 {
		dataTTL = 24 * time.Hour
	}
	extra := dataTTL
	if extra < 12*time.Hour {
		extra = 12 * time.Hour
	}
	total := dataTTL + extra
	if total > 30*24*time.Hour {
		total = 30 * 24 * time.Hour
	}
	return total
}

var (
	// Rendertron 兼容的页面自声明状态码：
	reMetaStatusNameFirst = regexp.MustCompile(`(?i)<meta[^>]+name=["']render:status_code["'][^>]*content=["'](\d{3})["']`)
	reMetaStatusContFirst = regexp.MustCompile(`(?i)<meta[^>]+content=["'](\d{3})["'][^>]*name=["']render:status_code["']`)
	reStripTags           = regexp.MustCompile(`(?s)<[^>]*>`)
	reWhitespaceRun       = regexp.MustCompile(`\s+`)
	// 空壳特征：根挂载节点仍为空模板（JS 崩溃/超时产出的 thin content）
	reEmptyMount = regexp.MustCompile(`(?i)<(?:div|main|app)[^>]*(?:id|class)=["'](root|app|__next|#app)["'][^>]*>\s*</`)
)

// ExtractDeclaredStatus 从渲染产物中提取 render:status_code 自声明状态码；无声明返回 0。
func extractDeclaredStatus(html string) int {
	for _, re := range []*regexp.Regexp{reMetaStatusNameFirst, reMetaStatusContFirst} {
		if m := re.FindStringSubmatch(html); m != nil {
			switch m[1] {
			case "301", "302", "303", "307", "308", "400", "401", "403", "404", "410", "500", "503":
				return mustAtoi(m[1])
			}
		}
	}
	return 0
}

// visibleTextLength 估算可见文本长度（粗粒度去标签后压空白），用于空壳页质检。
func visibleTextLength(html string) int {
	t := reStripTags.ReplaceAllString(html, " ")
	t = reWhitespaceRun.ReplaceAllString(t, " ")
	return len([]rune(strings.TrimSpace(t)))
}

// isEmptyShell 判定渲染产物是否为 JS 未执行成功的空壳：
// 可见文本 < emptyShellTextThreshold 或命中典型空挂载点模板。
const emptyShellTextThreshold = 40

func isEmptyShell(html string) bool {
	if visibleTextLength(html) < emptyShellTextThreshold {
		return true
	}
	return reEmptyMount.MatchString(html)
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// noindexHeadFragment 注入到 <head> 的禁止收录声明
const noindexMetaTag = `<meta name="robots" content="noindex">`

var reHeadOpen = regexp.MustCompile(`(?i)<head[^>]*>`)

// injectNoindexMeta 将 noindex 声明插入 <head> 开头；无 head 标签时前置到文档首。
func injectNoindexMeta(html string) string {
	if reHeadOpen.MatchString(html) {
		return reHeadOpen.ReplaceAllStringFunc(html, func(m string) string {
			return m + noindexMetaTag
		})
	}
	return noindexMetaTag + html
}
