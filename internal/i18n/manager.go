package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Language 语言类型
type Language string

const (
	LanguageZh Language = "zh"
	LanguageEn Language = "en"
	LanguageJa Language = "ja"
	LanguageKo Language = "ko"
)

// Translations 翻译数据
type Translations map[string]string

// Manager 国际化管理器
type Manager struct {
	translations map[Language]Translations
	currentLang  Language
	mu           sync.RWMutex
}

// NewManager 创建国际化管理器
func NewManager() *Manager {
	return &Manager{
		translations: make(map[Language]Translations),
		currentLang:  LanguageZh,
	}
}

// LoadTranslations 加载翻译文件
func (m *Manager) LoadTranslations(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 支持的语言
	languages := []Language{LanguageZh, LanguageEn, LanguageJa, LanguageKo}

	for _, lang := range languages {
		filePath := filepath.Join(dir, fmt.Sprintf("%s.json", lang))
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var translations Translations
		if err := json.Unmarshal(data, &translations); err != nil {
			continue
		}

		m.translations[lang] = translations
	}

	return nil
}

// SetLanguage 设置当前语言
func (m *Manager) SetLanguage(lang Language) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentLang = lang
}

// GetLanguage 获取当前语言
func (m *Manager) GetLanguage() Language {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentLang
}

// Translate 获取翻译
func (m *Manager) Translate(key string, lang ...Language) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	l := m.currentLang
	if len(lang) > 0 {
		l = lang[0]
	}

	if translations, ok := m.translations[l]; ok {
		if value, ok := translations[key]; ok {
			return value
		}
	}

	// 回退到中文
	if l != LanguageZh {
		if translations, ok := m.translations[LanguageZh]; ok {
			if value, ok := translations[key]; ok {
				return value
			}
		}
	}

	return key
}

// T 翻译的简写
func (m *Manager) T(key string) string {
	return m.Translate(key)
}

// GetSupportedLanguages 获取支持的语言列表
func (m *Manager) GetSupportedLanguages() []Language {
	m.mu.RLock()
	defer m.mu.RUnlock()

	languages := make([]Language, 0, len(m.translations))
	for lang := range m.translations {
		languages = append(languages, lang)
	}
	return languages
}

// 默认翻译
var defaultTranslations = map[string]Translations{
	"zh": {
		"common.loading": "加载中...",
		"common.success": "操作成功",
		"common.error":   "操作失败",
		"menu.overview":  "概览",
		"menu.sites":     "站点管理",
		"menu.firewall":  "防火墙",
	},
	"en": {
		"common.loading": "Loading...",
		"common.success": "Operation Successful",
		"common.error":   "Operation Failed",
		"menu.overview":  "Overview",
		"menu.sites":     "Sites",
		"menu.firewall":  "Firewall",
	},
	"ja": {
		"common.loading": "読み込み中...",
		"common.success": "操作成功",
		"common.error":   "操作失敗",
		"menu.overview":  "概要",
		"menu.sites":     "サイト管理",
		"menu.firewall":  "ファイアウォール",
	},
	"ko": {
		"common.loading": "로딩 중...",
		"common.success": "작업 성공",
		"common.error":   "작업 실패",
		"menu.overview":  "개요",
		"menu.sites":     "사이트 관리",
		"menu.firewall":  "방화벽",
	},
}

// GetDefaultTranslation 获取默认翻译
func GetDefaultTranslation(key string, lang Language) string {
	if translations, ok := defaultTranslations[string(lang)]; ok {
		if value, ok := translations[key]; ok {
			return value
		}
	}
	return key
}
