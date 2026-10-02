# Run go test -race inside a golang container (Windows has no gcc for -race).
# ASCII only: Windows PowerShell 5.1 reads BOM-less scripts with the ANSI codepage.
#
#   ./scripts/test-race.ps1               # all tests
#   ./scripts/test-race.ps1 -Concurrent   # only TestConcurrent* with -count=100
#
# Requires `docker compose up -d` first: the container reaches postgres via network devpool-net.
param(
    [switch]$Concurrent
)

# No $ErrorActionPreference = 'Stop' here: PowerShell 5.1 turns docker's stderr
# (e.g. "go: downloading ...") into a terminating error. The exit code decides instead.

$root = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $root 'backend'

# Image tag follows the `go` line in go.mod
$goLine = Select-String -Path (Join-Path $backend 'go.mod') -Pattern '^go\s+(\S+)' | Select-Object -First 1
if (-not $goLine) { throw 'no go directive found in backend/go.mod' }
$goVersion = $goLine.Matches[0].Groups[1].Value
$image = "golang:$goVersion"

$testArgs = @('test', '-race', '-p', '1')
if ($Concurrent) {
    $testArgs += @('-run', '^TestConcurrent', '-count=100')
}
$testArgs += './...'

Write-Host "image: $image"
Write-Host "go $($testArgs -join ' ')"

docker run --rm `
    --network devpool-net `
    -v "${backend}:/src" `
    -w /src `
    -v devpool-gomod:/go/pkg/mod `
    -v devpool-gocache:/root/.cache/go-build `
    -e 'TEST_DATABASE_URL=postgres://devpool:devpool@postgres:5432/devpool_test?sslmode=disable' `
    $image go @testArgs

exit $LASTEXITCODE
