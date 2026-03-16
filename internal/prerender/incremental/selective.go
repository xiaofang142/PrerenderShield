package incremental

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SelectiveRenderer 选择性渲染器
type SelectiveRenderer struct {
	config   *SelectiveRenderConfig
	logger   *zap.Logger
	queue    *PriorityQueue
	detector *RegionDetector
	mu       sync.RWMutex
}

// SelectiveRenderConfig 选择性渲染配置
type SelectiveRenderConfig struct {
	EnablePriorityQueue  bool          `json:"enable_priority_queue"`  // 启用优先级队列
	EnableLazyRender     bool          `json:"enable_lazy_render"`     // 启用懒渲染
	MaxConcurrentRenders int           `json:"max_concurrent_renders"` // 最大并发渲染数
	BatchInterval        time.Duration `json:"batch_interval"`         // 批量渲染间隔
	ViewportHeight       int           `json:"viewport_height"`        // 视口高度
	ViewportWidth        int           `json:"viewport_width"`         // 视口宽度
	PrioritySelectors    []string      `json:"priority_selectors"`     // 高优先级选择器
	LazySelectors        []string      `json:"lazy_selectors"`         // 懒加载选择器
}

// DefaultSelectiveRenderConfig 返回默认配置
func DefaultSelectiveRenderConfig() *SelectiveRenderConfig {
	return &SelectiveRenderConfig{
		EnablePriorityQueue:  true,
		EnableLazyRender:     true,
		MaxConcurrentRenders: 4,
		BatchInterval:        50 * time.Millisecond,
		ViewportHeight:       800,
		ViewportWidth:        1280,
		PrioritySelectors: []string{
			"header",
			"nav",
			".hero",
			".above-the-fold",
			"main > :first-child",
			"[data-priority]",
		},
		LazySelectors: []string{
			"img[loading='lazy']",
			"iframe[loading='lazy']",
			"picture",
			"video",
			".below-the-fold",
			"footer",
		},
	}
}

// NewSelectiveRenderer 创建选择性渲染器
func NewSelectiveRenderer(config *SelectiveRenderConfig, logger *zap.Logger) *SelectiveRenderer {
	if config == nil {
		config = DefaultSelectiveRenderConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SelectiveRenderer{
		config:   config,
		logger:   logger,
		queue:    NewPriorityQueue(100),
		detector: NewRegionDetector(config),
	}
}

// RenderRegion 渲染区域
type RenderRegion struct {
	ID         string            `json:"id"`
	Path       string            `json:"path"`
	Tag        string            `json:"tag"`
	Priority   int               `json:"priority"` // 0-10，0 最高
	IsVisible  bool              `json:"is_visible"`
	IsCritical bool              `json:"is_critical"`
	Content    string            `json:"content"`
	Attributes map[string]string `json:"attributes"`
	Children   []*RenderRegion   `json:"children"`
}

// RegionDetector 区域检测器
type RegionDetector struct {
	config          *SelectiveRenderConfig
	priorityRegexes []*regexp.Regexp
	lazyRegexes     []*regexp.Regexp
	mu              sync.RWMutex
}

// NewRegionDetector 创建区域检测器
func NewRegionDetector(config *SelectiveRenderConfig) *RegionDetector {
	if config == nil {
		config = DefaultSelectiveRenderConfig()
	}

	detector := &RegionDetector{
		config:          config,
		priorityRegexes: make([]*regexp.Regexp, 0),
		lazyRegexes:     make([]*regexp.Regexp, 0),
	}

	// 编译优先级选择器正则
	for _, selector := range config.PrioritySelectors {
		if regex := detector.selectorToRegex(selector); regex != nil {
			detector.priorityRegexes = append(detector.priorityRegexes, regex)
		}
	}

	// 编译懒加载选择器正则
	for _, selector := range config.LazySelectors {
		if regex := detector.selectorToRegex(selector); regex != nil {
			detector.lazyRegexes = append(detector.lazyRegexes, regex)
		}
	}

	return detector
}

// selectorToRegex 将 CSS 选择器转换为正则（简化实现）
func (d *RegionDetector) selectorToRegex(selector string) *regexp.Regexp {
	// 处理标签选择器
	if !strings.Contains(selector, "[") && !strings.Contains(selector, ".") && !strings.Contains(selector, "#") {
		return regexp.MustCompile(`<` + selector + `[^>]*>`)
	}

	// 处理 class 选择器
	if strings.HasPrefix(selector, ".") {
		className := strings.TrimPrefix(selector, ".")
		return regexp.MustCompile(`<[^>]*class=["'][^"']*` + regexp.QuoteMeta(className) + `[^"']*["'][^>]*>`)
	}

	// 处理属性选择器
	if strings.Contains(selector, "[") {
		parts := strings.Split(selector, "[")
		tag := parts[0]
		attr := strings.TrimSuffix(parts[1], "]")
		// 处理属性值中的引号，支持单引号和双引号
		// 将 loading='lazy' 转换为 loading=["']lazy["']
		attrPattern := regexp.MustCompile(`(\w+)=['"]([^'"]+)['"]`)
		matches := attrPattern.FindStringSubmatch(attr)
		if len(matches) == 3 {
			attrKey := matches[1]
			attrValue := matches[2]
			return regexp.MustCompile(`<` + tag + `[^>]*` + regexp.QuoteMeta(attrKey) + `=["'][^"']*` + regexp.QuoteMeta(attrValue) + `[^"']*["'][^>]*>`)
		}
		// 没有值，只匹配属性名
		return regexp.MustCompile(`<` + tag + `[^>]*` + regexp.QuoteMeta(attr) + `[^>]*>`)
	}

	return nil
}

// DetectRegions 检测渲染区域
func (d *RegionDetector) DetectRegions(html string) []*RenderRegion {
	d.mu.RLock()
	defer d.mu.RUnlock()

	regions := make([]*RenderRegion, 0)

	// 简化实现：使用 strings.Contains 查找常见标签
	// 实际项目应该使用 HTML 解析器如 golang.org/x/net/html

	// 定义要检测的标签列表
	blockTags := []string{"header", "nav", "main", "section", "article", "aside", "footer", "div", "p", "h1", "h2", "h3", "h4", "h5", "h6"}
	selfClosingTags := []string{"img", "br", "hr", "input", "meta", "link"}

	// 匹配块级元素 <tag ...>content</tag>
	for _, tag := range blockTags {
		startTag := "<" + tag
		endTag := "</" + tag + ">"

		// 查找所有该标签的出现
		startIdx := 0
		for {
			pos := strings.Index(html[startIdx:], startTag)
			if pos == -1 {
				break
			}
			pos += startIdx

			// 找到结束标签（相对于 pos 的位置）
			endPos := strings.Index(html[pos:], endTag)
			if endPos == -1 {
				break
			}

			// 提取完整标签内容（endPos 已经是相对于 pos 的偏移）
			fullMatch := html[pos : pos+endPos+len(endTag)]

			// 提取属性部分
			attrEnd := strings.Index(fullMatch, ">")
			if attrEnd == -1 {
				startIdx = pos + 1
				continue
			}

			attrString := fullMatch[1:attrEnd] // 去掉开头的 <

			// 提取内容部分
			contentStart := attrEnd + 1
			contentEnd := len(fullMatch) - len(endTag)
			content := ""
			if contentStart < contentEnd {
				content = strings.TrimSpace(fullMatch[contentStart:contentEnd])
			}

			attrs := d.parseAttributes(attrString)

			region := &RenderRegion{
				ID:         attrs["id"],
				Tag:        tag,
				Attributes: attrs,
				Content:    content,
				Priority:   d.calculatePriority(tag, attrs),
				IsVisible:  d.isVisible(tag, attrs),
				IsCritical: d.isCritical(tag, attrs),
			}

			regions = append(regions, region)
			startIdx = pos + len(fullMatch)
		}
	}

	// 匹配自闭合标签
	for _, tag := range selfClosingTags {
		startTag := "<" + tag
		selfClose := "/>"

		startIdx := 0
		for {
			pos := strings.Index(html[startIdx:], startTag)
			if pos == -1 {
				break
			}
			pos += startIdx

			// 检查是否自闭合
			rest := html[pos:]
			closePos := strings.Index(rest, selfClose)
			if closePos == -1 {
				startIdx = pos + 1
				continue
			}

			// 提取完整标签
			fullMatch := rest[:closePos+2]

			// 提取属性
			attrStart := strings.Index(fullMatch, " ")
			attrString := ""
			if attrStart != -1 {
				attrString = fullMatch[attrStart : len(fullMatch)-2] // 去掉 />
			}

			attrs := d.parseAttributes(attrString)

			region := &RenderRegion{
				ID:         attrs["id"],
				Tag:        tag,
				Attributes: attrs,
				Content:    "",
				Priority:   d.calculatePriority(tag, attrs),
				IsVisible:  d.isVisible(tag, attrs),
				IsCritical: d.isCritical(tag, attrs),
			}

			regions = append(regions, region)
			startIdx = pos + len(fullMatch)
		}
	}

	return regions
}

// parseAttributes 解析属性
func (d *RegionDetector) parseAttributes(attrString string) map[string]string {
	attrs := make(map[string]string)

	// 提取 id
	idPattern := regexp.MustCompile(`id=["']([^"']+)["']`)
	if match := idPattern.FindStringSubmatch(attrString); len(match) > 1 {
		attrs["id"] = match[1]
	}

	// 提取 class
	classPattern := regexp.MustCompile(`class=["']([^"']+)["']`)
	if match := classPattern.FindStringSubmatch(attrString); len(match) > 1 {
		attrs["class"] = match[1]
	}

	// 提取 data-* 属性
	dataPattern := regexp.MustCompile(`data-([^="']+)=["']([^"']+)["']`)
	dataMatches := dataPattern.FindAllStringSubmatch(attrString, -1)
	for _, m := range dataMatches {
		attrs["data-"+m[1]] = m[2]
	}

	return attrs
}

// calculatePriority 计算优先级
func (d *RegionDetector) calculatePriority(tag string, attrs map[string]string) int {
	priority := 5 // 默认中等优先级

	// 检查是否在优先级选择器中
	for _, regex := range d.priorityRegexes {
		if regex.MatchString("<" + tag + " " + d.attrsToString(attrs) + ">") {
			return 0 // 最高优先级
		}
	}

	// 检查是否在懒加载选择器中
	for _, regex := range d.lazyRegexes {
		if regex.MatchString("<" + tag + " " + d.attrsToString(attrs) + ">") {
			return 10 // 最低优先级
		}
	}

	// 根据标签类型调整优先级
	switch tag {
	case "header", "nav", "main":
		priority = 1
	case "section", "article":
		priority = 3
	case "footer":
		priority = 8
	case "img", "video":
		priority = 7
	}

	// 根据 data-priority 属性调整
	if priorityStr, ok := attrs["data-priority"]; ok {
		if p := stringToInt(priorityStr); p >= 0 && p <= 10 {
			priority = p
		}
	}

	return priority
}

// attrsToString 属性转字符串
func (d *RegionDetector) attrsToString(attrs map[string]string) string {
	parts := make([]string, 0, len(attrs))
	for k, v := range attrs {
		parts = append(parts, k+"=\""+v+"\"")
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// isCritical 判断是否关键区域
func (d *RegionDetector) isCritical(tag string, attrs map[string]string) bool {
	// 检查 class 是否包含关键标识
	if class, ok := attrs["class"]; ok {
		if strings.Contains(class, "critical") || strings.Contains(class, "above-the-fold") {
			return true
		}
	}

	// 检查 data-critical 属性
	if _, ok := attrs["data-critical"]; ok {
		return true
	}

	// 检查标签类型
	switch tag {
	case "header", "nav":
		return true
	}

	return false
}

// isVisible 判断是否可见区域
func (d *RegionDetector) isVisible(tag string, attrs map[string]string) bool {
	// 检查 style 属性是否隐藏
	if style, ok := attrs["style"]; ok {
		if strings.Contains(style, "display:none") || strings.Contains(style, "visibility:hidden") {
			return false
		}
	}

	// 检查 hidden 属性
	if _, ok := attrs["hidden"]; ok {
		return false
	}

	// 检查 aria-hidden
	if ariaHidden, ok := attrs["aria-hidden"]; ok {
		if ariaHidden == "true" {
			return false
		}
	}

	return true
}

// stringToInt 字符串转整数
func stringToInt(s string) int {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		result = result*10 + int(c-'0')
	}
	return result
}

// PriorityQueue 优先级队列
type PriorityQueue struct {
	items   []*RenderRegion
	maxSize int
	mu      sync.RWMutex
}

// NewPriorityQueue 创建优先级队列
func NewPriorityQueue(maxSize int) *PriorityQueue {
	return &PriorityQueue{
		items:   make([]*RenderRegion, 0),
		maxSize: maxSize,
	}
}

// Push 推入队列
func (pq *PriorityQueue) Push(item *RenderRegion) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// 先添加新项
	pq.items = append(pq.items, item)

	// 按优先级排序（升序，0 为最高优先级）
	sort.Slice(pq.items, func(i, j int) bool {
		return pq.items[i].Priority < pq.items[j].Priority
	})

	// 如果超过最大容量，移除最低优先级的项（末尾）
	if len(pq.items) > pq.maxSize && pq.maxSize > 0 {
		pq.items = pq.items[:pq.maxSize]
	}
}

// Pop 弹出最高优先级项
func (pq *PriorityQueue) Pop() *RenderRegion {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items) == 0 {
		return nil
	}

	item := pq.items[0]
	pq.items = pq.items[1:]
	return item
}

// Len 返回队列长度
func (pq *PriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.items)
}

// Peek 查看最高优先级项但不弹出
func (pq *PriorityQueue) Peek() *RenderRegion {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	if len(pq.items) == 0 {
		return nil
	}
	return pq.items[0]
}

// RenderTask 渲染任务
type RenderTask struct {
	ID        string
	Region    *RenderRegion
	Diff      *DiffResult
	Callback  func(region *RenderRegion, html string)
	CreatedAt time.Time
}

// RenderOptions 渲染选项
type RenderOptions struct {
	SkipCache      bool          `json:"skip_cache"`      // 跳过缓存
	ForceRefresh   bool          `json:"force_refresh"`   // 强制刷新
	Timeout        time.Duration `json:"timeout"`         // 超时时间
	OnlyCritical   bool          `json:"only_critical"`   // 仅渲染关键区域
	IncludeLazy    bool          `json:"include_lazy"`    // 包含懒加载区域
	MaxRegions     int           `json:"max_regions"`     // 最大渲染区域数
	PriorityFilter int           `json:"priority_filter"` // 优先级过滤阈值
}

// DefaultRenderOptions 返回默认渲染选项
func DefaultRenderOptions() *RenderOptions {
	return &RenderOptions{
		SkipCache:      false,
		ForceRefresh:   false,
		Timeout:        30 * time.Second,
		OnlyCritical:   false,
		IncludeLazy:    false,
		MaxRegions:     10,
		PriorityFilter: 10,
	}
}

// SelectiveRenderResult 选择性渲染结果
type SelectiveRenderResult struct {
	RenderedRegions []string `json:"rendered_regions"` // 已渲染区域 ID 列表
	SkippedRegions  []string `json:"skipped_regions"`  // 跳过的区域 ID 列表
	TotalRegions    int      `json:"total_regions"`    // 总区域数
	RenderTime      int64    `json:"render_time_ms"`   // 渲染耗时
}

// RenderSelective 选择性渲染
func (r *SelectiveRenderer) RenderSelective(html string, diff *DiffResult, options *RenderOptions) *SelectiveRenderResult {
	startTime := time.Now()

	if options == nil {
		options = DefaultRenderOptions()
	}

	result := &SelectiveRenderResult{
		RenderedRegions: make([]string, 0),
		SkippedRegions:  make([]string, 0),
	}

	// 1. 检测所有区域
	regions := r.detector.DetectRegions(html)
	result.TotalRegions = len(regions)

	if len(regions) == 0 {
		return result
	}

	// 2. 清空优先级队列
	r.ClearQueue()

	for _, region := range regions {
		// 根据选项过滤
		if options.OnlyCritical && !region.IsCritical {
			result.SkippedRegions = append(result.SkippedRegions, region.ID)
			continue
		}

		if !options.IncludeLazy && region.Priority >= 8 {
			result.SkippedRegions = append(result.SkippedRegions, region.ID)
			continue
		}

		if region.Priority > options.PriorityFilter {
			result.SkippedRegions = append(result.SkippedRegions, region.ID)
			continue
		}

		r.queue.Push(region)
	}

	// 3. 按优先级渲染区域
	renderedCount := 0
	for renderedCount < options.MaxRegions {
		region := r.queue.Pop()
		if region == nil {
			break
		}

		// 检查是否在差异结果中需要更新
		needsUpdate := r.needsUpdate(region, diff)

		if needsUpdate || options.ForceRefresh {
			result.RenderedRegions = append(result.RenderedRegions, region.ID)
			renderedCount++
		} else {
			result.SkippedRegions = append(result.SkippedRegions, region.ID)
		}
	}

	result.RenderTime = time.Since(startTime).Milliseconds()

	r.logger.Debug("选择性渲染完成",
		zap.Int("total_regions", result.TotalRegions),
		zap.Int("rendered", len(result.RenderedRegions)),
		zap.Int("skipped", len(result.SkippedRegions)),
		zap.Int64("duration_ms", result.RenderTime),
	)

	return result
}

// needsUpdate 检查区域是否需要更新
func (r *SelectiveRenderer) needsUpdate(region *RenderRegion, diff *DiffResult) bool {
	if diff == nil {
		return false
	}

	// 检查路径是否在变更列表中
	for _, d := range diff.Diffs {
		if d.ParentPath != "" && strings.Contains(d.ParentPath, region.ID) {
			return true
		}

		if d.Node != nil && d.Node.ID == region.ID {
			return true
		}
	}

	return false
}

// ScheduleRender 调度渲染任务
func (r *SelectiveRenderer) ScheduleRender(task *RenderTask) {
	// 直接推入队列，不持有锁（queue.Push 有自己的锁）
	r.queue.Push(task.Region)

	r.logger.Debug("渲染任务已调度",
		zap.String("task_id", task.ID),
		zap.String("region_id", task.Region.ID),
		zap.Int("priority", task.Region.Priority),
	)
}

// GetChangedRegionPaths 获取变更区域的路径
func (r *SelectiveRenderer) GetChangedRegionPaths(diff *DiffResult) []string {
	paths := make([]string, 0)

	if diff == nil {
		return paths
	}

	seenPaths := make(map[string]bool)

	for _, d := range diff.Diffs {
		path := d.ParentPath
		if path != "" && !seenPaths[path] {
			paths = append(paths, path)
			seenPaths[path] = true
		}

		// 添加节点路径
		if d.Node != nil && d.Node.ID != "" {
			nodePath := "/" + d.Node.Tag + "[" + d.Node.ID + "]"
			if !seenPaths[nodePath] {
				paths = append(paths, nodePath)
				seenPaths[nodePath] = true
			}
		}
	}

	return paths
}

// BuildRenderHTML 构建渲染 HTML（仅渲染变更区域）
func (r *SelectiveRenderer) BuildRenderHTML(baseHTML string, diff *DiffResult, options *RenderOptions) string {
	if options == nil {
		options = DefaultRenderOptions()
	}

	// 获取变更路径
	changedPaths := r.GetChangedRegionPaths(diff)

	if len(changedPaths) == 0 {
		return baseHTML
	}

	// 标记变更区域
	result := baseHTML
	for _, path := range changedPaths {
		// 提取路径中的标签
		tagPattern := regexp.MustCompile(`/([^/\[]+)`)
		matches := tagPattern.FindAllStringSubmatch(path, -1)

		if len(matches) > 0 {
			lastTag := matches[len(matches)-1][1]
			// 添加 data-dirty 标记
			result = strings.Replace(result, "<"+lastTag, "<"+lastTag+" data-dirty=\"true\"", 1)
		}
	}

	return result
}

// ClearQueue 清空队列
func (r *SelectiveRenderer) ClearQueue() {
	r.queue.mu.Lock()
	defer r.queue.mu.Unlock()
	r.queue.items = make([]*RenderRegion, 0)
}

// GetQueueLength 获取队列长度
func (r *SelectiveRenderer) GetQueueLength() int {
	return r.queue.Len()
}
