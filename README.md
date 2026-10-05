# SC Webserver

A Windows desktop screenshot server for NEXT slow control, written in Go with an
embedded Vue 3.5.22 production page. The executable includes all web assets; no
Node.js, Go installation, CDN, or additional runtime is needed on Windows.

Captures the visible virtual desktop immediately and every **two seconds**, with
one capture loop shared by all viewers. Windows GDI copies desktop pixels without
invoking LabVIEW's embedded snapshot server or PrintWindow. Each capture
overwrites `latest.jpg` in `%TEMP%\sc-webserver`; there is no screenshot history.
HTTP serves complete in-memory frames with `Cache-Control: no-store`.
The browser refreshes every two seconds, shows the capture time, and warns about
errors or stale frames. `/healthz` and `/screenshot.jpg` return 503 when capture
fails or the last frame is more than six seconds old.

## Build and run

```sh
go test -race ./...
go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/sc-webserver.exe .
GOOS=windows GOARCH=amd64 go vet ./...
```

Run inside the unlocked Windows desktop session containing the panels:

```powershell
.\sc-webserver.exe -listen 127.0.0.1:8085
# Optional fixed crop, in native desktop pixels:
.\sc-webserver.exe -x 100 -y 100 -width 1280 -height 720
```

Use `-listen` with your private interface address for remote viewing. `-temp-dir`
selects the capture directory; `-quality` sets JPEG quality (default 85). Default
binding is loopback. Capture and refresh periods are two seconds. Ctrl+C shuts
down and removes the image. An abrupt exit leaves at most the latest image,
which the next launch overwrites. Independent instances need separate capture
directories.

There is no authentication: restrict access to the intended private network or
use an authenticated gateway before broader access. Desktop captures may expose
unrelated windows. Minimized or covered panels are not reconstructed. A locked
or unavailable desktop reports an error; session 0 Windows services cannot view
the operator's desktop. A fresh screenshot does not prove healthy control loops.

## Scheduled deployment

Copy `dist/sc-webserver.exe`, `scripts/run.ps1`, and `scripts/install.ps1` into
the same Windows directory. Run the installer from an administrative PowerShell
session, supplying your private interface address, allowed client address or
subnet, and interactive desktop username:

```powershell
.\install.ps1 -LocalAddress 'YOUR_PRIVATE_IP' -AllowedRemoteAddress 'YOUR_CLIENT_SUBNET' -DesktopUser 'YOUR_DESKTOP_USER'
```

`-Port` defaults to 8085. The installer creates `NEXT SC Webserver` with an
interactive logon trigger and a firewall rule scoped to the supplied interface
and clients. The task has no execution limit and retries failures up to three
times. The launcher overwrites `sc-webserver.log` at each start; there are no
per-frame logs.

Deployment settings are saved locally in `deployment.json`, excluded from Git.
Keep actual hostnames, addresses, screenshots, credentials, and deployment
configuration out of the repository. Open `http://YOUR_PRIVATE_IP:8085/` from an
allowed client. The task starts at desktop-user login after reboot; capture
requires an unlocked session.

Inspect or stop:

```powershell
Get-ScheduledTask -TaskName 'NEXT SC Webserver'
Get-ScheduledTaskInfo -TaskName 'NEXT SC Webserver'
Get-Content .\sc-webserver.log
Stop-ScheduledTask -TaskName 'NEXT SC Webserver'
# A forced task stop may leave its executable running; stop only this installation:
$exe = (Resolve-Path .\sc-webserver.exe).Path
Get-CimInstance Win32_Process | Where-Object ExecutablePath -eq $exe | ForEach-Object { Stop-Process -Id $_.ProcessId }
```

To upgrade, stop the task and executable, replace files, then rerun the installer.
To remove it, stop the task and executable, then:

```powershell
Unregister-ScheduledTask -TaskName 'NEXT SC Webserver' -Confirm:$false
Remove-NetFirewallRule -Name 'NEXT-SC-Webserver-ZT'
```

## Browser check

`tests/browser.spec.js` checks image decoding, repeated refreshes, stale warnings,
simulated request failures, and recovery. Install Playwright in your development
environment, then run against a live Windows instance:

```sh
SC_WEBSERVER_URL=http://YOUR_PRIVATE_IP:8085/ npx playwright test
```

`SC_WEBSERVER_URL` defaults to localhost. Set `SC_SCREENSHOT_PATH` outside the
repository to save an optional browser screenshot. Request-failure simulations
apply only in the test browser and do not change the Windows session.

Vue is vendored with its MIT license from
https://unpkg.com/vue@3.5.22/dist/vue.global.prod.js.
Capture uses Windows GDI primitives documented at
https://learn.microsoft.com/en-us/windows/win32/gdi/capturing-an-image.
