[CmdletBinding()]
param([string]$BaseURL = $env:BASE_URL, [string]$OutputDirectory = "k6-results")

$ErrorActionPreference = "Stop"
if (-not $BaseURL) { throw "Pass -BaseURL or set BASE_URL." }
if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) { throw "k6 is required." }
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null

$scenarios = @(
    @{ Name = "functional-workflow"; File = "k6/functional-workflow.js" },
    @{ Name = "cancellation-idempotency"; File = "k6/cancellation-idempotency.js" },
    @{ Name = "ride-burst"; File = "k6/ride-burst.js" },
    @{ Name = "location-updates"; File = "k6/location-updates.js" },
    @{ Name = "matching-race"; File = "k6/matching-race.js" }
)

foreach ($scenario in $scenarios) {
    $output = Join-Path $OutputDirectory "$($scenario.Name).json"
    Write-Host "Running $($scenario.Name) against $BaseURL"
    k6 run --env BASE_URL=$BaseURL --summary-export=$output $scenario.File
    if ($LASTEXITCODE -ne 0) { throw "$($scenario.Name) failed." }
}
