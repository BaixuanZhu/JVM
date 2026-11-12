// Package semeru 是 IBM Semeru (OpenJDK 类库 + Eclipse OpenJ9 虚拟机) 发行版
// 的 provider 适配器。
//
// 数据源: IBM 官方 ibmruntimes/semeru*-binaries GitHub releases, 布局是
// "每个大版本一个仓库" (semeru8-binaries / semeru21-binaries ...):
//   - Available: 静态活跃线表 (LTS 8/11/17/21/25 + 当期短期 26), 零网络
//     (zulu activeVersions / graalvm cpuReleases 写死先例); 表格行内 latest
//     由 cmd 层经 LatestPatch 逐条查询 (available-cache TTL 兜底)。
//   - LatestPatch: GET /releases/latest (latest 端点自动跳过 prerelease;
//     无合格 Windows 资产时回退全量列表取首条, 防 certified / Linux-only 部分
//     重发等极端情况)。
//   - ListVersions / Resolve 精确路径: GET /releases?per_page=100
//     (release 不删历史, 实测最大单仓 68 条, 单页足够)。
//
// tag/资产命名在 2026-04 CPU 起切换新格式, 新旧并存必须都解析:
//   - 旧: jdk-21.0.5+11_openj9-0.48.0 / 8 线 jdk8u402-b06_openj9-0.43.0
//   - 过渡 (2026-02): tag 仍旧格式但资产名已切新格式 —— 故禁止由 tag 拼资产名,
//     一律在 release.assets 里按前缀+后缀挑
//   - 新: jdk-21.0.13.0 (四段点分, 第 4 段是 OpenJ9 refresh)
//
// 完整性校验: 同 release 内旁路资产 "<zip 名>.sha256.txt" (标准 sha256sum
// 双列文本, 取第一列), ChecksumAlgo 走默认 sha256。
//
// 目标架构: IBM Semeru 官方没有 Windows ARM64 构建 (多仓 latest 全量资产实测,
// Windows 仅 x64), arch=aarch64 时所有查询/安装入口统一报 errNoWindowsARM64,
// 引导用户改用 temurin / microsoft。
//
// 直连 GitHub release 资产 (无国内镜像, temurin-ea 先例), MirrorURL 恒空。
package semeru

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"jvm/internal/app"
	"jvm/internal/provider"
)

const (
	// apiBase 是 GitHub API 的 ibmruntimes 组织仓库根
	apiBase = "https://api.github.com/repos/ibmruntimes"

	// distroName 是发行版标识, 用作目录命名前缀和 CLI 的 distro@ 前缀
	distroName = "semeru"

	// zipAssetPrefix 是 Windows x64 JDK zip 资产名前缀, 恰好排除 msi / jre /
	// debugimage / testimage / x86-32 / certified 变体 (certified 前缀是
	// ibm-semeru-certified-)
	zipAssetPrefix = "ibm-semeru-open-jdk_x64_windows_"

	// zipAssetSuffix 是 zip 资产名后缀
	zipAssetSuffix = ".zip"

	// checksumSuffix 是旁路校验资产的后缀, 拼在 zip 资产名后
	checksumSuffix = ".sha256.txt"
)

// arch 是目标架构 (x64 / aarch64), 由 Configure 在启动期一次性设置
// (经 provider.ConfigureAll 分发), 之后进程内只读。Semeru 仅 x64 可用,
// aarch64 由 checkArch 统一拦截。
var arch = app.ArchX64

// activeMajors 是 Available 展示的活跃大版本: LTS 8/11/17/21/25 (IBM 官方
// 下载页标注) + 当期短期版 26 (非 LTS, LTS 节奏 21→25→29)。每个大版本对应
// 一个 semeru{major}-binaries 仓库; 27 出 GA 后在此追加一行。Resolve /
// ListVersions 不受此表限制 (EOL 短线仓库仍在, 仍可精确安装历史版本)。
var activeMajors = []app.Release{
	{Major: 8, LTS: true},
	{Major: 11, LTS: true},
	{Major: 17, LTS: true},
	{Major: 21, LTS: true},
	{Major: 25, LTS: true},
	{Major: 26, LTS: false},
}

// milestoneTails 是 tag 末段 (最后一个 "-" 之后) 的非 GA 标识黑名单:
// OpenJ9 milestone (m0/m1/m1a/m2/m2a) 与 rc 前缀。正常以 .prerelease 布尔
// 为主, 此表是防上游漏标的第二道保险。
var milestoneTails = map[string]bool{"m0": true, "m1": true, "m1a": true, "m2": true, "m2a": true}

// checksumRe 校验旁路文件解析出的 SHA256 (64 位 hex)。
var checksumRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// errNoWindowsARM64 是 arch=aarch64 时的统一错误。
// IBM Semeru 官方没有 Windows ARM64 构建 (Windows 平台仅 x64), 明确报错并
// 给出替代建议, 避免用户误以为装了原生 ARM64 JDK。
var errNoWindowsARM64 = errors.New(
	"IBM Semeru (OpenJ9) 暂未提供 Windows ARM64 (aarch64) 构建。\n" +
		"   可改用 temurin 或 microsoft: jvm install 21  /  jvm install microsoft@21")

// checkArch 在目标架构不受支持时返回错误; 目前仅 x64 可用。
func checkArch() error {
	if arch == app.ArchARM64 {
		return errNoWindowsARM64
	}
	return nil
}

func init() {
	provider.Register(semeru{})
}

// semeru 是 Provider 适配器。嵌入 provider.Base 拿默认实现:
//   - ShortSemver: Semeru ReleaseName (21.0.13.0 / 21.0.5+11_openj9-0.48.0)
//     已是干净形式, 透传即可
//   - ResolveReleaseName: ReleaseName 即用户可输入的完整版本号, 透传即可
type semeru struct {
	provider.Base
}

// Name 返回发行版标识。
func (semeru) Name() string { return distroName }

// DisplayName 返回用户可见的发行版名。
func (semeru) DisplayName() string { return "IBM Semeru (OpenJ9)" }

// Configure 实现 provider.Configurable: 设置目标架构 (x64 / aarch64)。
// mirror 参数被忽略: Semeru 直连 GitHub release 资产, 无镜像源 (temurin-ea 先例)。
// 非法 arch 警告并回退 x64; 空值保持默认。
func (semeru) Configure(cfgArch, _ string) {
	if a := strings.TrimSpace(cfgArch); a != "" {
		if norm, ok := app.NormArch(a); ok {
			arch = norm
		} else {
			fmt.Fprintf(os.Stderr, "不支持的架构 %q (仅支持 x64 / aarch64), 回退 x64\n", a)
		}
	}
}

// Available 列出展示用的活跃大版本 (静态表, 零网络)。
// LTS 标记依据 IBM 官方下载页: 8/11/17/21/25 为 LTS, 26 为当期短期版。
func (semeru) Available() ([]app.Release, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	out := make([]app.Release, len(activeMajors))
	copy(out, activeMajors)
	return out, nil
}

// Resolve 按 VersionSpec 查单个版本, 返回发行版无关的 Asset (含校验和, 供
// install 下载链路使用)。两种输入:
//   - 纯大版本号 → 该 major 仓库的最新 GA
//   - 完整版本号 → 列表内按 ReleaseName 匹配: 精确相等优先; 否则接受省略
//     "_openj9-..." 尾巴的短形 (如 21.0.5+11), 组内唯一命中才接受
func (s semeru) Resolve(spec app.VersionSpec) (*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	v := strings.TrimSpace(spec.Version)

	major, err := app.ParseMajorOfVersion(v)
	if err != nil {
		return nil, err
	}

	// 纯大版本号 → 最新 GA; 与 LatestPatch 同源 (latest 端点 + 回退列表),
	// 但要补拉校验和 (install 下载链路依赖)
	if app.IsPureMajor(v) {
		rel, err := fetchLatestGA(major)
		if err != nil {
			return nil, err
		}
		return fullAsset(rel, major)
	}

	// 完整版本号 → 列表内精确/短形匹配 (只在 GA 且带合格资产的 release 里匹配,
	// prerelease milestone 即使输入精确 tag 也不给装)
	rels, err := fetchReleaseList(major)
	if err != nil {
		return nil, err
	}
	qual := make([]ghRelease, 0, len(rels))
	names := make([]string, 0, len(rels))
	for _, rel := range rels {
		if !hasWindowsZip(rel) {
			continue
		}
		qual = append(qual, rel)
		names = append(names, releaseNameOf(rel.TagName))
	}
	idx, err := matchVersion(v, names)
	if err != nil {
		return nil, err
	}
	return fullAsset(qual[idx], major)
}

// LatestPatch 返回指定大版本的最新 GA (供 jvm available 表格 / outdated / update)。
// 轻量查询: 只填版本与下载直链, 不拉校验和 (install 走 Resolve 正规链路)。
func (semeru) LatestPatch(major int) (*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	rel, err := fetchLatestGA(major)
	if err != nil {
		return nil, err
	}
	return lightAsset(rel, major), nil
}

// ListVersions 返回指定大版本的全部 GA 子版本 (按版本号降序)。
// 不查校验和 (install 走 Resolve 才拉旁路文件, zulu 先例)。
func (semeru) ListVersions(major int) ([]*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	rels, err := fetchReleaseList(major)
	if err != nil {
		return nil, err
	}
	assets := releasesToAssets(rels, major)
	if len(assets) == 0 {
		return nil, fmt.Errorf("Semeru 大版本 %d 没有 Windows/%s 的 GA 版本", major, arch)
	}
	return assets, nil
}

// ---- 以下为包内私有实现 ----

// ghRelease 对应 GitHub /releases 响应里的单条 release (只声明需要的字段,
// 其余由 encoding/json 忽略)。
type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

// ghAsset 是 release 里的单个附件。
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// latestURL 拼某大版本仓库的 /releases/latest 端点 (自动跳过 prerelease)。
func latestURL(major int) string {
	return fmt.Sprintf("%s/semeru%d-binaries/releases/latest", apiBase, major)
}

// releasesURL 拼某大版本仓库的全量 release 列表端点 (默认按创建时间降序,
// 全历史在架, 实测最大单仓 68 条, per_page=100 单页足够)。
func releasesURL(major int) string {
	return fmt.Sprintf("%s/semeru%d-binaries/releases?per_page=100", apiBase, major)
}

// fetchLatestGA 取某大版本仓库的最新 GA release, 并保证它带有合格的
// Windows x64 JDK zip 资产: /releases/latest 无合格资产时 (如混入的
// certified, 或只带 Linux 资产的部分重发) 回退全量列表取首条 (列表按创建
// 时间降序, 首条合格者即最新)。
func fetchLatestGA(major int) (ghRelease, error) {
	body, err := app.HTTPGetJSON(latestURL(major))
	if err != nil {
		return ghRelease{}, wrapQueryErr(fmt.Sprintf("查询 Semeru 大版本 %d 最新版", major), err)
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return ghRelease{}, fmt.Errorf("解析 Semeru release 响应失败: %w", err)
	}
	if hasWindowsZip(rel) {
		return rel, nil
	}

	// 回退: 列表里第一个带合格资产的 GA
	rels, err := fetchReleaseList(major)
	if err != nil {
		return ghRelease{}, err
	}
	for _, r := range rels {
		if hasWindowsZip(r) {
			return r, nil
		}
	}
	return ghRelease{}, fmt.Errorf("Semeru 大版本 %d 没有 Windows/%s 的 GA 版本", major, arch)
}

// fetchReleaseList 拉某大版本仓库的全量 release 列表 (单页 100 条)。
func fetchReleaseList(major int) ([]ghRelease, error) {
	body, err := app.HTTPGetJSON(releasesURL(major))
	if err != nil {
		return nil, wrapQueryErr(fmt.Sprintf("查询 Semeru 大版本 %d 版本列表", major), err)
	}
	var rels []ghRelease
	if err := json.Unmarshal(body, &rels); err != nil {
		return nil, fmt.Errorf("解析 Semeru 版本列表失败: %w", err)
	}
	return rels, nil
}

// wrapQueryErr 给 GitHub API 查询错误补上下文; 403/429 大概率是未认证限流
// (60 次/时/IP), 附提示。app.HTTPGetJSON 对非 200 的错误文案含状态码, 此处
// 按状态码字符串识别。
func wrapQueryErr(action string, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "403") || strings.Contains(msg, "429") {
		return fmt.Errorf("%s失败 (windows/%s): %w\n   可能是 GitHub API 限流 (未认证 60 次/时), 请稍后再试", action, arch, err)
	}
	return fmt.Errorf("%s失败 (windows/%s): %w", action, arch, err)
}

// releaseNameOf 把 release tag 归一为本地 ReleaseName: 去 "jdk-" / "jdk"
// 前缀, 其余原样保留 (信息无损, 与 release 页可直读):
//   - 新格式 "jdk-21.0.13.0" → "21.0.13.0"
//   - 旧格式 "jdk-21.0.5+11_openj9-0.48.0" → "21.0.5+11_openj9-0.48.0"
//   - 8 线旧格式 "jdk8u402-b06_openj9-0.43.0" → "8u402-b06_openj9-0.43.0"
//   - 8 线无前缀历史变体 "8u345-b01_openj9-0.33.0" → 原样
//
// 纯函数, 便于表测。
func releaseNameOf(tag string) string {
	if s := strings.TrimPrefix(tag, "jdk-"); s != tag {
		return s
	}
	return strings.TrimPrefix(tag, "jdk")
}

// isGA 判断 release 是否 GA: .prerelease 布尔为主, tag 末段黑名单
// (OpenJ9 milestone m0/m1/m1a/m2/m2a 与 rc 前缀) 防上游漏标。
// certified 变体 (GA 标记但资产前缀不同) 不在此拦, 由资产前缀天然排除。
// 纯函数, 便于表测。
func isGA(tag string, prerelease bool) bool {
	if prerelease {
		return false
	}
	i := strings.LastIndexByte(tag, '-')
	if i >= 0 {
		tail := tag[i+1:]
		if milestoneTails[tail] || strings.HasPrefix(tail, "rc") {
			return false
		}
	}
	return true
}

// findZipAsset 在 release 资产里挑 Windows x64 JDK zip: 前缀
// ibm-semeru-open-jdk_x64_windows_ + 后缀 .zip, 一并排除 msi / jre /
// debugimage / testimage / x86-32 / certified。禁止由 tag 拼资产名 ——
// 2026-02 过渡期 tag 仍旧格式而资产名已切新格式, 两者并不一致。
// 纯函数, 便于表测。
func findZipAsset(rel ghRelease) (ghAsset, bool) {
	for _, a := range rel.Assets {
		if strings.HasPrefix(a.Name, zipAssetPrefix) && strings.HasSuffix(a.Name, zipAssetSuffix) {
			return a, true
		}
	}
	return ghAsset{}, false
}

// hasWindowsZip 判断 release 是否 GA 且带合格的 Windows x64 JDK zip 资产。
func hasWindowsZip(rel ghRelease) bool {
	if !isGA(rel.TagName, rel.Prerelease) {
		return false
	}
	_, ok := findZipAsset(rel)
	return ok
}

// matchVersion 在 (按新到旧排列的) ReleaseName 列表里匹配用户输入。
// 规则: 精确相等优先返回首个; 否则短形 —— ReleaseName 以 "输入+_openj9-"
// 开头视为命中 (即用户省略了 openj9 尾巴, 如 21.0.5+11 → 21.0.5+11_openj9-0.48.0),
// 唯一命中才接受, 多命中报错列出候选, 零命中报错引导。
// 纯函数, 便于表测。
func matchVersion(input string, names []string) (int, error) {
	var shortHits []int
	for i, name := range names {
		if name == input {
			return i, nil
		}
		if strings.HasPrefix(name, input+"_openj9-") {
			shortHits = append(shortHits, i)
		}
	}
	switch len(shortHits) {
	case 1:
		return shortHits[0], nil
	case 0:
		return -1, fmt.Errorf("没有找到 Semeru 版本 %q (windows/%s), 用 jvm available semeru -a 查看可安装版本", input, arch)
	default:
		var cands []string
		for _, i := range shortHits {
			cands = append(cands, names[i])
		}
		return -1, fmt.Errorf("%q 匹配到多个 Semeru 版本 (%s), 请输入完整版本号", input, strings.Join(cands, ", "))
	}
}

// releasesToAssets 把列表里全部 GA 且带合格资产的 release 转为轻量 Asset
// (不含校验和), 按版本号降序 (app.CompareVersions, version-parsing.md
// 禁止 provider 局部比较器)。用稳定排序保证同版本重复 tag 的确定性。
//
// 已知限制: 旧格式 ReleaseName 里的 "_openj9-0.48.0" 段参与切段比较
// (字母段中的数字会被切段), 同一 CPU 版本仅 sub-build 不同的过渡期特例
// (21.0.10+7 与 21.0.10+7.1) 会排反; junction.versionParts 对本地目录
// 用同款切段, 展示顺序与本地语义一致, 且两者都已被更新版本超越,
// 不做特判 (版本比较的单一事实源在 app, 不在 provider)。
// 纯函数, 便于表测。
func releasesToAssets(rels []ghRelease, major int) []*app.Asset {
	out := make([]*app.Asset, 0, len(rels))
	for _, rel := range rels {
		if !hasWindowsZip(rel) {
			continue
		}
		out = append(out, lightAsset(rel, major))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return app.CompareVersions(out[i].ReleaseName, out[j].ReleaseName) > 0
	})
	return out
}

// lightAsset 从一条合格 release 构造轻量 Asset (版本 + 下载直链, 不含校验和)。
func lightAsset(rel ghRelease, major int) *app.Asset {
	name := releaseNameOf(rel.TagName)
	zip, _ := findZipAsset(rel)
	return &app.Asset{
		Semver:      name,
		Major:       major,
		ZipURL:      zip.BrowserDownloadURL,
		ReleaseName: name,
		Distro:      distroName,
		// MirrorURL 留空: 直连 GitHub release 资产, 无镜像
	}
}

// fullAsset 从一条合格 release 构造完整 Asset: 额外拉旁路 ".sha256.txt"
// 校验资产 (install 下载链路依赖完整性校验, 缺失即报错)。
func fullAsset(rel ghRelease, major int) (*app.Asset, error) {
	zip, ok := findZipAsset(rel)
	if !ok {
		return nil, fmt.Errorf("Semeru %s 没有 Windows/%s 的 JDK zip 资产", releaseNameOf(rel.TagName), arch)
	}
	sum, err := fetchChecksum(rel, zip.Name)
	if err != nil {
		return nil, err
	}
	name := releaseNameOf(rel.TagName)
	return &app.Asset{
		Semver:      name,
		Major:       major,
		ZipURL:      zip.BrowserDownloadURL,
		Checksum:    sum,
		ReleaseName: name,
		Distro:      distroName,
		// MirrorURL 留空: 直连 GitHub release 资产, 无镜像
	}, nil
}

// fetchChecksum 拉取同 release 内 "<zip 名>.sha256.txt" 旁路资产并解析哈希。
func fetchChecksum(rel ghRelease, zipName string) (string, error) {
	want := zipName + checksumSuffix
	for _, a := range rel.Assets {
		if a.Name != want {
			continue
		}
		body, err := app.HTTPGetText(a.BrowserDownloadURL)
		if err != nil {
			return "", fmt.Errorf("获取 Semeru SHA256 失败: %w", err)
		}
		sum, err := parseSidecarSum(string(body))
		if err != nil {
			return "", fmt.Errorf("Semeru SHA256 文件 %s 格式异常: %w", want, err)
		}
		return sum, nil
	}
	return "", fmt.Errorf("Semeru release 缺少旁路校验文件 %s", want)
}

// parseSidecarSum 解析 sha256sum 双列文本 ("<hash>  <文件名>"), 取第一列
// 并校验为 64 位 hex。纯函数, 便于表测。
func parseSidecarSum(body string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(body))
	if len(fields) < 1 || !checksumRe.MatchString(fields[0]) {
		return "", fmt.Errorf("不是 sha256sum 格式 (hash + 文件名)")
	}
	return strings.ToLower(fields[0]), nil
}
