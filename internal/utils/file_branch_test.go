package utils

import (
	"archive/zip"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractArchive_Branches(t *testing.T) {
	dir := t.TempDir()

	// 不支持的格式
	if err := ExtractArchive(filepath.Join(dir, "a.tar.gz"), dir); err == nil {
		t.Fatal("unsupported format must error")
	}
	// RAR 明确提示
	if err := ExtractArchive(filepath.Join(dir, "a.rar"), dir); err == nil || !strings.Contains(err.Error(), "RAR") {
		t.Fatalf("rar branch broken: %v", err)
	}

	// ZIP 成功解压（含子目录）
	zipPath := filepath.Join(dir, "ok.zip")
	if err := makeTestZip(zipPath); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := ExtractArchive(zipPath, out); err != nil {
		t.Fatalf("zip extract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "sub", "inner.txt")); err != nil {
		t.Fatalf("nested file missing: %v", err)
	}

	// ZIP 恶意路径（zip slip 防御）
	evil := filepath.Join(dir, "evil.zip")
	if err := makeEvilZip(evil); err != nil {
		t.Fatal(err)
	}
	if err := ExtractArchive(evil, filepath.Join(dir, "out2")); err == nil {
		t.Fatal("zip slip must be rejected")
	}
}

func makeTestZip(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, e := range []struct{ name, body string }{
		{"root.txt", "root"},
		{"sub/inner.txt", "inner"},
	} {
		w, err := zw.Create(e.name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			return err
		}
	}
	return zw.Close()
}

func makeEvilZip(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create("../escape.txt")
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("evil"))
	if err != nil {
		return err
	}
	return zw.Close()
}

func TestIsPortAvailable_Branches(t *testing.T) {
	// 占用分支：与实现相同的通配地址绑定，确保冲突
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if IsPortAvailable(port) {
		t.Fatal("occupied (wildcard) port must be unavailable")
	}
	ln.Close()
	if !IsPortAvailable(port) {
		t.Fatal("freed port must be available")
	}
}

// ExtractZIP 完整分支：目录条目/子目录创建/根级文件
func TestExtractZIP_StructureBranches(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "struct.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	hdrDir := &zip.FileHeader{Name: "dir/", Method: zip.Store}
	hdrDir.SetMode(os.ModeDir | 0755)
	if w, err := zw.CreateHeader(hdrDir); err != nil {
		t.Fatal(err)
	} else {
		w.Write(nil)
	}
	hdrFile := &zip.FileHeader{Name: "dir/file.txt", Method: zip.Deflate}
	hdrFile.SetMode(0644)
	if w, err := zw.CreateHeader(hdrFile); err != nil {
		t.Fatal(err)
	} else {
		w.Write([]byte("content"))
	}
	hdrTop := &zip.FileHeader{Name: "toplevel.txt", Method: zip.Deflate}
	hdrTop.SetMode(0644)
	if w, err := zw.CreateHeader(hdrTop); err != nil {
		t.Fatal(err)
	} else {
		w.Write([]byte("content"))
	}
	zw.Close()
	f.Close()

	out := filepath.Join(dir, "out")
	if err := ExtractZIP(zipPath, out); err != nil {
		t.Fatalf("structured zip extract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "dir", "file.txt")); err != nil {
		t.Fatalf("dir entry handling broken: %v", err)
	}
	// 损坏 zip → OpenReader 错误
	bad := filepath.Join(dir, "bad.zip")
	os.WriteFile(bad, []byte("not a zip"), 0644)
	if err := ExtractZIP(bad, filepath.Join(dir, "out3")); err == nil {
		t.Fatal("corrupt zip must error")
	}
}

// IsOriginAllowed 全分支：空/静态表/自定义表
func TestIsOriginAllowed_Branches(t *testing.T) {
	if IsOriginAllowed("") {
		t.Fatal("empty origin must be rejected")
	}
	SetAllowedOrigins([]string{"https://custom.example"})
	if !IsOriginAllowed("https://custom.example") {
		t.Fatal("custom origin must be allowed")
	}
	if IsOriginAllowed("https://other.example") {
		t.Fatal("unlisted origin must be rejected")
	}
	// 静态表命一路径（已知安全 origin；若表为空则跳过）
	for o := range allowedOriginsStatic {
		if !IsOriginAllowed(o) {
			t.Fatalf("static origin %q must be allowed", o)
		}
		break
	}
}

// 静态 origin 表命中分支（同包注入表项）
func TestIsOriginAllowed_StaticTable(t *testing.T) {
	allowedOriginsStatic["https://static-inject.example"] = true
	t.Cleanup(func() { delete(allowedOriginsStatic, "https://static-inject.example") })
	if !IsOriginAllowed("https://static-inject.example") {
		t.Fatal("static table hit must be allowed")
	}
}

// ZIP 错误分支注入：目录冲突(MkdirAll fail)/坏 CRC(io.Copy fail)/非法模式
func TestExtractZIP_ErrorBranches(t *testing.T) {
	dir := t.TempDir()

	// 目录冲突：先写文件 "conflict"，zip 内含 "conflict/inner" → MkdirAll 失败
	conflict := filepath.Join(dir, "conflict")
	os.WriteFile(conflict, []byte("file"), 0644)
	zp1 := filepath.Join(dir, "conflict.zip")
	f1, _ := os.Create(zp1)
	zw1 := zip.NewWriter(f1)
	h1 := &zip.FileHeader{Name: "conflict/inner.txt"}
	h1.SetMode(0644)
	if w, err := zw1.CreateHeader(h1); err == nil {
		w.Write([]byte("x"))
	}
	zw1.Close()
	f1.Close()
	if err := ExtractZIP(zp1, dir); err == nil {
		t.Fatal("dir/file conflict must error (MkdirAll branch)")
	}

	// 坏 CRC：生成合法 zip 后翻转数据字节 → 读取时 CRC 校验失败 → io.Copy 分支
	badCrc := filepath.Join(dir, "badcrc.zip")
	f2, _ := os.Create(badCrc)
	zw2 := zip.NewWriter(f2)
	h2 := &zip.FileHeader{Name: "crc.txt", Method: zip.Store}
	h2.SetMode(0644)
	if w, err := zw2.CreateHeader(h2); err == nil {
		w.Write([]byte("1234567890"))
	}
	zw2.Close()
	f2.Close()
	raw, _ := os.ReadFile(badCrc)
	// 本地文件头 30B + 文件名 7B = 数据起点；翻转首数据字节破坏 CRC
	if len(raw) > 40 {
		raw[38] ^= 0xFF
	}
	os.WriteFile(badCrc, raw, 0644)
	if err := ExtractZIP(badCrc, filepath.Join(dir, "out-crc")); err == nil {
		t.Fatal("bad CRC must error (io.Copy branch)")
	}
}

// 剩余错误分支：destDir MkdirAll 失败 / 目录条目冲突 / 只读目录 OpenFile 失败 / 未知压缩方法
func TestExtractZIP_RemainingErrorBranches(t *testing.T) {
	dir := t.TempDir()

	// destDir 上层是文件 → MkdirAll(destDir) 失败（59 分支）
	blocker := filepath.Join(dir, "blocker")
	os.WriteFile(blocker, []byte("f"), 0644)
	if err := ExtractZIP(makeSimpleZip(t, "x.txt"), filepath.Join(blocker, "sub")); err == nil {
		t.Fatal("destDir under file must error (MkdirAll)")
	}

	// 目录条目与既有文件冲突 → MkdirAll(dir entry) 失败（72 分支）
	conflict := filepath.Join(dir, "conflict2")
	os.WriteFile(conflict, []byte("f"), 0644)
	zp := filepath.Join(dir, "dirc.zip")
	f, _ := os.Create(zp)
	zw := zip.NewWriter(f)
	h := &zip.FileHeader{Name: "conflict2/"}
	h.SetMode(os.ModeDir | 0755)
	if w, err := zw.CreateHeader(h); err == nil {
		w.Write(nil)
	}
	zw.Close()
	f.Close()
	if err := ExtractZIP(zp, dir); err == nil {
		t.Fatal("dir entry conflicting with file must error (72)")
	}

	// 只读目录：目录条目 MkdirAll 成功但 OpenFile 失败（83 分支）
	roDir := filepath.Join(dir, "ro")
	os.MkdirAll(filepath.Join(roDir, "sub"), 0o755)
	os.Chmod(filepath.Join(roDir, "sub"), 0o555)
	defer os.Chmod(filepath.Join(roDir, "sub"), 0o755)
	zp2 := filepath.Join(dir, "ro.zip")
	f2, _ := os.Create(zp2)
	zw2 := zip.NewWriter(f2)
	h2 := &zip.FileHeader{Name: "sub/f.txt"}
	h2.SetMode(0644)
	if w, err := zw2.CreateHeader(h2); err == nil {
		w.Write([]byte("x"))
	}
	zw2.Close()
	f2.Close()
	if err := ExtractZIP(zp2, roDir); err == nil {
		t.Fatal("readonly dest must error (OpenFile 83)")
	}

	// 加密标志条目 → file.Open 返回 ErrUnsupported（88 分支）：
	// zip.Writer 无法设置加密位，直接改写已生成 zip 的 general purpose flag（本地头偏移 6）
	zp3 := filepath.Join(dir, "enc.zip")
	f3, _ := os.Create(zp3)
	zw3 := zip.NewWriter(f3)
	h3 := &zip.FileHeader{Name: "enc.txt"}
	h3.SetMode(0644)
	if w, err := zw3.CreateHeader(h3); err == nil {
		w.Write([]byte("x"))
	}
	zw3.Close()
	f3.Close()
	raw3, _ := os.ReadFile(zp3)
	if len(raw3) > 4 {
		raw3[0] ^= 0xFF // 破坏本地头签名：中心目录完好，findBodyOffset 报错 → Open 失败（88 分支）
	}
	os.WriteFile(zp3, raw3, 0644)
	if err := ExtractZIP(zp3, filepath.Join(dir, "out-m")); err == nil {
		t.Fatal("corrupt local header must error (Open 88)")
	}
}

func makeSimpleZip(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.zip")
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	h := &zip.FileHeader{Name: name}
	h.SetMode(0644)
	if w, err := zw.CreateHeader(h); err == nil {
		w.Write([]byte("x"))
	}
	zw.Close()
	f.Close()
	return p
}
