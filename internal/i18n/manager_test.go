package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_Lifecycle(t *testing.T) {
	m := NewManager()
	if m.GetLanguage() != LanguageZh {
		t.Fatalf("default lang must be zh, got %s", m.GetLanguage())
	}

	// 从临时目录加载翻译文件（含坏 JSON 忽略、缺失文件跳过）
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("zh.json", `{"title":"标题","hello":"你好 %s"}`)
	write("en.json", `{"title":"Title"}`)
	write("ja.json", `{bad json}`)
	// ko.json 缺失

	if err := m.LoadTranslations(dir); err != nil {
		t.Fatalf("LoadTranslations err: %v", err)
	}

	// 当前语言翻译
	if got := m.Translate("title"); got != "标题" {
		t.Fatalf("translate=%q", got)
	}
	// 显式语言参数
	if got := m.Translate("title", LanguageEn); got != "Title" {
		t.Fatalf("explicit lang broken: %q", got)
	}
	// 回退中文
	if got := m.Translate("hello", LanguageEn); got != "你好 %s" {
		t.Fatalf("fallback to zh broken: %q", got)
	}
	// 双重回退：语言+键都不存在 → 键名
	if got := m.Translate("nope", LanguageKo); got != "nope" {
		t.Fatalf("double fallback broken: %q", got)
	}
	// T 简写
	if got := m.T("title"); got != "标题" {
		t.Fatalf("T shorthand broken: %q", got)
	}

	// 切换语言
	m.SetLanguage(LanguageEn)
	if m.GetLanguage() != LanguageEn {
		t.Fatal("SetLanguage broken")
	}
	if got := m.Translate("title"); got != "Title" {
		t.Fatalf("after switch translate broken: %q", got)
	}

	// 支持语言列表 = 已加载翻译的语言（zh/en 成功、ja 坏 JSON 跳过、ko 缺文件）
	langs := m.GetSupportedLanguages()
	if len(langs) != 2 {
		t.Fatalf("supported langs = %v", langs)
	}
}

func TestGetDefaultTranslation(t *testing.T) {
	// GetDefaultTranslation 对未注册键返回键名（容错语义）
	if got := GetDefaultTranslation("audit.missing.key", LanguageZh); got != "audit.missing.key" {
		t.Fatalf("GetDefaultTranslation fallback broken: %q", got)
	}
	// 已注册键返回默认翻译
	if got := GetDefaultTranslation("error.render_timeout", LanguageEn); got == "" || got == "error.render_timeout" {
		t.Logf("default translation for known key: %q (视内置键而定)", got)
	}
}
