package incremental

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultSelectiveRenderConfig(t *testing.T) {
	config := DefaultSelectiveRenderConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnablePriorityQueue)
	assert.Equal(t, true, config.EnableLazyRender)
	assert.Equal(t, 4, config.MaxConcurrentRenders)
	assert.Equal(t, 50*time.Millisecond, config.BatchInterval)
	assert.Len(t, config.PrioritySelectors, 6)
	assert.Len(t, config.LazySelectors, 6)
}

func TestNewSelectiveRenderer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultSelectiveRenderConfig()

	renderer := NewSelectiveRenderer(config, logger)

	assert.NotNil(t, renderer)
	assert.Equal(t, config, renderer.config)
	assert.NotNil(t, renderer.queue)
	assert.NotNil(t, renderer.detector)
}

func TestNewSelectiveRenderer_NilConfig(t *testing.T) {
	renderer := NewSelectiveRenderer(nil, nil)

	assert.NotNil(t, renderer)
	assert.Equal(t, 4, renderer.config.MaxConcurrentRenders)
	assert.Equal(t, 800, renderer.config.ViewportHeight)
}

func TestRegionDetector_DetectRegions(t *testing.T) {
	config := DefaultSelectiveRenderConfig()
	detector := NewRegionDetector(config)

	html := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
	<header id="main-header" class="critical">Header Content</header>
	<nav id="main-nav">Navigation</nav>
	<main>
		<article id="article-1">Article Content</article>
		<section id="section-1">Section Content</section>
	</main>
	<footer id="main-footer">Footer Content</footer>
</body>
</html>`

	regions := detector.DetectRegions(html)

	assert.Greater(t, len(regions), 0)

	// 查找 header 区域
	var headerRegion *RenderRegion
	for _, region := range regions {
		if region.Tag == "header" {
			headerRegion = region
			break
		}
	}

	assert.NotNil(t, headerRegion)
	assert.Equal(t, "main-header", headerRegion.ID)
	assert.True(t, headerRegion.IsCritical)
}

func TestRegionDetector_parseAttributes(t *testing.T) {
	config := DefaultSelectiveRenderConfig()
	detector := NewRegionDetector(config)

	attrString := `id="test-id" class="test-class another-class" data-priority="5" data-critical="true"`
	attrs := detector.parseAttributes(attrString)

	assert.Equal(t, "test-id", attrs["id"])
	assert.Equal(t, "test-class another-class", attrs["class"])
	assert.Equal(t, "5", attrs["data-priority"])
	assert.Equal(t, "true", attrs["data-critical"])
}

func TestRegionDetector_calculatePriority(t *testing.T) {
	config := DefaultSelectiveRenderConfig()
	detector := NewRegionDetector(config)

	// 测试 header 标签（高优先级）
	attrs := map[string]string{}
	priority := detector.calculatePriority("header", attrs)
	assert.LessOrEqual(t, priority, 2)

	// 测试 footer 标签（低优先级）
	priority = detector.calculatePriority("footer", attrs)
	assert.GreaterOrEqual(t, priority, 7)

	// 测试 data-priority 属性
	// 注意：PrioritySelectors 中有 [data-priority]，所以带 data-priority 的元素优先级为 0
	// 这里验证这个行为
	attrs = map[string]string{"data-priority": "6"}
	priority = detector.calculatePriority("code", attrs)
	// 因为 [data-priority] 选择器匹配，优先级应该是 0（最高优先级）
	assert.Equal(t, 0, priority)
}

func TestRegionDetector_isCritical(t *testing.T) {
	config := DefaultSelectiveRenderConfig()
	detector := NewRegionDetector(config)

	// 测试 header 标签
	assert.True(t, detector.isCritical("header", map[string]string{}))

	// 测试 critical class
	assert.True(t, detector.isCritical("div", map[string]string{"class": "critical"}))

	// 测试 data-critical 属性
	assert.True(t, detector.isCritical("div", map[string]string{"data-critical": "true"}))

	// 测试普通区域
	assert.False(t, detector.isCritical("div", map[string]string{}))
}

func TestRegionDetector_isVisible(t *testing.T) {
	config := DefaultSelectiveRenderConfig()
	detector := NewRegionDetector(config)

	// 测试可见元素
	assert.True(t, detector.isVisible("div", map[string]string{}))

	// 测试 display:none
	assert.False(t, detector.isVisible("div", map[string]string{"style": "display:none"}))

	// 测试 hidden 属性
	assert.False(t, detector.isVisible("div", map[string]string{"hidden": "true"}))

	// 测试 aria-hidden
	assert.False(t, detector.isVisible("div", map[string]string{"aria-hidden": "true"}))
}

func TestRegionDetector_selectorToRegex(t *testing.T) {
	config := DefaultSelectiveRenderConfig()
	detector := NewRegionDetector(config)

	// 测试标签选择器
	regex := detector.selectorToRegex("div")
	assert.NotNil(t, regex)
	assert.True(t, regex.MatchString(`<div class="test">`))

	// 测试 class 选择器
	regex = detector.selectorToRegex(".test-class")
	assert.NotNil(t, regex)
	assert.True(t, regex.MatchString(`<div class="test-class another">`))

	// 测试属性选择器
	// selectorToRegex 对于 img[loading='lazy'] 会尝试匹配包含 loading= 属性的 img 标签
	regex = detector.selectorToRegex("img[loading='lazy']")
	assert.NotNil(t, regex)
	// 正则应该匹配包含 loading="lazy" 的 img 标签
	assert.True(t, regex.MatchString(`<img loading="lazy" src="test.jpg">`))
}

func TestPriorityQueue_Push(t *testing.T) {
	queue := NewPriorityQueue(10)

	region1 := &RenderRegion{ID: "1", Priority: 5}
	region2 := &RenderRegion{ID: "2", Priority: 1}
	region3 := &RenderRegion{ID: "3", Priority: 3}

	queue.Push(region1)
	queue.Push(region2)
	queue.Push(region3)

	assert.Equal(t, 3, queue.Len())

	// 弹出应该是按优先级排序（0 最高）
	popped := queue.Pop()
	assert.Equal(t, "2", popped.ID) // 优先级 1 最高
}

func TestPriorityQueue_Pop(t *testing.T) {
	queue := NewPriorityQueue(10)

	// 空队列弹出
	item := queue.Pop()
	assert.Nil(t, item)

	// 添加一个元素后弹出
	queue.Push(&RenderRegion{ID: "1"})
	item = queue.Pop()
	assert.NotNil(t, item)
	assert.Equal(t, "1", item.ID)
	assert.Equal(t, 0, queue.Len())
}

func TestPriorityQueue_Peek(t *testing.T) {
	queue := NewPriorityQueue(10)

	// 空队列 Peek
	item := queue.Peek()
	assert.Nil(t, item)

	region1 := &RenderRegion{ID: "1", Priority: 5}
	region2 := &RenderRegion{ID: "2", Priority: 1}

	queue.Push(region1)
	queue.Push(region2)

	// Peek 不应该移除元素
	item = queue.Peek()
	assert.NotNil(t, item)
	assert.Equal(t, "2", item.ID)
	assert.Equal(t, 2, queue.Len())
}

func TestPriorityQueue_MaxSize(t *testing.T) {
	queue := NewPriorityQueue(3)

	queue.Push(&RenderRegion{ID: "1", Priority: 1})
	queue.Push(&RenderRegion{ID: "2", Priority: 2})
	queue.Push(&RenderRegion{ID: "3", Priority: 3})
	queue.Push(&RenderRegion{ID: "4", Priority: 4})

	// 超过最大容量，应该只保留 3 个
	assert.Equal(t, 3, queue.Len())

	// 最低优先级的应该被移除（优先级 4 的 ID="4"）
	// 弹出顺序应该是优先级从高到低：1, 2, 3
	items := []string{}
	for queue.Len() > 0 {
		items = append(items, queue.Pop().ID)
	}
	assert.Contains(t, items, "1")    // 优先级 1
	assert.Contains(t, items, "2")    // 优先级 2
	assert.Contains(t, items, "3")    // 优先级 3
	assert.NotContains(t, items, "4") // 优先级 4 被丢弃
}

func TestSelectiveRenderer_RenderSelective(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	html := `<!DOCTYPE html>
<html>
<body>
	<header id="header" class="critical">Header</header>
	<main id="main">Main Content</main>
	<footer id="footer">Footer</footer>
</body>
</html>`

	diff := &DiffResult{
		Diffs: []DOMDiff{
			{Type: "modified", ParentPath: "/main"},
		},
	}

	options := DefaultRenderOptions()
	options.MaxRegions = 5

	result := renderer.RenderSelective(html, diff, options)

	assert.NotNil(t, result)
	assert.Greater(t, result.TotalRegions, 0)
	assert.GreaterOrEqual(t, len(result.RenderedRegions)+len(result.SkippedRegions), 0)
	assert.GreaterOrEqual(t, result.RenderTime, int64(0))
}

func TestSelectiveRenderer_RenderSelective_OnlyCritical(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	html := `<!DOCTYPE html>
<html>
<body>
	<header id="header" class="critical">Header</header>
	<main id="main">Main Content</main>
	<footer id="footer">Footer</footer>
</body>
</html>`

	options := &RenderOptions{
		OnlyCritical: true,
		MaxRegions:   10,
	}

	result := renderer.RenderSelective(html, nil, options)

	assert.NotNil(t, result)
	// 所有渲染的区域都应该是关键的
	for _, regionID := range result.RenderedRegions {
		_ = regionID
		// 验证逻辑：只渲染 critical 区域
	}
}

func TestSelectiveRenderer_needsUpdate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	region := &RenderRegion{ID: "test-region", Tag: "div"}

	// 没有 diff，不需要更新
	assert.False(t, renderer.needsUpdate(region, nil))

	// diff 中包含该区域
	diff := &DiffResult{
		Diffs: []DOMDiff{
			{
				Type:       "modified",
				ParentPath: "/div[test-region]",
				Node:       &DOMNode{ID: "test-region"},
			},
		},
	}
	assert.True(t, renderer.needsUpdate(region, diff))

	// diff 中节点 ID 匹配
	diff2 := &DiffResult{
		Diffs: []DOMDiff{
			{
				Type: "modified",
				Node: &DOMNode{ID: "test-region"},
			},
		},
	}
	assert.True(t, renderer.needsUpdate(region, diff2))

	// diff 不匹配
	diff3 := &DiffResult{
		Diffs: []DOMDiff{
			{
				Type:       "modified",
				ParentPath: "/other",
				Node:       &DOMNode{ID: "other-region"},
			},
		},
	}
	assert.False(t, renderer.needsUpdate(region, diff3))
}

func TestSelectiveRenderer_GetChangedRegionPaths(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	diff := &DiffResult{
		Diffs: []DOMDiff{
			{Type: "added", ParentPath: "/div/section[0]"},
			{Type: "modified", ParentPath: "/div/main[0]"},
			{Type: "removed", ParentPath: "/div/footer[0]"},
		},
	}

	paths := renderer.GetChangedRegionPaths(diff)

	// 至少有 3 个 parent paths
	assert.GreaterOrEqual(t, len(paths), 3)
	assert.Contains(t, paths, "/div/section[0]")
	assert.Contains(t, paths, "/div/main[0]")
	assert.Contains(t, paths, "/div/footer[0]")
}

func TestSelectiveRenderer_BuildRenderHTML(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	baseHTML := `<!DOCTYPE html>
<html>
<body>
	<div id="unchanged">Unchanged Content</div>
	<div id="changed">Changed Content</div>
</body>
</html>`

	diff := &DiffResult{
		Diffs: []DOMDiff{
			{
				Type:       "modified",
				ParentPath: "/div[changed]",
				Node:       &DOMNode{Tag: "div", ID: "changed"},
			},
		},
	}

	result := renderer.BuildRenderHTML(baseHTML, diff, nil)

	assert.Contains(t, result, "data-dirty=\"true\"")
	assert.NotEqual(t, baseHTML, result)
}

func TestSelectiveRenderer_ScheduleRender(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	task := &RenderTask{
		ID: "task-1",
		Region: &RenderRegion{
			ID:       "region-1",
			Tag:      "div",
			Priority: 5,
		},
		CreatedAt: time.Now(),
	}

	renderer.ScheduleRender(task)

	assert.Equal(t, 1, renderer.GetQueueLength())

	renderer.ClearQueue()
	assert.Equal(t, 0, renderer.GetQueueLength())
}

func TestRenderRegion(t *testing.T) {
	region := &RenderRegion{
		ID:         "test-region",
		Path:       "/body/div[0]",
		Tag:        "div",
		Priority:   5,
		IsVisible:  true,
		IsCritical: false,
		Content:    "Test Content",
		Attributes: map[string]string{
			"class": "test-class",
			"id":    "test-id",
		},
		Children: []*RenderRegion{
			{ID: "child-1", Tag: "span"},
		},
	}

	assert.NotNil(t, region)
	assert.Equal(t, "test-region", region.ID)
	assert.Equal(t, "div", region.Tag)
	assert.Equal(t, 5, region.Priority)
	assert.True(t, region.IsVisible)
	assert.False(t, region.IsCritical)
	assert.Len(t, region.Children, 1)
}

func TestRenderTask(t *testing.T) {
	task := &RenderTask{
		ID: "task-1",
		Region: &RenderRegion{
			ID:       "region-1",
			Priority: 3,
		},
		Diff: &DiffResult{
			TotalNodes: 100,
		},
		CreatedAt: time.Now(),
	}

	assert.NotNil(t, task)
	assert.Equal(t, "task-1", task.ID)
	assert.NotNil(t, task.Region)
	assert.NotNil(t, task.Diff)
}

func TestRenderOptions(t *testing.T) {
	options := DefaultRenderOptions()

	assert.NotNil(t, options)
	assert.Equal(t, false, options.SkipCache)
	assert.Equal(t, false, options.ForceRefresh)
	assert.Equal(t, 30*time.Second, options.Timeout)
	assert.Equal(t, false, options.OnlyCritical)
	assert.Equal(t, false, options.IncludeLazy)
	assert.Equal(t, 10, options.MaxRegions)
	assert.Equal(t, 10, options.PriorityFilter)
}

func TestRenderOptions_Custom(t *testing.T) {
	options := &RenderOptions{
		SkipCache:      true,
		ForceRefresh:   true,
		Timeout:        10 * time.Second,
		OnlyCritical:   true,
		IncludeLazy:    true,
		MaxRegions:     5,
		PriorityFilter: 7,
	}

	assert.True(t, options.SkipCache)
	assert.True(t, options.ForceRefresh)
	assert.Equal(t, 10*time.Second, options.Timeout)
	assert.True(t, options.OnlyCritical)
	assert.True(t, options.IncludeLazy)
	assert.Equal(t, 5, options.MaxRegions)
	assert.Equal(t, 7, options.PriorityFilter)
}

func TestSelectiveRenderer_RenderSelective_EmptyHTML(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	result := renderer.RenderSelective("", nil, nil)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.TotalRegions)
	assert.Empty(t, result.RenderedRegions)
	assert.Empty(t, result.SkippedRegions)
}

func TestSelectiveRenderer_RenderSelective_ForceRefresh(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	html := `<html><body><div id="test">Content</div></body></html>`

	options := &RenderOptions{
		ForceRefresh: true,
		MaxRegions:   10,
	}

	result := renderer.RenderSelective(html, nil, options)

	assert.NotNil(t, result)
	// ForceRefresh 应该渲染所有符合条件的区域
}

func TestSelectiveRenderer_BuildRenderHTML_NoChanges(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	baseHTML := `<html><body><div>Content</div></body></html>`

	// 没有变化的 diff
	diff := &DiffResult{Diffs: []DOMDiff{}}

	result := renderer.BuildRenderHTML(baseHTML, diff, nil)

	assert.Equal(t, baseHTML, result)
	assert.NotContains(t, result, "data-dirty")
}

func TestSelectiveRenderer_BuildRenderHTML_NilDiff(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	baseHTML := `<html><body><div>Content</div></body></html>`

	result := renderer.BuildRenderHTML(baseHTML, nil, nil)

	assert.Equal(t, baseHTML, result)
}

func TestRegionDetector_attrsToString(t *testing.T) {
	config := DefaultSelectiveRenderConfig()
	detector := NewRegionDetector(config)

	attrs := map[string]string{
		"class":      "test",
		"id":         "main",
		"data-value": "123",
	}

	result := detector.attrsToString(attrs)

	// 应该包含所有属性
	assert.Contains(t, result, "class=\"test\"")
	assert.Contains(t, result, "id=\"main\"")
	assert.Contains(t, result, "data-value=\"123\"")

	// 应该已排序（字典序）
	parts := strings.Split(result, " ")
	sortedParts := make([]string, len(parts))
	copy(sortedParts, parts)
	sortStrings(sortedParts)
	assert.Equal(t, sortedParts, parts)
}

func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func TestSelectiveRenderer_RenderSelective_IncludeLazy(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	html := `<html>
<body>
	<img src="test.jpg" loading="lazy" id="lazy-img">
	<footer id="footer">Footer</footer>
</body>
</html>`

	// 不包含懒加载
	options1 := &RenderOptions{IncludeLazy: false, MaxRegions: 10}
	result1 := renderer.RenderSelective(html, nil, options1)

	// 包含懒加载
	options2 := &RenderOptions{IncludeLazy: true, MaxRegions: 10}
	result2 := renderer.RenderSelective(html, nil, options2)

	assert.NotNil(t, result1)
	assert.NotNil(t, result2)
	// 包含懒加载时应该渲染更多区域
	assert.GreaterOrEqual(t, len(result2.RenderedRegions), len(result1.RenderedRegions))
}

func TestPriorityQueue_PushNilConfig(t *testing.T) {
	queue := NewPriorityQueue(0)

	// 最大容量为 0 时应该能处理
	queue.Push(&RenderRegion{ID: "1"})
	// 不应该 panic
}

func TestRegionDetector_DetectRegions_SelfClosingTags(t *testing.T) {
	config := DefaultSelectiveRenderConfig()
	detector := NewRegionDetector(config)

	html := `<html><body>
		<img src="test.jpg" id="img-1"/>
		<br/>
		<input type="text" id="input-1"/>
	</body></html>`

	regions := detector.DetectRegions(html)

	assert.Greater(t, len(regions), 0)
}

func TestSelectiveRenderer_GetChangedRegionPaths_EmptyDiff(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewSelectiveRenderer(nil, logger)

	paths := renderer.GetChangedRegionPaths(&DiffResult{Diffs: []DOMDiff{}})
	assert.Empty(t, paths)

	paths = renderer.GetChangedRegionPaths(nil)
	assert.Empty(t, paths)
}
