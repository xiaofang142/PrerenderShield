package incremental

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultDOMDiffConfig(t *testing.T) {
	config := DefaultDOMDiffConfig()

	assert.NotNil(t, config)
	assert.Equal(t, true, config.EnableHashCache)
	assert.Equal(t, 50, config.MaxDepth)
	assert.Equal(t, true, config.IgnoreWhitespace)
	assert.Equal(t, 5*time.Second, config.Timeout)
}

func TestNewDOMDiffEngine(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.config)
	assert.Equal(t, 50, engine.config.MaxDepth)
}

func TestDOMDiffEngine_ComputeDiff(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	oldTree := &DOMTree{
		Root: &DOMNode{
			Tag:  "div",
			Text: "old content",
			Children: []*DOMNode{
				{Tag: "span", Text: "child 1"},
				{Tag: "p", Text: "child 2"},
			},
		},
		NodeIndex: make(map[string]*DOMNode),
	}

	newTree := &DOMTree{
		Root: &DOMNode{
			Tag:  "div",
			Text: "new content",
			Children: []*DOMNode{
				{Tag: "span", Text: "child 1 modified"},
				{Tag: "p", Text: "modified child 2"},
			},
		},
		NodeIndex: make(map[string]*DOMNode),
	}

	result := engine.ComputeDiff(oldTree, newTree)

	assert.NotNil(t, result)
	// 验证基本结果
	assert.GreaterOrEqual(t, result.TotalNodes, 0)
	assert.GreaterOrEqual(t, result.Duration, int64(0))
}

func TestDOMDiffEngine_ComputeDiff_SameTree(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	node := &DOMNode{
		Tag:  "div",
		Text: "same content",
		Children: []*DOMNode{
			{Tag: "span", Text: "child"},
		},
		Hash: "same-hash",
	}

	oldTree := &DOMTree{Root: node, NodeIndex: make(map[string]*DOMNode)}
	newTree := &DOMTree{Root: node, NodeIndex: make(map[string]*DOMNode)}

	result := engine.ComputeDiff(oldTree, newTree)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.ChangedNodes)
	assert.Equal(t, 0, result.AddedNodes)
	assert.Equal(t, 0, result.RemovedNodes)
}

func TestDOMDiffEngine_ComputeDiff_AddedNode(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	oldTree := &DOMTree{
		Root:      &DOMNode{Tag: "div", Children: []*DOMNode{}},
		NodeIndex: make(map[string]*DOMNode),
	}

	newTree := &DOMTree{
		Root: &DOMNode{
			Tag: "div",
			Children: []*DOMNode{
				{Tag: "span", Text: "new child"},
			},
		},
		NodeIndex: make(map[string]*DOMNode),
	}

	result := engine.ComputeDiff(oldTree, newTree)

	assert.NotNil(t, result)
	assert.Greater(t, result.AddedNodes, 0)
}

func TestDOMDiffEngine_ComputeDiff_RemovedNode(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	oldTree := &DOMTree{
		Root: &DOMNode{
			Tag: "div",
			Children: []*DOMNode{
				{Tag: "span", Text: "old child"},
			},
		},
		NodeIndex: make(map[string]*DOMNode),
	}

	newTree := &DOMTree{
		Root:      &DOMNode{Tag: "div", Children: []*DOMNode{}},
		NodeIndex: make(map[string]*DOMNode),
	}

	result := engine.ComputeDiff(oldTree, newTree)

	assert.NotNil(t, result)
	assert.Greater(t, result.RemovedNodes, 0)
}

func TestDOMDiffEngine_compareAttributes(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	oldAttrs := map[string]string{
		"class": "old-class",
		"id":    "same-id",
		"data-old": "removed",
	}

	newAttrs := map[string]string{
		"class": "new-class",
		"id":    "same-id",
		"data-new": "added",
	}

	changes := engine.compareAttributes(oldAttrs, newAttrs)

	assert.Greater(t, len(changes), 0)

	// 检查是否有 class 变化
	hasClassChange := false
	for _, change := range changes {
		if change.Key == "class" {
			hasClassChange = true
			assert.Equal(t, "old-class", change.OldValue)
			assert.Equal(t, "new-class", change.NewValue)
		}
	}
	assert.True(t, hasClassChange)
}

func TestDOMDiffEngine_shouldIgnoreAttribute(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	assert.True(t, engine.shouldIgnoreAttribute("data-reactid"))
	assert.True(t, engine.shouldIgnoreAttribute("data-reactroot"))
	assert.False(t, engine.shouldIgnoreAttribute("class"))
	assert.False(t, engine.shouldIgnoreAttribute("id"))
}

func TestDOMDiffEngine_countNodes(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	node := &DOMNode{
		Tag: "div",
		Children: []*DOMNode{
			{Tag: "span", Children: []*DOMNode{
				{Tag: "b"},
			}},
			{Tag: "p"},
		},
	}

	count := engine.countNodes(node)
	assert.Equal(t, 4, count)
}

func TestDOMDiffEngine_ParseHTML(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	html := `<div class="container"><p>Hello</p></div>`
	tree, err := engine.ParseHTML(html)

	assert.NoError(t, err)
	assert.NotNil(t, tree)
	assert.NotNil(t, tree.Root)
	assert.Greater(t, len(tree.Root.Children), 0)
}

func TestDOMDiffEngine_computeHash(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	node1 := &DOMNode{
		Tag:  "div",
		Text: "content",
		Attributes: map[string]string{
			"class": "test",
		},
	}

	node2 := &DOMNode{
		Tag:  "div",
		Text: "content",
		Attributes: map[string]string{
			"class": "test",
		},
	}

	hash1 := engine.computeHash(node1)
	hash2 := engine.computeHash(node2)

	assert.Equal(t, hash1, hash2)

	// 不同内容应该有不同的哈希
	node3 := &DOMNode{
		Tag:  "div",
		Text: "different",
	}
	hash3 := engine.computeHash(node3)
	assert.NotEqual(t, hash1, hash3)
}

func TestDOMDiffEngine_ApplyDiffs(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	tree := &DOMTree{
		Root: &DOMNode{
			Tag:        "div",
			Children:   make([]*DOMNode, 0),
			Attributes: make(map[string]string),
		},
		NodeIndex: make(map[string]*DOMNode),
	}

	newNode := &DOMNode{
		ID:   "new-node",
		Tag:  "span",
		Text: "new content",
	}

	diffs := []DOMDiff{
		{
			Type:       "added",
			Node:       newNode,
			ParentPath: "", // root 路径
		},
	}

	err := engine.ApplyDiffs(tree, diffs)

	assert.NoError(t, err)
	// 节点应该被添加到 root
	assert.NotNil(t, tree.NodeIndex["new-node"])
}

func TestDOMDiffEngine_GetPatch(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	diffs := []DOMDiff{
		{
			Type: "added",
			Node: &DOMNode{
				ID:   "node-1",
				Tag:  "span",
				Text: "text",
			},
			ParentPath: "/div",
		},
		{
			Type: "modified",
			Node: &DOMNode{
				ID:   "node-2",
				Tag:  "p",
			},
			OldNode: &DOMNode{
				ID:   "node-2",
				Tag:  "p",
				Text: "old",
			},
			ParentPath: "/div",
			Changes: []Change{
				{Field: "text", OldValue: "old", NewValue: "new"},
			},
		},
	}

	patch := engine.GetPatch(diffs)

	assert.NotNil(t, patch)
	assert.NotNil(t, patch["operations"])
	ops := patch["operations"].([]map[string]interface{})
	assert.Len(t, ops, 2)
}

func TestDOMDiffEngine_GetChangedElements(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	result := &DiffResult{
		Diffs: []DOMDiff{
			{
				Type: "added",
				Node: &DOMNode{ID: "new-1"},
			},
			{
				Type: "removed",
				OldNode: &DOMNode{ID: "old-1"},
			},
		},
	}

	elements := engine.GetChangedElements(result)

	assert.Len(t, elements, 2)
	assert.Contains(t, elements, "new-1")
	assert.Contains(t, elements, "old-1")
}

func TestDOMDiffEngine_GetChangedPaths(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	result := &DiffResult{
		Diffs: []DOMDiff{
			{
				Type: "added",
				ParentPath: "/div/section[0]",
			},
			{
				Type: "modified",
				ParentPath: "/div/p[0]",
			},
		},
	}

	paths := engine.GetChangedPaths(result)

	assert.Len(t, paths, 2)
	assert.Contains(t, paths, "/div/section[0]")
	assert.Contains(t, paths, "/div/p[0]")
}

func TestDOMNode(t *testing.T) {
	node := &DOMNode{
		ID:    "test-node",
		Tag:   "div",
		Text:  "content",
		Attributes: map[string]string{
			"class": "container",
			"id":    "main",
		},
		Hash: "abc123",
	}

	assert.NotNil(t, node)
	assert.Equal(t, "test-node", node.ID)
	assert.Equal(t, "div", node.Tag)
	assert.Equal(t, "content", node.Text)
	assert.Len(t, node.Attributes, 2)
}

func TestDOMTree(t *testing.T) {
	root := &DOMNode{
		Tag: "root",
		Children: []*DOMNode{
			{Tag: "child1"},
			{Tag: "child2"},
		},
	}

	tree := &DOMTree{
		Root:      root,
		NodeIndex: map[string]*DOMNode{"child1": {Tag: "child1"}},
	}

	assert.NotNil(t, tree)
	assert.Equal(t, root, tree.Root)
	assert.Len(t, tree.NodeIndex, 1)
}

func TestDOMDiff(t *testing.T) {
	diff := DOMDiff{
		Type:       "modified",
		Node:       &DOMNode{ID: "node1", Tag: "div"},
		OldNode:    &DOMNode{ID: "node1", Tag: "span"},
		ParentPath: "/root",
		Index:      0,
		Changes: []Change{
			{Field: "tag", OldValue: "span", NewValue: "div"},
		},
	}

	assert.Equal(t, "modified", diff.Type)
	assert.NotNil(t, diff.Node)
	assert.NotNil(t, diff.OldNode)
	assert.Len(t, diff.Changes, 1)
}

func TestChange(t *testing.T) {
	change := Change{
		Field:    "attributes",
		Key:      "class",
		OldValue: "old-class",
		NewValue: "new-class",
	}

	assert.Equal(t, "attributes", change.Field)
	assert.Equal(t, "class", change.Key)
	assert.Equal(t, "old-class", change.OldValue)
	assert.Equal(t, "new-class", change.NewValue)
}

func TestDiffResult(t *testing.T) {
	result := &DiffResult{
		Diffs:        make([]DOMDiff, 0),
		TotalNodes:   100,
		ChangedNodes: 10,
		AddedNodes:   5,
		RemovedNodes: 3,
		Duration:     50,
	}

	assert.NotNil(t, result)
	assert.Equal(t, 100, result.TotalNodes)
	assert.Equal(t, 10, result.ChangedNodes)
	assert.Equal(t, 5, result.AddedNodes)
	assert.Equal(t, 3, result.RemovedNodes)
	assert.Equal(t, int64(50), result.Duration)
}

func TestDOMDiffEngine_NilConfig(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.config)
	assert.Equal(t, true, engine.config.EnableHashCache)
}

func TestDOMDiffEngine_CustomConfig(t *testing.T) {
	config := &DOMDiffConfig{
		EnableHashCache:  false,
		MaxDepth:         10,
		IgnoreWhitespace: false,
	}

	engine := NewDOMDiffEngine(config)

	assert.NotNil(t, engine)
	assert.Equal(t, false, engine.config.EnableHashCache)
	assert.Equal(t, 10, engine.config.MaxDepth)
	assert.Equal(t, false, engine.config.IgnoreWhitespace)
}

func TestFindNodeByPath(t *testing.T) {
	engine := NewDOMDiffEngine(nil)

	root := &DOMNode{
		Tag: "root",
		Children: []*DOMNode{
			{
				Tag: "div",
				Children: []*DOMNode{
					{Tag: "span", ID: "target"},
				},
			},
		},
	}

	node := engine.findNodeByPath(root, "/root/div")
	assert.NotNil(t, node)
	assert.Equal(t, "div", node.Tag)
}
