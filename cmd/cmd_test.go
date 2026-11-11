package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jvm/internal/app"
	"jvm/internal/junction"
	"jvm/internal/paths"
	"jvm/internal/provider"
)

// TestParseAvailableArgs 覆盖 jvm available 的 flag 解析。
// Available 的默认表格 / 分组输出依赖网络, 不在单测范围; 这里只测纯函数解析逻辑。
func TestParseAvailableArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    AvailableOptions
		wantErr bool
	}{
		// 空 args → 默认选项 (走表格输出)
		{"empty", nil, AvailableOptions{}, false},
		{"empty slice", []string{}, AvailableOptions{}, false},

		// -a / --all
		{"short -a", []string{"-a"}, AvailableOptions{All: true}, false},
		{"long --all", []string{"--all"}, AvailableOptions{All: true}, false},

		// -m <N> / --major <N> / --major=<N>
		{"short -m", []string{"-m", "17"}, AvailableOptions{Major: 17}, false},
		{"long --major", []string{"--major", "21"}, AvailableOptions{Major: 21}, false},
		{"--major= form", []string{"--major=8"}, AvailableOptions{Major: 8}, false},

		// 互斥: -a 与 -m 同给报错 (两种顺序)
		{"-a then -m", []string{"-a", "-m", "17"}, AvailableOptions{}, true},
		{"-m then -a", []string{"-m", "17", "-a"}, AvailableOptions{}, true},
		{"-a then --major=", []string{"-a", "--major=17"}, AvailableOptions{}, true},

		// -m / --major 缺参数
		{"-m no arg", []string{"-m"}, AvailableOptions{}, true},
		{"--major no arg", []string{"--major"}, AvailableOptions{}, true},

		// 非法 major
		{"-m non-numeric", []string{"-m", "abc"}, AvailableOptions{}, true},
		{"-m zero", []string{"-m", "0"}, AvailableOptions{}, true},
		{"-m negative", []string{"-m", "-1"}, AvailableOptions{}, true},
		{"--major= empty", []string{"--major="}, AvailableOptions{}, true},

		// -r / --refresh (可与 -a/-m/distro 组合)
		{"short -r", []string{"-r"}, AvailableOptions{Refresh: true}, false},
		{"long --refresh", []string{"--refresh"}, AvailableOptions{Refresh: true}, false},
		{"-r + -a", []string{"-r", "-a"}, AvailableOptions{All: true, Refresh: true}, false},
		{"-r + -m", []string{"-m", "17", "--refresh"}, AvailableOptions{Major: 17, Refresh: true}, false},
		{"-r + distro", []string{"corretto", "-r"}, AvailableOptions{Distro: "corretto", Refresh: true}, false},

		// --json / -j (可与全部其他选项组合)
		{"long --json", []string{"--json"}, AvailableOptions{JSON: true}, false},
		{"short -j", []string{"-j"}, AvailableOptions{JSON: true}, false},
		{"--json + -a + distro", []string{"corretto", "-a", "--json"}, AvailableOptions{Distro: "corretto", All: true, JSON: true}, false},
		{"--json + -r + -m", []string{"-j", "-r", "-m", "17"}, AvailableOptions{Major: 17, Refresh: true, JSON: true}, false},

		// 未识别 flag / 多余位置参数
		{"unknown flag", []string{"-x"}, AvailableOptions{}, true},
		{"unknown long flag", []string{"--foo"}, AvailableOptions{}, true},

		// 位置参数: 第一个当 distro (合法), 第二个多余 (报错)
		{"distro positional", []string{"corretto"}, AvailableOptions{Distro: "corretto"}, false},
		{"distro + flag", []string{"corretto", "-a"}, AvailableOptions{Distro: "corretto", All: true}, false},
		{"flag + distro", []string{"-a", "corretto"}, AvailableOptions{Distro: "corretto", All: true}, false},
		{"two positionals", []string{"corretto", "temurin"}, AvailableOptions{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAvailableArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseAvailableArgs(%v) 期望报错, 实际 nil", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAvailableArgs(%v) 未预期错误: %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("ParseAvailableArgs(%v) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

// fakeProvider 注入 mock 数据, 供 availableTable/availableGroups 的并发测试。
// 嵌入 provider.Base 拿 ShortSemver/ResolveReleaseName 默认实现, 只 override 查询方法。
// 注册到全局 registry (跟随 shell/completion_test 的既有模式), 用唯一名避免冲突。
type fakeProvider struct {
	provider.Base
	name string
}

func (f fakeProvider) Name() string      { return f.name }
func (fakeProvider) DisplayName() string { return "Fake" }
func (fakeProvider) Available() ([]app.Release, error) {
	// 多个大版本 → 触发 availableTable/Groups 的多 goroutine 并发
	return []app.Release{{Major: 17, LTS: true}, {Major: 21, LTS: true}, {Major: 25, LTS: false}}, nil
}
func (fakeProvider) LatestPatch(major int) (*app.Asset, error) {
	return &app.Asset{ReleaseName: fmt.Sprintf("%d.0.1+1", major), Major: major}, nil
}
func (fakeProvider) Resolve(app.VersionSpec) (*app.Asset, error) { return nil, nil }
func (fakeProvider) ListVersions(major int) ([]*app.Asset, error) {
	return []*app.Asset{
		{ReleaseName: fmt.Sprintf("%d.0.1+1", major)},
		{ReleaseName: fmt.Sprintf("%d.0.0+1", major)},
	}, nil
}

// TestAvailableTableConcurrent 验证 availableTable 的并发 (sync.WaitGroup + goroutine
// 各写 rows[i]) 在 -race 下无数据竞争。此前这段并发逻辑零覆盖 (依赖真实 provider 网络,
// 无法单测); 注入 fake provider 提供 mock 数据后可离线跑。
// paths.Root 换到临时目录, availableTable 的缓存读写不落真实 ~/.jvm。
func TestAvailableTableConcurrent(t *testing.T) {
	withTempVersions(t)
	name := "fake-cmd-table-" + t.Name()
	provider.Register(fakeProvider{name: name})
	availableTable(AvailableOptions{}, name) // 并发查 LatestPatch; -race 下无竞争报告即通过
}

// TestAvailableGroupsConcurrent 验证 availableGroups 的并发 (goroutine 各写 groups[i])。
func TestAvailableGroupsConcurrent(t *testing.T) {
	withTempVersions(t)
	name := "fake-cmd-groups-" + t.Name()
	provider.Register(fakeProvider{name: name})
	availableGroups(AvailableOptions{All: true}, name)
}

// TestHome 验证 jvm home 输出 current 链接路径 (临时目录 + 真实 junction,
// 与 junction 包单测同款手法; 未选版本的 Fail 路径含 os.Exit, 不可单测)。
func TestHome(t *testing.T) {
	root := withTempVersions(t)
	target := filepath.Join(root, "versions", "temurin-21.0.8+7")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := junction.Create(paths.CurrentLink, target); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, Home)
	if got := strings.TrimSpace(out); got != paths.CurrentLink {
		t.Errorf("home 输出 = %q, 想 %q", got, paths.CurrentLink)
	}
}

func TestParseJSONFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    bool
		wantErr bool
	}{
		{"无参", nil, false, false},
		{"--json", []string{"--json"}, true, false},
		{"短名 -j", []string{"-j"}, true, false},
		{"重复不报错", []string{"--json", "-j"}, true, false},
		{"未知参数", []string{"--foo"}, false, true},
		{"位置参数", []string{"21"}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJSONFlag(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseJSONFlag(%v) 期望报错", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseJSONFlag(%v) 未预期错误: %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("parseJSONFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// TestListJSON 验证 jvm list --json: 字段拆分 (distro/version/major/dir)、
// current 标记、旧无前缀目录归 temurin、空安装输出空数组。
func TestListJSON(t *testing.T) {
	root := withTempVersions(t)
	for _, d := range []string{"temurin-21.0.8+7", "temurin-17.0.12+8", "corretto-21.0.5+6", "21.0.12+8"} {
		if err := os.MkdirAll(filepath.Join(root, "versions", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := junction.Create(paths.CurrentLink, filepath.Join(root, "versions", "corretto-21.0.5+6")); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() { List([]string{"--json"}) })
	var got listOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, out)
	}
	if len(got.Installed) != 4 {
		t.Fatalf("应有 4 条, 实际 %d: %+v", len(got.Installed), got.Installed)
	}
	byDir := map[string]listEntry{}
	for _, e := range got.Installed {
		byDir[e.Dir] = e
	}
	// current 标记: 只有 corretto-21.0.5+6
	cur := 0
	for _, e := range got.Installed {
		if e.Current {
			cur++
			if e.Dir != "corretto-21.0.5+6" {
				t.Errorf("current 标错了: %+v", e)
			}
		}
	}
	if cur != 1 {
		t.Errorf("应恰有 1 条 current, 实际 %d", cur)
	}
	// 字段拆分
	if e := byDir["temurin-21.0.8+7"]; e.Distro != "temurin" || e.Version != "21.0.8+7" || e.Major != 21 {
		t.Errorf("temurin 条目 = %+v", e)
	}
	// 旧无前缀目录归 temurin (与 DisplayName 语义一致), dir 保留原始目录名
	if e := byDir["21.0.12+8"]; e.Distro != "temurin" || e.Version != "21.0.12+8" || e.Major != 21 {
		t.Errorf("旧目录条目 = %+v, 想归为 temurin", e)
	}
}

// TestListJSONEmpty 验证空安装时输出 {"installed":[]} 而非人类提示语。
func TestListJSONEmpty(t *testing.T) {
	withTempVersions(t)
	out := captureStdout(t, func() { List([]string{"--json"}) })
	if got := strings.TrimSpace(out); got != `{"installed":[]}` {
		t.Errorf("空安装输出 = %q, 想 {\"installed\":[]}", got)
	}
}

// TestAvailableTableJSON 验证表格形态 --json: 顶层 distro/display_name + 行字段,
// 且不输出进度行与安装提示 (输出整份可被 JSON 解析)。
func TestAvailableTableJSON(t *testing.T) {
	withTempVersions(t)
	name := "fake-json-table-" + t.Name()
	provider.Register(fakeProvider{name: name})

	out := captureStdout(t, func() { Available(AvailableOptions{Distro: name, JSON: true}) })
	if strings.Contains(out, "正在查询") || strings.Contains(out, "安装:") {
		t.Errorf("--json 不应输出进度行或提示:\n%s", out)
	}
	var got tableOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, out)
	}
	if got.Distro != name || got.DisplayName != "Fake" {
		t.Errorf("顶层 = %+v", got)
	}
	if len(got.Rows) != 3 {
		t.Fatalf("fakeProvider 有 3 个大版本, 实际 %d: %+v", len(got.Rows), got.Rows)
	}
	// Available 顺序 17/21/25, 输出应倒序 (新在前)
	if got.Rows[0].Major != 25 || got.Rows[0].Latest != "25.0.1+1" || got.Rows[0].Failed {
		t.Errorf("首行 = %+v, 想 25 组最新", got.Rows[0])
	}
	if !got.Rows[1].LTS || got.Rows[1].Major != 21 {
		t.Errorf("次行 = %+v, 想 21 LTS", got.Rows[1])
	}
}

// TestAvailableGroupsJSON 验证分组形态 --json (-a): 组结构与 versions 数组。
func TestAvailableGroupsJSON(t *testing.T) {
	withTempVersions(t)
	name := "fake-json-groups-" + t.Name()
	provider.Register(fakeProvider{name: name})

	out := captureStdout(t, func() { Available(AvailableOptions{Distro: name, All: true, JSON: true}) })
	var got groupsOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, out)
	}
	if got.Distro != name || len(got.Groups) != 3 {
		t.Errorf("分组结果 = %s (distro=%s, %d 组)", out, got.Distro, len(got.Groups))
	}
	// fakeProvider.ListVersions 每组返回两条, 降序 (新在前)
	for _, g := range got.Groups {
		if g.Failed || len(g.Versions) != 2 || g.Versions[0] != fmt.Sprintf("%d.0.1+1", g.Major) {
			t.Errorf("组 = %+v", g)
		}
	}
}

// === available 本地已装标记 (表格"已装"列 / 分组 (已装) 后缀 / --json installed 字段) ===

// TestBuildInstalledIndex 覆盖本地已装索引构建: (distro, major) 归组、降序输入
// 组内首个即最新、旧无前缀目录归 temurin、temurin-ea 多段 distro 贪心拆分、
// 非版本目录跳过、未装查询的空值语义。
func TestBuildInstalledIndex(t *testing.T) {
	idx := buildInstalledIndex([]string{
		"temurin-21.0.8+7",
		"temurin-21.0.5+11",        // 同组旧 patch, 不改变组内最新
		"temurin-17.0.12+8",        // temurin 另一组
		"corretto-21.0.12.8.1",     // 跨 distro 不混淆 (同为 21)
		"temurin-ea-28+14-ea-beta", // 多段 distro 贪心拆分
		"25.0.1+12",                // 旧无前缀目录 → temurin
		".tmp-extract-xxx",         // 非版本目录, 跳过
	})

	// latestInstalled: 组内语义最新 (降序输入的首个)
	if got := idx.latestInstalled("temurin", 21); got != "21.0.8+7" {
		t.Errorf("temurin@21 最新 = %q, 想 21.0.8+7", got)
	}
	if got := idx.latestInstalled("temurin", 17); got != "17.0.12+8" {
		t.Errorf("temurin@17 最新 = %q, 想 17.0.12+8", got)
	}
	if got := idx.latestInstalled("corretto", 21); got != "21.0.12.8.1" {
		t.Errorf("corretto@21 最新 = %q, 想 21.0.12.8.1", got)
	}
	if got := idx.latestInstalled("temurin", 25); got != "25.0.1+12" {
		t.Errorf("旧无前缀目录应归 temurin@25, 实际 %q", got)
	}
	if got := idx.latestInstalled("temurin-ea", 28); got != "28+14-ea-beta" {
		t.Errorf("temurin-ea@28 最新 = %q, 想 28+14-ea-beta", got)
	}
	if got := idx.latestInstalled("temurin", 11); got != "" {
		t.Errorf("未装组应返回空串, 实际 %q", got)
	}

	// installedMembers: 组内全量 (降序), 供分组形态逐行标记
	if got := idx.installedMembers("temurin", 21); len(got) != 2 || got[0] != "21.0.8+7" || got[1] != "21.0.5+11" {
		t.Errorf("temurin@21 全量 = %v, 想 [21.0.8+7 21.0.5+11]", got)
	}
	if got := idx.installedMembers("temurin", 11); got != nil {
		t.Errorf("未装组应返回 nil, 实际 %v", got)
	}
}

// TestPrintAvailableTableInstalled 验证四列表格渲染: 已装列显示组内本地最新
// 版本号、未装显示 "-"、按显示宽度对齐 (整行精确匹配, padding 错误即失配)。
func TestPrintAvailableTableInstalled(t *testing.T) {
	rows := []availableRow{
		{major: 25, latest: "25.0.4+1"},
		{major: 21, latest: "21.0.12+8", lts: true, installed: "21.0.8+7"},
		{major: 17, failed: true, lts: true}, // latest 空 → N/A, installed 空 → -
	}
	out := captureStdout(t, func() { printAvailableTable(rows, "Temurin") })
	// 列宽: 大版本 6 (表头), 最新版本 9 (21.0.12+8), 类型 4 (表头), 已装 8 (21.0.8+7)
	for _, want := range []string{
		"| 大版本 | 最新版本  | 类型 | 已装     |",
		"|     21 | 21.0.12+8 | LTS  | 21.0.8+7 |",
		"|     25 | 25.0.4+1  |      | -        |",
		"|     17 | N/A       | LTS  | -        |",
	} {
		if !strings.Contains(out, want+"\n") {
			t.Errorf("表格输出缺少行 %q:\n%s", want, out)
		}
	}
}

// TestPrintAvailableGroupsInstalled 验证分组行标记: 已装子版本行追加 (已装)
// 后缀, 最新且已装时与 ← 最新 并列, 组内多版本已装则多行都标。
func TestPrintAvailableGroupsInstalled(t *testing.T) {
	groups := []versionGroup{
		{major: 21, lts: true, versions: []string{"21.0.12+8", "21.0.8+7"}, installed: []string{"21.0.8+7"}},
		{major: 17, lts: true, versions: []string{"17.0.12+8"}, installed: []string{"17.0.12+8"}},
		{major: 11, versions: []string{"11.0.20+1", "11.0.15+1", "11.0.10+1"}, installed: []string{"11.0.15+1", "11.0.10+1"}},
	}
	out := captureStdout(t, func() { printAvailableGroups(groups, "Temurin") })
	for _, want := range []string{
		"  21.0.12+8  ← 最新\n",      // 最新且未装: 仅 ← 最新
		"  21.0.8+7 (已装)\n",        // 已装非最新: 仅 (已装)
		"  17.0.12+8  ← 最新 (已装)\n", // 最新且已装: 两个标记并列
		"  11.0.20+1  ← 最新\n",      // 未装的最新行不带标记
		"  11.0.15+1 (已装)\n",       // 组内多版本已装: 多行都标
		"  11.0.10+1 (已装)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("分组输出缺少行 %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "11.0.20+1 (已装)") {
		t.Errorf("未装子版本不应带 (已装) 标记:\n%s", out)
	}
}

// TestAvailableInstalledJSON 验证 --json 两形态的 installed 新字段: 表格行
// installed 为组内本地最新已装版本号 (未装空串), 分组 installed 为组内已装
// 子版本列表 (未装为 [] 而非 null)。
func TestAvailableInstalledJSON(t *testing.T) {
	root := withTempVersions(t)
	// provider 名与目录名前缀须一致且不含 "-" (SplitDistro 按第一个 "-" 切前缀)
	name := "fakeinsjson" + t.Name()
	mkdirVersion := func(dir string) {
		if err := os.MkdirAll(filepath.Join(root, "versions", dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mkdirVersion(name + "-21.0.8+7")
	mkdirVersion(name + "-17.0.12+8")
	provider.Register(fakeProvider{name: name})

	// 表格形态: 行级 installed
	out := captureStdout(t, func() { Available(AvailableOptions{Distro: name, JSON: true}) })
	var table tableOutput
	if err := json.Unmarshal([]byte(out), &table); err != nil {
		t.Fatalf("表格输出不是合法 JSON: %v\n%s", err, out)
	}
	rowByMajor := map[int]rowJSON{}
	for _, r := range table.Rows {
		rowByMajor[r.Major] = r
	}
	if got := rowByMajor[21].Installed; got != "21.0.8+7" {
		t.Errorf("21 行 installed = %q, 想 21.0.8+7", got)
	}
	if got := rowByMajor[17].Installed; got != "17.0.12+8" {
		t.Errorf("17 行 installed = %q, 想 17.0.12+8", got)
	}
	if got := rowByMajor[25].Installed; got != "" {
		t.Errorf("未装的 25 行 installed 应为空串, 实际 %q", got)
	}

	// 分组形态 (-a): 组级 installed 列表
	out2 := captureStdout(t, func() { Available(AvailableOptions{Distro: name, All: true, JSON: true}) })
	var groups groupsOutput
	if err := json.Unmarshal([]byte(out2), &groups); err != nil {
		t.Fatalf("分组输出不是合法 JSON: %v\n%s", err, out2)
	}
	groupByMajor := map[int]groupJSON{}
	for _, g := range groups.Groups {
		groupByMajor[g.Major] = g
	}
	if g := groupByMajor[21]; len(g.Installed) != 1 || g.Installed[0] != "21.0.8+7" {
		t.Errorf("21 组 installed = %v, 想 [21.0.8+7]", g.Installed)
	}
	// 未装组规范化为 [] 而非 null (unmarshal 后 nil 切片等价于 null)
	if g := groupByMajor[25]; g.Installed == nil || len(g.Installed) != 0 {
		t.Errorf("未装的 25 组 installed 应为 [] 而非 null, 实际 %v", g.Installed)
	}
	if strings.Contains(out2, "\"installed\":null") {
		t.Errorf("installed 不应输出 null:\n%s", out2)
	}
}

// TestAvailableInstalledCacheHit 验证本地已装标记不进缓存: 首查落缓存 → 改动
// 本地版本目录 (卸载 21 / 新装 25) → TTL 内二次查询零网络走缓存命中路径,
// 表格"已装"列与分组 (已装) 标记随本地状态即时变化。
func TestAvailableInstalledCacheHit(t *testing.T) {
	root := withTempVersions(t)
	name := "fakeinsc" + t.Name()
	mkdirVersion := func(dir string) {
		if err := os.MkdirAll(filepath.Join(root, "versions", dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	removeVersion := func(dir string) {
		if err := os.RemoveAll(filepath.Join(root, "versions", dir)); err != nil {
			t.Fatal(err)
		}
	}
	mkdirVersion(name + "-21.0.1+1") // 与远端组内最新同版本 → 分组行可标记
	mkdirVersion(name + "-17.0.8+7") // 本地落后远端 → 表格已装列显示旧 patch
	provider.Register(fakeProvider{name: name})

	// 首查: 走网络路径, 表格与分组各落一份缓存
	out1t := captureStdout(t, func() { availableTable(AvailableOptions{}, name) })
	if strings.Contains(out1t, "分钟内的缓存") {
		t.Fatalf("首查不应命中缓存:\n%s", out1t)
	}
	for _, want := range []string{
		"|     21 | 21.0.1+1 | LTS  | 21.0.1+1 |",
		"|     17 | 17.0.1+1 | LTS  | 17.0.8+7 |",
		"|     25 | 25.0.1+1 |      | -        |",
	} {
		if !strings.Contains(out1t, want+"\n") {
			t.Errorf("首查表格缺少行 %q:\n%s", want, out1t)
		}
	}
	out1g := captureStdout(t, func() { availableGroups(AvailableOptions{All: true}, name) })
	if !strings.Contains(out1g, "  21.0.1+1  ← 最新 (已装)\n") {
		t.Errorf("首查分组应标记已装的 21.0.1+1:\n%s", out1g)
	}

	// 本地装/卸 (缓存文件不动): 卸 21.0.1+1, 装 25.0.1+1
	removeVersion(name + "-21.0.1+1")
	mkdirVersion(name + "-25.0.1+1")

	// 二次查询: 表格/分组全部条目命中缓存 (零网络), 已装标记仍随本地变化
	out2t := captureStdout(t, func() { availableTable(AvailableOptions{}, name) })
	if !strings.Contains(out2t, "分钟内的缓存") {
		t.Errorf("二次查询应走缓存命中路径:\n%s", out2t)
	}
	for _, want := range []string{
		"|     21 | 21.0.1+1 | LTS  | -        |", // 卸载后回退到 "-"
		"|     17 | 17.0.1+1 | LTS  | 17.0.8+7 |", // 未动的组不变
		"|     25 | 25.0.1+1 |      | 25.0.1+1 |", // 新装组出现标记
	} {
		if !strings.Contains(out2t, want+"\n") {
			t.Errorf("缓存命中路径的表格缺少行 %q:\n%s", want, out2t)
		}
	}
	if strings.Contains(out2t, "| LTS  | 21.0.1+1 |") {
		t.Errorf("已卸载的 21.0.1+1 不应再出现在已装列:\n%s", out2t)
	}
	out2g := captureStdout(t, func() { availableGroups(AvailableOptions{All: true}, name) })
	if !strings.Contains(out2g, "分钟内的缓存") {
		t.Errorf("二次分组查询应走缓存命中路径:\n%s", out2g)
	}
	if !strings.Contains(out2g, "  21.0.1+1  ← 最新\n") {
		t.Errorf("已卸载的 21 行不应再带 (已装) 标记:\n%s", out2g)
	}
	if !strings.Contains(out2g, "  25.0.1+1  ← 最新 (已装)\n") {
		t.Errorf("新装的 25 行应带 (已装) 标记:\n%s", out2g)
	}
}

// TestAvailableMajorCacheHitInstalled 验证 --major 单组缓存命中早退分支同样
// 叠加本地已装标记: 首查落缓存 → 改动本地版本目录 → TTL 内二查零网络走
// 命中早退路径 (输出附缓存说明行), (已装) 标记随本地状态变化。
func TestAvailableMajorCacheHitInstalled(t *testing.T) {
	root := withTempVersions(t)
	name := "fakemajor" + t.Name()
	mkdirVersion := func(dir string) {
		if err := os.MkdirAll(filepath.Join(root, "versions", dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	removeVersion := func(dir string) {
		if err := os.RemoveAll(filepath.Join(root, "versions", dir)); err != nil {
			t.Fatal(err)
		}
	}
	mkdirVersion(name + "-21.0.1+1") // fakeProvider 组内最新子版本 → 最新行可标记
	provider.Register(fakeProvider{name: name})

	// 首查: 缓存 miss 走网络路径, --major 单组条目落缓存
	out1 := captureStdout(t, func() { availableGroups(AvailableOptions{Major: 21}, name) })
	if strings.Contains(out1, "分钟内的缓存") {
		t.Fatalf("首查不应命中缓存:\n%s", out1)
	}
	if !strings.Contains(out1, "  21.0.1+1  ← 最新 (已装)\n") {
		t.Errorf("首查应标记已装的组内最新 21.0.1+1:\n%s", out1)
	}

	// 本地装/卸 (缓存文件不动): 卸组内最新, 装组内次新
	removeVersion(name + "-21.0.1+1")
	mkdirVersion(name + "-21.0.0+1")

	// 二查: --major 单组缓存命中早退路径 (零网络), 标记随本地变化
	out2 := captureStdout(t, func() { availableGroups(AvailableOptions{Major: 21}, name) })
	if !strings.Contains(out2, "分钟内的缓存") {
		t.Errorf("二次查询应走 --major 缓存命中早退路径:\n%s", out2)
	}
	if !strings.Contains(out2, "  21.0.1+1  ← 最新\n") {
		t.Errorf("已卸载的组内最新行不应再带 (已装) 标记:\n%s", out2)
	}
	if !strings.Contains(out2, "  21.0.0+1 (已装)\n") {
		t.Errorf("新装的 21.0.0+1 行应带 (已装) 标记:\n%s", out2)
	}
}
