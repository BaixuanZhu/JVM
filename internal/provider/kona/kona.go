// Package kona 是腾讯 Tencent KonaJDK (Kona) 发行版的 provider 适配器。
//
// 数据源: 每个大版本一个官方仓库 Tencent/TencentKona-{8,11,17,21,25} 的 GitHub
// releases (全 LTS 线; 历史全量保留且单仓 ≤100 期, ?per_page=100 一页拿全,
// 超过 100 期时需在此引入分页)。
//
// tag 命名按线分三套模板: 17/21/25 "TencentKona-<ver>"、11 "kona<ver>"、
// 8 "<ver>-GA" (早期另有 "v" 前缀)。ReleaseName 取 tag 派生版本号, 不含资产
// build 后缀 .bN: "21.0.13.0.3" / "17.0.4.1" / "11.0.33" / "8.0.28" —— 组内
// 唯一、tag 可逆推, 且 11/17/21/25 线与 openjdk version 输出逐字相等
// ("+1-LTS" 是 JVM build 串, 不进版本号)。版本段数不固定 (2026-10 周期起
// 21/17/25 三线从三段切五段), 比较直接复用 app.CompareVersions (数值段比较
// 与段数无关, 禁止 provider 局部比较器)。
//
// GA 过滤不靠 .prerelease 字段 (五仓全部历史 release 恒 false, 无区分度),
// 靠命名排除: fiber 增强版 / vector-api 试验版 / 版本段数不足或异形的 tag
// (如 21 线首期 "TencentKona-21-ga" 去装饰后是 "21-ga", 弃) / 无合格 Windows
// 资产的旧期与 EA 期 (17.0.0 的资产无 signed 后缀, 资产模式天然过滤)。
//
// 资产挑选: Windows 只发布 x86_64 signed zip 一种形态, 在 release 资产列表里
// 按「含 jdk_windows-x86_64 + 以 _signed.zip 结尾」匹配 (11/17/21/25 线资产
// 为 …bN_jdk_windows-x86_64_signed.zip, 8 线多一段 Oracle 更新号尾缀
// …jdk_windows-x86_64_8u<NNN>_signed.zip; 匹配不含 .bN 分隔符, 天然容忍
// "b1_jdk" 下划线 / "b1-jdk" 连字符混用)。禁止由 tag 拼资产名 —— .b1/.b2 重建
// 后缀只出现在资产名且不稳定 (17.0.4.1 的资产实为 17.0.4.b2, tag 推不出)。
//
// 校验和 (两源混合, 逐版本取最强 —— 用户 2026-11-11 拍板, 跟随厂商官方供给):
//  1. 官方 SHA256 索引 https://tencent.github.io/konajdk/releases/kona-v1.json
//     (腾讯官方 GitHub Pages, actions/setup-java 的 KonaDistribution 同源消费),
//     按 version+filename 匹配。索引每线只保留最近数期、滞后 1~2 个季度且更新
//     节奏不受控, 故只作校验和源、不作版本发现源; 拉取失败静默降级 (索引是
//     增强不是依赖)。
//  2. 索引未命中的版本 (通常是最新发布) → GitHub release 的 .md5 旁路
//     (BSD 格式 "MD5 (file) = <32hex>")。旁路是官方手工挂的、存在漏挂先例,
//     缺失或解析失败一律报错中止安装, 禁止跳过完整性校验。
//
// 官方无 Windows ARM64 构建 (五仓全部 106 期历史 release 零 windows-aarch64),
// arch=aarch64 时各入口统一报错并建议改用 temurin/microsoft。
// 直连 GitHub release 资产, 无镜像源, MirrorURL 留空。
package kona

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"jvm/internal/app"
	"jvm/internal/provider"
)

const (
	// releasesURL 是版本发现端点: 每个大版本一个官方仓, releases 单页全量
	releasesURL = "https://api.github.com/repos/Tencent/TencentKona-%d/releases?per_page=100"

	// checksumIndexURL 是官方 SHA256 索引 (腾讯 GitHub Pages), 仅作校验和源
	checksumIndexURL = "https://tencent.github.io/konajdk/releases/kona-v1.json"

	// Windows 资产名的稳定特征: 11/17/21/25 线为 "…bN_jdk_windows-x86_64_signed.zip",
	// 8 线多一段 Oracle 更新号尾缀 "…jdk_windows-x86_64_8u<NNN>_signed.zip",
	// 故用「含 jdk_windows-x86_64 + 以 _signed.zip 结尾」双条件匹配 (EA 期
	// 资产无 signed 后缀、非 Windows 平台资产、.md5 旁路均不满足)。
	winAssetMarker = "jdk_windows-x86_64"
	winAssetSuffix = "_signed.zip"

	// distroName 是发行版标识, 用作目录命名前缀和 CLI 的 distro@ 前缀
	distroName = "kona"
)

// konaMajors 是官方维护的大版本清单 (全 LTS 线, 每线一个仓库)。写死不发现:
// Kona 只跟随上游 LTS, 新 LTS 线发布后在此追加。
var konaMajors = []int{8, 11, 17, 21, 25}

// versionRe 校验 tag 派生出的版本号: 至少两段纯数字点分组 (21.0.13.0.3 /
// 8.0.28)。单段 ("21") 或异形 ("21-ga") 一律弃 —— 单段版本破坏目录/比较语义。
var versionRe = regexp.MustCompile(`^\d+(\.\d+)+$`)

// sha256HexRe 校验索引里 checksum 字段的形态 (64 位 hex), 异常条目视为未命中。
var sha256HexRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// md5SidecarRe 从 BSD 格式 .md5 旁路内容提取哈希: "MD5 (file) = <32hex>"。
var md5SidecarRe = regexp.MustCompile(`=\s*([0-9a-fA-F]{32})`)

// arch 是目标架构 (app.ArchX64 / app.ArchARM64)。Kona 官方仅发布 Windows x64,
// aarch64 由 checkArch 统一拦截, 本变量只为跟全局配置对齐。
// 由 Configure 在启动期一次性设置, 之后进程内只读。
var arch = app.ArchX64

func init() {
	provider.Register(kona{})
}

// kona 是 Provider 适配器。嵌入 provider.Base 拿默认实现:
//   - ShortSemver: Kona 版本号 (21.0.13.0.3) 已是干净形式, 透传即可
//   - ResolveReleaseName: 版本号即 release 标识, 透传即可
type kona struct {
	provider.Base
}

// Name 返回发行版标识。
func (kona) Name() string { return distroName }

// DisplayName 返回用户可见的发行版名。
func (kona) DisplayName() string { return "Tencent KonaJDK" }

// Configure 实现 provider.Configurable: 设置目标架构。
// mirror 参数被忽略: Kona 直连 GitHub release 资产, 无镜像源。
// 非法 arch 警告并回退 x64; 空值保持当前。
func (kona) Configure(cfgArch, _ string) {
	if a := strings.TrimSpace(cfgArch); a != "" {
		if norm, ok := app.NormArch(a); ok {
			arch = norm
		} else {
			fmt.Fprintf(os.Stderr, "不支持的架构 %q (仅支持 x64 / aarch64), 回退 x64\n", a)
		}
	}
}

// errNoWindowsARM64 是 arch=aarch64 时的统一错误。
// Kona 官方从未发布 Windows aarch64 构建 (五仓全部历史 release 零
// windows-aarch64 资产), 明确报错并给出替代建议。
var errNoWindowsARM64 = errors.New(
	"Tencent Kona 暂未提供 Windows ARM64 (aarch64) 构建。\n" +
		"   可改用 temurin 或 microsoft: jvm install 21  /  jvm install microsoft@21")

// checkArch 在目标架构不受支持时返回错误; 目前仅 x64 可用。
func checkArch() error {
	if arch == app.ArchARM64 {
		return errNoWindowsARM64
	}
	return nil
}

// Available 列出所有可安装的大版本 (静态, 不触网)。
// Kona 五线全为 LTS; 与 zulu 一致按升序返回, 展示层 (cmd) 自行倒序。
func (kona) Available() ([]app.Release, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	result := make([]app.Release, 0, len(konaMajors))
	for _, m := range konaMajors {
		result = append(result, app.Release{Major: m, LTS: true})
	}
	return result, nil
}

// LatestPatch 返回指定大版本的最新版本 (供 jvm available 表格/outdated 用)。
// 轻量查询: 只填 Semver/Major/ReleaseName/Distro, 不拉校验和
// (下载走 Resolve 正规链路, graalvm 先例)。
func (kona) LatestPatch(major int) (*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	rel, err := latestOf(major)
	if err != nil {
		return nil, err
	}
	return &app.Asset{
		Semver:      rel.Version,
		Major:       major,
		ReleaseName: rel.Version,
		Distro:      distroName,
	}, nil
}

// ListVersions 返回指定大版本的全部可安装子版本 (降序, 最新在前)。
// 同 LatestPatch 只填轻量字段, 校验和在 Resolve 时才拉。
func (kona) ListVersions(major int) ([]*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	if !supported(major) {
		return nil, fmt.Errorf("Kona 仅提供 LTS 版本 %v, 不支持大版本 %d", konaMajors, major)
	}
	list, err := fetchFiltered(major)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("没有找到 Kona 大版本 %d 的可用版本", major)
	}
	assets := make([]*app.Asset, 0, len(list))
	for _, r := range list {
		assets = append(assets, &app.Asset{
			Semver:      r.Version,
			Major:       major,
			ReleaseName: r.Version,
			Distro:      distroName,
		})
	}
	return assets, nil
}

// Resolve 按 VersionSpec 查单个版本, 返回发行版无关的 Asset (含校验和)。
// 两种输入:
//   - 纯大版本号 → 该大版本最新版
//   - 完整版本号 (ReleaseName, 如 21.0.13.0.3 / 17.0.4.1 / 8.0.28) → 列表内
//     精确匹配; .bN 是资产 build 后缀不进版本号, 无需也无法输入
func (k kona) Resolve(spec app.VersionSpec) (*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	v := strings.TrimSpace(spec.Version)

	major, err := app.ParseMajorOfVersion(v)
	if err != nil {
		return nil, err
	}
	if !supported(major) {
		return nil, fmt.Errorf("Kona 仅提供 LTS 版本 %v, 不支持大版本 %d", konaMajors, major)
	}

	list, err := fetchFiltered(major)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("没有找到 Kona 大版本 %d 的可用版本", major)
	}

	// 纯大版本号 → 最新版 (降序列表首条); 其余按 ReleaseName 精确匹配
	rel := list[0]
	if !app.IsPureMajor(v) {
		rel = nil
		for _, r := range list {
			if r.Version == v {
				rel = r
				break
			}
		}
		if rel == nil {
			return nil, fmt.Errorf(
				"没有找到 Kona 大版本 %d 的版本 %q (最新是 %s, 用 jvm install kona@%d 装最新)",
				major, v, list[0].Version, major)
		}
	}
	return buildAsset(fetchChecksumIndex(), rel)
}

// ---- 以下为包内私有实现 ----

// ghAsset 对应 GitHub release 的单个资产 (只取需要的字段)
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ghRelease 对应 GitHub releases API 的单条 release。
// 刻意不含 prerelease 字段: Kona 五仓该字段恒 false, GA 过滤靠命名 (见包注释)。
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// konaRelease 是过滤后的有效版本: tag 派生版本号 + 挑好的 Windows 资产
type konaRelease struct {
	Version string  // ReleaseName, 如 "21.0.13.0.3" (不含 .bN)
	Asset   ghAsset // windows-x86_64 signed zip 资产
}

// releaseCache 是 fetchFiltered 的进程内缓存 (major → 过滤+降序后的列表)。
// CLI 单命令进程生命周期短, 无 TTL; available 并发查五个大版本后, 后续
// Resolve/ListVersions 直接复用, 同进程零重复请求。
var (
	cacheMu      sync.Mutex
	releaseCache = map[int][]*konaRelease{}
)

// checksumIndex 对应 kona-v1.json: 大版本字符串 → 版本条目列表
type checksumIndex map[string][]indexRelease

// indexRelease 对应索引内单个版本条目 (只消费 version 与 files, 其余字段忽略)
type indexRelease struct {
	Version string       `json:"version"`
	Files   []indexEntry `json:"files"`
}

// indexEntry 对应索引内单个文件条目 (checksum 为 64 位 hex SHA256)
type indexEntry struct {
	OS       string `json:"os"`
	Filename string `json:"filename"`
	Checksum string `json:"checksum"`
}

// idxOnce / idxCache 实现索引的进程内单次拉取; idxCache 为 nil 表示拉取失败
// (或尚未拉取), 校验路由视为未命中、静默降级走 .md5 旁路。
var (
	idxOnce  sync.Once
	idxCache checksumIndex
)

// fetchChecksumIndex 拉取官方 SHA256 索引 (进程内一次)。失败静默记 miss:
// 索引是校验增强不是依赖, 缺席时由 .md5 旁路兜底。
func fetchChecksumIndex() checksumIndex {
	idxOnce.Do(func() {
		body, err := app.HTTPGetJSON(checksumIndexURL)
		if err != nil {
			return
		}
		var idx checksumIndex
		if json.Unmarshal(body, &idx) != nil {
			return
		}
		idxCache = idx
	})
	return idxCache
}

// fetchFiltered 拉取并过滤指定大版本的 release 列表 (进程内缓存)。
// 过滤规则见包注释; 返回按版本号降序 (最新在前)。
func fetchFiltered(major int) ([]*konaRelease, error) {
	cacheMu.Lock()
	if list, ok := releaseCache[major]; ok {
		cacheMu.Unlock()
		return list, nil
	}
	cacheMu.Unlock()

	body, err := app.HTTPGetJSON(fmt.Sprintf(releasesURL, major))
	if err != nil {
		return nil, fmt.Errorf("查询 Kona 大版本 %d 的版本列表失败: %w", major, err)
	}
	list, err := parseReleases(body, major)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	releaseCache[major] = list
	cacheMu.Unlock()
	return list, nil
}

// latestOf 返回指定大版本的最新版本 (过滤+降序后的首条)。纯函数式复用
// fetchFiltered, LatestPatch 与 Resolve 的纯大版本号分支共用。
func latestOf(major int) (*konaRelease, error) {
	if !supported(major) {
		return nil, fmt.Errorf("Kona 仅提供 LTS 版本 %v, 不支持大版本 %d", konaMajors, major)
	}
	list, err := fetchFiltered(major)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("没有找到 Kona 大版本 %d 的可用版本", major)
	}
	return list[0], nil
}

// tagToVersion 按线内模板把 tag 剥成版本号: 17/21/25 去 "TencentKona-" 前缀,
// 11 去 "kona" 前缀, 8 去早期 "v" 前缀 + 去 "-GA" 后缀。
// 产出是否合法由调用方的 versionRe 判定 (异形返回原样, 自然被弃)。纯函数。
func tagToVersion(tag string, major int) string {
	v := tag
	switch major {
	case 11:
		v = strings.TrimPrefix(v, "kona")
	case 8:
		v = strings.TrimPrefix(v, "v")
		v = strings.TrimSuffix(v, "-GA")
	default: // 17 / 21 / 25
		v = strings.TrimPrefix(v, "TencentKona-")
	}
	return v
}

// parseReleases 解析 GitHub releases 响应并过滤出有效版本, 按版本号降序。
// 排除: fiber / vector-api 变体、版本号不合法的 tag、无合格 Windows 资产的
// release。纯函数 (body 注入), 便于表测与真实响应对拍。
func parseReleases(body []byte, major int) ([]*konaRelease, error) {
	var raw []ghRelease
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析 Kona release 列表失败: %w", err)
	}
	var list []*konaRelease
	for _, r := range raw {
		tag := strings.ToLower(r.TagName)
		if strings.Contains(tag, "fiber") || strings.Contains(tag, "vector-api") {
			continue // 非 GA 主线形态
		}
		v := tagToVersion(r.TagName, major)
		if !versionRe.MatchString(v) {
			continue // 单段/异形版本 (如 "TencentKona-21-ga" → "21-ga")
		}
		asset := pickWindowsAsset(r.Assets)
		if asset == nil {
			continue // 无 Windows x64 signed zip (旧期/EA 天然过滤)
		}
		list = append(list, &konaRelease{Version: v, Asset: *asset})
	}
	// 按版本号降序 (app.CompareVersions 数值段比较, 3/4/5 段混排天然正确),
	// 不依赖 GitHub 的发布时间序 (tag 与 JVM 串存在历史性错位先例)
	sort.Slice(list, func(i, j int) bool {
		return app.CompareVersions(list[i].Version, list[j].Version) > 0
	})
	return list, nil
}

// pickWindowsAsset 在资产列表里挑 Windows x64 signed zip: 按双条件匹配
// (见 winAssetMarker/winAssetSuffix 常量注释; 容忍 .b1/.b2 与下划线/连字符
// 分隔差异及 8 线 8u 尾缀, ".md5" 旁路文件不以 _signed.zip 结尾自然排除),
// fiber 变体资产显式排除。返回 nil 表示该 release 无合格资产。纯函数。
func pickWindowsAsset(assets []ghAsset) *ghAsset {
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.Contains(name, "fiber") {
			continue
		}
		if strings.Contains(name, winAssetMarker) && strings.HasSuffix(name, winAssetSuffix) {
			return &assets[i]
		}
	}
	return nil
}

// buildAsset 把过滤后的 release 组装成完整 Asset (含校验和)。
// 校验和填充是 Resolve 专属 (LatestPatch/ListVersions 走轻量字段)。
func buildAsset(idx checksumIndex, r *konaRelease) (*app.Asset, error) {
	sum, algo, err := resolveChecksum(idx, r)
	if err != nil {
		return nil, err
	}
	return &app.Asset{
		Semver:       r.Version,
		Major:        app.MajorOfVersion(r.Version),
		ZipURL:       r.Asset.BrowserDownloadURL,
		Checksum:     sum,
		ChecksumAlgo: algo,
		ReleaseName:  r.Version,
		Distro:       distroName,
		// MirrorURL 留空: 直连 GitHub release 资产, 无镜像
	}, nil
}

// resolveChecksum 决定单个资产的校验和, 逐版本取最强:
// 索引命中 → SHA256; 未命中 → .md5 旁路 → MD5; 旁路缺失/异常 → 报错
// (禁止跳过完整性校验)。idx 参数注入 (nil = 索引不可用), 便于表测。
func resolveChecksum(idx checksumIndex, r *konaRelease) (checksum, algo string, err error) {
	if sum, ok := matchIndexChecksum(idx, r.Version, r.Asset.Name); ok {
		return sum, "sha256", nil
	}
	body, err := app.HTTPGetText(r.Asset.BrowserDownloadURL + ".md5")
	if err != nil {
		return "", "", fmt.Errorf(
			"Kona %s 无法获取校验和 (官方索引未收录, .md5 旁路也缺失): %w", r.Version, err)
	}
	sum, err := parseMD5Sidecar(string(body))
	if err != nil {
		return "", "", fmt.Errorf("解析 Kona %s 的 .md5 校验文件失败: %w", r.Version, err)
	}
	return sum, "md5", nil
}

// matchIndexChecksum 在官方索引里按 大版本+version+filename 匹配 SHA256。
// 索引不可用 (nil)、版本未收录或 checksum 形态异常均返回未命中。纯函数。
func matchIndexChecksum(idx checksumIndex, version, filename string) (string, bool) {
	if idx == nil {
		return "", false
	}
	for _, rel := range idx[strconv.Itoa(app.MajorOfVersion(version))] {
		if rel.Version != version {
			continue
		}
		for _, f := range rel.Files {
			if f.Filename == filename && sha256HexRe.MatchString(f.Checksum) {
				return strings.ToLower(f.Checksum), true
			}
		}
	}
	return "", false
}

// parseMD5Sidecar 从 BSD 格式 .md5 旁路内容解析 32 位 hex 哈希
// ("MD5 (file) = <32hex>", 非裸 hex 亦非 md5sum 格式)。纯函数。
func parseMD5Sidecar(body string) (string, error) {
	if m := md5SidecarRe.FindStringSubmatch(body); m != nil {
		return strings.ToLower(m[1]), nil
	}
	return "", fmt.Errorf("不是 BSD md5 格式 (期望 \"MD5 (文件名) = <32位hex>\")")
}

// supported 判断 major 是否在 Kona 维护的 LTS 清单里。
func supported(major int) bool {
	for _, v := range konaMajors {
		if v == major {
			return true
		}
	}
	return false
}
