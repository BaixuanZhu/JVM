// kona provider 的表驱动测试。
//
// fixture 的 tag/资产名/索引条目全部取自 2026-11-12 对五仓 GitHub releases 与
// kona-v1.json 的实测原文 (gh api 抓取), 三个陷阱样本 (TencentKona-21-ga 单段、
// 17.0.4.1 的 .b2 重建资产、fiber 期) 为必测项。
package kona

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"jvm/internal/app"
)

// fixtureAsset 构造测试资产, 下载 URL 指向可控的假源 (仅文件名有语义)。
func fixtureAsset(name string) ghAsset {
	return ghAsset{Name: name, BrowserDownloadURL: "https://example.test/download/" + name}
}

// mustReleasesJSON 把 release 结构编成 JSON body, 喂给 parseReleases。
func mustReleasesJSON(t *testing.T, rels ...ghRelease) []byte {
	t.Helper()
	b, err := json.Marshal(rels)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// 版本清单助手: 顺序断言过滤结果的版本号序列。
func versionsOf(list []*konaRelease) []string {
	out := make([]string, len(list))
	for i, r := range list {
		out[i] = r.Version
	}
	return out
}

// winAssets 是真实 release 的典型资产面 (linux tar.gz / windows signed zip +
// .md5 旁路), 多个用例复用。
func winAssets(zipName string) []ghAsset {
	return []ghAsset{
		fixtureAsset("TencentKona-21.0.13.0.3.b1-jdk_linux-x86_64.tar.gz"),
		fixtureAsset(zipName),
		fixtureAsset(zipName + ".md5"),
	}
}

func TestTagToVersion(t *testing.T) {
	tests := []struct {
		name  string
		tag   string
		major int
		want  string
	}{
		{"17/21/25 线模板", "TencentKona-21.0.13.0.3", 21, "21.0.13.0.3"},
		{"17 线", "TencentKona-17.0.21.0.3", 17, "17.0.21.0.3"},
		{"25 线", "TencentKona-25.0.5.0.3", 25, "25.0.5.0.3"},
		{"11 线模板", "kona11.0.33", 11, "11.0.33"},
		{"11 线重建版", "kona11.0.16.1", 11, "11.0.16.1"},
		{"8 线模板", "8.0.28-GA", 8, "8.0.28"},
		{"8 线早期 v 前缀", "v8.0.0-GA", 8, "8.0.0"},
		// 陷阱样本: 首期 tag 无完整版本号, 剥完前缀是 "21-ga",
		// 合法性由 parseReleases 的 versionRe 判定 (本函数原样返回)
		{"21 线首期单段陷阱", "TencentKona-21-ga", 21, "21-ga"},
		{"8 线 AARCH64 异形", "AARCH64_8u242", 8, "AARCH64_8u242"},
	}
	for _, tt := range tests {
		if got := tagToVersion(tt.tag, tt.major); got != tt.want {
			t.Errorf("tagToVersion(%q, %d) = %q, want %q (%s)", tt.tag, tt.major, got, tt.want, tt.name)
		}
	}
}

func TestParseReleases(t *testing.T) {
	tests := []struct {
		name     string
		major    int
		releases []ghRelease
		want     []string // 过滤+降序后的版本号序列
	}{
		{
			name:  "21 线: 五段新周期 + 三段历史, 首期 TencentKona-21-ga 单段弃",
			major: 21,
			releases: []ghRelease{
				{TagName: "TencentKona-21.0.13.0.3", Assets: winAssets("TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip")},
				{TagName: "TencentKona-21.0.12", Assets: winAssets("TencentKona-21.0.12.b1_jdk_windows-x86_64_signed.zip")},
				{TagName: "TencentKona-21-ga", Assets: winAssets("TencentKona-21.0.0.b1_jdk_windows-x86_64_signed.zip")},
			},
			want: []string{"21.0.13.0.3", "21.0.12"},
		},
		{
			name:  "17 线: 17.0.4.1 的资产实为 17.0.4.b2 (tag 推不出资产名)",
			major: 17,
			releases: []ghRelease{
				{TagName: "TencentKona-17.0.4.1", Assets: winAssets("TencentKona-17.0.4.b2_jdk_windows-x86_64_signed.zip")},
				{TagName: "TencentKona-17.0.4", Assets: winAssets("TencentKona-17.0.4.b1_jdk_windows-x86_64_signed.zip")},
			},
			want: []string{"17.0.4.1", "17.0.4"},
		},
		{
			name:  "17 线: EA 期资产无 signed 后缀, 天然过滤",
			major: 17,
			releases: []ghRelease{
				{TagName: "TencentKona-17.0.0", Assets: []ghAsset{
					fixtureAsset("TencentKona-17.0.0.b1-ea_jdk_windows-x86_64.zip"),
					fixtureAsset("TencentKona-17.0.0.b1-ea_jdk_windows-x86_64.zip.md5"),
				}},
				{TagName: "TencentKona-17.0.1", Assets: winAssets("TencentKona-17.0.1.b1_jdk_windows-x86_64_signed.zip")},
			},
			want: []string{"17.0.1"},
		},
		{
			name:  "11 线: fiber/vector-api/无 windows 资产旧期全弃",
			major: 11,
			releases: []ghRelease{
				{TagName: "kona11.0.33", Assets: winAssets("TencentKona-11.0.33.b1_jdk_windows-x86_64_signed.zip")},
				{TagName: "kona11.0.15-fiber", Assets: []ghAsset{
					fixtureAsset("TencentKona-11.0.15.b1_jdk_fiber_windows-x86_64_signed.zip"),
				}},
				{TagName: "kona11.0.13-fiber", Assets: []ghAsset{
					// 连字符分隔的 fiber 资产 (历史混用实测样本)
					fixtureAsset("TencentKona-11.0.13.b1-jdk_fiber_windows-x86_64_signed.zip"),
				}},
				{TagName: "kona11.0.8-vector-api"},
				{TagName: "kona11.0.12"}, // 旧期无任何 windows 资产
			},
			want: []string{"11.0.33"},
		},
		{
			name:  "8 线: -GA 后缀 + Fiber 弃 + 无 windows 旧期弃 + AARCH64 弃",
			major: 8,
			releases: []ghRelease{
				{TagName: "8.0.28-GA", Assets: winAssets("TencentKona8.0.28.b1_jdk_windows-x86_64_8u512_signed.zip")},
				{TagName: "Fiber-8.0.13-GA", Assets: []ghAsset{
					fixtureAsset("TencentKona8.0.13.b1_jdk_fiber_windows-x86_64_8u362_signed.zip"),
				}},
				{TagName: "8.0.13-GA", Assets: winAssets("TencentKona8.0.13.b1_jdk_windows-x86_64_8u362_signed.zip")},
				{TagName: "8.0.11-GA", Assets: winAssets("TencentKona8.0.11.b2_jdk_windows-x86_64_8u345_signed.zip")},
				{TagName: "8.0.7-GA"},  // 旧期无 windows 资产
				{TagName: "v8.0.0-GA"}, // 早期 v 前缀, 亦无 windows 资产
				{TagName: "AARCH64_8u242"},
			},
			want: []string{"8.0.28", "8.0.13", "8.0.11"},
		},
		{
			name:  "排序: 3/4/5 段混排按数值段降序, 与段数无关",
			major: 21,
			releases: []ghRelease{
				{TagName: "TencentKona-21.0.4", Assets: winAssets("TencentKona-21.0.4.b1_jdk_windows-x86_64_signed.zip")},
				{TagName: "TencentKona-21.0.13.0.3", Assets: winAssets("TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip")},
				{TagName: "TencentKona-21.0.5", Assets: winAssets("TencentKona-21.0.5.b1_jdk_windows-x86_64_signed.zip")},
			},
			want: []string{"21.0.13.0.3", "21.0.5", "21.0.4"},
		},
		{
			name:  "排序: 同 patch 的重建版 (.1 后缀) 排在其基础版之前",
			major: 11,
			releases: []ghRelease{
				{TagName: "kona11.0.16", Assets: winAssets("TencentKona-11.0.16.b1_jdk_windows-x86_64_signed.zip")},
				{TagName: "kona11.0.16.1", Assets: winAssets("TencentKona-11.0.16.b2_jdk_windows-x86_64_signed.zip")},
			},
			want: []string{"11.0.16.1", "11.0.16"},
		},
		{
			name:  "全被过滤: 返回空列表不报错",
			major: 11,
			releases: []ghRelease{
				{TagName: "kona11.0.15-fiber"},
				{TagName: "kona11.0.12"},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		list, err := parseReleases(mustReleasesJSON(t, tt.releases...), tt.major)
		if err != nil {
			t.Fatalf("%s: parseReleases 出错: %v", tt.name, err)
		}
		got := versionsOf(list)
		if len(got) != len(tt.want) {
			t.Errorf("%s: 过滤结果 %v, want %v", tt.name, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: 过滤结果 %v, want %v", tt.name, got, tt.want)
				break
			}
		}
	}
}

func TestParseReleases_RebuildAssetKeptVerbatim(t *testing.T) {
	// 陷阱样本专项: 17.0.4.1 选中 .b2 重建资产, 资产名/URL 原样保留
	// (目录命名用 tag 派生版本号, 不用资产名)
	list, err := parseReleases(mustReleasesJSON(t,
		ghRelease{TagName: "TencentKona-17.0.4.1", Assets: winAssets("TencentKona-17.0.4.b2_jdk_windows-x86_64_signed.zip")},
	), 17)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("过滤结果条数 = %d, want 1", len(list))
	}
	const wantName = "TencentKona-17.0.4.b2_jdk_windows-x86_64_signed.zip"
	if list[0].Asset.Name != wantName {
		t.Errorf("选中资产名 = %q, want %q", list[0].Asset.Name, wantName)
	}
	if wantURL := "https://example.test/download/" + wantName; list[0].Asset.BrowserDownloadURL != wantURL {
		t.Errorf("资产 URL = %q, want %q", list[0].Asset.BrowserDownloadURL, wantURL)
	}
}

func TestParseReleases_BadJSON(t *testing.T) {
	if _, err := parseReleases([]byte("not json"), 21); err == nil {
		t.Error("畸形 JSON 期望报错")
	}
}

func TestPickWindowsAsset(t *testing.T) {
	tests := []struct {
		name   string
		assets []ghAsset
		want   string // 期望选中的资产名, 空串表示无合格资产
	}{
		{
			name: "常规 b1 资产与 .md5 旁路并存, 只选 zip",
			assets: []ghAsset{
				fixtureAsset("TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip.md5"),
				fixtureAsset("TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip"),
			},
			want: "TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip",
		},
		{
			name: ".b2 重建资产",
			assets: []ghAsset{
				fixtureAsset("TencentKona-17.0.4.b2_jdk_windows-x86_64_signed.zip"),
			},
			want: "TencentKona-17.0.4.b2_jdk_windows-x86_64_signed.zip",
		},
		{
			name: "8 线 8u 尾缀资产",
			assets: []ghAsset{
				fixtureAsset("TencentKona8.0.28.b1_jdk_windows-x86_64_8u512_signed.zip"),
			},
			want: "TencentKona8.0.28.b1_jdk_windows-x86_64_8u512_signed.zip",
		},
		{
			name: "连字符分隔 b1-jdk 也命中 (后缀不含分隔符)",
			assets: []ghAsset{
				fixtureAsset("TencentKona-11.0.13.b1-jdk_windows-x86_64_signed.zip"),
			},
			want: "TencentKona-11.0.13.b1-jdk_windows-x86_64_signed.zip",
		},
		{
			name: "fiber 资产排除",
			assets: []ghAsset{
				fixtureAsset("TencentKona-11.0.15.b1_jdk_fiber_windows-x86_64_signed.zip"),
				fixtureAsset("TencentKona-11.0.15.b1_jdk_windows-x86_64_signed.zip"),
			},
			want: "TencentKona-11.0.15.b1_jdk_windows-x86_64_signed.zip",
		},
		{
			name: "只有 fiber 资产 → 无合格",
			assets: []ghAsset{
				fixtureAsset("TencentKona-11.0.13.b1-jdk_fiber_windows-x86_64_signed.zip"),
			},
			want: "",
		},
		{
			name: "EA 资产无 signed 后缀 → 无合格",
			assets: []ghAsset{
				fixtureAsset("TencentKona-17.0.0.b1-ea_jdk_windows-x86_64.zip"),
			},
			want: "",
		},
		{
			name: "仅非 Windows 平台资产 → 无合格",
			assets: []ghAsset{
				fixtureAsset("TencentKona-21.0.13.0.3.b1-jdk_linux-x86_64.tar.gz"),
				fixtureAsset("TencentKona-21.0.13.0.3.b1_jdk_macosx-x86_64_notarized.tar.gz"),
			},
			want: "",
		},
		{name: "空资产列表", assets: nil, want: ""},
	}
	for _, tt := range tests {
		got := pickWindowsAsset(tt.assets)
		if tt.want == "" {
			if got != nil {
				t.Errorf("%s: 期望无合格资产, 选中 %q", tt.name, got.Name)
			}
			continue
		}
		if got == nil || got.Name != tt.want {
			t.Errorf("%s: 选中 %v, want %q", tt.name, got, tt.want)
		}
	}
}

func TestParseMD5Sidecar(t *testing.T) {
	// 两条实测原文 (BSD md5 格式, 非裸 hex)
	const real21 = "MD5 (TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip) = 82a87aefb260ec0e672986bbc43abc08\n"
	const real8 = "MD5 (TencentKona8.0.28.b1_jdk_windows-x86_64_8u512_signed.zip) = 2196083e32ab493c8035a13da2b50e82\n"

	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{"21 线实测原文", real21, "82a87aefb260ec0e672986bbc43abc08", false},
		{"8 线实测原文", real8, "2196083e32ab493c8035a13da2b50e82", false},
		{"大写 hex 转小写", "MD5 (x.zip) = 82A87AEFB260EC0E672986BBC43ABC08", "82a87aefb260ec0e672986bbc43abc08", false},
		{"裸 hex (md5sum 风格) 不是 BSD 格式, 报错", "82a87aefb260ec0e672986bbc43abc08  x.zip", "", true},
		{"空内容", "", "", true},
		{"垃圾内容", "not a checksum file", "", true},
		{"等号后非 32 位 hex", "MD5 (x.zip) = zzzz", "", true},
	}
	for _, tt := range tests {
		got, err := parseMD5Sidecar(tt.body)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: 期望报错, got %q", tt.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: 出错 %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// realIndexFixture 镜像 kona-v1.json 的真实结构 (条目为实测原文;
// 8.0.26 的 sha256 经真实下载逐字节验证一致)。
func realIndexFixture() checksumIndex {
	return checksumIndex{
		"21": []indexRelease{
			{
				Version: "21.0.11",
				Files: []indexEntry{
					{OS: "linux", Filename: "TencentKona-21.0.11.b1-jdk_linux-x86_64.tar.gz", Checksum: "96f71aabef907853673f8d79189570d28eb668aea41904f40905f4b7e0619f51"},
					{OS: "windows", Filename: "TencentKona-21.0.11.b1_jdk_windows-x86_64_signed.zip", Checksum: "ca272ae1c7361c6a996264bc7c1c1080a719c8c062662fc6bd45c50cb4044309"},
				},
			},
		},
		"8": []indexRelease{
			{
				Version: "8.0.26",
				Files: []indexEntry{
					{OS: "windows", Filename: "TencentKona8.0.26.b1_jdk_windows-x86_64_8u492_signed.zip", Checksum: "37d3a7bee43435ebe034a7d38813c35fd38f64979aa82f5c053872d9beb056dd"},
				},
			},
		},
	}
}

func TestMatchIndexChecksum(t *testing.T) {
	idx := realIndexFixture()
	tests := []struct {
		name     string
		idx      checksumIndex
		version  string
		filename string
		want     string // 命中的 sha256; 空串表示未命中
	}{
		{
			name:     "命中: 21.0.11 windows zip (索引最新一期)",
			idx:      idx,
			version:  "21.0.11",
			filename: "TencentKona-21.0.11.b1_jdk_windows-x86_64_signed.zip",
			want:     "ca272ae1c7361c6a996264bc7c1c1080a719c8c062662fc6bd45c50cb4044309",
		},
		{
			name:     "命中: 8.0.26 windows zip (实测下载验证过的值)",
			idx:      idx,
			version:  "8.0.26",
			filename: "TencentKona8.0.26.b1_jdk_windows-x86_64_8u492_signed.zip",
			want:     "37d3a7bee43435ebe034a7d38813c35fd38f64979aa82f5c053872d9beb056dd",
		},
		{
			name:     "未命中: 索引滞后的最新版 (真实滞后场景)",
			idx:      idx,
			version:  "21.0.13.0.3",
			filename: "TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip",
			want:     "",
		},
		{
			name:     "未命中: 版本命中但文件名不同 (build 号不匹配)",
			idx:      idx,
			version:  "21.0.11",
			filename: "TencentKona-21.0.11.b2_jdk_windows-x86_64_signed.zip",
			want:     "",
		},
		{
			name:     "未命中: 索引不可用 (nil = 拉取失败静默降级)",
			idx:      nil,
			version:  "21.0.11",
			filename: "TencentKona-21.0.11.b1_jdk_windows-x86_64_signed.zip",
			want:     "",
		},
		{
			name: "未命中: checksum 非 64 位 hex 视为脏条目",
			idx: checksumIndex{"21": []indexRelease{
				{Version: "21.0.10", Files: []indexEntry{
					{OS: "windows", Filename: "TencentKona-21.0.10.b1_jdk_windows-x86_64_signed.zip", Checksum: "abc123"},
				}},
			}},
			version:  "21.0.10",
			filename: "TencentKona-21.0.10.b1_jdk_windows-x86_64_signed.zip",
			want:     "",
		},
	}
	for _, tt := range tests {
		got, ok := matchIndexChecksum(tt.idx, tt.version, tt.filename)
		if tt.want == "" {
			if ok {
				t.Errorf("%s: 期望未命中, got %q", tt.name, got)
			}
			continue
		}
		if !ok || got != tt.want {
			t.Errorf("%s = %q (ok=%v), want %q", tt.name, got, ok, tt.want)
		}
	}
}

// TestResolveChecksum 验证两源混合路由: 索引命中 → sha256 且零网络;
// 未命中 → .md5 旁路; 旁路缺失/脏内容 → 报错 (禁止跳过校验)。
func TestResolveChecksum(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		switch r.URL.Path {
		case "/ok.zip.md5":
			io.WriteString(w, "MD5 (ok.zip) = 82a87aefb260ec0e672986bbc43abc08\n")
		case "/bad.zip.md5":
			io.WriteString(w, "checksum service unavailable")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	okRel := &konaRelease{Version: "21.0.13.0.3", Asset: ghAsset{
		Name: "ok.zip", BrowserDownloadURL: srv.URL + "/ok.zip"}}
	indexedRel := &konaRelease{Version: "21.0.11", Asset: ghAsset{
		Name: "TencentKona-21.0.11.b1_jdk_windows-x86_64_signed.zip", BrowserDownloadURL: srv.URL + "/indexed.zip"}}

	// 索引未命中 (nil) → .md5 旁路 → md5
	sum, algo, err := resolveChecksum(nil, okRel)
	if err != nil || sum != "82a87aefb260ec0e672986bbc43abc08" || algo != "md5" {
		t.Errorf("旁路路由 = (%q, %q, %v), want (82a8..., md5, nil)", sum, algo, err)
	}

	// 索引命中 → sha256 且不发任何网络请求
	before := atomic.LoadInt32(&hits)
	sum, algo, err = resolveChecksum(realIndexFixture(), indexedRel)
	after := atomic.LoadInt32(&hits)
	if err != nil || algo != "sha256" || sum != "ca272ae1c7361c6a996264bc7c1c1080a719c8c062662fc6bd45c50cb4044309" {
		t.Errorf("索引路由 = (%q, %q, %v), want (ca27..., sha256, nil)", sum, algo, err)
	}
	if after != before {
		t.Errorf("索引命中仍发了 %d 个网络请求", after-before)
	}

	// 旁路缺失 (404) → 报错中止
	missRel := &konaRelease{Version: "21.0.13.0.3", Asset: ghAsset{
		Name: "missing.zip", BrowserDownloadURL: srv.URL + "/missing.zip"}}
	if _, _, err := resolveChecksum(nil, missRel); err == nil {
		t.Error("旁路缺失期望报错 (禁止跳过校验)")
	} else if !strings.Contains(err.Error(), "校验和") {
		t.Errorf("旁路缺失错误文案应说明校验和缺失: %v", err)
	}

	// 旁路内容不可解析 → 报错中止
	badRel := &konaRelease{Version: "21.0.13.0.3", Asset: ghAsset{
		Name: "bad.zip", BrowserDownloadURL: srv.URL + "/bad.zip"}}
	if _, _, err := resolveChecksum(nil, badRel); err == nil {
		t.Error("旁路脏内容期望报错")
	}
}

// withArch 临时切换目标架构并在测试结束后恢复。
func withArch(t *testing.T, a string) {
	t.Helper()
	old := arch
	arch = a
	t.Cleanup(func() { arch = old })
}

func TestArchGuard(t *testing.T) {
	withArch(t, app.ArchARM64)
	k := kona{}
	if _, err := k.Available(); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("Available: aarch64 期望 errNoWindowsARM64, got %v", err)
	}
	if _, err := k.LatestPatch(21); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("LatestPatch: aarch64 期望 errNoWindowsARM64, got %v", err)
	}
	if _, err := k.ListVersions(21); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("ListVersions: aarch64 期望 errNoWindowsARM64, got %v", err)
	}
	if _, err := k.Resolve(app.VersionSpec{Distro: "kona", Version: "21"}); !errors.Is(err, errNoWindowsARM64) {
		t.Errorf("Resolve: aarch64 期望 errNoWindowsARM64, got %v", err)
	}
	// 恢复 x64 后静态 Available 正常 (守卫确因 arch 而非其他)
	withArch(t, app.ArchX64)
	rels, err := k.Available()
	if err != nil {
		t.Fatalf("x64 Available 出错: %v", err)
	}
	if len(rels) != len(konaMajors) {
		t.Errorf("Available 条数 = %d, want %d", len(rels), len(konaMajors))
	}
}

func TestAvailable(t *testing.T) {
	withArch(t, app.ArchX64)
	rels, err := kona{}.Available()
	if err != nil {
		t.Fatal(err)
	}
	want := []int{8, 11, 17, 21, 25} // 升序 (展示层自行倒序), 全 LTS
	if len(rels) != len(want) {
		t.Fatalf("Available 条数 = %d, want %d", len(rels), len(want))
	}
	for i, r := range rels {
		if r.Major != want[i] || !r.LTS {
			t.Errorf("Available[%d] = {Major: %d, LTS: %v}, want {Major: %d, LTS: true}", i, r.Major, r.LTS, want[i])
		}
	}
}

func TestUnsupportedMajorGuards(t *testing.T) {
	withArch(t, app.ArchX64)
	k := kona{}
	// 守卫在 fetchFiltered 之前短路, 不触网
	if _, err := k.LatestPatch(26); err == nil || !strings.Contains(err.Error(), "不支持大版本 26") {
		t.Errorf("LatestPatch(26) 错误 = %v, 期望不支持大版本", err)
	}
	if _, err := k.ListVersions(26); err == nil || !strings.Contains(err.Error(), "不支持大版本 26") {
		t.Errorf("ListVersions(26) 错误 = %v, 期望不支持大版本", err)
	}
	if _, err := k.Resolve(app.VersionSpec{Distro: "kona", Version: "26"}); err == nil ||
		!strings.Contains(err.Error(), "不支持大版本 26") {
		t.Errorf("Resolve(26) 错误 = %v, 期望不支持大版本", err)
	}
}

func TestConfigure(t *testing.T) {
	withArch(t, app.ArchX64)
	kona{}.Configure("", "https://mirror.example") // 空值保持默认, mirror 忽略
	if arch != app.ArchX64 {
		t.Errorf("空 arch 后 arch = %q, want x64", arch)
	}
	kona{}.Configure("arm64", "") // 别名规范化
	if arch != app.ArchARM64 {
		t.Errorf("arm64 别名后 arch = %q, want aarch64", arch)
	}
}

func TestSupported(t *testing.T) {
	for _, m := range konaMajors {
		if !supported(m) {
			t.Errorf("supported(%d) = false, want true", m)
		}
	}
	for _, m := range []int{0, 7, 9, 13, 26} {
		if supported(m) {
			t.Errorf("supported(%d) = true, want false", m)
		}
	}
}
