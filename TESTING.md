# Windows validation

## Initial desktop-backend pilot

Initial desktop-backend validation on 2026-10-05, before WGC was added, at
`C:\next\sc-webserver\sc-webserver.exe`, task `NEXT SC Webserver`.

- `go test -race ./...`: passed, including concurrent readers/frame replacement,
  fresh/stale/failed image responses, health checks, and embedded assets.
- `go vet ./...` and Windows cross-target vet: passed.
- Windows cross-build: passed; deployed SHA256 matches the local executable:
  `9f123de1b8b866bd874be2ecf1cdccf5bfdb1bbbac9ade9cf64df049a337d579`.
- Real capture: decoded and visually inspected a 1920 x 1200 Windows desktop JPEG.
  Captured the visible desktop, without requesting application-side rendering.
- Captures sampled over nine seconds: five distinct sequences, inter-capture
  intervals 2.000394, 1.999780, 2.000111, 2.000101 seconds; capture/encode/file-write
  duration 76–78 ms in that sample.
- Sixteen concurrent HTTP image requests: successful complete JPEG responses,
  with `Cache-Control: no-store`.
- Chromium repository test: passed on the final deployment; six images received,
  natural dimensions 1920 x 1200, no page errors; verified refresh, simulated
  stale status, simulated network loss, and automatic recovery.
- Windows process runs in interactive session 1. Scheduled task is Running;
  HTTP `/healthz` returns `ok`. Firewall allows TCP 8085 only at the ZeroTier IP
  from the ZeroTier subnet.
- Temporary folder retains only `latest.jpg`; its write time advances across
  successive captures. No screenshot history is retained.
- Brief resource sample: 0.21875 process CPU seconds over 6.0346 wall seconds,
  about 40.3 MiB working set, 233 handles. This is a short pilot measurement.

At this initial stage, desktop lock/unlock, minimized/covered panel behavior, RDP disconnect,
reboot/login, and long-duration operation were not exercised. The browser outage
checks simulate responses only in the test browser. The screenshot program uses
the currently visible desktop; independent LabVIEW telemetry remains necessary
for control-system health.


## Windows Graphics Capture backend

Validated on Windows 10 build 19045, amd64, using a running legacy LabVIEW viewer
panel. The backend was implemented with native WinRT/COM and D3D11 calls, with no
external capture executable or runtime.

- Visually inspected a 1466 x 934 window screenshot, including both plot areas.
- Repeated captures continued after keeping the worker's WinRT apartment alive
  for its full lifetime. Short-lived capture sessions close between ticks.
- Covered-window test: a temporary fullscreen magenta overlay appeared in the
  desktop capture (97.34% magenta in the sampled image), while the WGC screenshot
  continued to contain the viewer panel (0% sampled magenta). Overlay removed.
- Minimized-window test: API error `target window is minimized`, health HTTP 503;
  restoring the panel resumed successful capture automatically.
- A partly offscreen legacy window initially left the offscreen area unpainted.
  Positioning it fully onscreen produced the complete panel. WGC is not a promise
  that an application will render every hidden/offscreen region.
- Actual host/window selectors are stored only in the excluded deployment
  configuration, not in this repository. Test screenshots remain outside Git.
- RDP disconnection, application restart, reboot/login, and extended soak testing
  remain untested. See the lock/unlock comparison below.

- Final build SHA256 verified against the deployed executable:
  `480ff63e14ba5c05a0e180afc043955e9f12413d58fbc47daecd1785394e7d18`.
- Final WGC timing sample: six distinct captures; intervals 2.000166, 1.999887,
  2.000049, 1.999673, 1.999807 seconds; capture durations 97–101 ms. Sixteen
  concurrent screenshot requests returned valid JPEGs.
- Chromium refresh/stale/network-error/recovery check passed against the WGC
  deployment, with six decoded images and no page errors.
- Final executable's desktop backend separately tested in the same interactive
  session: successful 1920 x 1200 capture, 76 ms in the sampled frame.
- Temporary behavior-test tasks and screenshots removed; normal deployment uses
  one overwritten temporary `latest.jpg`. Real deployment configuration remains
  outside Git. Gitleaks scan passed.

## Lock/unlock comparison

Tested both backends concurrently on 2026-10-05 in the same Windows 10 build
19045 console session, with the legacy viewer running. Locked the session using
`LockWorkStation`, inspected the lock and password-entry screens through VNC,
then unlocked the existing session using credentials obtained in memory.

- Desktop/GDI: the lock-screen wallpaper could initially be captured. At the
  secure password-entry desktop, capture reported `interactive desktop
  unavailable` (access denied). Sequence and successful capture time stopped
  advancing; both `/healthz` and `/screenshot.jpg` returned HTTP 503.
- WGC: continued returning valid 1466 x 934 viewer images, HTTP 200, and advancing
  capture sequences at two-second intervals, including at the password screen.
  Three sampled JPEGs across four seconds were byte-identical. The viewer's
  displayed plots were static, so this does not establish whether application
  data would update while locked. Successful capture timestamps describe capture
  attempts, not the age of the data drawn by the application.
- Unlock: desktop/GDI resumed successful captures automatically without a process
  restart. WGC remained healthy throughout and after unlock. The viewer process
  and normal WGC server retained their original process IDs.
- RDP and VNC listeners were verified; recovery used VNC to preserve the existing
  console session. RDP login/disconnect behavior was not tested.
- Temporary tasks, secondary capture process, and remote test images were removed.
  The session was left unlocked, with the viewer and normal server running.

This result applies to this deployment and application state. It is not a
guarantee of live telemetry during a lock or remote-session transition.

## Desktop deployment behind a path-prefix proxy

Validated on 2026-10-05 after changing embedded asset/API URLs to relative paths.

- Windows build and native/Windows-target vet passed; race tests passed.
- Deployed executable SHA256 verified:
  `89f5c273b454e3cd0247625c1fc00c6a5f3511721789d9c82a3c4837e622f97c`.
- Desktop backend reports 1920 x 1200 at a two-second interval; sampled capture
  durations 71–74 ms. Interactive task is enabled and running.
- Reverse proxy strips the path prefix and redirects the bare prefix to a
  trailing slash. Authenticated assets, status, health and JPEG return HTTP 200;
  unauthenticated page, status and JPEG return HTTP 401. HTTPS trust validated.
- Chromium checks passed against both the private origin and authenticated HTTPS
  prefix, each with six image responses, 1920 x 1200 decoded dimensions, no page
  errors, and simulated stale/network failure/recovery.
- Ingress was validated before a graceful reload. Existing authenticated and
  Guacamole route checks returned their expected responses.
- Credentials are supplied to tests through environment variables and remain
  outside this repository. Deployment addresses and proxy configuration remain
  host-local.
