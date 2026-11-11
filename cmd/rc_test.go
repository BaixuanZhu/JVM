// rc_test.go 覆盖 rc.go 的宽松版本匹配 (外来 rc 格式的半截版本号降级)
// 与 use/exec 共用的版本解析入口 resolveUseDir (失败文案带统一安装提示)。
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jvm/internal/app"
)

// TestResolveVersionLoose 用临时版本目录验证宽松匹配的三个分支:
// 原样命中 / 降级到大版本组内最新 / 彻底失败返回原样错误。
func TestResolveVersionLoose(t *testing.T) {
	root := withTempVersions(t)
	// 已装: temurin 21 组两个 patch + corretto 17 组一个
	for _, d := range []string{"temurin-21.0.5+11", "temurin-21.0.2+8", "corretto-17.0.12.8.1"} {
		if err := os.MkdirAll(filepath.Join(root, "versions", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name    string
		spec    string
		want    string // 期望目录名; "" 表示期望报错
		wantErr string // 期望错误文案包含的关键字 (want 非空时忽略)
	}{
		// 原样命中: 完整版本号 (含 build 号) 不需要降级
		{"完整版本号原样命中", "temurin@21.0.5+11", "temurin-21.0.5+11", ""},
		// 纯大版本号: ResolveVersion 的 major 分支直接给组内最新
		{"纯大版本号取组内最新", "temurin@21", "temurin-21.0.5+11", ""},
		// 半截版本号 (sdkman/asdf 生态常态): 降级为大版本组内最新
		{"半截版本降级到组内最新", "temurin@21.0.2", "temurin-21.0.5+11", ""},
		{"sdkman 四段式降级", "corretto@17.0.12", "corretto-17.0.12.8.1", ""},
		// 彻底失败: 组/发行版不存在, 降级也无济于事, 返回原样错误
		{"组不存在", "temurin@25.0.1", "", "没有找到"},
		{"发行版不存在", "zulu@21.0.2", "", "没有安装"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := app.ParseVersionSpec(tt.spec)
			if err != nil {
				t.Fatalf("ParseVersionSpec(%q): %v", tt.spec, err)
			}
			got, err := resolveVersionLoose(spec)
			if tt.want != "" {
				if err != nil {
					t.Fatalf("resolveVersionLoose(%q) 意外报错: %v", tt.spec, err)
				}
				if got != tt.want {
					t.Errorf("resolveVersionLoose(%q) = %q, want %q", tt.spec, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("resolveVersionLoose(%q) 期望报错, got %q", tt.spec, got)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("错误文案应含 %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// TestResolveUseDir 覆盖 use/exec 共用解析入口的成败分支: 成功返回目录名;
// 失败文案 = junction 原错误 + 统一先安装提示 (installHint), 与 exec 既有
// 输出同款 (use 的两条失败路径由此对齐)。
func TestResolveUseDir(t *testing.T) {
	root := withTempVersions(t)
	// 已装: temurin 21 组一个 (跨组/跨发行版场景未装)
	if err := os.MkdirAll(filepath.Join(root, "versions", "temurin-21.0.5+11"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		spec     string
		loose    bool
		wantDir  string   // 期望目录名; "" 表示期望失败
		wantMsgs []string // 失败文案应包含的片段 (wantDir 非空时忽略)
	}{
		{"严格命中完整版本", "temurin@21.0.5+11", false, "temurin-21.0.5+11", nil},
		{"严格命中纯大版本", "temurin@21", false, "temurin-21.0.5+11", nil},
		{"宽松命中半截版本", "temurin@21.0.2", true, "temurin-21.0.5+11", nil},
		// use 显式参数失败: junction 错误 + 原始输入回显的先安装提示
		{"严格未装: 错误+提示", "temurin@25.0.1", false, "",
			[]string{"没有找到", "\n  先安装: jvm install temurin@25.0.1"}},
		{"严格未装组: 错误+提示", "temurin@17", false, "",
			[]string{"没有安装 temurin 大版本 17", "\n  先安装: jvm install temurin@17"}},
		// use 无参外来 rc 宽松路径失败: 同样补提示 (对齐 exec 的 loose 分支)
		{"宽松未装: 错误+提示", "temurin@25.0.2", true, "",
			[]string{"没有找到", "\n  先安装: jvm install temurin@25.0.2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := app.ParseVersionSpec(tt.spec)
			if err != nil {
				t.Fatalf("ParseVersionSpec(%q): %v", tt.spec, err)
			}
			dir, failMsg := resolveUseDir(spec, tt.loose)
			if tt.wantDir != "" {
				if failMsg != "" {
					t.Fatalf("resolveUseDir(%q) 意外失败: %s", tt.spec, failMsg)
				}
				if dir != tt.wantDir {
					t.Errorf("resolveUseDir(%q) = %q, want %q", tt.spec, dir, tt.wantDir)
				}
				return
			}
			if failMsg == "" {
				t.Fatalf("resolveUseDir(%q) 期望失败, got 目录 %q", tt.spec, dir)
			}
			for _, want := range tt.wantMsgs {
				if !strings.Contains(failMsg, want) {
					t.Errorf("失败文案应含 %q, 实际:\n%s", want, failMsg)
				}
			}
		})
	}
}

// TestResolveUseDirNoVersions 锁定「一台版本都没有」时的文案: junction 的
// 既有安装引导保留原样, 提示行照常追加 (与 exec 行为一致)。
func TestResolveUseDirNoVersions(t *testing.T) {
	withTempVersions(t) // 空版本目录
	spec, err := app.ParseVersionSpec("temurin@21")
	if err != nil {
		t.Fatalf("ParseVersionSpec: %v", err)
	}
	_, failMsg := resolveUseDir(spec, false)
	for _, want := range []string{"还没有安装任何版本", "\n  先安装: jvm install temurin@21"} {
		if !strings.Contains(failMsg, want) {
			t.Errorf("失败文案应含 %q, 实际:\n%s", want, failMsg)
		}
	}
}
