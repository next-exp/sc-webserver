$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
# Runs in the operator's interactive session; session 0 cannot capture the desktop.
$config = Get-Content -Raw "$PSScriptRoot\deployment.json" | ConvertFrom-Json
if (-not $config.Listen) { throw 'Missing Listen setting in deployment.json; run install.ps1 first.' }
& "$PSScriptRoot\sc-webserver.exe" -listen $config.Listen -temp-dir "$env:TEMP\sc-webserver" *> "$PSScriptRoot\sc-webserver.log"
exit $LASTEXITCODE
