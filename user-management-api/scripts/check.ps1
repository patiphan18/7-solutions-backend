$ErrorActionPreference = 'Stop'
Push-Location (Split-Path -Parent $PSScriptRoot)
try {
    $files = @(Get-ChildItem -LiteralPath cmd, internal, pkg, scripts -Recurse -Filter *.go | Select-Object -ExpandProperty FullName)
    $unformatted = @(& gofmt -l @files)
    if ($LASTEXITCODE -ne 0) { throw 'gofmt failed' }
    if ($unformatted.Count -gt 0) { throw "Unformatted files: $($unformatted -join ', ')" }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
} finally {
    Pop-Location
}
