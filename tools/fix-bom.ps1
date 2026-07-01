# Fix BOM — removes UTF-8 BOM from all .go/.ts/.tsx files in the project
# Usage: pwsh tools/fix-bom.ps1

$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot/.."

$fixed = 0
Get-ChildItem -Recurse -Include *.go,*.ts,*.tsx |
    Where-Object { $_.FullName -notmatch '\\node_modules\\' -and $_.FullName -notmatch '\\.git\\' } |
    ForEach-Object {
        $bytes = [System.IO.File]::ReadAllBytes($_.FullName)
        if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
            $newBytes = $bytes[3..($bytes.Length - 1)]
            [System.IO.File]::WriteAllBytes($_.FullName, $newBytes)
            Write-Host "Fixed: $($_.FullName)"
            $fixed++
        }
    }

Write-Host "Done. $fixed file(s) fixed."
