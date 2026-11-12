package dragonwell

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"jvm/internal/app"
)

// realSidecar 是 21 线 latest 的 .sha256.txt 原文 (research Q5 curl 实取):
// 两列 sha256sum 格式, 第二列是内部构建路径而非资产名。
const realSidecar = "9b6e4aa9a58d41d0e8b0ab699b0cfe11f5f1b264bb9042ea525cf4a35204925c  " +
	"target/windows/x64/dragonwell/OpenJDK21U-jdk_x64_windows_dragonwell_2026-09-01-07-46.zip\n"

// TestIsStandardGATag 验证 GA/风味过滤: 只认 "dragonwell-standard-" 前缀 + "-ga" 尾缀。
func TestIsStandardGATag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{"21 线 standard GA", "dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga", true},
		{"25 线 standard GA", "dragonwell-standard-25.0.4.0.4+7_jdk-25.0.4-ga", true},
		{"11 线 standard GA (tag 无 build 段)", "dragonwell-standard-11.0.32.28_jdk-11.0.32-ga", true},
		{"8 线短形 (jdk8u CPU 号)", "dragonwell-standard-8.30.29_jdk8u502-ga", true},
		{"extended 风味", "dragonwell-extended-11.0.32.29_jdk-11.0.32-ga", false},
		{"拆分前裸前缀 (2022-08 前)", "dragonwell-17.0.1.0.1+12_jdk-17.0.1-ga", false},
		{"非 ga 尾巴 (+0 变体)", "dragonwell-11.0.9.4_jdk-11.0.9+0", false},
		{"下划线古董", "dragonwell_11.0.7.2_GA", false},
		{"连字符古董", "dragonwell-8.3.3-GA", false},
		{"v 前缀古董", "v8.0.0-GA", false},
		{"preview (prerelease=false 但命名可辨)", "dragonwell_11.0.5.1-preview", false},
		{"空串", "", false},
	}
	for _, tt := range tests {
		if got := isStandardGATag(tt.tag); got != tt.want {
			t.Errorf("isStandardGATag(%q) = %v, want %v (%s)", tt.tag, got, tt.want, tt.name)
		}
	}
}

// TestReleaseNameOfTag 验证三线 tag → ReleaseName (dragonwell 侧版本原样, 含 `+`)。
func TestReleaseNameOfTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{"21 线五段+build", "dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga", "21.0.12.0.12+8"},
		{"25 线五段+build", "dragonwell-standard-25.0.4.0.4+7_jdk-25.0.4-ga", "25.0.4.0.4+7"},
		{"11 线四段无 build", "dragonwell-standard-11.0.32.28_jdk-11.0.32-ga", "11.0.32.28"},
		{"8 线短形", "dragonwell-standard-8.30.29_jdk8u502-ga", "8.30.29"},
		{"无 _jdk- 分隔 (畸形)", "dragonwell-standard-8.30.29-ga", ""},
		{"版本段为空 (畸形)", "dragonwell-standard-_jdk-21.0.12-ga", ""},
	}
	for _, tt := range tests {
		if got := releaseNameOfTag(tt.tag); got != tt.want {
			t.Errorf("releaseNameOfTag(%q) = %q, want %q (%s)", tt.tag, got, tt.want, tt.name)
		}
	}
}

// assetSet21 是 21 线 latest 的全套资产形态 (zip + 三旁路, 名称截自 research Q2)。
func assetSet21(version string) []ghAsset {
	base := "https://github.com/dragonwell-project/dragonwell21/releases/download/" +
		"dragonwell-standard-" + version + "_jdk-21.0.12-ga/"
	name := "Alibaba_Dragonwell_Standard_" + version + "_x64_windows"
	return []ghAsset{
		{Name: name + ".zip", BrowserDownloadURL: base + name + ".zip"},
		{Name: name + ".zip.sha256.txt", BrowserDownloadURL: base + name + ".zip.sha256.txt"},
		{Name: name + ".zip.json", BrowserDownloadURL: base + name + ".zip.json"},
		{Name: name + "-sbom.json", BrowserDownloadURL: base + name + "-sbom.json"},
		{Name: "Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_linux.zip", BrowserDownloadURL: base + "linux.zip"},
	}
}

// TestPickWindowsAssets 验证资产挑选: 前缀+后缀锚定, 排除旁路与 -fix 变体;
// zip 与 .sha256.txt 旁路缺一不可。
func TestPickWindowsAssets(t *testing.T) {
	// 21 线全套: 命中 zip 与其旁路 (资产名里 `+` 已是 `.8`, 与 tag 版本不同形)
	zip, sha, ok := pickWindowsAssets(assetSet21("21.0.12.0.12.8"))
	if !ok {
		t.Fatal("应命中 x64_windows zip + 旁路")
	}
	if want := "Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip"; zip.Name != want {
		t.Errorf("zip.Name = %q, want %q", zip.Name, want)
	}
	if want := "Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip.sha256.txt"; sha.Name != want {
		t.Errorf("sha.Name = %q, want %q", sha.Name, want)
	}

	// 25 线陷阱: 同 release 有 "..._x64_windows-fix.zip" 变体, 后缀锚定必须排除
	assets25 := []ghAsset{
		{Name: "Alibaba_Dragonwell_Standard_25.0.4.0.4.7_x64_windows.zip", BrowserDownloadURL: "https://example.com/25.zip"},
		{Name: "Alibaba_Dragonwell_Standard_25.0.4.0.4.7_x64_windows-fix.zip", BrowserDownloadURL: "https://example.com/25-fix.zip"},
		{Name: "Alibaba_Dragonwell_Standard_25.0.4.0.4.7_x64_windows.zip.sha256.txt", BrowserDownloadURL: "https://example.com/25.sha256.txt"},
	}
	zip, sha, ok = pickWindowsAssets(assets25)
	if !ok || zip.Name != "Alibaba_Dragonwell_Standard_25.0.4.0.4.7_x64_windows.zip" {
		t.Errorf("-fix.zip 陷阱: zip = %q, ok = %v", zip.Name, ok)
	}
	if !ok || !strings.HasSuffix(sha.Name, "25.0.4.0.4.7_x64_windows.zip.sha256.txt") {
		t.Errorf("-fix.zip 陷阱: sha = %q, ok = %v", sha.Name, ok)
	}

	// 只有 zip 无旁路 → 不合格
	if _, _, ok := pickWindowsAssets([]ghAsset{
		{Name: "Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip"},
	}); ok {
		t.Error("缺 .sha256.txt 旁路应判不合格")
	}

	// 只有旁路无 zip → 不合格
	if _, _, ok := pickWindowsAssets([]ghAsset{
		{Name: "Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip.sha256.txt"},
	}); ok {
		t.Error("缺 zip 资产应判不合格")
	}

	// 空资产列表
	if _, _, ok := pickWindowsAssets(nil); ok {
		t.Error("空资产列表应判不合格")
	}
}

// TestParseSidecar 验证 .sha256.txt 解析: 只认第一列 hex, 第二列是内部构建路径。
func TestParseSidecar(t *testing.T) {
	sum, err := parseSidecar(realSidecar)
	if err != nil {
		t.Fatalf("解析真实旁路报错: %v", err)
	}
	if want := "9b6e4aa9a58d41d0e8b0ab699b0cfe11f5f1b264bb9042ea525cf4a35204925c"; sum != want {
		t.Errorf("sum = %q, want %q", sum, want)
	}
	// 大写 hex 归一为小写
	if sum, err := parseSidecar("9B6E4AA9A58D41D0E8B0AB699B0CFE11F5F1B264BB9042EA525CF4A35204925C  x.zip"); err != nil || sum != "9b6e4aa9a58d41d0e8b0ab699b0cfe11f5f1b264bb9042ea525cf4a35204925c" {
		t.Errorf("大写归一: got (%q, %v)", sum, err)
	}
	// 非 64 位 hex / 空内容报错
	if _, err := parseSidecar("not-a-hash  x.zip"); err == nil {
		t.Error("非 hex 内容应报错")
	}
	if _, err := parseSidecar(""); err == nil {
		t.Error("空内容应报错")
	}
}

// filterFixture 是混排的 11 线样例: standard 与 extended 同期并行, 且 extended
// 的 dragonwell update 号更高 (.29 > .28) —— 前缀过滤必须在前, 不能按"最新一条"取。
// 另含 8 线短形与无 Windows 资产的 release。
func filterFixture() []ghRelease {
	mk := func(tag string, assets ...ghAsset) ghRelease {
		return ghRelease{TagName: tag, Assets: assets}
	}
	return []ghRelease{
		mk("dragonwell-extended-11.0.32.29_jdk-11.0.32-ga", // extended, 版本号更高, 必须被排除
			ghAsset{Name: "Alibaba_Dragonwell_Extended_11.0.32.29.9_x64_windows.zip", BrowserDownloadURL: "https://e/29.zip"},
			ghAsset{Name: "Alibaba_Dragonwell_Extended_11.0.32.29.9_x64_windows.zip.sha256.txt", BrowserDownloadURL: "https://e/29.sha256.txt"}),
		mk("dragonwell-standard-11.0.31.27_jdk-11.0.31-ga",
			ghAsset{Name: "Alibaba_Dragonwell_Standard_11.0.31.27.8_x64_windows.zip", BrowserDownloadURL: "https://s/31.zip"},
			ghAsset{Name: "Alibaba_Dragonwell_Standard_11.0.31.27.8_x64_windows.zip.sha256.txt", BrowserDownloadURL: "https://s/31.sha256.txt"}),
		mk("dragonwell-standard-11.0.32.28_jdk-11.0.32-ga", // 最新 standard, 排在输入末位 (混排)
			ghAsset{Name: "Alibaba_Dragonwell_Standard_11.0.32.28.9_x64_windows.zip", BrowserDownloadURL: "https://s/32.zip"},
			ghAsset{Name: "Alibaba_Dragonwell_Standard_11.0.32.28.9_x64_windows.zip.sha256.txt", BrowserDownloadURL: "https://s/32.sha256.txt"}),
		mk("dragonwell-standard-11.0.30.26_jdk-11.0.30-ga"), // 无 Windows 资产, 不入表
		mk("dragonwell-11.0.15.11_jdk-11.0.15-ga"),          // 拆分前裸前缀, 排除
		mk("dragonwell_11.0.7.2_GA"),                        // 古董, 排除
		mk("dragonwell-standard-11.0.28.24_jdk-11.0.28-ga", // 合格, 排最后
			ghAsset{Name: "Alibaba_Dragonwell_Standard_11.0.28.24.10_x64_windows.zip", BrowserDownloadURL: "https://s/28.zip"},
			ghAsset{Name: "Alibaba_Dragonwell_Standard_11.0.28.24.10_x64_windows.zip.sha256.txt", BrowserDownloadURL: "https://s/28.sha256.txt"}),
	}
}

// TestFilterStandardGA 验证过滤管线: 风味/资产过滤 + CompareVersions 降序排序
// (extended 版本号更高也不被误选, 前缀过滤在前)。
func TestFilterStandardGA(t *testing.T) {
	out := filterStandardGA(filterFixture())
	wantNames := []string{"11.0.32.28", "11.0.31.27", "11.0.28.24"}
	gotNames := make([]string, len(out))
	for i, r := range out {
		gotNames[i] = r.ReleaseName
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Errorf("过滤+排序结果 = %v, want %v", gotNames, wantNames)
	}
	// 11 线陷阱: ReleaseName 来自 tag (11.0.32.28), 资产名的 ".9" build 段不进版本号
	if out[0].ZipURL != "https://s/32.zip" {
		t.Errorf("最新条目应取 standard 11.0.32.28 的资产, ZipURL = %q", out[0].ZipURL)
	}
}

// TestParseAndFilterReleases 用真实 API 形态的 JSON (含未知字段) 验证结构体映射。
func TestParseAndFilterReleases(t *testing.T) {
	body := `[
	  {
	    "url": "https://api.github.com/repos/dragonwell-project/dragonwell21/releases/1",
	    "tag_name": "dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga",
	    "name": "dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga",
	    "prerelease": false,
	    "created_at": "2026-09-01T07:46:05Z",
	    "assets": [
	      {"id": 1, "name": "Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip",
	       "browser_download_url": "https://github.com/dragonwell-project/dragonwell21/releases/download/dragonwell-standard-21.0.12.0.12%2B8_jdk-21.0.12-ga/Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip"},
	      {"id": 2, "name": "Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip.sha256.txt",
	       "browser_download_url": "https://github.com/dragonwell-project/dragonwell21/releases/download/dragonwell-standard-21.0.12.0.12%2B8_jdk-21.0.12-ga/Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip.sha256.txt"}
	    ]
	  }
	]`
	rels, err := parseAndFilterReleases([]byte(body))
	if err != nil {
		t.Fatalf("parseAndFilterReleases 报错: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("应过滤出 1 条, got %d", len(rels))
	}
	r := rels[0]
	if r.ReleaseName != "21.0.12.0.12+8" {
		t.Errorf("ReleaseName = %q, want 21.0.12.0.12+8", r.ReleaseName)
	}
	// browser_download_url 原样使用 (含 %2B 编码)
	if !strings.Contains(r.ZipURL, "21.0.12.0.12%2B8") {
		t.Errorf("ZipURL 应原样保留 %%2B 编码: %q", r.ZipURL)
	}
	if !strings.HasSuffix(r.SHA256URL, ".sha256.txt") {
		t.Errorf("SHA256URL = %q", r.SHA256URL)
	}

	// 非法 JSON 报错
	if _, err := parseAndFilterReleases([]byte("not-json")); err == nil {
		t.Error("非法 JSON 应报错")
	}
}

// ga 构造一条带合成 Windows 资产 (zip + 旁路) 的 Standard GA release,
// 供排序等不关心资产名细节的用例使用 (无资产的 release 不入表)。
func ga(tag string) ghRelease {
	name := "Alibaba_Dragonwell_Standard_x64_windows.zip"
	return ghRelease{TagName: tag, Assets: []ghAsset{
		{Name: name, BrowserDownloadURL: "https://x/" + name},
		{Name: name + ".sha256.txt", BrowserDownloadURL: "https://x/" + name + ".sha256.txt"},
	}}
}

// TestSortMixedLines 验证混排排序: 21 线新旧混合 (+build 位不主导比较) 与 8 线短形。
func TestSortMixedLines(t *testing.T) {
	// 21 线乱序输入 (含 21.0.1.0.1+12 这类 build 号大但版本旧的形态)
	in := []ghRelease{
		ga("dragonwell-standard-21.0.4.0.4+7_jdk-21.0.4-ga"),
		ga("dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga"),
		ga("dragonwell-standard-21.0.1.0.1+12_jdk-21.0.1-ga"),
		ga("dragonwell-standard-21.0.11.0.11+10_jdk-21.0.11-ga"),
	}
	want := []string{"21.0.12.0.12+8", "21.0.11.0.11+10", "21.0.4.0.4+7", "21.0.1.0.1+12"}
	out := filterStandardGA(in)
	if len(out) != len(want) {
		t.Fatalf("21 线排序: 条数 = %d, want %d", len(out), len(want))
	}
	for i, r := range out {
		if r.ReleaseName != want[i] {
			t.Errorf("21 线排序[%d] = %q, want %q", i, r.ReleaseName, want[i])
		}
	}

	// 8 线三段短形 (分隔符是 "_jdk8u...", 无横线)
	in8 := []ghRelease{
		ga("dragonwell-standard-8.28.27_jdk8u482-ga"),
		ga("dragonwell-standard-8.30.29_jdk8u502-ga"),
		ga("dragonwell-standard-8.29.28_jdk8u492-ga"),
	}
	want8 := []string{"8.30.29", "8.29.28", "8.28.27"}
	out8 := filterStandardGA(in8)
	for i, r := range out8 {
		if r.ReleaseName != want8[i] {
			t.Errorf("8 线排序[%d] = %q, want %q", i, r.ReleaseName, want8[i])
		}
	}
}

// seedReleases 把过滤结果直接写入进程内缓存, 让四入口测试免网络。
func seedReleases(t *testing.T, major int, rels []dwRelease) {
	t.Helper()
	releasesMu.Lock()
	releasesCache[major] = rels
	releasesMu.Unlock()
	t.Cleanup(func() {
		releasesMu.Lock()
		delete(releasesCache, major)
		releasesMu.Unlock()
	})
}

// TestAvailable 验证静态大版本清单: 全 LTS、降序展示由上层负责, 此处按写死顺序。
func TestAvailable(t *testing.T) {
	d := dragonwell{}
	releases, err := d.Available()
	if err != nil {
		t.Fatalf("Available 报错: %v", err)
	}
	want := []app.Release{
		{Major: 8, LTS: true}, {Major: 11, LTS: true}, {Major: 17, LTS: true},
		{Major: 21, LTS: true}, {Major: 25, LTS: true},
	}
	if !reflect.DeepEqual(releases, want) {
		t.Errorf("Available = %v, want %v", releases, want)
	}
}

// seed21 是四入口测试用的 21 线缓存数据 (SHA256URL 指向 httptest 服务)。
func seed21(t *testing.T, shaURL string) {
	t.Helper()
	seedReleases(t, 21, []dwRelease{
		{ReleaseName: "21.0.12.0.12+8", ZipURL: "https://github.com/fake/21.0.12.zip", SHA256URL: shaURL},
		{ReleaseName: "21.0.4.0.4+7", ZipURL: "https://github.com/fake/21.0.4.zip", SHA256URL: shaURL},
	})
}

// TestResolve 用 httptest 旁路验证 Resolve 全路径: 纯大版本/精确匹配/未命中/无效 major。
func TestResolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			_, _ = w.Write([]byte("garbage"))
			return
		}
		_, _ = w.Write([]byte(realSidecar))
	}))
	defer srv.Close()
	seed21(t, srv.URL+"/sum.txt")

	d := dragonwell{}

	// 纯大版本号 → 最新 (降序首条), 校验和来自旁路
	a, err := d.Resolve(app.VersionSpec{Distro: "dragonwell", Version: "21"})
	if err != nil {
		t.Fatalf("Resolve(21) 报错: %v", err)
	}
	if a.ReleaseName != "21.0.12.0.12+8" || a.Major != 21 || a.Distro != "dragonwell" {
		t.Errorf("Resolve(21) = %+v", a)
	}
	if a.Checksum != "9b6e4aa9a58d41d0e8b0ab699b0cfe11f5f1b264bb9042ea525cf4a35204925c" {
		t.Errorf("Checksum = %q", a.Checksum)
	}
	if a.ZipURL != "https://github.com/fake/21.0.12.zip" {
		t.Errorf("ZipURL = %q", a.ZipURL)
	}
	if a.MirrorURL != "" {
		t.Errorf("MirrorURL 应留空 (直连 GitHub), got %q", a.MirrorURL)
	}
	if a.ChecksumAlgo != "" {
		t.Errorf("ChecksumAlgo 应留空 (默认 sha256), got %q", a.ChecksumAlgo)
	}

	// 完整版本号 → ReleaseName 精确匹配 (旧 patch 也可装)
	a, err = d.Resolve(app.VersionSpec{Distro: "dragonwell", Version: "21.0.4.0.4+7"})
	if err != nil || a.ReleaseName != "21.0.4.0.4+7" {
		t.Errorf("Resolve(21.0.4.0.4+7) = (%+v, %v)", a, err)
	}

	// 未命中版本报错
	if _, err := d.Resolve(app.VersionSpec{Distro: "dragonwell", Version: "21.0.99"}); err == nil {
		t.Error("未命中版本应报错")
	}

	// 不支持的大版本报错 (26 不在 LTS 清单)
	if _, err := d.Resolve(app.VersionSpec{Distro: "dragonwell", Version: "26"}); err == nil {
		t.Error("不支持的大版本应报错")
	}

	// 旁路内容损坏 → 报错 (seed 指向返回垃圾内容的服务路径)
	seed21(t, srv.URL+"/bad")
	if _, err := d.Resolve(app.VersionSpec{Distro: "dragonwell", Version: "21.0.12.0.12+8"}); err == nil {
		t.Error("旁路损坏应报错")
	}
}

// TestLatestPatchAndListVersions 验证两个列表入口: 轻量 Asset 与降序全量。
func TestLatestPatchAndListVersions(t *testing.T) {
	seed21(t, "")
	d := dragonwell{}

	a, err := d.LatestPatch(21)
	if err != nil {
		t.Fatalf("LatestPatch(21) 报错: %v", err)
	}
	if a.ReleaseName != "21.0.12.0.12+8" || a.Major != 21 || a.Semver != "21.0.12.0.12+8" {
		t.Errorf("LatestPatch(21) = %+v", a)
	}
	if _, err := d.LatestPatch(26); err == nil {
		t.Error("不支持的大版本应报错")
	}

	list, err := d.ListVersions(21)
	if err != nil {
		t.Fatalf("ListVersions(21) 报错: %v", err)
	}
	if len(list) != 2 || list[0].ReleaseName != "21.0.12.0.12+8" || list[1].ReleaseName != "21.0.4.0.4+7" {
		t.Errorf("ListVersions(21) 顺序异常: %+v", list)
	}
	if list[0].ZipURL == "" {
		t.Error("ListVersions 应带 zip 直链 (列表自带字段, 零额外请求)")
	}
	if list[0].Checksum != "" {
		t.Errorf("ListVersions 不拉逐版本校验和, got %q", list[0].Checksum)
	}
}

// TestArchGuard 验证 aarch64 哨兵: 四入口统一拦截, 且优先于任何网络/缓存访问。
func TestArchGuard(t *testing.T) {
	saved := arch
	arch = app.ArchARM64
	defer func() { arch = saved }()

	// 故意不 seed 缓存且 major 未注册支持: 拦截必须发生在网络查询之前
	d := dragonwell{}
	if _, err := d.Available(); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("Available 应返回 aarch64 哨兵, got %v", err)
	}
	if _, err := d.LatestPatch(21); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("LatestPatch 应返回 aarch64 哨兵, got %v", err)
	}
	if _, err := d.ListVersions(21); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("ListVersions 应返回 aarch64 哨兵, got %v", err)
	}
	if _, err := d.Resolve(app.VersionSpec{Distro: "dragonwell", Version: "21"}); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("Resolve 应返回 aarch64 哨兵, got %v", err)
	}
}

// TestConfigure 验证 Configure 只消费 arch (mirror 忽略)。
func TestConfigure(t *testing.T) {
	saved := arch
	defer func() { arch = saved }()

	dragonwell{}.Configure("aarch64", "https://mirror.example.com")
	if arch != app.ArchARM64 {
		t.Errorf("Configure(aarch64) 后 arch = %q, want aarch64", arch)
	}
	dragonwell{}.Configure("amd64", "ignored")
	if arch != app.ArchX64 {
		t.Errorf("Configure(amd64) 应归一为 x64, got %q", arch)
	}
}

// TestNameAndDisplay 验证注册身份。
func TestNameAndDisplay(t *testing.T) {
	d := dragonwell{}
	if d.Name() != "dragonwell" {
		t.Errorf("Name = %q, want dragonwell", d.Name())
	}
	if d.DisplayName() == "" {
		t.Error("DisplayName 不应为空")
	}
}
