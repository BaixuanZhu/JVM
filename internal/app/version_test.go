// IsPureMajor / MajorOfVersion 的表驱动测试。
//
// fixture 覆盖全部已发行版本串的真实形态 —— 这份清单是"版本号形态注册表":
// 新增发行版或上游改命名时, 在这里补用例即可防回归。
package app

import "testing"

func TestIsPureMajor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"纯大版本", "8", true},
		{"纯大版本两位", "21", true},
		{"纯大版本三位", "25", true},
		{"空串", "", false},
		{"零", "0", false},
		{"负数", "-8", false},
		{"带空白", " 21", false},
		{"temurin 8 release_name 形态", "jdk8u512-b09", false},
		{"liberica 8 版本形态", "8u504+1", false},
		{"点分 semver", "21.0.5+11", false},
		{"四段式新版本号", "25.0.4.1+1", false},
		{"corretto 点分形态", "8.452.09.1", false},
		{"semeru 新格式四段", "21.0.13.0", false},
		{"semeru 旧格式含 openj9 串", "21.0.5+11_openj9-0.48.0", false},
		{"semeru 8 线旧格式", "8u402-b06_openj9-0.43.0", false},
		{"semeru 8 线新格式", "8.0.492.0", false},
		{"dragonwell 五段+build 形态", "21.0.12.0.12+8", false},
		{"dragonwell 8 线三段形态", "8.30.29", false},
		{"kona 五段点分形态", "21.0.13.0.3", false},
		{"kona 四段重建形态", "17.0.4.1", false},
		{"EA 形态", "28+14-ea-beta", false},
		{"垃圾输入", "abc", false},
		{"数字带字母尾巴", "21a", false},
	}

	for _, tt := range tests {
		if got := IsPureMajor(tt.input); got != tt.want {
			t.Errorf("IsPureMajor(%q) = %v, want %v (%s)", tt.input, got, tt.want, tt.name)
		}
	}
}

func TestParseMajorOfVersion(t *testing.T) {
	if m, err := ParseMajorOfVersion("8u504+1"); err != nil || m != 8 {
		t.Errorf("ParseMajorOfVersion(\"8u504+1\") = %d, %v; want 8, nil", m, err)
	}
	if _, err := ParseMajorOfVersion("abc"); err == nil {
		t.Error("ParseMajorOfVersion(\"abc\") 期望报错")
	}
}

func TestMajorOfVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		// 各发行版真实版本串形态 (已逐一实测上游 API)
		{"纯大版本", "8", 8},
		{"temurin 点分", "21.0.5+11", 21},
		{"temurin jdk- 前缀", "jdk-21.0.12+8", 21},
		{"temurin 8 release_name (无点, jdk8u 前缀)", "jdk8u512-b09", 8},
		{"temurin 8 旧 release_name", "jdk8u504-b01", 8},
		{"temurin 四段式", "25.0.4.1+1", 25},
		{"liberica 8 (无点, u 形态)", "8u504+1", 8},
		{"corretto 8 点分", "8.452.09.1", 8},
		{"corretto 四段点分", "17.0.20.8.1", 17},
		{"corretto 五段点分", "26.0.2.10.1", 26},
		{"semeru 新格式四段", "21.0.13.0", 21},
		{"semeru 旧格式含 openj9 串", "21.0.5+11_openj9-0.48.0", 21},
		{"semeru 8 线旧格式 (无点, jdk8u 风格)", "8u402-b06_openj9-0.43.0", 8},
		{"semeru 8 线新格式", "8.0.492.0", 8},
		{"dragonwell 21 线五段+build", "21.0.12.0.12+8", 21},
		{"dragonwell 25 线五段+build", "25.0.4.0.4+7", 25},
		{"dragonwell 11 线四段", "11.0.32.28", 11},
		{"dragonwell 8 线三段", "8.30.29", 8},
		{"kona 五段点分 (2026-10 周期起)", "21.0.13.0.3", 21},
		{"kona 四段重建版", "17.0.4.1", 17},
		{"kona 三段", "11.0.33", 11},
		{"kona 8 线三段", "8.0.28", 8},
		{"zulu 8 点分", "8.0.452+9", 8},
		{"EA build 形态", "28+14-ea-beta", 28},
		{"EA 无 jdk- 前缀", "jdk-25.0.1+13", 25},
		{"大写 JDK 前缀", "JDK-17.0.20+8", 17},
		{"小写无横线前缀", "jdk17.0.20+8", 17},
		// 解析失败返回 0, 由调用方决定报错
		{"空串", "", 0},
		{"纯字母", "abc", 0},
		{"只有前缀", "jdk", 0},
		{"前缀后无数字", "jdk-ea-beta", 0},
		{"u 开头", "u452", 0},
		{"目录名不带 distro 拆分不归本函数管", "temurin-21.0.5+11", 0},
	}
	for _, tt := range tests {
		if got := MajorOfVersion(tt.input); got != tt.want {
			t.Errorf("MajorOfVersion(%q) = %d, want %d (%s)", tt.input, got, tt.want, tt.name)
		}
	}
}
