package incremental

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
	"sync"
	"time"
)

// DOMNode DOM 节点
type DOMNode struct {
	ID         string                 `json:"id"`
	Tag        string                 `json:"tag"`
	Text       string                 `json:"text,omitempty"`
	Attributes map[string]string      `json:"attributes,omitempty"`
	Children   []*DOMNode             `json:"children,omitempty"`
	Parent     *DOMNode               `json:"-"`
	Hash       string                 `json:"hash"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// DOMDiff DOM 差异
type DOMDiff struct {
	Type       string   `json:"type"` // added, removed, modified, moved
	Node       *DOMNode `json:"node"`
	OldNode    *DOMNode `json:"old_node,omitempty"`
	ParentPath string   `json:"parent_path"`
	Index      int      `json:"index"`
	Changes    []Change `json:"changes,omitempty"`
}

// Change 属性或文本变化
type Change struct {
	Field    string      `json:"field"` // attributes, text
	Key      string      `json:"key,omitempty"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// DOMTree DOM 树
type DOMTree struct {
	Root      *DOMNode            `json:"root"`
	NodeIndex map[string]*DOMNode `json:"-"` // ID -> Node 映射
	mu        sync.RWMutex
}

// DiffResult 差异结果
type DiffResult struct {
	Diffs        []DOMDiff `json:"diffs"`
	TotalNodes   int       `json:"total_nodes"`
	ChangedNodes int       `json:"changed_nodes"`
	AddedNodes   int       `json:"added_nodes"`
	RemovedNodes int       `json:"removed_nodes"`
	Duration     int64     `json:"duration_ms"`
}

// DOMDiffEngine DOM 差异引擎
type DOMDiffEngine struct {
	config *DOMDiffConfig
	pool   sync.Pool
	mu     sync.Mutex
}

// DOMDiffConfig 差异配置
type DOMDiffConfig struct {
	EnableHashCache    bool          `json:"enable_hash_cache"`
	MaxDepth           int           `json:"max_depth"`
	IgnoreWhitespace   bool          `json:"ignore_whitespace"`
	IgnoreAttributes   []string      `json:"ignore_attributes"`
	CompareTextContent bool          `json:"compare_text_content"`
	Timeout            time.Duration `json:"timeout"`
}

// DefaultDOMDiffConfig 返回默认配置
func DefaultDOMDiffConfig() *DOMDiffConfig {
	return &DOMDiffConfig{
		EnableHashCache:    true,
		MaxDepth:           50,
		IgnoreWhitespace:   true,
		IgnoreAttributes:   []string{"data-reactid", "data-reactroot"},
		CompareTextContent: true,
		Timeout:            5 * time.Second,
	}
}

// NewDOMDiffEngine 创建 DOM 差异引擎
func NewDOMDiffEngine(config *DOMDiffConfig) *DOMDiffEngine {
	if config == nil {
		config = DefaultDOMDiffConfig()
	}

	engine := &DOMDiffEngine{
		config: config,
		pool: sync.Pool{
			New: func() interface{} {
				return sha256.New()
			},
		},
	}

	return engine
}

// ComputeDiff 计算两个 DOM 树的差异
func (e *DOMDiffEngine) ComputeDiff(oldTree, newTree *DOMTree) *DiffResult {
	startTime := time.Now()
	result := &DiffResult{
		Diffs: make([]DOMDiff, 0),
	}

	// 计算差异
	e.computeNodeDiff(oldTree.Root, newTree.Root, "", 0, result)

	result.Duration = time.Since(startTime).Milliseconds()
	result.TotalNodes = e.countNodes(newTree.Root)

	return result
}

// computeNodeDiff 递归计算节点差异
func (e *DOMDiffEngine) computeNodeDiff(oldNode, newNode *DOMNode, parentPath string, depth int, result *DiffResult) {
	if depth > e.config.MaxDepth {
		return
	}

	// 处理新增节点
	if oldNode == nil && newNode != nil {
		result.Diffs = append(result.Diffs, DOMDiff{
			Type:       "added",
			Node:       newNode,
			ParentPath: parentPath,
		})
		result.AddedNodes++
		result.ChangedNodes++
		return
	}

	// 处理删除节点
	if oldNode != nil && newNode == nil {
		result.Diffs = append(result.Diffs, DOMDiff{
			Type:       "removed",
			OldNode:    oldNode,
			ParentPath: parentPath,
		})
		result.RemovedNodes++
		result.ChangedNodes++
		return
	}

	// 比较节点
	if oldNode != nil && newNode != nil {
		// 检查 Hash 是否相同
		if e.config.EnableHashCache && oldNode.Hash != "" && newNode.Hash != "" && oldNode.Hash == newNode.Hash {
			return // 无变化
		}

		// 比较标签
		if oldNode.Tag != newNode.Tag {
			result.Diffs = append(result.Diffs, DOMDiff{
				Type:       "modified",
				Node:       newNode,
				OldNode:    oldNode,
				ParentPath: parentPath,
				Changes: []Change{{
					Field:    "tag",
					OldValue: oldNode.Tag,
					NewValue: newNode.Tag,
				}},
			})
			result.ChangedNodes++
			return
		}

		// 比较属性
		attrChanges := e.compareAttributes(oldNode.Attributes, newNode.Attributes)
		if len(attrChanges) > 0 {
			result.Diffs = append(result.Diffs, DOMDiff{
				Type:       "modified",
				Node:       newNode,
				OldNode:    oldNode,
				ParentPath: parentPath,
				Changes:    attrChanges,
			})
			result.ChangedNodes++
		}

		// 比较文本
		if e.config.CompareTextContent && oldNode.Text != newNode.Text {
			oldText := oldNode.Text
			newText := newNode.Text
			if e.config.IgnoreWhitespace {
				oldText = strings.TrimSpace(oldText)
				newText = strings.TrimSpace(newText)
			}
			if oldText != newText {
				result.Diffs = append(result.Diffs, DOMDiff{
					Type:       "modified",
					Node:       newNode,
					OldNode:    oldNode,
					ParentPath: parentPath,
					Changes: append(attrChanges, Change{
						Field:    "text",
						OldValue: oldNode.Text,
						NewValue: newNode.Text,
					}),
				})
				result.ChangedNodes++
			}
		}

		// 递归比较子节点
		maxLen := max(len(oldNode.Children), len(newNode.Children))
		for i := 0; i < maxLen; i++ {
			childPath := fmt.Sprintf("%s/%s[%d]", parentPath, newNode.Tag, i)
			var oldChild, newChild *DOMNode
			if i < len(oldNode.Children) {
				oldChild = oldNode.Children[i]
			}
			if i < len(newNode.Children) {
				newChild = newNode.Children[i]
			}
			e.computeNodeDiff(oldChild, newChild, childPath, depth+1, result)
		}
	}
}

// compareAttributes 比较属性
func (e *DOMDiffEngine) compareAttributes(oldAttrs, newAttrs map[string]string) []Change {
	changes := make([]Change, 0)

	// 检查删除的属性
	for key := range oldAttrs {
		if e.shouldIgnoreAttribute(key) {
			continue
		}
		if _, exists := newAttrs[key]; !exists {
			changes = append(changes, Change{
				Field:    "attributes",
				Key:      key,
				OldValue: oldAttrs[key],
				NewValue: nil,
			})
		}
	}

	// 检查新增或修改的属性
	for key, newValue := range newAttrs {
		if e.shouldIgnoreAttribute(key) {
			continue
		}
		oldValue, exists := oldAttrs[key]
		if !exists || oldValue != newValue {
			changes = append(changes, Change{
				Field:    "attributes",
				Key:      key,
				OldValue: oldValue,
				NewValue: newValue,
			})
		}
	}

	return changes
}

// shouldIgnoreAttribute 检查是否应该忽略属性
func (e *DOMDiffEngine) shouldIgnoreAttribute(key string) bool {
	for _, ignore := range e.config.IgnoreAttributes {
		if key == ignore || strings.HasPrefix(key, ignore) {
			return true
		}
	}
	return false
}

// countNodes 计算节点数
func (e *DOMDiffEngine) countNodes(node *DOMNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += e.countNodes(child)
	}
	return count
}

// ParseHTML 解析 HTML 为 DOM 树（简化实现）
func (e *DOMDiffEngine) ParseHTML(html string) (*DOMTree, error) {
	tree := &DOMTree{
		Root: &DOMNode{
			Tag:        "root",
			Attributes: make(map[string]string),
			Children:   make([]*DOMNode, 0),
		},
		NodeIndex: make(map[string]*DOMNode),
	}

	// 简化解析：实际应该使用 HTML 解析器
	// 这里仅做演示
	node := e.createNode("div", html)
	tree.Root.Children = append(tree.Root.Children, node)
	node.Parent = tree.Root

	return tree, nil
}

// createNode 创建节点
func (e *DOMDiffEngine) createNode(tag, content string) *DOMNode {
	node := &DOMNode{
		ID:         generateID(),
		Tag:        tag,
		Text:       content,
		Attributes: make(map[string]string),
		Children:   make([]*DOMNode, 0),
	}
	node.Hash = e.computeHash(node)
	return node
}

// computeHash 计算节点哈希
func (e *DOMDiffEngine) computeHash(node *DOMNode) string {
	h := e.pool.Get().(hash.Hash)
	defer e.pool.Put(h)
	h.Reset()

	h.Write([]byte(node.Tag))
	h.Write([]byte(node.Text))

	// 排序属性
	keys := make([]string, 0, len(node.Attributes))
	for k := range node.Attributes {
		if !e.shouldIgnoreAttribute(k) {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(node.Attributes[k]))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// generateID 生成 ID
func generateID() string {
	return fmt.Sprintf("node_%d", time.Now().UnixNano())
}

// max 返回较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ApplyDiffs 应用差异
func (e *DOMDiffEngine) ApplyDiffs(tree *DOMTree, diffs []DOMDiff) error {
	tree.mu.Lock()
	defer tree.mu.Unlock()

	for _, diff := range diffs {
		switch diff.Type {
		case "added":
			e.applyAddedNode(tree, diff)
		case "removed":
			e.applyRemovedNode(tree, diff)
		case "modified":
			e.applyModifiedNode(tree, diff)
		}
	}

	return nil
}

// applyAddedNode 应用新增节点
func (e *DOMDiffEngine) applyAddedNode(tree *DOMTree, diff DOMDiff) {
	parent := e.findNodeByPath(tree.Root, diff.ParentPath)
	if parent != nil {
		parent.Children = append(parent.Children, diff.Node)
		diff.Node.Parent = parent
		tree.NodeIndex[diff.Node.ID] = diff.Node
	}
}

// applyRemovedNode 应用删除节点
func (e *DOMDiffEngine) applyRemovedNode(tree *DOMTree, diff DOMDiff) {
	if diff.OldNode != nil && diff.OldNode.Parent != nil {
		children := diff.OldNode.Parent.Children
		for i, child := range children {
			if child == diff.OldNode {
				children = append(children[:i], children[i+1:]...)
				diff.OldNode.Parent.Children = children
				delete(tree.NodeIndex, diff.OldNode.ID)
				break
			}
		}
	}
}

// applyModifiedNode 应用修改节点
func (e *DOMDiffEngine) applyModifiedNode(tree *DOMTree, diff DOMDiff) {
	if diff.OldNode != nil {
		// 更新属性
		for _, change := range diff.Changes {
			if change.Field == "attributes" {
				if change.NewValue == nil {
					delete(diff.Node.Attributes, change.Key)
				} else {
					diff.Node.Attributes[change.Key] = change.NewValue.(string)
				}
			} else if change.Field == "text" {
				diff.Node.Text = change.NewValue.(string)
			}
		}
		diff.Node.Hash = e.computeHash(diff.Node)
		tree.NodeIndex[diff.Node.ID] = diff.Node
	}
}

// findNodeByPath 通过路径查找节点
func (e *DOMDiffEngine) findNodeByPath(root *DOMNode, path string) *DOMNode {
	if path == "" {
		return root
	}

	parts := strings.Split(path, "/")
	current := root

	for _, part := range parts {
		if part == "" || part == "root" {
			continue
		}
		// 解析 tag[index]
		tagIndex := strings.Split(part, "[")
		if len(tagIndex) < 1 {
			continue
		}
		tag := tagIndex[0]

		// 查找子节点
		found := false
		for _, child := range current.Children {
			if child.Tag == tag {
				current = child
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	return current
}

// GetPatch 获取补丁（用于前端应用）
func (e *DOMDiffEngine) GetPatch(diffs []DOMDiff) map[string]interface{} {
	operations := make([]map[string]interface{}, 0)

	for _, diff := range diffs {
		op := make(map[string]interface{})
		switch diff.Type {
		case "added":
			op["op"] = "add"
			op["path"] = diff.ParentPath + "/" + diff.Node.ID
			op["value"] = e.nodeToMap(diff.Node)
		case "removed":
			op["op"] = "remove"
			op["path"] = diff.ParentPath + "/" + diff.OldNode.ID
		case "modified":
			op["op"] = "replace"
			op["path"] = diff.ParentPath + "/" + diff.Node.ID
			op["changes"] = diff.Changes
		}
		operations = append(operations, op)
	}

	return map[string]interface{}{
		"operations": operations,
	}
}

// nodeToMap 节点转 Map
func (e *DOMDiffEngine) nodeToMap(node *DOMNode) map[string]interface{} {
	return map[string]interface{}{
		"id":         node.ID,
		"tag":        node.Tag,
		"text":       node.Text,
		"attributes": node.Attributes,
	}
}

// GetChangedElements 获取变化的元素（用于选择性渲染）
func (e *DOMDiffEngine) GetChangedElements(result *DiffResult) []string {
	elements := make([]string, 0)
	for _, diff := range result.Diffs {
		if diff.Node != nil {
			elements = append(elements, diff.Node.ID)
		}
		if diff.OldNode != nil {
			elements = append(elements, diff.OldNode.ID)
		}
	}
	return elements
}

// GetChangedPaths 获取变化的路径
func (e *DOMDiffEngine) GetChangedPaths(result *DiffResult) []string {
	paths := make([]string, 0)
	for _, diff := range result.Diffs {
		paths = append(paths, diff.ParentPath)
	}
	return paths
}
