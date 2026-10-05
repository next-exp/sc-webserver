param(
 [Parameter(Mandatory=$true)][string]$LocalAddress,
 [Parameter(Mandatory=$true)][string]$AllowedRemoteAddress,
 [string]$DesktopUser = $env:USERNAME,
 [ValidateRange(1,65535)][int]$Port = 8085,
 [ValidateSet('desktop','wgc')][string]$Capture = 'desktop',
 [string]$WindowTitle = '',
 [string]$WindowProcess = '',
 [ValidateSet('jpeg','png')][string]$Format = 'jpeg'
)
$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$taskName = 'NEXT SC Webserver'
$existing = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($existing -and $existing.State -eq 'Running') { throw 'Stop the existing task before upgrading.' }
$parsedIP = [System.Net.IPAddress]::Parse($LocalAddress)
if ($parsedIP.AddressFamily -ne [System.Net.Sockets.AddressFamily]::InterNetwork) { throw 'Use an IPv4 interface address.' }
if ($Capture -eq 'wgc' -and -not ($WindowTitle -or $WindowProcess)) { throw 'WGC requires WindowTitle or WindowProcess.' }
@{Listen = "${LocalAddress}:$Port"; Capture = $Capture; Format = $Format; WindowTitle = $WindowTitle; WindowProcess = $WindowProcess} | ConvertTo-Json | Set-Content -Encoding UTF8 "$root\deployment.json"
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$root\run.ps1`"" -WorkingDirectory $root
$account = "$env:COMPUTERNAME\$DesktopUser"
$principal = New-ScheduledTaskPrincipal -UserId $account -LogonType Interactive -RunLevel Limited
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $account
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
Register-ScheduledTask -TaskName $taskName -Action $action -Principal $principal -Trigger $trigger -Settings $settings -Description 'Desktop screenshot HTTP server; every two seconds in an interactive session.' -Force | Out-Null
$rule = Get-NetFirewallRule -Name 'NEXT-SC-Webserver-ZT' -ErrorAction SilentlyContinue
if ($rule) {
 Set-NetFirewallRule -Name 'NEXT-SC-Webserver-ZT' -Direction Inbound -Action Allow -Enabled True -Protocol TCP -LocalPort $Port -LocalAddress $LocalAddress -RemoteAddress $AllowedRemoteAddress -Program "$root\sc-webserver.exe" -Profile Any | Out-Null
} else {
 New-NetFirewallRule -Name 'NEXT-SC-Webserver-ZT' -DisplayName 'NEXT SC Webserver (private network)' -Direction Inbound -Action Allow -Protocol TCP -LocalPort $Port -LocalAddress $LocalAddress -RemoteAddress $AllowedRemoteAddress -Program "$root\sc-webserver.exe" -Profile Any | Out-Null
}
Start-ScheduledTask -TaskName $taskName
Get-ScheduledTask -TaskName $taskName | Select-Object TaskName,State
