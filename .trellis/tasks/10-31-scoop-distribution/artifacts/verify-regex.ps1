# 演练 scoop autoupdate 的 hash 提取（复刻 ScoopInstaller v0.6.0 lib/autoupdate.ps1
# 的 find_hash_in_textfile 替换管线），对 checksums.txt 逐架构验证 regex 命中。
# 用法: powershell -NoProfile -ExecutionPolicy Bypass -File verify-regex.ps1 <checksums.txt 路径>
param([Parameter(Mandatory = $true)][string]$ChecksumsPath)

$hashfile = Get-Content -Raw $ChecksumsPath

# manifest 里写的 regex 原文（与 jvm.json 的 autoupdate.hash.regex 完全一致）
$rawRegex = '$sha256\s+$basename'

# 第一步：模板替换（scoop 源码 substitute $regex $templates $false，不转义）
# autoupdate.ps1 第 49 行: '$sha256' = '([a-fA-F0-9]{64})'
$templates = @{ '$sha256' = '([a-fA-F0-9]{64})' }
$regex = $rawRegex
foreach ($k in $templates.Keys) { $regex = $regex.Replace($k, $templates[$k]) }

# 第二步：用户替换（substitute ... $true，值经 [Regex]::Escape，core.ps1 第 1318 行）
foreach ($basename in 'jvm-windows-amd64.zip', 'jvm-windows-arm64.zip') {
    $r = $regex.Replace('$basename', [Regex]::Escape($basename))
    if ($hashfile -match $r) {
        '{0} -> {1}' -f $basename, $Matches[1]
    } else {
        '{0} -> NO MATCH' -f $basename
    }
}
