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

Physical desktop lock/unlock, minimized/covered panel behavior, RDP disconnect,
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
- Desktop locks, RDP disconnection, application restart, reboot/login, and extended
  soak testing remain untested. No lock-screen capture guarantee is made.

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
