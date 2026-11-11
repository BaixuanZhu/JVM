// cache_zip_test.go 覆盖缓存 zip 生命周期联动的各分支: 纯函数 (路径约定、
// 大小格式化、询问文案)、update 顺带清理 (含 zip 删除失败宽容与目录删除失败
// 保留 zip)、uninstall 的询问/保留/-y 默认与无缓存静默。
//
// Windows 删除失败用共享冲突句柄模拟 (以 FILE_SHARE_READ 打开文件, 句柄存活
// 期间 os.Remove 报 sharing violation) —— 与"DLL 被运行中的 java 进程加载"
// 是同一阻塞机制, 比 chmod 只读可靠 (Go 的 os.Remove 会自清只读属性)。
package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"jvm/internal/paths"
)

// lockFileNoDelete 以 FILE_SHARE_READ 打开文件并返回句柄: 句柄存活期间该文件
// 无法被删除 (os.Remove 报 ERROR_SHARING_VIOLATION), 模拟文件被进程占用。
// 调用方负责 CloseHandle。
func lockFileNoDelete(t *testing.T, path string) windows.Handle {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("路径转 UTF16 失败: %v", err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatalf("CreateFile 失败: %v", err)
	}
	return h
}

// withStdin 把 os.Stdin 临时替换为喂入 input 的管道并恢复 (confirm 每次调用
// 都 bufio.NewReader(os.Stdin) 新建 reader, 换 os.Stdin 变量即可注入作答)。
// 注意: 多行输入会被首个 reader 的缓冲整块吞掉, 只适用于单次 confirm 的场景。
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("写管道失败: %v", err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})
}

func TestCacheZipPath(t *testing.T) {
	got := cacheZipPath("temurin-21.0.5+11")
	want := filepath.Join(paths.CacheDir, "temurin-21.0.5+11.zip")
	if got != want {
		t.Errorf("cacheZipPath = %q, want %q", got, want)
	}
}

func TestFormatMB(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0.0 MB"},
		{512 * 1024, "0.5 MB"},
		{1048576, "1.0 MB"},
		{200 * 1024 * 1024, "200.0 MB"},
		{191155405, "182.3 MB"}, // 182.3 MiB
	}
	for _, tt := range tests {
		if got := formatMB(tt.in); got != tt.want {
			t.Errorf("formatMB(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTotalCacheZipSize(t *testing.T) {
	zips := []cacheZip{{size: 1024}, {size: 2048}, {}}
	if got := totalCacheZipSize(zips); got != 3072 {
		t.Errorf("totalCacheZipSize = %d, want 3072", got)
	}
	if got := totalCacheZipSize(nil); got != 0 {
		t.Errorf("nil 合计 = %d, want 0", got)
	}
}

func TestCollectCacheZips(t *testing.T) {
	dir := withTempCache(t)
	os.WriteFile(filepath.Join(dir, "temurin-21.0.5+11.zip"), []byte("abc"), 0o644)
	os.WriteFile(filepath.Join(dir, "temurin-21.0.8+7.zip"), []byte("abcdefgh"), 0o644)
	// 同名目录不参与 (防御: 缓存里只认文件)
	os.Mkdir(filepath.Join(dir, "zulu-17.0.12+7.zip"), 0o755)

	got := collectCacheZips([]string{"temurin-21.0.5+11", "temurin-21.0.8+7", "zulu-17.0.12+7", "corretto-21.0.12.8.1"})
	want := []cacheZip{
		{name: "temurin-21.0.5+11.zip", size: 3},
		{name: "temurin-21.0.8+7.zip", size: 8},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectCacheZips = %v, want %v", got, want)
	}
}

func TestBuildCacheZipPrompt(t *testing.T) {
	t.Run("单个 zip: 内联括号", func(t *testing.T) {
		got := buildCacheZipPrompt([]cacheZip{{name: "temurin-21.0.5+11.zip", size: 1048576}})
		want := "是否同时删除缓存 zip? (temurin-21.0.5+11.zip, 1.0 MB) [y/N] "
		if got != want {
			t.Errorf("prompt = %q, want %q", got, want)
		}
	})
	t.Run("多个 zip: 清单与合计", func(t *testing.T) {
		got := buildCacheZipPrompt([]cacheZip{
			{name: "a.zip", size: 1048576},
			{name: "b.zip", size: 2097152},
		})
		want := "是否同时删除以下 2 个缓存 zip (合计 3.0 MB)?\n  a.zip (1.0 MB)\n  b.zip (2.0 MB)\n[y/N] "
		if got != want {
			t.Errorf("prompt = %q, want %q", got, want)
		}
	})
}

func TestRemoveCacheZipIfPresent(t *testing.T) {
	dir := withTempCache(t)
	zipFile := filepath.Join(dir, "temurin-21.0.5+11.zip")
	os.WriteFile(zipFile, []byte("x"), 0o644)

	t.Run("存在: 删除且无输出", func(t *testing.T) {
		out := captureStdout(t, func() { removeCacheZipIfPresent("temurin-21.0.5+11") })
		if out != "" {
			t.Errorf("成功删除不应有输出: %q", out)
		}
		if _, err := os.Stat(zipFile); !os.IsNotExist(err) {
			t.Error("缓存 zip 应被删除")
		}
	})
	t.Run("不存在: 静默", func(t *testing.T) {
		out := captureStdout(t, func() { removeCacheZipIfPresent("temurin-21.0.8+7") })
		if out != "" {
			t.Errorf("zip 不存在应静默: %q", out)
		}
	})
	t.Run("被占用删不掉: 宽容提示指向 cache clean", func(t *testing.T) {
		os.WriteFile(zipFile, []byte("x"), 0o644)
		h := lockFileNoDelete(t, zipFile)
		defer windows.CloseHandle(h)
		out := captureStdout(t, func() { removeCacheZipIfPresent("temurin-21.0.5+11") })
		for _, want := range []string{"删除失败", "jvm cache clean"} {
			if !strings.Contains(out, want) {
				t.Errorf("失败提示应含 %q:\n%s", want, out)
			}
		}
		if _, err := os.Stat(zipFile); err != nil {
			t.Errorf("删除失败时 zip 应仍在: %v", err)
		}
	})
}

func TestPromptCacheCleanup(t *testing.T) {
	t.Run("无缓存 zip: 不询问不提示", func(t *testing.T) {
		withTempCache(t)
		out := captureStdout(t, func() { promptCacheCleanup([]string{"temurin-21.0.5+11"}, false) })
		if out != "" {
			t.Errorf("无 zip 应零输出: %q", out)
		}
	})
	t.Run("答 y: 逐个删除", func(t *testing.T) {
		dir := withTempCache(t)
		a := filepath.Join(dir, "a-1.0.zip")
		b := filepath.Join(dir, "b-2.0.zip")
		os.WriteFile(a, []byte("x"), 0o644)
		os.WriteFile(b, []byte("y"), 0o644)
		withStdin(t, "y\n")
		out := captureStdout(t, func() { promptCacheCleanup([]string{"a-1.0", "b-2.0"}, false) })
		for gone, name := range map[string]string{a: "a.zip", b: "b.zip"} {
			if _, err := os.Stat(gone); !os.IsNotExist(err) {
				t.Errorf("%s 应被删除", name)
			}
		}
		if strings.Contains(out, "已保留") {
			t.Errorf("答 y 不应打印保留提示:\n%s", out)
		}
	})
	t.Run("答 n: 保留并提示", func(t *testing.T) {
		dir := withTempCache(t)
		zipFile := filepath.Join(dir, "temurin-21.0.5+11.zip")
		os.WriteFile(zipFile, []byte("xx"), 0o644)
		withStdin(t, "n\n")
		out := captureStdout(t, func() { promptCacheCleanup([]string{"temurin-21.0.5+11"}, false) })
		if !strings.Contains(out, "安装包缓存已保留 (1 个, ") {
			t.Errorf("应打印保留提示 (含个数):\n%s", out)
		}
		if !strings.Contains(out, "jvm cache clean") {
			t.Errorf("保留提示应指向 jvm cache clean:\n%s", out)
		}
		if _, err := os.Stat(zipFile); err != nil {
			t.Error("答 n 时 zip 应保留")
		}
	})
	t.Run("EOF 视为 N", func(t *testing.T) {
		dir := withTempCache(t)
		os.WriteFile(filepath.Join(dir, "a-1.0.zip"), []byte("x"), 0o644)
		withStdin(t, "")
		out := captureStdout(t, func() { promptCacheCleanup([]string{"a-1.0"}, false) })
		if !strings.Contains(out, "已保留") {
			t.Errorf("EOF 应按 N 处理:\n%s", out)
		}
	})
	t.Run("-y: 默认保留并提示, 不读输入", func(t *testing.T) {
		dir := withTempCache(t)
		zipFile := filepath.Join(dir, "temurin-21.0.5+11.zip")
		os.WriteFile(zipFile, []byte("xx"), 0o644)
		withStdin(t, "y\n") // -y 下不应被消费 (即便喂了 y 也保留)
		out := captureStdout(t, func() { promptCacheCleanup([]string{"temurin-21.0.5+11"}, true) })
		if !strings.Contains(out, "安装包缓存已保留 (1 个, ") {
			t.Errorf("-y 应默认保留并提示:\n%s", out)
		}
		if _, err := os.Stat(zipFile); err != nil {
			t.Error("-y 下 zip 应保留")
		}
	})
	t.Run("多个 zip 询问列清单与合计", func(t *testing.T) {
		dir := withTempCache(t)
		os.WriteFile(filepath.Join(dir, "a-1.0.zip"), []byte("aa"), 0o644)
		os.WriteFile(filepath.Join(dir, "b-2.0.zip"), []byte("bb"), 0o644)
		withStdin(t, "n\n")
		out := captureStdout(t, func() { promptCacheCleanup([]string{"a-1.0", "b-2.0"}, false) })
		for _, want := range []string{"以下 2 个缓存 zip", "合计", "a-1.0.zip", "b-2.0.zip", "[y/N]"} {
			if !strings.Contains(out, want) {
				t.Errorf("组询问应含 %q:\n%s", want, out)
			}
		}
	})
}

// offlineUpdatePlan 构造一个不走网络的升级计划: 最新版已在组内
// (latestInstalled), 只剩删旧目录 (与 zip) 的清理段。
func offlineUpdatePlan(t *testing.T, newDir string, toDelete []string) updatePlan {
	t.Helper()
	dirs := append([]string{newDir}, toDelete...)
	_, latestVer := splitDirName(newDir)
	return updatePlan{
		distro: "temurin", major: 21,
		group:  updateGroup{dirs: dirs, latestVer: latestVer},
		latest: latestVer, newDir: newDir,
		toDelete:        toDelete,
		latestInstalled: true,
		status:          planUpgrade,
	}
}

// splitDirName 拆 {distro}-{version} 目录名为版本号 (测试构造用)。
func splitDirName(dir string) (string, string) {
	if i := strings.IndexByte(dir, '-'); i >= 0 {
		return dir[:i], dir[i+1:]
	}
	return "", dir
}

func TestExecUpdatePlanCleansOldCacheZip(t *testing.T) {
	root := withTempVersions(t)
	cache := withTempCache(t)
	newDir, oldDir := "temurin-21.0.8+7", "temurin-21.0.5+11"
	for _, d := range []string{newDir, oldDir} {
		os.MkdirAll(filepath.Join(root, "versions", d), 0o755)
		os.WriteFile(filepath.Join(cache, d+".zip"), []byte("zip"), 0o644)
	}

	var failed []string
	out := captureStdout(t, func() {
		var err error
		failed, err = execUpdatePlan(offlineUpdatePlan(t, newDir, []string{oldDir}), nil)
		if err != nil {
			t.Errorf("execUpdatePlan 意外报错: %v", err)
		}
	})
	if len(failed) != 0 {
		t.Errorf("failed = %v, 想为空", failed)
	}
	// 旧目录与旧 zip 一并消失
	if _, err := os.Stat(filepath.Join(root, "versions", oldDir)); !os.IsNotExist(err) {
		t.Error("旧版本目录应被删除")
	}
	if _, err := os.Stat(filepath.Join(cache, oldDir+".zip")); !os.IsNotExist(err) {
		t.Error("旧 patch 的缓存 zip 应顺带删除")
	}
	// 新版目录与 zip 留存
	if _, err := os.Stat(filepath.Join(root, "versions", newDir)); err != nil {
		t.Error("新版目录不应被动")
	}
	if _, err := os.Stat(filepath.Join(cache, newDir+".zip")); err != nil {
		t.Error("新版的缓存 zip 应留存 (回退/重装免下载)")
	}
	if strings.Contains(out, "已保留") {
		t.Errorf("update 直接删不询问, 不应打印保留提示:\n%s", out)
	}
}

func TestExecUpdatePlanZipDeleteFailureTolerant(t *testing.T) {
	root := withTempVersions(t)
	cache := withTempCache(t)
	newDir, oldDir := "temurin-21.0.8+7", "temurin-21.0.5+11"
	os.MkdirAll(filepath.Join(root, "versions", newDir), 0o755)
	os.MkdirAll(filepath.Join(root, "versions", oldDir), 0o755)
	oldZip := filepath.Join(cache, oldDir+".zip")
	os.WriteFile(oldZip, []byte("zip"), 0o644)
	h := lockFileNoDelete(t, oldZip) // 模拟 zip 被占用删不掉
	defer windows.CloseHandle(h)

	var failed []string
	out := captureStdout(t, func() {
		var err error
		failed, err = execUpdatePlan(offlineUpdatePlan(t, newDir, []string{oldDir}), nil)
		if err != nil {
			t.Errorf("zip 删除失败不应让升级报错: %v", err)
		}
	})
	if len(failed) != 0 {
		t.Errorf("zip 删除失败不应计入目录删除失败列表: %v", failed)
	}
	if _, err := os.Stat(filepath.Join(root, "versions", oldDir)); !os.IsNotExist(err) {
		t.Error("旧版本目录应已删除 (zip 失败不影响目录)")
	}
	if _, err := os.Stat(oldZip); err != nil {
		t.Error("删不掉的 zip 应仍在")
	}
	for _, want := range []string{"删除失败", "jvm cache clean"} {
		if !strings.Contains(out, want) {
			t.Errorf("失败提示应含 %q:\n%s", want, out)
		}
	}
}

func TestExecUpdatePlanDirDeleteFailureKeepsZip(t *testing.T) {
	root := withTempVersions(t)
	cache := withTempCache(t)
	newDir, oldDir := "temurin-21.0.8+7", "temurin-21.0.5+11"
	os.MkdirAll(filepath.Join(root, "versions", newDir), 0o755)
	lockedDir := filepath.Join(root, "versions", oldDir)
	os.MkdirAll(lockedDir, 0o755)
	// 目录里放被占用文件: Windows 下 RemoveAll 删不掉 (模拟 DLL 被进程加载)
	locked := filepath.Join(lockedDir, "locked.dll")
	os.WriteFile(locked, []byte("dll"), 0o644)
	h := lockFileNoDelete(t, locked)
	defer windows.CloseHandle(h)
	os.WriteFile(filepath.Join(cache, oldDir+".zip"), []byte("zip"), 0o644)

	var failed []string
	captureStdout(t, func() {
		var err error
		failed, err = execUpdatePlan(offlineUpdatePlan(t, newDir, []string{oldDir}), nil)
		if err != nil {
			t.Errorf("execUpdatePlan 意外报错: %v", err)
		}
	})
	if !reflect.DeepEqual(failed, []string{oldDir}) {
		t.Errorf("failed = %v, 想 [%s]", failed, oldDir)
	}
	// 目录没删掉 (版本仍在), 其缓存 zip 一并保留
	if _, err := os.Stat(lockedDir); err != nil {
		t.Error("被占用的目录应仍在")
	}
	if _, err := os.Stat(filepath.Join(cache, oldDir+".zip")); err != nil {
		t.Error("目录删除失败时缓存 zip 应保留")
	}
}
