// cache_zip.go 维护版本目录与下载缓存 zip 的生命周期联动。
//
// internal/jdk 安装时把安装包 zip 以 {最终目录名}.zip 留存缓存目录 (下载落盘
// 的命名见 jdk.Install 里的 cacheFile 拼装), 本文件按同一约定反向定位版本目录
// 对应的缓存 zip: update 收敛到新 patch 时旧 zip 直接删 (update 语义下旧 patch
// 不会再装, 缓存是死重), uninstall 则删完版本目录后交用户决定 (交互询问)。
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jvm/internal/paths"
)

// cacheZip 是一个版本目录对应的缓存 zip 条目 (文件名与大小), 供卸载时列清单、
// 合计大小与逐个删除。
type cacheZip struct {
	name string // 缓存文件名 (版本目录名 + .zip)
	size int64  // 字节数
}

// cacheZipPath 返回版本目录 dir 对应的缓存 zip 路径: {CacheDir}/{dir}.zip。
// 与 internal/jdk 下载落盘的命名 (jdk.Install 里 cacheFile 的拼装) 是同一约定,
// 目录名到缓存文件名的规则若变须两处同步 (jdk 侧是写入方, 这里是消费方)。
func cacheZipPath(dir string) string {
	return filepath.Join(paths.CacheDir, dir+".zip")
}

// collectCacheZips 收集 dirs 中实际存在对应缓存 zip 的条目 (文件名+大小),
// 不存在或异常的静默跳过 —— 结果为空即上层不询问不提示 (无噪音原则)。
func collectCacheZips(dirs []string) []cacheZip {
	var out []cacheZip
	for _, d := range dirs {
		info, err := os.Stat(cacheZipPath(d))
		if err != nil || info.IsDir() {
			continue
		}
		out = append(out, cacheZip{name: d + ".zip", size: info.Size()})
	}
	return out
}

// formatMB 把字节数格式化为 "X.X MB" (与全库 %.1f MB 口径一致)。纯函数。
func formatMB(n int64) string {
	return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
}

// totalCacheZipSize 求缓存 zip 条目的合计字节数。纯函数。
func totalCacheZipSize(zips []cacheZip) int64 {
	var total int64
	for _, z := range zips {
		total += z.size
	}
	return total
}

// buildCacheZipPrompt 构造缓存 zip 的清理询问文案: 单个 zip 内联在括号里,
// 多个 zip 先列清单再给合计, 末尾统一接 [y/N] 供 confirm 使用。
// 纯函数, 便于表驱动测试。
func buildCacheZipPrompt(zips []cacheZip) string {
	if len(zips) == 1 {
		return fmt.Sprintf("是否同时删除缓存 zip? (%s, %s) [y/N] ", zips[0].name, formatMB(zips[0].size))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "是否同时删除以下 %d 个缓存 zip (合计 %s)?\n", len(zips), formatMB(totalCacheZipSize(zips)))
	for _, z := range zips {
		fmt.Fprintf(&b, "  %s (%s)\n", z.name, formatMB(z.size))
	}
	b.WriteString("[y/N] ")
	return b.String()
}

// removeCacheZipIfPresent 删除版本目录 dir 对应的缓存 zip (不存在则静默返回)。
// update 清理旧 patch 目录时顺带调用: 删除失败宽容继续 (可能被占用), 只提示
// 一句并指向 jvm cache clean, 不影响升级成败判定。
func removeCacheZipIfPresent(dir string) {
	p := cacheZipPath(dir)
	if info, err := os.Stat(p); err != nil || info.IsDir() {
		return
	}
	if err := os.Remove(p); err != nil {
		fmt.Printf("缓存 zip %s.zip 删除失败, 可稍后 jvm cache clean 清理\n", dir)
	}
}

// promptCacheCleanup 在版本目录删除完成后处理缓存 zip 去留 (uninstall 单版本
// 与 --all 组卸载共用): 收集实际存在的缓存 zip, 无则静默返回 (无缓存时行为与
// 从前一致); 有则按 assumeYes 与用户作答决定 —— -y 跳过询问且默认保留 (脚本
// 场景不静默删大文件, 保守默认), 交互式询问一次, 答 y 逐个删除 (失败宽容),
// 其余保留并提示缓存还在、多大、怎么清。
func promptCacheCleanup(dirs []string, assumeYes bool) {
	zips := collectCacheZips(dirs)
	if len(zips) == 0 {
		return
	}
	if assumeYes {
		printCacheKept(zips)
		return
	}
	if confirm(buildCacheZipPrompt(zips)) {
		removeCacheZips(zips)
		return
	}
	printCacheKept(zips)
}

// removeCacheZips 逐个删除缓存 zip (uninstall 询问后答 y 的执行段):
// 删除失败宽容继续 (可能被占用), 提示指向 jvm cache clean。
func removeCacheZips(zips []cacheZip) {
	for _, z := range zips {
		if err := os.Remove(filepath.Join(paths.CacheDir, z.name)); err != nil {
			fmt.Printf("缓存 zip %s 删除失败, 可稍后 jvm cache clean 清理\n", z.name)
		}
	}
}

// printCacheKept 打印缓存保留提示 (答 N 或 -y 默认保留时): 用户明确知道缓存
// 还在、多大、怎么清。
func printCacheKept(zips []cacheZip) {
	fmt.Printf("安装包缓存已保留 (%d 个, %s), 重装免下载; jvm cache clean 可清理\n",
		len(zips), formatMB(totalCacheZipSize(zips)))
}
