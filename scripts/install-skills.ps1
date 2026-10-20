param([switch]$MigrateExisting)

$ErrorActionPreference = 'Stop'
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$sourceRoot = Join-Path $repoRoot '.agents\skills'
$personalRoot = Join-Path $HOME '.codex\skills'
$names = @('local-doc-search', 'local-doc-sync')

function Assert-ChildPath([string]$parent, [string]$child, [string]$expectedName) {
    $fullParent = [System.IO.Path]::GetFullPath($parent).TrimEnd('\')
    $fullChild = [System.IO.Path]::GetFullPath($child)
    if (-not $fullChild.StartsWith($fullParent + '\', [System.StringComparison]::OrdinalIgnoreCase) -or
        [System.IO.Path]::GetFileName($fullChild) -ne $expectedName) {
        throw "Unexpected path: $fullChild"
    }
}

foreach ($name in $names) {
    $source = Join-Path $sourceRoot $name
    $destination = Join-Path $personalRoot $name
    Assert-ChildPath $sourceRoot $source $name
    Assert-ChildPath $personalRoot $destination $name
    if (-not (Test-Path -LiteralPath (Join-Path $source 'SKILL.md') -PathType Leaf)) {
        throw "Missing repository skill: $source"
    }
    if (Test-Path -LiteralPath $destination) {
        $item = Get-Item -LiteralPath $destination -Force
        if ($item.LinkType -eq 'SymbolicLink') {
            $resolved = $item.ResolveLinkTarget($true)
            if ($null -ne $resolved -and [System.IO.Path]::GetFullPath($resolved.FullName) -eq [System.IO.Path]::GetFullPath($source)) {
                continue
            }
            throw "A different link already exists: $destination"
        }
        if (-not $MigrateExisting) {
            throw "Existing personal skill: $destination. Run with -MigrateExisting to back it up."
        }
    }
}

New-Item -ItemType Directory -Path $personalRoot -Force | Out-Null
$backupRoot = Join-Path $repoRoot ('data\skill-backups\' + (Get-Date -Format 'yyyyMMdd-HHmmss'))

foreach ($name in $names) {
    $source = Join-Path $sourceRoot $name
    $destination = Join-Path $personalRoot $name
    if (Test-Path -LiteralPath $destination) {
        $item = Get-Item -LiteralPath $destination -Force
        if ($item.LinkType -eq 'SymbolicLink') { continue }
        New-Item -ItemType Directory -Path $backupRoot -Force | Out-Null
        $backup = Join-Path $backupRoot $name
        Assert-ChildPath $backupRoot $backup $name
        if (Test-Path -LiteralPath $backup) { throw "Backup already exists: $backup" }
        Move-Item -LiteralPath $destination -Destination $backup
        try {
            New-Item -ItemType SymbolicLink -Path $destination -Target $source -ErrorAction Stop | Out-Null
        } catch {
            Move-Item -LiteralPath $backup -Destination $destination
            throw
        }
        Write-Output "Linked $name (previous version saved in $backup)"
    } else {
        New-Item -ItemType SymbolicLink -Path $destination -Target $source -ErrorAction Stop | Out-Null
        Write-Output "Linked $name"
    }
}
