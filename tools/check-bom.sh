#!/usr/bin/env bash
# BOM Check — 检查项目中的 Go / TSX 文件是否含有 UTF-8 BOM
# 可在 pre-commit hook 或 CI 中调用
# 用法: bash tools/check-bom.sh

set -euo pipefail
cd "$(dirname "$0")/.."

has_bom=0
while IFS= read -r -d '' file; do
    if [[ "$(head -c3 "$file")" == $'\xef\xbb\xbf' ]]; then
        echo "BOM found: $file"
        has_bom=1
    fi
done < <(find . -type f \( -name '*.go' -o -name '*.tsx' -o -name '*.ts' \) \
    -not -path './web/admin/node_modules/*' \
    -not -path './.git/*' \
    -print0)

if [[ $has_bom -eq 1 ]]; then
    echo ""
    echo "ERROR: UTF-8 BOM detected in files listed above."
    echo "Run: pwsh tools/fix-bom.ps1"
    exit 1
fi
echo "No BOM issues detected."
