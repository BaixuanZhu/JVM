// Package dragonwell 是 Alibaba Dragonwell (Standard 风味) 发行版的 provider 适配器。
//
// 数据源: dragonwell-project org 下各主线仓的 GitHub releases
// (2026 年全家族已从 alibaba org 迁至 dragonwell-project, 旧地址仅剩转移重定向,
// 路由表单 org 一张: dragonwell-project/dragonwell{major})。
//   - 版本枚举: 每仓 1 次 GET /releases?per_page=100 + 本地过滤
//     (全历史在架、单仓 ≤50 条, 单页足够; 不用 /releases/latest —— 该端点对
//     Standard/Extended 风味的取舍无契约保证, 实测机制黑盒)
//   - GA/风味过滤: tag 前缀 "dragonwell-standard-" + 尾缀 "-ga"
//     (.prerelease 字段不可用: extended/preview 也标 false; 裸前缀/古董/非 ga
//     尾巴的变体全被前后缀天然排除)
//   - ReleaseName = tag 去前缀、去 "_jdk-...-ga" 尾巴 (dragonwell 侧版本原样):
//     "21.0.12.0.12+8" / "25.0.4.0.4+7" / "11.0.32.28" / "8.30.29"
//   - Windows 资产: release.assets 内按前缀 "Alibaba_Dragonwell_Standard_" +
//     后缀 "_x64_windows.zip" 锚定匹配 —— 不能由 tag 拼资产名 (11 线 tag
//     11.0.32.28 的资产实为 ..._11.0.32.28.9_..., 多一个 OpenJDK build 段)
//   - 校验和: 同 release 资产 "<zip名>.sha256.txt" (两列 sha256sum 格式,
//     第二列是内部构建路径非资产名, 只认第一列 hex), SHA256
//
// 仅接 Standard 风味 (跟随上游 OpenJDK 的标准构建); Extended (阿里私有增强)
// 如需接入按 temurin-ea 先例另起第二 provider 变体。
//
// 官方无 Windows ARM64 构建 (仅 x64_windows), arch=aarch64 时各入口统一报错
// 并建议改用 temurin/microsoft。
//
// 直连 GitHub (无国内镜像), MirrorURL 留空 (temurin-ea 先例)。
package dragonwell

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"jvm/internal/app"
	"jvm/internal/provider"
)

const (
	// apiBase 是 GitHub API 基址, repo 名为 "dragonwell" + major
	apiBase = "https://api.github.com/repos/dragonwell-project/"

	// pageLimit 是单次请求的 release 条数上限 (GitHub API 单页最大 100;
	// 最大仓 dragonwell8 实测 50 条, 一页拉全)
	pageLimit = 100

	// distroName 是发行版标识, 用作目录命名前缀和 CLI 的 distro@ 前缀
	distroName = "dragonwell"

	// tagPrefix 是 Standard 风味 GA tag 的前缀 (extended 是 "dragonwell-extended-")
	tagPrefix = "dragonwell-standard-"

	// tagSuffix 是 GA tag 的尾缀 (存在 "dragonwell-11.0.9.4_jdk-11.0.9+0" 这类非 ga 尾巴变体)
	tagSuffix = "-ga"

	// tagSep 是 tag 里 dragonwell 版本与 OpenJDK 侧版本的分隔符:
	// 11/17/21/25 线是 "_jdk-21.0.12-ga", 8 线是 "_jdk8u502-ga" (jdk 后无横线),
	// 故只锚定 "_jdk" (dragonwell 侧版本只含数字/点/+, 不会撞)
	tagSep = "_jdk"

	// zipPrefix 是 Windows x64 zip 资产名前缀
	zipPrefix = "Alibaba_Dragonwell_Standard_"

	// zipSuffix 是 Windows x64 zip 资产名后缀 (锚定结尾, 排除 "-fix.zip" 变体
	// 与 ".sha256.txt" / ".zip.json" / "-sbom.json" / ".sig" 等旁路)
	zipSuffix = "_x64_windows.zip"

	// sidecarSuffix 是 SHA256 旁路资产名后缀 (完整名 = zip 资产名 + 本后缀)
	sidecarSuffix = ".sha256.txt"
)

// majors 是全部在维护的主线大版本 (全 LTS)。25 线 2026 年已有 Standard GA,
// 纳入; 新 LTS 线出现时在此追加 (graalvm cpuReleases 写死先例)。
var majors = []int{8, 11, 17, 21, 25}

// arch 是目标架构, 由 Configure 在启动期设置。Dragonwell 仅消费其合法性
// (跟全局配置对齐), 实际只支持 x64 —— aarch64 由 checkArch 统一拦截。
var arch = app.ArchX64

func init() {
	provider.Register(dragonwell{})
}

// errNoWindowsARM64 是 arch=aarch64 时的统一错误。
// Alibaba Dragonwell 官方只发布 x64_windows 资产 (21 线 latest 资产全集穷举验证),
// 明确报错并给出替代建议, 避免用户误以为装了原生 ARM64 JDK。
var errNoWindowsARM64 = errors.New(
	"Alibaba Dragonwell 暂未提供 Windows ARM64 (aarch64) 构建。\n" +
		"   可改用 temurin 或 microsoft: jvm install 21  /  jvm install microsoft@21")

// checkArch 在目标架构不受支持时返回错误; 目前仅 x64 可用。
func checkArch() error {
	if arch == app.ArchARM64 {
		return errNoWindowsARM64
	}
	return nil
}

// checkMajor 校验大版本是否在主线清单里, 不在则报错并列出全部可用线。
func checkMajor(major int) error {
	if !supported(major) {
		return fmt.Errorf("Alibaba Dragonwell 仅提供 LTS 大版本 %v, 不支持大版本 %d", majors, major)
	}
	return nil
}

// dragonwell 是 Provider 适配器。嵌入 provider.Base 拿默认实现:
//   - ShortSemver: Dragonwell 版本号 (21.0.12.0.12+8) 已是干净形式, 透传即可
//   - ResolveReleaseName: 版本号即 release 标识, 透传即可 (名字无冗余尾巴, 无短形)
type dragonwell struct {
	provider.Base
}

// Name 返回发行版标识。
func (dragonwell) Name() string { return distroName }

// DisplayName 返回用户可见的发行版名。
func (dragonwell) DisplayName() string { return "Alibaba Dragonwell (Standard)" }

// Configure 实现 provider.Configurable: 设置目标架构 (x64 / aarch64)。
// mirror 参数被忽略: Dragonwell 直连 GitHub release 资产, 无镜像源。
// 非法 arch 警告并回退 x64; 空值保持默认。
func (dragonwell) Configure(cfgArch, _ string) {
	if a := strings.TrimSpace(cfgArch); a != "" {
		if norm, ok := app.NormArch(a); ok {
			arch = norm
		} else {
			fmt.Fprintf(os.Stderr, "不支持的架构 %q (仅支持 x64 / aarch64), 回退 x64\n", a)
		}
	}
}

// Available 列出所有可安装的大版本。
// 写死的 LTS 清单 (零请求); 表格行内 latest 由 cmd 层逐条查 LatestPatch。
func (dragonwell) Available() ([]app.Release, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	result := make([]app.Release, 0, len(majors))
	for _, m := range majors {
		result = append(result, app.Release{Major: m, LTS: true})
	}
	return result, nil
}

// Resolve 按 VersionSpec 查单个版本, 返回发行版无关的 Asset。
// 两种输入:
//   - 纯大版本号 → 该线最新 Standard GA
//   - 完整版本号 (如 21.0.12.0.12+8) → 过滤后列表内 ReleaseName 精确匹配
//     (版本号即 release 标识, 无短形 —— 名字本身没有可省略的尾巴)
//
// 两条路径最终都拉 .sha256.txt 旁路补齐校验和。
func (d dragonwell) Resolve(spec app.VersionSpec) (*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	v := strings.TrimSpace(spec.Version)

	major, err := app.ParseMajorOfVersion(v)
	if err != nil {
		return nil, err
	}
	if err := checkMajor(major); err != nil {
		return nil, err
	}

	rels, err := fetchFiltered(major)
	if err != nil {
		return nil, err
	}

	// 纯大版本号 → 降序列表首条即最新; 其余 → ReleaseName 精确匹配
	rel := rels[0]
	if !app.IsPureMajor(v) {
		found := false
		for _, r := range rels {
			if r.ReleaseName == v {
				rel, found = r, true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Alibaba Dragonwell 没有 %s 版本 (查看全部: jvm available -a dragonwell)", v)
		}
	}
	return buildAsset(rel, major)
}

// LatestPatch 返回指定线的最新 Standard GA 版本。
// 轻量查询: 只填 Semver/ReleaseName/Major/Distro, 不拉校验和
// (下载走 Resolve 正规链路), 供 available 表格 / outdated / update 复用。
func (dragonwell) LatestPatch(major int) (*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	if err := checkMajor(major); err != nil {
		return nil, err
	}
	rels, err := fetchFiltered(major)
	if err != nil {
		return nil, err
	}
	r := rels[0] // 列表已按版本降序
	return &app.Asset{Semver: r.ReleaseName, Major: major, ReleaseName: r.ReleaseName, Distro: distroName}, nil
}

// ListVersions 返回指定线的全部 Standard GA 版本 (按版本降序)。
// zip 直链来自 release 列表自带字段, 零额外请求; 不拉逐版本校验和
// (install 走 Resolve 才查旁路, zulu 先例)。
func (dragonwell) ListVersions(major int) ([]*app.Asset, error) {
	if err := checkArch(); err != nil {
		return nil, err
	}
	if err := checkMajor(major); err != nil {
		return nil, err
	}
	rels, err := fetchFiltered(major)
	if err != nil {
		return nil, err
	}
	out := make([]*app.Asset, 0, len(rels))
	for _, r := range rels {
		out = append(out, &app.Asset{
			Semver:      r.ReleaseName,
			Major:       major,
			ZipURL:      r.ZipURL,
			ReleaseName: r.ReleaseName,
			Distro:      distroName,
			// MirrorURL 留空: 直连 GitHub
		})
	}
	return out, nil
}

// ---- 以下为包内私有实现 ----

// ghRelease 对应 GitHub /releases 响应里的单条 release (仅取本适配器关心的字段,
// 其余字段 json.Unmarshal 自动忽略)。
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// ghAsset 对应 release 内的单个资产。
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// dwRelease 是过滤+解析后的一条 Standard GA release。
type dwRelease struct {
	ReleaseName string // dragonwell 侧版本, 如 "21.0.12.0.12+8" / "8.30.29"
	ZipURL      string // x64_windows zip 直链 (browser_download_url 原样, %2B 已编码)
	SHA256URL   string // .sha256.txt 旁路直链
}

// releasesMu 保护 releasesCache (available/outdated 会并发调 LatestPatch,
// Go map 并发写是 data race, 即便 key 不同)。
var (
	releasesMu    sync.Mutex
	releasesCache = map[int][]dwRelease{}
)

// fetchFiltered 拉取并过滤指定线仓库的全部 Standard GA release (带进程内缓存)。
// CLI 单进程单命令, 简单 map 即可, 无 TTL 需求; 返回的切片是缓存共享的,
// 调用方只读不可改。过滤后为空视为该线不可用 (写死的 majors 都有 GA, 空 = 上游异常)。
func fetchFiltered(major int) ([]dwRelease, error) {
	releasesMu.Lock()
	cached, ok := releasesCache[major]
	releasesMu.Unlock()
	if ok {
		return cached, nil
	}

	u := fmt.Sprintf("%sdragonwell%d/releases?per_page=%d", apiBase, major, pageLimit)
	body, err := app.HTTPGetJSON(u)
	if err != nil {
		if isRateLimited(err) {
			return nil, fmt.Errorf("查询 Dragonwell %d 线版本列表失败: GitHub API 限流, "+
				"稍后再试或 jvm available --refresh 绕过缓存前请等待: %w", major, err)
		}
		return nil, fmt.Errorf("查询 Dragonwell %d 线版本列表失败: %w", major, err)
	}
	rels, err := parseAndFilterReleases(body)
	if err != nil {
		return nil, fmt.Errorf("解析 Dragonwell %d 线版本列表失败: %w", major, err)
	}
	if len(rels) == 0 {
		return nil, fmt.Errorf("Dragonwell %d 线没有 Standard GA 版本 (windows/x64)", major)
	}

	releasesMu.Lock()
	releasesCache[major] = rels
	releasesMu.Unlock()
	return rels, nil
}

// parseAndFilterReleases 解析 GitHub /releases 响应 JSON 并过滤。
// 拆出来收 []byte 便于用真实响应灌入表测/对拍。
func parseAndFilterReleases(body []byte) ([]dwRelease, error) {
	var releases []ghRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("解析 GitHub 响应失败: %w", err)
	}
	return filterStandardGA(releases), nil
}

// filterStandardGA 过滤+解析 release 列表, 输出按 CompareVersions 降序:
// tag 风味/GA 过滤 → ReleaseName 提取 → 合格 Windows 资产 (zip 与 .sha256.txt
// 旁路缺一不可, 无资产的 release 不入表)。纯函数, 便于表测。
func filterStandardGA(releases []ghRelease) []dwRelease {
	var out []dwRelease
	for _, r := range releases {
		if !isStandardGATag(r.TagName) {
			continue
		}
		name := releaseNameOfTag(r.TagName)
		if name == "" || app.MajorOfVersion(name) <= 0 {
			continue // 版本段缺失/非数字的畸形 tag, 防御性跳过
		}
		zip, sha, ok := pickWindowsAssets(r.Assets)
		if !ok {
			continue
		}
		out = append(out, dwRelease{
			ReleaseName: name,
			ZipURL:      zip.BrowserDownloadURL,
			SHA256URL:   sha.BrowserDownloadURL,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return app.CompareVersions(out[i].ReleaseName, out[j].ReleaseName) > 0
	})
	return out
}

// isStandardGATag 判断 tag 是否为 Standard 风味 GA release:
// 前缀区分风味 (extended/preview/2022-08 前的裸前缀/下划线古董全排除),
// 尾缀排除非 GA 变体 (如 "dragonwell-11.0.9.4_jdk-11.0.9+0")。
// .prerelease 字段不可用 (extended 与 preview 也标 false), 只认命名。
func isStandardGATag(tag string) bool {
	return strings.HasPrefix(tag, tagPrefix) && strings.HasSuffix(tag, tagSuffix)
}

// releaseNameOfTag 从 Standard GA tag 提取 ReleaseName (dragonwell 侧版本原样):
// 去掉前缀与 "_jdk...-ga" 尾巴 (8 线尾巴是 "_jdk8u502-ga", jdk 后无横线)。
// 无 "_jdk" 分隔或版本段为空的 tag 返回空串 (调用方跳过)。三线示例:
//
//	"dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga" → "21.0.12.0.12+8"
//	"dragonwell-standard-11.0.32.28_jdk-11.0.32-ga"     → "11.0.32.28"
//	"dragonwell-standard-8.30.29_jdk8u502-ga"           → "8.30.29"
func releaseNameOfTag(tag string) string {
	rest := strings.TrimPrefix(tag, tagPrefix)
	if i := strings.Index(rest, tagSep); i > 0 {
		return rest[:i]
	}
	return ""
}

// pickWindowsAssets 从 release 资产列表挑选 x64_windows zip 及其 .sha256.txt 旁路,
// 找不到任一返回 ok=false。zip 按前缀 + 后缀锚定结尾匹配 —— 禁止由 tag 拼资产名
// (11 线 tag 11.0.32.28 的资产实为 ..._11.0.32.28.9_x64_windows.zip, 多一个
// OpenJDK build 段); 后缀锚定同时排除 "-fix.zip" 变体与各类旁路文件。
func pickWindowsAssets(assets []ghAsset) (zip, sha ghAsset, ok bool) {
	var zipName string
	for _, a := range assets {
		if strings.HasPrefix(a.Name, zipPrefix) && strings.HasSuffix(a.Name, zipSuffix) {
			zip, zipName = a, a.Name
			break
		}
	}
	if zipName == "" {
		return ghAsset{}, ghAsset{}, false
	}
	want := zipName + sidecarSuffix
	for _, a := range assets {
		if a.Name == want {
			return zip, a, true
		}
	}
	return ghAsset{}, ghAsset{}, false
}

// buildAsset 拉取 .sha256.txt 旁路补齐校验和, 组装完整 Asset。
func buildAsset(rel dwRelease, major int) (*app.Asset, error) {
	sum, err := fetchSHA256(rel.SHA256URL)
	if err != nil {
		return nil, err
	}
	return &app.Asset{
		Semver:      rel.ReleaseName,
		Major:       major,
		ZipURL:      rel.ZipURL,
		Checksum:    sum,
		ReleaseName: rel.ReleaseName,
		Distro:      distroName,
		// MirrorURL 留空: 直连 GitHub; ChecksumAlgo 留空 (默认 sha256)
	}, nil
}

// fetchSHA256 拉取 .sha256.txt 旁路并解析哈希。
// 旁路是两列 sha256sum 格式, 但第二列是内部构建路径 (target/windows/...)
// 而非资产名 —— 解析只认第一列 hex, 不按文件名匹配。
func fetchSHA256(u string) (string, error) {
	body, err := app.HTTPGetText(u)
	if err != nil {
		return "", fmt.Errorf("获取 Dragonwell SHA256 失败: %w", err)
	}
	sum, err := parseSidecar(string(body))
	if err != nil {
		return "", fmt.Errorf("Dragonwell SHA256 文件格式异常: %w", err)
	}
	return sum, nil
}

// sidecarHexRe 校验 SHA256 旁路解析出的哈希 (64 位 hex)。
var sidecarHexRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// parseSidecar 从旁路文件内容解析 SHA256 哈希: 首个空白分隔 token 且必须
// 是 64 位 hex。纯函数, 便于表测。
func parseSidecar(body string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(body))
	if len(fields) < 1 || !sidecarHexRe.MatchString(fields[0]) {
		return "", fmt.Errorf("不是 64 位 hex 哈希")
	}
	return strings.ToLower(fields[0]), nil
}

// isRateLimited 识别 GitHub API 限流 (app.HTTPGetJSON 的错误串形如 "API 返回 403")。
// 未认证限额 60 次/小时, 常规查询不触顶; 连续 --refresh 或与其他走 GitHub 的
// provider 叠加时可能命中。
func isRateLimited(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "API 返回 403") || strings.Contains(msg, "API 返回 429")
}

// supported 判断 major 是否在主线清单里。
func supported(major int) bool {
	for _, m := range majors {
		if m == major {
			return true
		}
	}
	return false
}
