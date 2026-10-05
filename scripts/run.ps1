$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
# Runs in the operator's interactive session; session 0 cannot capture the desktop.
$config = Get-Content -Raw "$PSScriptRoot\deployment.json" | ConvertFrom-Json
if (-not $config.Listen) { throw 'Missing Listen setting in deployment.json; run install.ps1 first.' }
$arguments = @('-listen', $config.Listen, '-temp-dir', "$env:TEMP\sc-webserver")
if ($config.Format) { $arguments += @('-format', $config.Format) }
if ($config.Capture) { $arguments += @('-capture', $config.Capture) }
if ($config.WindowTitle) { $arguments += @('-window-title', $config.WindowTitle) }
if ($config.WindowProcess) { $arguments += @('-window-process', $config.WindowProcess) }
& "$PSScriptRoot\sc-webserver.exe" @arguments *> "$PSScriptRoot\sc-webserver.log"
exit $LASTEXITCODE
