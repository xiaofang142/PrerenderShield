package streaming

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultFirstScreenConfig(t *testing.T) {
	config := DefaultFirstScreenConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 800, config.ViewportHeight)
	assert.Equal(t, 1280, config.ViewportWidth)
	assert.Equal(t, true, config.EnableLazyLoad)
	assert.Equal(t, true, config.EnablePreload)
}

func TestNewFirstScreenRenderer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultFirstScreenConfig()

	renderer := NewFirstScreenRenderer(config, logger)

	assert.NotNil(t, renderer)
	assert.Equal(t, config, renderer.config)
}

func TestFirstScreenRenderer_OptimizeForFirstScreen(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
	<link rel="stylesheet" href="styles.css">
</head>
<body>
	<img src="image1.jpg" alt="Image 1">
	<img src="image2.jpg" alt="Image 2">
	<img src="image3.jpg" alt="Image 3">
</body>
</html>`

	optimized := renderer.OptimizeForFirstScreen(html)

	assert.Contains(t, optimized, "preload")
	assert.Contains(t, optimized, `loading="lazy"`)
	assert.Contains(t, optimized, "firstScreenLoaded")
}

func TestFirstScreenRenderer_inlineCriticalCSS(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	// 使用 httptest.Server 提供 CSS 内容
	cssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte("body { margin: 0; } /* Critical CSS */"))
	}))
	defer cssServer.Close()

	html := `<html><head><link rel="stylesheet" href="` + cssServer.URL + `/critical.css"></head><body></body></html>`

	result := renderer.inlineCriticalCSS(html)

	assert.Contains(t, result, "Critical CSS")
	assert.NotContains(t, result, `<link rel="stylesheet"`)
}

func TestFirstScreenRenderer_addPreloads(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	html := `<html><head></head><body>
		<img src="image1.jpg">
		<img src="image2.jpg">
		<img src="image3.jpg">
		<img src="image4.jpg">
	</body></html>`

	result := renderer.addPreloads(html)

	assert.Contains(t, result, `<link rel="preload"`)
	assert.Contains(t, result, `as="image"`)
}

func TestFirstScreenRenderer_addLazyLoading(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	html := `<html><body>
		<img src="test.jpg" alt="Test">
		<img src="test2.jpg">
	</body></html>`

	result := renderer.addLazyLoading(html)

	assert.Contains(t, result, `loading="lazy"`)
}

func TestFirstScreenRenderer_addFirstScreenDetection(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	html := `<html><body>Content</body></html>`

	result := renderer.addFirstScreenDetection(html)

	assert.Contains(t, result, "firstScreenLoaded")
	assert.Contains(t, result, "IntersectionObserver")
}

func TestFirstScreenRenderer_parseHTML(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Test Title</title>
	<meta charset="utf-8">
	<style>body{color:red;}</style>
</head>
<body>
	<h1>Hello</h1>
	<p>World</p>
</body>
</html>`

	doc := renderer.parseHTML(html)

	assert.NotNil(t, doc)
	assert.NotNil(t, doc.Head)
	assert.NotNil(t, doc.Body)
	assert.Equal(t, "Test Title", doc.Head.Title)
	assert.Greater(t, len(doc.Head.Metas), 0)
}

func TestFirstScreenRenderer_filterCriticalScripts(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	scripts := []string{
		`<script>console.log("critical");</script>`,
		`<script async src="analytics.js"></script>`,
		`<script defer src="app.js"></script>`,
		`<script>console.log("also critical");</script>`,
	}

	critical := renderer.filterCriticalScripts(scripts)

	assert.Len(t, critical, 2)
	assert.NotContains(t, critical, `<script async src="analytics.js"></script>`)
	assert.NotContains(t, critical, `<script defer src="app.js"></script>`)
}

func TestFirstScreenRenderer_RenderWithPriority(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	ctx := context.Background()
	html := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
	<header>Header Content</header>
	<main>Main Content</main>
	<footer>Footer Content</footer>
</body>
</html>`

	recorder := httptest.NewRecorder()
	writer := &testFlushWriter{recorder}

	err := renderer.RenderWithPriority(ctx, html, writer)

	assert.NoError(t, err)
	assert.Contains(t, recorder.Body.String(), "<!DOCTYPE html>")
	assert.Contains(t, recorder.Body.String(), "Header Content")
}

func TestFirstScreenRenderer_RenderWithPriority_Timeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultFirstScreenConfig()
	config.Timeout = 1 * time.Millisecond
	renderer := NewFirstScreenRenderer(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	html := `<!DOCTYPE html><html><head></head><body>Content</body></html>`

	var buf bytes.Buffer
	writer := &testFlushWriter{httptest.NewRecorder()}
	_ = buf

	// 不应该超时，因为渲染是同步的
	err := renderer.RenderWithPriority(ctx, html, writer)

	// 可能超时也可能不超时，取决于执行速度
	_ = err
}

func TestHtmlDocument(t *testing.T) {
	doc := &htmlDocument{
		Head: &htmlHead{
			Title:   "Test",
			Metas:   []string{`<meta charset="utf-8">`},
			Styles:  []string{`<style>body{}</style>`},
			Scripts: []string{`<script></script>`},
		},
		Body: &htmlBody{
			Content: []string{"<div>Content</div>"},
		},
	}

	assert.NotNil(t, doc)
	assert.Equal(t, "Test", doc.Head.Title)
	assert.Len(t, doc.Head.Metas, 1)
}

func TestLazyLoadManager(t *testing.T) {
	config := DefaultFirstScreenConfig()
	manager := NewLazyLoadManager(config)

	// 添加元素
	manager.AddElement("img", "img1", "lazy", `<img src="test.jpg">`)
	manager.AddElement("img", "img2", "lazy", `<img src="test2.jpg">`)

	elements := manager.GetElements()
	assert.Len(t, elements, 2)

	placeholder := manager.GetPlaceholderHTML(elements[0])
	assert.Contains(t, placeholder, "lazy-placeholder")
}

func TestContentSplitter_Split(t *testing.T) {
	splitter := NewContentSplitter(10)

	content := "0123456789ABCDEF"
	chunks := splitter.Split(content)

	assert.Len(t, chunks, 2)
	assert.Equal(t, "0123456789", chunks[0])
	assert.Equal(t, "ABCDEF", chunks[1])
}

func TestContentSplitter_SplitByElement(t *testing.T) {
	splitter := NewContentSplitter(0)

	html := `<div>Hello</div><p>World</p><img src="test.jpg">`
	elements := splitter.SplitByElement(html)

	assert.Greater(t, len(elements), 0)
}

func TestContentSplitter_EmptyContent(t *testing.T) {
	splitter := NewContentSplitter(10)

	chunks := splitter.Split("")
	assert.Len(t, chunks, 0)
}

func TestFirstScreenRenderer_NilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(nil, logger)

	assert.NotNil(t, renderer)
	assert.Equal(t, 800, renderer.config.ViewportHeight)
}

func TestFirstScreenRenderer_NilLogger(t *testing.T) {
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), nil)

	assert.NotNil(t, renderer)
}

func TestFirstScreenRenderer_extractFirstScreen(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	doc := &htmlDocument{
		Head: &htmlHead{
			Title:   "Test",
			Metas:   []string{`<meta charset="utf-8">`, `<meta name="viewport">`},
			Styles:  []string{`<style>body{}</style>`, `<style>header{}</style>`, `<style>main{}</style>`, `<style>footer{}</style>`},
			Scripts: []string{`<script>critical()</script>`, `<script async src="async.js"></script>`},
		},
		Body: &htmlBody{
			Content: make([]string, 20),
		},
	}

	for i := range doc.Body.Content {
		doc.Body.Content[i] = "<div>Content " + string(rune('A'+i)) + "</div>"
	}

	firstScreen := renderer.extractFirstScreen(doc)

	assert.NotNil(t, firstScreen)
	assert.NotNil(t, firstScreen.Head)
	// 样式应该被限制
	assert.LessOrEqual(t, len(firstScreen.Head.Styles), 3)
}

func TestFirstScreenRenderer_extractRemaining(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	doc := &htmlDocument{
		Head: &htmlHead{
			Title: "Test",
		},
		Body: &htmlBody{
			Content: make([]string, 20),
		},
	}

	for i := range doc.Body.Content {
		doc.Body.Content[i] = "<div>Content " + string(rune('A'+i)) + "</div>"
	}

	remaining := renderer.extractRemaining(doc)

	assert.NotNil(t, remaining)
	assert.NotNil(t, remaining.Head)
	// 头部应该为空
	assert.Equal(t, &htmlHead{}, remaining.Head)
}

func TestLazyElement(t *testing.T) {
	element := &LazyElement{
		Tag:         "img",
		ID:          "test-img",
		Class:       "lazy",
		Original:    `<img src="test.jpg">`,
		Placeholder: "placeholder",
		Loaded:      false,
	}

	assert.NotNil(t, element)
	assert.False(t, element.Loaded)
	assert.Equal(t, "img", element.Tag)
}

func TestPriorityContent(t *testing.T) {
	content := &PriorityContent{
		Content:  "<div>Test</div>",
		Priority: 1,
		Type:     "critical",
	}

	assert.NotNil(t, content)
	assert.Equal(t, 1, content.Priority)
	assert.Equal(t, "critical", content.Type)
}

func TestMin(t *testing.T) {
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 3, min(10, 3))
	assert.Equal(t, 5, min(5, 5))
}

func TestFirstScreenRenderer_sendHead(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	head := &htmlHead{
		Title:   "Test Title",
		Metas:   []string{`<meta charset="utf-8">`},
		Styles:  []string{`<style>body{color:red;}</style>`},
		Scripts: []string{`<script>console.log("test");</script>`},
	}

	recorder := httptest.NewRecorder()
	writer := &testFlushWriter{recorder}

	renderer.sendHead(head, writer)

	assert.Contains(t, recorder.Body.String(), "<head>")
	assert.Contains(t, recorder.Body.String(), "Test Title")
	assert.Contains(t, recorder.Body.String(), `charset="utf-8"`)
}

func TestFirstScreenRenderer_addPreloads_MultipleImages(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	renderer := NewFirstScreenRenderer(DefaultFirstScreenConfig(), logger)

	html := `<html><head></head><body>
		<img src="image1.jpg">
		<img src="image2.jpg">
		<img src="image3.jpg">
		<img src="image4.jpg">
		<img src="image5.jpg">
	</body></html>`

	result := renderer.addPreloads(html)

	// 应该只预加载前 3 个图片
	count := strings.Count(result, `<link rel="preload"`)
	assert.Equal(t, 3, count)
}
