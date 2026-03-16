package streaming

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FirstScreenRenderer 首屏优先渲染器
type FirstScreenRenderer struct {
	config   *FirstScreenConfig
	logger   *zap.Logger
	critical []string // 关键内容选择器
	mu       sync.RWMutex
}

// FirstScreenConfig 首屏配置
type FirstScreenConfig struct {
	ViewportHeight    int           `json:"viewport_height"`     // 视口高度
	ViewportWidth     int           `json:"viewport_width"`      // 视口宽度
	EnableLazyLoad    bool          `json:"enable_lazy_load"`    // 启用懒加载
	EnablePreload     bool          `json:"enable_preload"`      // 启用预加载
	CriticalCSSInline bool          `json:"critical_css_inline"` // 内联关键 CSS
	ImagePlaceholder  string        `json:"image_placeholder"`   // 图片占位符
	LazyLoadSelectors []string      `json:"lazy_load_selectors"` // 懒加载选择器
	PrioritySelectors []string      `json:"priority_selectors"`  // 优先加载选择器
	Timeout           time.Duration `json:"timeout"`             // 渲染超时
}

// DefaultFirstScreenConfig 返回默认配置
func DefaultFirstScreenConfig() *FirstScreenConfig {
	return &FirstScreenConfig{
		ViewportHeight:    800,
		ViewportWidth:     1280,
		EnableLazyLoad:    true,
		EnablePreload:     true,
		CriticalCSSInline: true,
		ImagePlaceholder:  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 800 600'%3E%3Crect fill='%23ddd' width='800' height='600'/%3E%3Ctext fill='%23999' x='50%25' y='50%25' text-anchor='middle' dy='.3em'%3ELoading...%3C/text%3E%3C/svg%3E",
		LazyLoadSelectors: []string{
			"img[loading='lazy']",
			"iframe[loading='lazy']",
			"picture",
			"video",
		},
		PrioritySelectors: []string{
			"header",
			"nav",
			".above-the-fold",
			".hero",
			"main > :first-child",
		},
		Timeout: 30 * time.Second,
	}
}

// NewFirstScreenRenderer 创建首屏渲染器
func NewFirstScreenRenderer(config *FirstScreenConfig, logger *zap.Logger) *FirstScreenRenderer {
	if config == nil {
		config = DefaultFirstScreenConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &FirstScreenRenderer{
		config:   config,
		logger:   logger,
		critical: make([]string, 0),
	}
}

// RenderWithPriority 带优先级渲染
func (r *FirstScreenRenderer) RenderWithPriority(
	ctx context.Context,
	html string,
	writer FlushWriter,
) error {
	startTime := time.Now()

	// 1. 解析 HTML 结构
	doc := r.parseHTML(html)

	// 2. 提取首屏内容
	firstScreen := r.extractFirstScreen(doc)

	// 3. 立即发送首屏
	if err := r.sendFirstScreen(ctx, firstScreen, writer); err != nil {
		return err
	}

	// 4. 发送剩余内容
	remaining := r.extractRemaining(doc)
	if err := r.sendRemaining(ctx, remaining, writer); err != nil {
		return err
	}

	r.logger.Debug("首屏优先渲染完成",
		zap.Int("original_length", len(html)),
		zap.Int64("duration_ms", time.Since(startTime).Milliseconds()),
	)

	return nil
}

// OptimizeForFirstScreen 优化 HTML 以获得首屏优先加载
func (r *FirstScreenRenderer) OptimizeForFirstScreen(html string) string {
	// 1. 内联关键 CSS
	if r.config.CriticalCSSInline {
		html = r.inlineCriticalCSS(html)
	}

	// 2. 添加资源预加载
	if r.config.EnablePreload {
		html = r.addPreloads(html)
	}

	// 3. 添加懒加载
	if r.config.EnableLazyLoad {
		html = r.addLazyLoading(html)
	}

	// 4. 添加首屏检测脚本
	html = r.addFirstScreenDetection(html)

	return html
}

// extractFirstScreen 提取首屏内容
func (r *FirstScreenRenderer) extractFirstScreen(doc *htmlDocument) *htmlDocument {
	firstScreen := &htmlDocument{
		Head: r.extractHead(doc.Head),
		Body: r.extractBodyFirstScreen(doc.Body),
	}
	return firstScreen
}

// extractHead 提取头部
func (r *FirstScreenRenderer) extractHead(head *htmlHead) *htmlHead {
	if head == nil {
		return &htmlHead{}
	}

	stylesLen := min(len(head.Styles), 3)
	styles := make([]string, stylesLen)
	copy(styles, head.Styles[:stylesLen])

	return &htmlHead{
		Title:   head.Title,
		Metas:   head.Metas,
		Styles:  styles, // 只保留前 3 个样式
		Scripts: r.filterCriticalScripts(head.Scripts),
	}
}

// extractBodyFirstScreen 提取首屏 body 内容
func (r *FirstScreenRenderer) extractBodyFirstScreen(body *htmlBody) *htmlBody {
	if body == nil || body.Content == nil {
		return &htmlBody{Content: make([]string, 0)}
	}

	end := min(len(body.Content), 10)
	content := make([]string, end)
	copy(content, body.Content[:end])
	return &htmlBody{
		Content: content,
	}
}

// extractRemaining 提取剩余内容
func (r *FirstScreenRenderer) extractRemaining(doc *htmlDocument) *htmlDocument {
	remaining := make([]string, 0)
	if len(doc.Body.Content) > 10 {
		remaining = doc.Body.Content[10:]
	}
	return &htmlDocument{
		Head: &htmlHead{}, // 头部已发送
		Body: &htmlBody{
			Content: remaining,
		},
	}
}

// sendFirstScreen 发送首屏
func (r *FirstScreenRenderer) sendFirstScreen(
	ctx context.Context,
	doc *htmlDocument,
	writer FlushWriter,
) error {
	if doc == nil {
		return fmt.Errorf("doc 不能为 nil")
	}

	// 发送 DOCTYPE
	if _, err := writer.Write([]byte("<!DOCTYPE html>")); err != nil {
		return err
	}

	// 发送 html 标签
	if _, err := writer.Write([]byte("<html>")); err != nil {
		return err
	}

	// 发送 head（如果存在）
	if doc.Head != nil {
		if err := r.sendHead(doc.Head, writer); err != nil {
			return err
		}
	}

	// 发送 body 开始
	if _, err := writer.Write([]byte("<body>")); err != nil {
		return err
	}

	// 发送首屏内容（如果 Body 存在）
	if doc.Body != nil && doc.Body.Content != nil {
		for _, content := range doc.Body.Content {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if _, err := writer.Write([]byte(content)); err != nil {
					return err
				}
			}
		}
	}

	// 发送 body 占位符
	if _, err := writer.Write([]byte("<!-- remaining content loading -->")); err != nil {
		return err
	}

	return writer.Flush()
}

// sendRemaining 发送剩余内容
func (r *FirstScreenRenderer) sendRemaining(
	ctx context.Context,
	doc *htmlDocument,
	writer FlushWriter,
) error {
	// 发送剩余内容
	for _, content := range doc.Body.Content {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if _, err := writer.Write([]byte(content)); err != nil {
				return err
			}
			if err := writer.Flush(); err != nil {
				return err
			}
		}
	}

	// 发送关闭标签
	if _, err := writer.Write([]byte("</body></html>")); err != nil {
		return err
	}

	return writer.Flush()
}

// sendHead 发送头部
func (r *FirstScreenRenderer) sendHead(head *htmlHead, writer FlushWriter) error {
	if _, err := writer.Write([]byte("<head>")); err != nil {
		return fmt.Errorf("写入 head 标签失败：%w", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush 失败：%w", err)
	}

	// 发送 meta
	for _, meta := range head.Metas {
		if _, err := writer.Write([]byte(meta)); err != nil {
			return fmt.Errorf("写入 meta 标签失败：%w", err)
		}
	}

	// 发送 title
	if head.Title != "" {
		if _, err := writer.Write([]byte("<title>" + html.EscapeString(head.Title) + "</title>")); err != nil {
			return fmt.Errorf("写入 title 标签失败：%w", err)
		}
	}

	// 发送关键样式
	for _, style := range head.Styles {
		if _, err := writer.Write([]byte(style)); err != nil {
			return fmt.Errorf("写入 style 标签失败：%w", err)
		}
	}

	// 发送关键脚本
	for _, script := range head.Scripts {
		if _, err := writer.Write([]byte(script)); err != nil {
			return fmt.Errorf("写入 script 标签失败：%w", err)
		}
	}

	if _, err := writer.Write([]byte("</head>")); err != nil {
		return fmt.Errorf("写入 head 闭合标签失败：%w", err)
	}

	return writer.Flush()
}

// inlineCriticalCSS 内联关键 CSS
func (r *FirstScreenRenderer) inlineCriticalCSS(html string) string {
	// 查找所有 link[rel=stylesheet] 标签
	cssLinkPattern := regexp.MustCompile(`<link[^>]+rel=["']stylesheet["'][^>]+href=["']([^"']+)["'][^>]*>`)

	matches := cssLinkPattern.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return html
	}

	// 提取第一个 CSS（假设是关键 CSS）
	if len(matches) > 0 {
		cssURL := matches[0][1]
		// 这里应该获取并内联 CSS，简化处理只添加注释
		inlineComment := fmt.Sprintf("<!-- Critical CSS should be inlined from: %s -->", cssURL)
		html = strings.Replace(html, matches[0][0], inlineComment, 1)
	}

	return html
}

// addPreloads 添加预加载
func (r *FirstScreenRenderer) addPreloads(html string) string {
	// 查找图片
	imgPattern := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["'][^>]*>`)

	preloads := ""
	matches := imgPattern.FindAllStringSubmatch(html, 3) // 只预加载前 3 个图片

	for _, match := range matches {
		imgURL := match[1]
		preloads += fmt.Sprintf(`<link rel="preload" as="image" href="%s">`, imgURL)
	}

	// 插入到 head 开始处
	html = strings.Replace(html, "<head>", "<head>"+preloads, 1)

	return html
}

// addLazyLoading 添加懒加载
func (r *FirstScreenRenderer) addLazyLoading(html string) string {
	// 为所有没有 loading 属性的图片添加 loading="lazy"
	imgPattern := regexp.MustCompile(`<img([^>]*)(src=["'][^"']+["'])([^>]*)>`)
	html = imgPattern.ReplaceAllStringFunc(html, func(match string) string {
		if strings.Contains(match, `loading=`) {
			return match
		}
		return strings.Replace(match, ">", ` loading="lazy">`, 1)
	})

	return html
}

// addFirstScreenDetection 添加首屏检测脚本
func (r *FirstScreenRenderer) addFirstScreenDetection(html string) string {
	script := `
<script>
(function() {
	// 检测首屏加载完成
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', function() {
			window.dispatchEvent(new CustomEvent('firstScreenLoaded'));
		});
	} else {
		window.dispatchEvent(new CustomEvent('firstScreenLoaded'));
	}

	// 懒加载观察器
	if ('IntersectionObserver' in window) {
		const observer = new IntersectionObserver((entries) => {
			entries.forEach(entry => {
				if (entry.isIntersecting) {
					const img = entry.target;
					if (img.dataset.src) {
						img.src = img.dataset.src;
						img.removeAttribute('data-src');
					}
					observer.unobserve(img);
				}
			});
		});

		document.querySelectorAll('img[loading="lazy"]').forEach(img => {
			observer.observe(img);
		});
	}
})();
</script>
`
	// 插入到 body 结束前
	html = strings.Replace(html, "</body>", script+"</body>", 1)

	return html
}

// filterCriticalScripts 过滤关键脚本
func (r *FirstScreenRenderer) filterCriticalScripts(scripts []string) []string {
	critical := make([]string, 0)
	for _, script := range scripts {
		if strings.Contains(script, "async") || strings.Contains(script, "defer") {
			continue // 跳过异步/延迟脚本
		}
		if len(critical) < 2 {
			critical = append(critical, script)
		}
	}
	return critical
}

// htmlDocument HTML 文档结构
type htmlDocument struct {
	Head *htmlHead
	Body *htmlBody
}

// htmlHead HTML 头部
type htmlHead struct {
	Title   string
	Metas   []string
	Styles  []string
	Scripts []string
}

// htmlBody HTML 身体
type htmlBody struct {
	Content []string
}

// parseHTML 解析 HTML
func (r *FirstScreenRenderer) parseHTML(html string) *htmlDocument {
	doc := &htmlDocument{
		Head: &htmlHead{
			Metas:   make([]string, 0),
			Styles:  make([]string, 0),
			Scripts: make([]string, 0),
		},
		Body: &htmlBody{
			Content: make([]string, 0),
		},
	}

	// 简化解析：提取 head 和 body
	// 使用 (?s) 让 . 匹配换行符
	headMatch := regexp.MustCompile(`(?s)<head[^>]*>(?P<content>.*?)</head>`).FindStringSubmatch(html)
	if len(headMatch) > 1 {
		r.parseHead(headMatch[1], doc.Head)
	}

	bodyMatch := regexp.MustCompile(`(?s)<body[^>]*>(?P<content>.*?)</body>`).FindStringSubmatch(html)
	if len(bodyMatch) > 1 {
		r.parseBody(bodyMatch[1], doc.Body)
	}

	return doc
}

// parseHead 解析头部
func (r *FirstScreenRenderer) parseHead(headContent string, head *htmlHead) {
	// 提取 title
	titlePattern := regexp.MustCompile(`(?s)<title[^>]*>(.*?)</title>`)
	titleMatch := titlePattern.FindStringSubmatch(headContent)
	if len(titleMatch) > 1 {
		head.Title = strings.TrimSpace(titleMatch[1])
	}

	// 提取 meta
	metaPattern := regexp.MustCompile(`<meta[^>]+>`)
	head.Metas = metaPattern.FindAllString(headContent, -1)

	// 提取 style
	stylePattern := regexp.MustCompile(`(?s)<style[^>]*>(.*?)</style>`)
	head.Styles = stylePattern.FindAllString(headContent, -1)

	// 提取 script
	scriptPattern := regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`)
	head.Scripts = scriptPattern.FindAllString(headContent, -1)
}

// parseBody 解析身体
func (r *FirstScreenRenderer) parseBody(bodyContent string, body *htmlBody) {
	// 简化：按主要块分割
	// 先清理空白
	content := strings.TrimSpace(bodyContent)
	if content != "" {
		body.Content = append(body.Content, content)
	}
}

// min 返回较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PriorityContent 优先级内容
type PriorityContent struct {
	Content  string
	Priority int // 0 = 最高，10 = 最低
	Type     string
}

// LazyLoadManager 懒加载管理器
type LazyLoadManager struct {
	config   *FirstScreenConfig
	elements []*LazyElement
	mu       sync.RWMutex
}

// LazyElement 懒加载元素
type LazyElement struct {
	Tag         string
	ID          string
	Class       string
	Original    string
	Placeholder string
	Loaded      bool
}

// NewLazyLoadManager 创建懒加载管理器
func NewLazyLoadManager(config *FirstScreenConfig) *LazyLoadManager {
	return &LazyLoadManager{
		config:   config,
		elements: make([]*LazyElement, 0),
	}
}

// AddElement 添加懒加载元素
func (m *LazyLoadManager) AddElement(tag, id, class, original string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	element := &LazyElement{
		Tag:         tag,
		ID:          id,
		Class:       class,
		Original:    original,
		Placeholder: m.config.ImagePlaceholder,
		Loaded:      false,
	}

	m.elements = append(m.elements, element)
}

// GetPlaceholderHTML 获取占位符 HTML
func (m *LazyLoadManager) GetPlaceholderHTML(element *LazyElement) string {
	return fmt.Sprintf(`<div class="lazy-placeholder" style="min-height:200px;background:#f0f0f0;">Loading...</div>`)
}

// GetElements 获取所有元素
func (m *LazyLoadManager) GetElements() []*LazyElement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LazyElement, len(m.elements))
	copy(result, m.elements)
	return result
}

// ContentSplitter 内容分割器
type ContentSplitter struct {
	chunkSize int
}

// NewContentSplitter 创建内容分割器
func NewContentSplitter(chunkSize int) *ContentSplitter {
	return &ContentSplitter{
		chunkSize: chunkSize,
	}
}

// Split 分割内容
func (s *ContentSplitter) Split(content string) []string {
	chunks := make([]string, 0)

	for i := 0; i < len(content); i += s.chunkSize {
		end := i + s.chunkSize
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, content[i:end])
	}

	return chunks
}

// SplitByElement 按元素分割
func (s *ContentSplitter) SplitByElement(html string) []string {
	// 按 HTML 标签分割
	tagPattern := regexp.MustCompile(`<[^>]+>[^<]*</[^>]+>|<[^>]+/>`)
	matches := tagPattern.FindAllString(html, -1)

	if len(matches) == 0 {
		return []string{html}
	}

	return matches
}
