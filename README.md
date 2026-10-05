# SC Webserver

A Windows desktop screenshot server for NEXT slow control, written in Go with an
embedded Vue 3.5.22 production page. The executable includes all web assets; no
Node.js, Go installation, CDN, or additional runtime is needed on Windows.

Captures the visible virtual desktop immediately and every **two seconds**, with
one capture loop shared by all viewers. The default GDI backend copies desktop pixels; the optional Windows Graphics
Capture backend targets one application window. Neither invokes LabVIEW's
embedded snapshot server or PrintWindow. Each capture
overwrites `latest.jpg` (or `latest.png` with `-format png`) in `%TEMP%\sc-webserver`; there is no screenshot history.
HTTP serves complete in-memory frames with `Cache-Control: no-store`.
The browser refreshes every two seconds, shows the capture time, and warns about
errors or stale frames. `/healthz` and `/screenshot.jpg` return 503 when capture
fails or the last frame is more than six seconds old.

## Continuous integration

GitHub Actions runs on pushes, pull requests, and manual dispatch. It checks Go
formatting, runs vet and tests on Linux and Windows, and runs the race detector
on Linux. After both platforms pass, it builds the Windows amd64 executable
with the Vue assets embedded and uploads `sc-webserver-windows-amd64` for 30 days.
Download that artifact from the successful workflow run: it includes the
executable, installation/launcher scripts, documentation, and `SHA256SUMS.txt`.
CI uses GitHub-hosted runners and requires no deployment credentials. Live
screen capture and browser checks against a Windows desktop remain deployment
validation steps.

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

Use `-format png` for lossless PNG with Go's default compression; JPEG quality
85 remains the default and `-quality` applies only to JPEG. The browser loads the
format-independent `/screenshot` endpoint. `/screenshot.jpg` and
`/screenshot.png` serve only their matching configured format (otherwise 404).
The status includes `format`, `imageBytes`, and `encodeMs`; `captureMs` remains
the entire capture/encode/file-write duration. Switching formats removes the
other format's latest image on startup, so there is no image history.

Use `-listen` with your private interface address for remote viewing. `-temp-dir`
selects the capture directory; `-quality` sets JPEG quality (default 85). Default
binding is loopback. Capture and refresh periods are two seconds. Ctrl+C shuts
down and removes the image. An abrupt exit leaves at most the latest image,
which the next launch overwrites. Independent instances need separate capture
directories.

There is no authentication: restrict access to the intended private network or
use an authenticated gateway before broader access. Desktop captures may expose
unrelated windows. Desktop capture does not reconstruct minimized or covered
panels. A locked or unavailable desktop reports an error; session 0 Windows
services cannot view the operator's desktop. A fresh screenshot does not prove healthy control loops.

## Windows Graphics Capture

The optional **WGC** backend targets a particular window with Windows Graphics
Capture and Direct3D 11, in the same Go executable (no cgo or helper runtime).
It requires Windows amd64, Windows 10 1903 or newer, and a D3D11-capable graphics
adapter. List visible titled windows from the application's interactive session:

```powershell
.\sc-webserver.exe -list-windows
.\sc-webserver.exe -capture wgc -window-title 'Example panel' -window-process 'example.exe'
```

Title matching is case-insensitive substring matching; process matching uses the
executable basename. Both selectors can be combined. Multiple matches produce an
error rather than capturing an arbitrary window. `-window-hwnd` accepts a numeric
or hexadecimal handle when explicit selection is needed. Title/process selectors
rediscover the window on every capture, allowing recovery after a window restart;
a numeric handle must be updated after replacement. Desktop crop flags cannot be
combined with WGC.

The worker keeps a WinRT MTA apartment on one Windows thread for its lifetime.
Each two-second tick creates a short-lived capture session and a two-frame pool,
waits up to one second for a fresh frame, reads its pixels, then closes/releases
all resources. There is no continuous capture between ticks. GPU readback is also
bounded; initialization calls are synchronous. Capture errors produce the same
stale warning and HTTP 503 behavior as desktop errors. The API status includes
`backend` and the configured `windowTitle`.

WGC can capture a covered window, but rendering remains application-dependent:
some older applications may leave unpainted regions. Minimized windows explicitly
report an error; restoring the window resumes capture. Windows may show a capture
border. Locked desktops, RDP disconnection, protected windows, and remote-session
transitions are not guaranteed to work and require deployment-specific tests.
This backend is not a workaround for Windows session locking.

The page uses relative asset/API URLs and can run behind a proxy that strips a
path prefix. Redirect the bare prefix to a trailing slash, and proxy all paths
under that prefix to this server.

## Scheduled deployment

Copy `dist/sc-webserver.exe`, `scripts/run.ps1`, and `scripts/install.ps1` into
the same Windows directory. Run the installer from an administrative PowerShell
session, supplying your private interface address, allowed client address or
subnet, and interactive desktop username:

```powershell
.\install.ps1 -LocalAddress 'YOUR_PRIVATE_IP' -AllowedRemoteAddress 'YOUR_CLIENT_SUBNET' -DesktopUser 'YOUR_DESKTOP_USER'
```

For WGC, also pass `-Capture wgc -WindowTitle 'Example panel'` and/or
`-WindowProcess 'example.exe'`. These selectors stay in the excluded host-local
configuration. Existing configurations without a capture setting keep desktop
capture.

`-Format png` selects PNG in the installer; existing deployments without a format
setting remain JPEG. `-Port` defaults to 8085. The installer creates `NEXT SC Webserver` with an
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

Measured JPEG/PNG size, CPU and memory results are in [BENCHMARKS.md](BENCHMARKS.md).

## Windows image-format benchmark

Run in the interactive desktop session:

```powershell
.\sc-webserver.exe -capture desktop -benchmark-frames 30 -benchmark-output benchmark.json
```

The benchmark captures the raw desktop once per two-second tick, encodes those
same pixels as JPEG (the chosen `-quality`, default 85) and PNG, and alternates
encoder order. It warms up three frames, verifies PNG pixels round-trip exactly,
and reports image size, wall time, this process's CPU time and Go heap allocation
traffic for capture, encoding and buffered file writes. CPU accounting includes
GC and has Windows timer granularity; heap allocation is not peak resident RAM.
It does not measure LabVIEW CPU or control timing. Temporary benchmark images
are overwritten and removed on completion; the JSON contains measurements only.
Keep JSON reports outside Git unless reviewed and sanitized.

## Browser check

`tests/browser.spec.js` checks image decoding, repeated refreshes, stale warnings,
simulated request failures, and recovery. Install Playwright in your development
environment, then run against a live Windows instance:

```sh
SC_WEBSERVER_URL=http://YOUR_PRIVATE_IP:8085/ npx playwright test
```

`SC_WEBSERVER_URL` defaults to localhost. Set `SC_SCREENSHOT_PATH` outside the
repository to save an optional browser screenshot. For a proxy with HTTP Basic
Auth, supply `SC_HTTP_USERNAME` and `SC_HTTP_PASSWORD` through your secret manager
or environment; keep their values outside the repository. Request-failure simulations
apply only in the test browser and do not change the Windows session.

Vue is vendored with its MIT license from
https://unpkg.com/vue@3.5.22/dist/vue.global.prod.js.
Capture uses Windows GDI primitives documented at
https://learn.microsoft.com/en-us/windows/win32/gdi/capturing-an-image.
