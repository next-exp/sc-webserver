# JPEG and PNG desktop comparison

Measured on 2026-10-05 with the deployed native Go executable, Windows 10 build
19045, whole-desktop GDI capture, 1920 × 1200 pixels and a two-second interval.
These are measurements of the desktop visible during this test, not a guarantee
for other panels, backgrounds, chart densities, GPUs or machines.

## Same-pixel encoding comparison

After three warm-up captures, 30 raw desktop frames were each encoded with both
JPEG quality 85 and PNG default compression. Encoder order alternated. PNG
round-trip decoding matched every source pixel in the checked frame. The
benchmark kept only the latest temporary image per format and removed them on
completion. No screenshots or host configuration are included in this report.

| Mean per frame | JPEG 85 | PNG default |
| --- | ---: | ---: |
| Image bytes | 290,683 | 597,641 |
| Encoding wall time | 45.345 ms | 210.300 ms |
| Encoding CPU time | 45.833 ms | 204.167 ms |
| Encoding heap allocation traffic | 1,049,184 bytes | 2,154,810 bytes |
| Buffered file-write wall time | 0.396 ms | 0.553 ms |
| Capture + encode + write wall time | 73.966 ms | 239.078 ms |
| Encoding wall time, sampled p95 | 46.160 ms | 211.305 ms |

Both encoders shared the same source captures: mean capture wall time 28.225 ms,
CPU time 17.708 ms and heap allocation traffic 9,217,161 bytes. Image sizes ranged
from 290,677 to 290,686 bytes for JPEG and 597,586 to 597,665 bytes for PNG.

For this sample, PNG was about 2.06 times the image size, 4.64 times the encoding
wall time and 4.45 times the encoding CPU. At one image per two seconds, payload
per continuously viewing browser averages about 145 kB/s for JPEG or 299 kB/s for
PNG, excluding HTTP/TLS overhead. Both formats fit comfortably within this
capture interval in the observed sample.

CPU figures are this benchmark process's kernel + user time, including any GC,
measured with GetProcessTimes. Windows CPU accounting is quantized; averaging
multiple frames is more useful than individual readings. Buffered writes had
zero recorded CPU increments at that granularity, which does not establish
zero CPU cost. Writes were not forced to durable storage. Heap allocation
traffic is not retained heap, peak resident memory or application-side memory.

## Steady server-process comparison

Two independent instances of the same new executable captured the same desktop
at two-second intervals into separate temporary folders, one JPEG and one PNG.
After warm-up, a final 31.085-second sample measured both processes without
browser traffic to either instance. Each advanced by 16 captures. The ordinary
server remained running on this host; the figures below are for the two test
processes only, not total system or LabVIEW utilization.

| Process measurement | JPEG | PNG |
| --- | ---: | ---: |
| CPU seconds in sample | 0.984 | 3.453 |
| CPU utilization, percentage of one core | 3.17% | 11.11% |
| Mean resident working set | 31.96 MiB | 31.43 MiB |
| Maximum sampled working set | 31.96 MiB | 39.72 MiB |
| Mean private committed memory | 36.48 MiB | 38.15 MiB |
| Maximum sampled private memory | 36.48 MiB | 46.67 MiB |

Memory changes with GC and Windows working-set management. This short sample
shows similar average resident memory, with higher PNG transient memory; it is
not a leak/soak test. CPU percentages use one core as 100%, not the entire
multicore host. The sample measures screenshot-server resources and does not
measure control-loop latency or prove zero impact on LabVIEW.

## Functional checks and deployment

PNG HTTP responses carried image/png and decoded to 1920 × 1200 in Chromium.
Six refreshed images, stale warnings, simulated network failure and recovery
passed, with no page errors. JPEG endpoints returned image/jpeg. PNG decoding
and exact pixels, matching-format endpoints, mismatched-extension 404 and
capture-error 503 behavior also have Go tests.

The new executable and installer/launcher were deployed, with SHA256:

```text
14f8ef0d0568779a2140f2029a1a180665154c2d9b5587fb9cfb1bcd0f36656d
```

The normal deployment remains whole-desktop JPEG 85; PNG was tested on an
isolated loopback-only instance. The authenticated HTTPS prefix passed its
browser refresh/recovery test after upgrade. Temporary test instances are
removed after validation. Select PNG with -format png or the installer's
-Format png. Keep the same private-network listener/firewall settings when
changing format. See README.md for the reproducible benchmark command.
