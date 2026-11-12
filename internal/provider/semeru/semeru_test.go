package semeru

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"jvm/internal/app"
)

// TestReleaseNameOf 验证三代 tag 格式到 ReleaseName 的归一 (研究样本:
// 旧 jdk-{ver}+{build}_openj9-{oj9} / 过渡 +{build}.{n} / 新四段点分,
// 8 线 jdk8u 前缀与两枚历史杂例)。
func TestReleaseNameOf(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{"新格式四段", "jdk-21.0.13.0", "21.0.13.0"},
		{"新格式带 refresh", "jdk-21.0.12.10", "21.0.12.10"},
		{"旧格式三段", "jdk-21.0.5+11_openj9-0.48.0", "21.0.5+11_openj9-0.48.0"},
		{"旧格式四段", "jdk-11.0.16.1+1_openj9-0.33.1", "11.0.16.1+1_openj9-0.33.1"},
		{"大版本首发无 patch 段", "jdk-25+36_openj9-0.55.0", "25+36_openj9-0.55.0"},
		{"过渡期 refresh", "jdk-21.0.10+7.1_openj9-0.57.0", "21.0.10+7.1_openj9-0.57.0"},
		{"8 线旧格式 jdk8u 前缀", "jdk8u402-b06_openj9-0.43.0", "8u402-b06_openj9-0.43.0"},
		{"8 线新格式", "jdk-8.0.492.0", "8.0.492.0"},
		{"8 线无 jdk 前缀历史变体", "8u345-b01_openj9-0.33.0", "8u345-b01_openj9-0.33.0"},
		{"8 线连字符历史变体", "jdk8u352-b08-openj9-0.35.0", "8u352-b08-openj9-0.35.0"},
		{"无前缀原样", "21.0.13.0", "21.0.13.0"},
		{"空串", "", ""},
	}
	for _, tt := range tests {
		if got := releaseNameOf(tt.tag); got != tt.want {
			t.Errorf("releaseNameOf(%q) = %q, want %q (%s)", tt.tag, got, tt.want, tt.name)
		}
	}
}

// TestIsGA 验证 GA 过滤: .prerelease 布尔为主 + tag 末段黑名单
// (OpenJ9 milestone / rc) 双保险; certified 不在此层拦 (由资产前缀排除)。
func TestIsGA(t *testing.T) {
	tests := []struct {
		name       string
		tag        string
		prerelease bool
		want       bool
	}{
		{"正常 GA 新格式", "jdk-21.0.13.0", false, true},
		{"正常 GA 旧格式", "jdk-21.0.5+11_openj9-0.48.0", false, true},
		{"8 线 GA", "jdk8u402-b06_openj9-0.43.0", false, true},
		{"prerelease 标记", "jdk-21.0.13.0-m1", true, false},
		{"milestone m0 (漏标防护)", "jdk-21+35_openj9-0.42.0-m0", false, false},
		{"milestone m1a (漏标防护)", "jdk-25.0.1+8_openj9-0.57.0-m1a", false, false},
		{"milestone m2 (漏标防护)", "jdk-25.0.5.0-m2", false, false},
		{"rc1a 后缀 (漏标防护)", "8u342-b07_openj9-0.33.0-rc1a", false, false},
		{"certified 由资产层排除", "jdk-11.0.12.0-certified", false, true},
		{"末段是版本号不误伤", "jdk-21.0.5+11_openj9-0.48.0", false, true},
	}
	for _, tt := range tests {
		if got := isGA(tt.tag, tt.prerelease); got != tt.want {
			t.Errorf("isGA(%q, %v) = %v, want %v (%s)", tt.tag, tt.prerelease, got, tt.want, tt.name)
		}
	}
}

// mkRelease 构造测试用 ghRelease 的便捷函数。
func mkRelease(tag string, prerelease bool, assetNames ...string) ghRelease {
	rel := ghRelease{TagName: tag, Prerelease: prerelease}
	for _, n := range assetNames {
		rel.Assets = append(rel.Assets, ghAsset{
			Name:               n,
			BrowserDownloadURL: "https://example.invalid/" + tag + "/" + n,
		})
	}
	return rel
}

// TestFindZipAsset 验证 Windows x64 JDK zip 资产挑选: 前缀+后缀精确匹配,
// 排除 msi / jre / debugimage / testimage / x86-32 / certified。
func TestFindZipAsset(t *testing.T) {
	rel := mkRelease("jdk-21.0.13.0", false,
		"ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip",
		"ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip.sha256.txt",
		"ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip.json",
		"ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip.sig",
		"ibm-semeru-open-jdk_x64_windows_21.0.13.0.msi",
		"ibm-semeru-open-jre_x64_windows_21.0.13.0.zip",
		"ibm-semeru-open-debugimage_x64_windows_21.0.13.0.zip",
		"ibm-semeru-open-testimage_x64_windows_21.0.13.0.zip",
		"ibm-semeru-open-jdk_x86-32_windows_21.0.13.0.zip",
		"ibm-semeru-open-jdk_x64_linux_21.0.13.0.tar.gz",
		"ibm-semeru-open-jdk_aarch64_linux_21.0.13.0.tar.gz",
		"ibm-semeru-certified-jdk_x64_windows_21.0.13.0.zip",
	)
	zip, ok := findZipAsset(rel)
	if !ok {
		t.Fatal("应能挑到 Windows x64 JDK zip 资产")
	}
	if zip.Name != "ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip" {
		t.Errorf("挑到 %q, want ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip", zip.Name)
	}

	// 过渡期: tag 旧格式、资产名已切新格式 —— 按 tag 拼名会失配, 按前缀+后缀照常命中
	relTransition := mkRelease("jdk-21.0.10+7.1_openj9-0.57.0", false,
		"ibm-semeru-open-jdk_x64_windows_21.0.10.1.zip",
		"ibm-semeru-open-jdk_x64_windows_21.0.10.1.zip.sha256.txt",
	)
	zip, ok = findZipAsset(relTransition)
	if !ok || zip.Name != "ibm-semeru-open-jdk_x64_windows_21.0.10.1.zip" {
		t.Errorf("过渡期资产挑选失败: ok=%v name=%q", ok, zip.Name)
	}

	// 无合格资产 (certified release)
	_, ok = findZipAsset(mkRelease("jdk-11.0.12.0-certified", false,
		"ibm-semeru-certified-jdk_x64_windows_11.0.12.0.zip"))
	if ok {
		t.Error("certified 资产不应被选中")
	}
}

// TestParseSidecarSum 验证 .sha256.txt 旁路解析: sha256sum 双列格式取第一列。
func TestParseSidecarSum(t *testing.T) {
	// 研究样本原文 (新格式 21.0.13.0)
	newFmt := "ebea8e701ca80769e0575e0c72db4c5131b4afde3a33fcac3fe46cc96996c68e  ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip\r\n"
	// 研究样本原文 (旧格式 21.0.5+11)
	oldFmt := "81f683cef58420eeb54c5cba24141753a58261f8c07399f9b0fd501789ea177e  ibm-semeru-open-jdk_x64_windows_21.0.5_11_openj9-0.48.0.zip"

	if got, err := parseSidecarSum(newFmt); err != nil ||
		got != "ebea8e701ca80769e0575e0c72db4c5131b4afde3a33fcac3fe46cc96996c68e" {
		t.Errorf("parseSidecarSum(新格式) = %q, %v", got, err)
	}
	if got, err := parseSidecarSum(oldFmt); err != nil ||
		got != "81f683cef58420eeb54c5cba24141753a58261f8c07399f9b0fd501789ea177e" {
		t.Errorf("parseSidecarSum(旧格式) = %q, %v", got, err)
	}
	// 大写 hex 归一为小写
	if got, _ := parseSidecarSum("EBEA8E701CA80769E0575E0C72DB4C5131B4AFDE3A33FCAC3FE46CC96996C68E  x.zip"); got != "ebea8e701ca80769e0575e0c72db4c5131b4afde3a33fcac3fe46cc96996c68e" {
		t.Errorf("大写 hex 应归一为小写, got %q", got)
	}
	for name, bad := range map[string]string{
		"空内容":           "",
		"非 hex":         "not-a-hash  file.zip",
		"缺文件名列但 hash 短": "abc123",
	} {
		if _, err := parseSidecarSum(bad); err == nil {
			t.Errorf("parseSidecarSum(%s) 应报错", name)
		}
	}
}

// TestMatchVersion 验证精确/短形两态匹配: 精确相等优先; 短形 (省略 _openj9-
// 尾巴) 唯一命中才接受, 多命中报错列出候选, 零命中报错引导。
func TestMatchVersion(t *testing.T) {
	names := []string{
		"21.0.13.0",
		"21.0.10+7.1_openj9-0.57.0",
		"21.0.10+7_openj9-0.57.0",
		"21.0.5+11_openj9-0.48.0",
		"21.0.1+12_openj9-0.42.0",
	}

	// 精确: 新格式全串
	if i, err := matchVersion("21.0.13.0", names); err != nil || i != 0 {
		t.Errorf("精确匹配新格式 = %d, %v; want 0, nil", i, err)
	}
	// 精确: 旧格式全串
	if i, err := matchVersion("21.0.5+11_openj9-0.48.0", names); err != nil || i != 3 {
		t.Errorf("精确匹配旧格式 = %d, %v; want 3, nil", i, err)
	}
	// 短形唯一: 省略 openj9 尾巴
	if i, err := matchVersion("21.0.5+11", names); err != nil || i != 3 {
		t.Errorf("短形匹配 = %d, %v; want 3, nil", i, err)
	}
	// 短形不能越过 +build 段误命中: 21.0.1+1 不是 21.0.1+12_openj9-... 的短形
	if _, err := matchVersion("21.0.1+1", names); err == nil {
		t.Error("21.0.1+1 不应匹配 21.0.1+12_openj9-0.42.0")
	}
	// 零命中
	if _, err := matchVersion("21.0.99", names); err == nil {
		t.Error("零命中应报错")
	}

	// 短形多命中: 同一 CPU 版本多个 openj9 refresh
	dup := []string{
		"8u302-b08_openj9-0.27.0",
		"8u302-b08_openj9-0.27.1",
	}
	_, err := matchVersion("8u302-b08", dup)
	if err == nil {
		t.Fatal("短形多命中应报错")
	}
	for _, want := range dup {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("多命中错误应列出候选 %s, got: %v", want, err)
		}
	}
}

// TestReleasesToAssets 验证列表 → Asset 转换: GA/资产过滤 + 版本降序
// (app.CompareVersions, 新旧格式混排)。GitHub 列表按创建时间降序返回,
// 此处打乱输入验证排序不依赖 API 顺序。
//
// 排序断言不含过渡期 "+7 与 +7.1" 同 security 同 build 对: 旧格式
// ReleaseName 的 "_openj9-..." 段参与切段比较, 该特例会排反 (见
// releasesToAssets 注释的已知限制); 此处用不同 security 的旧格式对
// 覆盖新旧混排, 顺序在切段语义下正确。
func TestReleasesToAssets(t *testing.T) {
	rels := []ghRelease{
		mkRelease("jdk-21.0.5+11_openj9-0.48.0", false, "ibm-semeru-open-jdk_x64_windows_21.0.5_11_openj9-0.48.0.zip"),
		mkRelease("jdk-21.0.13.0-m1", true, "ibm-semeru-open-jdk_x64_windows_21.0.13.0-m1.zip"),
		mkRelease("jdk-21.0.9+10_openj9-0.56.0", false, "ibm-semeru-open-jdk_x64_windows_21.0.9_10_openj9-0.56.0.zip"),
		mkRelease("jdk-21.0.13.0", false, "ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip"),
		mkRelease("jdk8u302-b08_openj9-0.27.1", false, "ibm-semeru-open-jdk_x64_linux_8u302b08_openj9-0.27.1.tar.gz"), // Linux-only 部分重发
		mkRelease("jdk-21.0.10+7.1_openj9-0.57.0", false, "ibm-semeru-open-jdk_x64_windows_21.0.10.1.zip"),
		mkRelease("jdk-11.0.12.0-certified", false, "ibm-semeru-certified-jdk_x64_windows_11.0.12.0.zip"), // certified
	}
	want := []string{
		"21.0.13.0",
		"21.0.10+7.1_openj9-0.57.0",
		"21.0.9+10_openj9-0.56.0",
		"21.0.5+11_openj9-0.48.0",
	}
	assets := releasesToAssets(rels, 21)
	if len(assets) != len(want) {
		t.Fatalf("过滤后应剩 %d 条, got %d", len(want), len(assets))
	}
	for i, a := range assets {
		if a.ReleaseName != want[i] {
			t.Errorf("assets[%d].ReleaseName = %q, want %q", i, a.ReleaseName, want[i])
		}
		if a.Major != 21 || a.Distro != "semeru" || a.Semver != want[i] {
			t.Errorf("assets[%d] 字段异常: %+v", i, a)
		}
		if a.ZipURL == "" {
			t.Errorf("assets[%d] 应带下载直链", i)
		}
	}
}

// TestLightAsset 验证轻量 Asset 构造 (版本 + 直链, 不含校验和, MirrorURL 恒空)。
func TestLightAsset(t *testing.T) {
	rel := mkRelease("jdk-21.0.13.0", false,
		"ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip",
		"ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip.sha256.txt",
	)
	a := lightAsset(rel, 21)
	if a.Semver != "21.0.13.0" || a.ReleaseName != "21.0.13.0" {
		t.Errorf("Semver/ReleaseName = %q/%q, want 21.0.13.0", a.Semver, a.ReleaseName)
	}
	if a.Major != 21 || a.Distro != "semeru" {
		t.Errorf("Major/Distro = %d/%q", a.Major, a.Distro)
	}
	if !strings.HasSuffix(a.ZipURL, "/jdk-21.0.13.0/ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip") {
		t.Errorf("ZipURL = %q", a.ZipURL)
	}
	if a.Checksum != "" {
		t.Errorf("轻量 Asset 不应带校验和, got %q", a.Checksum)
	}
	if a.MirrorURL != "" {
		t.Errorf("MirrorURL 应为空 (直连 GitHub), got %q", a.MirrorURL)
	}
}

// TestFullAssetErrors 验证 fullAsset 对缺 zip 资产 / 缺旁路校验文件的报错
// (不触网络: 缺旁路文件在 fetchChecksum 遍历资产时即失败)。
func TestFullAssetErrors(t *testing.T) {
	// 无 zip 资产
	if _, err := fullAsset(mkRelease("jdk-21.0.13.0-certified", false,
		"ibm-semeru-certified-jdk_x64_windows_21.0.13.0.zip"), 21); err == nil {
		t.Error("无合格 zip 资产应报错")
	}
	// 有 zip 但无 .sha256.txt 旁路
	if _, err := fullAsset(mkRelease("jdk-21.0.13.0", false,
		"ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip"), 21); err == nil {
		t.Error("缺旁路校验文件应报错")
	}
}

// TestReleaseListJSONParse 用内嵌的真实结构 fixture 验证 GitHub /releases
// JSON 解析 (含未声明的多余字段被忽略、assets 数组结构)。
func TestReleaseListJSONParse(t *testing.T) {
	fixture := `[
  {
    "url": "https://api.github.com/repos/ibmruntimes/semeru21-binaries/releases/123456789",
    "tag_name": "jdk-21.0.13.0",
    "name": "IBM Semeru Runtime Open Edition 21.0.13.0",
    "prerelease": false,
    "created_at": "2026-10-24T02:11:44Z",
    "assets": [
      {
        "name": "ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip",
        "browser_download_url": "https://github.com/ibmruntimes/semeru21-binaries/releases/download/jdk-21.0.13.0/ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip",
        "size": 190000000,
        "state": "uploaded"
      },
      {
        "name": "ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip.sha256.txt",
        "browser_download_url": "https://github.com/ibmruntimes/semeru21-binaries/releases/download/jdk-21.0.13.0/ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip.sha256.txt"
      }
    ]
  },
  {
    "tag_name": "jdk-21.0.13.0-m1",
    "prerelease": true,
    "assets": [
      {"name": "ibm-semeru-open-jdk_x64_windows_21.0.13.0-m1.zip", "browser_download_url": "https://github.com/ibmruntimes/semeru21-binaries/releases/download/jdk-21.0.13.0-m1/ibm-semeru-open-jdk_x64_windows_21.0.13.0-m1.zip"}
    ]
  }
]`
	var rels []ghRelease
	if err := json.Unmarshal([]byte(fixture), &rels); err != nil {
		t.Fatalf("解析 fixture 失败: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("应解析出 2 条 release, got %d", len(rels))
	}
	assets := releasesToAssets(rels, 21)
	if len(assets) != 1 {
		t.Fatalf("过滤后应剩 1 条 (milestone 剔除), got %d", len(assets))
	}
	if assets[0].ReleaseName != "21.0.13.0" {
		t.Errorf("ReleaseName = %q, want 21.0.13.0", assets[0].ReleaseName)
	}
	if !strings.HasPrefix(assets[0].ZipURL, "https://github.com/ibmruntimes/semeru21-binaries/releases/download/") {
		t.Errorf("ZipURL = %q", assets[0].ZipURL)
	}
}

// TestAvailable 验证静态活跃线表 (LTS 8/11/17/21/25 + 非 LTS 26) 与
// 返回副本语义 (调用方改动不影响后续调用)。
func TestAvailable(t *testing.T) {
	got, err := semeru{}.Available()
	if err != nil {
		t.Fatalf("Available() 报错: %v", err)
	}
	want := []app.Release{
		{Major: 8, LTS: true},
		{Major: 11, LTS: true},
		{Major: 17, LTS: true},
		{Major: 21, LTS: true},
		{Major: 25, LTS: true},
		{Major: 26, LTS: false},
	}
	if len(got) != len(want) {
		t.Fatalf("Available() 返回 %d 条, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Available()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	// 副本语义
	got[0].Major = 999
	again, _ := semeru{}.Available()
	if again[0].Major != 8 {
		t.Error("Available() 应返回副本, 调用方改动不应影响后续调用")
	}
}

// TestNameAndDisplayName 验证核心标识方法。
func TestNameAndDisplayName(t *testing.T) {
	p := semeru{}
	if p.Name() != "semeru" {
		t.Errorf("Name() = %q, want %q", p.Name(), "semeru")
	}
	if p.DisplayName() != "IBM Semeru (OpenJ9)" {
		t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), "IBM Semeru (OpenJ9)")
	}
}

// TestConfigure 验证 Configure 设置目标架构 (含别名与非法值回退),
// 并在结束后恢复包级状态, 不污染其他测试。
func TestConfigure(t *testing.T) {
	origArch := arch
	t.Cleanup(func() { arch = origArch })

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"合法 x64", "x64", "x64"},
		{"合法 aarch64", "aarch64", "aarch64"},
		{"别名 arm64", "arm64", "aarch64"},
		{"非法回退 x64", "riscv", "x64"},
		{"空值保持默认", "", "x64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arch = "x64" // 每个子用例从已知状态开始
			semeru{}.Configure(tt.in, "https://ignored.mirror")
			if arch != tt.want {
				t.Errorf("Configure(%q) 后 arch = %q, want %q", tt.in, arch, tt.want)
			}
		})
	}
}

// TestCheckArch 验证架构守卫: x64 放行, aarch64 报统一错误并给出替代建议。
func TestCheckArch(t *testing.T) {
	origArch := arch
	t.Cleanup(func() { arch = origArch })

	arch = app.ArchX64
	if err := checkArch(); err != nil {
		t.Errorf("x64 下 checkArch() 应为 nil, got %v", err)
	}

	arch = app.ArchARM64
	err := checkArch()
	if !errors.Is(err, errNoWindowsARM64) {
		t.Fatalf("aarch64 下 checkArch() 应为 errNoWindowsARM64, got %v", err)
	}
	// 错误信息应引导用户改用有 ARM64 构建的发行版
	if !strings.Contains(err.Error(), "temurin") || !strings.Contains(err.Error(), "microsoft") {
		t.Errorf("错误信息应包含替代发行版建议, got: %v", err)
	}
}

// TestArchGuardBlocksAllEntries 验证 aarch64 下四个公开入口全部快速报错
// (守卫在网络请求之前生效, 故本测试无需 mock HTTP)。
func TestArchGuardBlocksAllEntries(t *testing.T) {
	origArch := arch
	t.Cleanup(func() { arch = origArch })
	arch = app.ArchARM64

	p := semeru{}
	if _, err := p.Available(); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("Available() 应报 errNoWindowsARM64, got %v", err)
	}
	if _, err := p.LatestPatch(21); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("LatestPatch() 应报 errNoWindowsARM64, got %v", err)
	}
	if _, err := p.ListVersions(21); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("ListVersions() 应报 errNoWindowsARM64, got %v", err)
	}
	if _, err := p.Resolve(app.VersionSpec{Distro: "semeru", Version: "21"}); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("Resolve() 应报 errNoWindowsARM64, got %v", err)
	}
}

// TestWrapQueryErr 验证 GitHub API 查询错误包装: 普通错误带上下文,
// 403/429 追加限流提示。
func TestWrapQueryErr(t *testing.T) {
	origArch := arch
	t.Cleanup(func() { arch = origArch })
	arch = app.ArchX64

	err := wrapQueryErr("查询 Semeru 大版本 21 最新版", errors.New("API 返回 500"))
	if err == nil || !strings.Contains(err.Error(), "(windows/x64)") || strings.Contains(err.Error(), "限流") {
		t.Errorf("普通错误应带 windows/x64 上下文且无限流提示, got: %v", err)
	}

	err = wrapQueryErr("查询 Semeru 大版本 21 最新版", errors.New("API 返回 403"))
	if err == nil || !strings.Contains(err.Error(), "GitHub API 限流") {
		t.Errorf("403 应提示 GitHub API 限流, got: %v", err)
	}

	err = wrapQueryErr("查询", errors.New("API 返回 429"))
	if err == nil || !strings.Contains(err.Error(), "GitHub API 限流") {
		t.Errorf("429 应提示 GitHub API 限流, got: %v", err)
	}
}
