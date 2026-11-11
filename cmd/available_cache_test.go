package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jvm/internal/app"
	"jvm/internal/provider"
)

// rewriteCacheFile 读改写缓存文件 (测试操纵条目时间戳/架构用)。
func rewriteCacheFile(t *testing.T, mutate func(c *availableCache)) {
	t.Helper()
	c := loadCacheFile()
	mutate(&c)
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(availableCachePath(), data, 0o644); err != nil {
		t.Fatalf("写缓存文件失败: %v", err)
	}
}

func TestTableCacheRoundTrip(t *testing.T) {
	dir := withTempVersions(t)

	// 骨架行只存 major+LTS; latest 是 LatestPatch 产物, 走独立的逐条缓存
	rows := []availableRow{
		{major: 25, latest: "25.0.4+1", lts: false},
		{major: 21, latest: "21.0.8+7", lts: true},
	}
	saveTableCache("fake-rt", rows)
	savePatchCache("fake-rt", 25, "25.0.4+1")

	got, ok := loadTableCache("fake-rt")
	if !ok {
		t.Fatal("保存后应命中")
	}
	if len(got) != 2 || got[0].major != 25 || got[0].latest != "" || !got[1].lts {
		t.Errorf("骨架往返结果 = %+v, 想 major/lts 骨架 (latest 为空, 由逐条缓存补)", got)
	}
	if v, ok := loadPatchCache("fake-rt", 25); !ok || v != "25.0.4+1" {
		t.Errorf("patch 条目 = %q ok=%v, 想 25.0.4+1", v, ok)
	}
	if _, ok := loadTableCache("other-distro"); ok {
		t.Error("未保存过的 distro 不应命中")
	}
	if _, ok := loadPatchCache("fake-rt", 21); ok {
		t.Error("未保存过的 major 不应命中 patch 条目")
	}
	if _, ok := loadPatchCache("other-distro", 25); ok {
		t.Error("未保存过的 distro 不应命中 patch 条目")
	}
	if !filepath.IsAbs(availableCachePath()) || !strings.HasPrefix(availableCachePath(), dir) {
		t.Errorf("缓存文件应落在临时 Root 下, 实际 %s", availableCachePath())
	}
}

func TestTableCacheTTlExpiry(t *testing.T) {
	withTempVersions(t)
	saveTableCache("fake-ttl", []availableRow{{major: 21}})

	// 时间戳拨回 TTL 之前 → 过期 miss
	rewriteCacheFile(t, func(c *availableCache) {
		t := c.Tables["fake-ttl"]
		t.SavedAt = time.Now().UTC().Add(-availableCacheTTL - time.Minute).Format(time.RFC3339)
		c.Tables["fake-ttl"] = t
	})
	if _, ok := loadTableCache("fake-ttl"); ok {
		t.Error("过期条目不应命中")
	}
}

func TestTableCacheArchMismatch(t *testing.T) {
	withTempVersions(t)
	saveTableCache("fake-arch", []availableRow{{major: 21}})

	// 架构改成与当前不同的规范值 → 整体 miss
	other := app.ArchX64
	if cacheArchKey() == app.ArchX64 {
		other = app.ArchARM64
	}
	rewriteCacheFile(t, func(c *availableCache) { c.Arch = other })
	if _, ok := loadTableCache("fake-arch"); ok {
		t.Error("架构不匹配不应命中")
	}
}

// TestPatchCacheEntryLevelMerge 验证 patch 条目逐条落盘互不清除
// (outdated 只查几个组时不得碰掉其他组的条目)。
func TestPatchCacheEntryLevelMerge(t *testing.T) {
	withTempVersions(t)
	savePatchCache("fakepm", 21, "21.0.8+7")
	savePatchCache("fakepm", 25, "25.0.4+1")
	savePatchCache("otherpm", 21, "21.0.9+1")

	if v, ok := loadPatchCache("fakepm", 21); !ok || v != "21.0.8+7" {
		t.Errorf("fakepm@21 = %q ok=%v", v, ok)
	}
	if v, ok := loadPatchCache("fakepm", 25); !ok || v != "25.0.4+1" {
		t.Errorf("fakepm@25 = %q ok=%v (应保留)", v, ok)
	}
	if v, ok := loadPatchCache("otherpm", 21); !ok || v != "21.0.9+1" {
		t.Errorf("otherpm@21 = %q ok=%v (不同 distro 互不影响)", v, ok)
	}
}

// TestPatchCacheTTlExpiry 验证 patch 条目 TTL 过期后 miss。
func TestPatchCacheTTlExpiry(t *testing.T) {
	withTempVersions(t)
	savePatchCache("fakept", 21, "21.0.8+7")

	rewriteCacheFile(t, func(c *availableCache) {
		key := groupCacheKey("fakept", 21)
		e := c.Patches[key]
		e.SavedAt = time.Now().UTC().Add(-availableCacheTTL - time.Minute).Format(time.RFC3339)
		c.Patches[key] = e
	})
	if _, ok := loadPatchCache("fakept", 21); ok {
		t.Error("过期 patch 条目不应命中")
	}
}

// TestPatchCacheArchMismatch 验证架构变化后 patch 条目整体 miss
// (x64/aarch64 版本集合不同)。
func TestPatchCacheArchMismatch(t *testing.T) {
	withTempVersions(t)
	savePatchCache("fakepa", 21, "21.0.8+7")

	other := app.ArchX64
	if cacheArchKey() == app.ArchX64 {
		other = app.ArchARM64
	}
	rewriteCacheFile(t, func(c *availableCache) { c.Arch = other })
	if _, ok := loadPatchCache("fakepa", 21); ok {
		t.Error("架构不匹配不应命中 patch 条目")
	}
}

// TestCacheOldFileCompat 旧版本缓存文件 (表格行带 latest 字段、无 patches 键):
// 新代码忽略未知 JSON 字段, 骨架照常命中; Patches 为空 → 逐条全 miss 自然重查
// 回填, 不需要迁移代码。
func TestCacheOldFileCompat(t *testing.T) {
	withTempVersions(t)
	now := time.Now().UTC().Format(time.RFC3339)
	old := fmt.Sprintf(`{"arch":%q,"tables":{"fake-old":{"saved_at":%q,"rows":[`+
		`{"major":21,"lts":true,"latest":"21.0.8+7"},{"major":17,"lts":true,"latest":"17.0.12+8"}]}},"groups":{}}`,
		cacheArchKey(), now)
	if err := os.WriteFile(availableCachePath(), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, ok := loadTableCache("fake-old")
	if !ok || len(rows) != 2 || rows[0].major != 21 || !rows[0].lts || !rows[1].lts {
		t.Errorf("旧文件骨架应命中 (latest 字段被忽略), 实际 ok=%v rows=%+v", ok, rows)
	}
	if _, ok := loadPatchCache("fake-old", 21); ok {
		t.Error("旧文件无 patches 条目, 应全 miss 重查回填")
	}
}

func TestGroupCacheIncrementalMerge(t *testing.T) {
	withTempVersions(t)

	// 先存 21 组, 再存 25 组: 条目级 merge, 两次保存互不清除
	saveGroupCache("fakeg", versionGroup{major: 21, lts: true, versions: []string{"21.0.8+7", "21.0.5+11"}})
	saveGroupCache("fakeg", versionGroup{major: 25, lts: false, versions: []string{"25.0.4+1"}})

	g21, ok := loadGroupCache("fakeg", 21)
	if !ok || !g21.lts || len(g21.versions) != 2 {
		t.Errorf("21 组 = %+v ok=%v, 想保留两条版本", g21, ok)
	}
	g25, ok := loadGroupCache("fakeg", 25)
	if !ok || g25.lts || g25.versions[0] != "25.0.4+1" {
		t.Errorf("25 组 = %+v ok=%v", g25, ok)
	}
	if _, ok := loadGroupCache("fakeg", 17); ok {
		t.Error("未保存的组不应命中")
	}
}

// countingProvider 统计 LatestPatch 调用次数, 供缓存命中路径断言零 API 调用。
type countingProvider struct {
	provider.Base
	name  string
	calls int32 // atomic; -race 下并发安全
}

func (c *countingProvider) Name() string     { return c.name }
func (countingProvider) DisplayName() string { return "Counting" }
func (countingProvider) Available() ([]app.Release, error) {
	return []app.Release{{Major: 17, LTS: true}, {Major: 21, LTS: true}}, nil
}
func (c *countingProvider) LatestPatch(major int) (*app.Asset, error) {
	atomic.AddInt32(&c.calls, 1)
	return &app.Asset{ReleaseName: fmt.Sprintf("%d.0.1+1", major), Major: major}, nil
}
func (countingProvider) Resolve(app.VersionSpec) (*app.Asset, error) { return nil, nil }
func (countingProvider) ListVersions(int) ([]*app.Asset, error)      { return nil, nil }

// TestAvailableTableCacheHit 验证二次查询命中缓存零 API 调用、--refresh 绕过。
func TestAvailableTableCacheHit(t *testing.T) {
	withTempVersions(t)
	p := &countingProvider{name: "fake-count-" + t.Name()}
	provider.Register(p)

	availableTable(AvailableOptions{}, p.name) // 首次: 直查 + 落缓存 (2 个 major → 2 次 LatestPatch)
	if n := atomic.LoadInt32(&p.calls); n != 2 {
		t.Fatalf("首次查询 LatestPatch 调用 %d 次, 想 2", n)
	}

	out := captureStdout(t, func() { availableTable(AvailableOptions{}, p.name) })
	if n := atomic.LoadInt32(&p.calls); n != 2 {
		t.Errorf("二次查询应命中缓存零 API 调用, 实际又调了 %d 次", n-2)
	}
	if !strings.Contains(out, "缓存") {
		t.Errorf("命中缓存应打印说明行, 输出: %s", out)
	}

	captureStdout(t, func() { availableTable(AvailableOptions{Refresh: true}, p.name) }) // --refresh 绕过
	if n := atomic.LoadInt32(&p.calls); n != 4 {
		t.Errorf("--refresh 应绕过缓存直查 (4 次), 实际 %d 次", n)
	}
}
