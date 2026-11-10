// 版本串语义判据的单一事实源。
//
// 「是否纯大版本号」与「从版本串提取大版本」原本在 6 个 provider 与 junction
// 各写一遍且语义不一 (首点切段遇无点形态全灭; "不含点"式的 isPureMajor 会把
// liberica 的 "8u504+1" 误判成大版本号查询)。全部路由判据收敛到本文件,
// provider 禁止再自建同类逻辑 —— 新发行版的版本串形态只须在本包表测的
// fixture 清单里补用例 (见 version_test.go)。
package app

import (
	"fmt"
	"strconv"
	"strings"
)

// IsPureMajor 报告 s 是否为纯数字大版本号 (如 "8" / "21") —— 用户输入中唯一
// 表示"取该大版本最新版"的形态。带 patch/build/更新号的串 (点分 semver、
// "jdk8u512-b09"、"8u504+1" 等) 都不是。不 trim 空白, 调用方自行处理。
func IsPureMajor(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	n, err := strconv.Atoi(s)
	return err == nil && n > 0
}

// MajorOfVersion 从版本串宽容提取大版本号: 先剥可选的 "jdk-"/"jdk"/"JDK-"/"JDK"
// 前缀 (顺序敏感: 带 "-" 的在前), 再取开头连续数字。
// "21.0.5+11"→21、"8.452.09.1"→8、"8u504+1"→8、"jdk8u512-b09"→8、"8"→8;
// 解析不出正整数返回 0, 由调用方决定如何报错。
// 注意: 输入是版本串; 目录名 ({distro}-{version}) 须先经 SplitDistro 拆分
// (junction.MajorOf 负责), 本函数不认识 distro 前缀。
func MajorOfVersion(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "jdk-")
	s = strings.TrimPrefix(s, "jdk")
	s = strings.TrimPrefix(s, "JDK-")
	s = strings.TrimPrefix(s, "JDK")
	end := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if end < 0 {
		end = len(s)
	}
	if end == 0 {
		return 0
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// ParseMajorOfVersion 是 MajorOfVersion 的报错版: 提取失败 (返回 0) 时给出
// 与 ParseMajorVersion 一致的「无效的版本号」错误。供 provider Resolve 在
// 「任意形态输入都要先拿到大版本」的场景统一使用, 错误文案不在各 provider 重复。
func ParseMajorOfVersion(s string) (int, error) {
	if m := MajorOfVersion(s); m > 0 {
		return m, nil
	}
	return 0, fmt.Errorf("无效的版本号: %s (请输入大版本号, 例如 21、17、11)", s)
}
