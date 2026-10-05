/* Vue 3.5.22 is embedded locally; no CDN is needed at runtime. */
Vue.createApp({
 data: () => ({status: {}, imageURL: '', warning: 'Connecting…', timer: null}),
 computed: { captured() { return this.status.capturedAt && this.status.sequence ? new Date(this.status.capturedAt).toLocaleString() : '—'; } },
 methods: {
  async refresh() {
   const started = performance.now();
   try {
    const response = await fetch('/api/status', {cache: 'no-store', signal: AbortSignal.timeout(5000)});
    if (!response.ok) throw new Error('Status unavailable');
    const status = await response.json();
    this.status = status;
    if (status.error || !status.sequence || Date.now() - Date.parse(status.capturedAt) > 6000) throw new Error(status.error || 'Capture unavailable or stale');
    const frame = await fetch('/screenshot.jpg?t=' + status.sequence, {cache: 'no-store', signal: AbortSignal.timeout(5000)});
    if (!frame.ok) throw new Error('Capture unavailable');
    const blob = await frame.blob();
    // Decode before replacing the visible frame to avoid flicker or partial images.
    const next = URL.createObjectURL(blob);
    const probe = new Image(); probe.src = next;
    try { await probe.decode(); } catch (error) { URL.revokeObjectURL(next); throw error; }
    const old = this.imageURL; this.imageURL = next;
    this.$nextTick(() => { if (old) URL.revokeObjectURL(old); });
    this.warning = '';
   } catch (error) { this.warning = error.message || 'Connection lost'; }
   finally { this.timer = setTimeout(() => this.refresh(), Math.max(0, 2000 - (performance.now() - started))); }
  }
 },
 mounted() { this.refresh(); },
 beforeUnmount() { clearTimeout(this.timer); if (this.imageURL) URL.revokeObjectURL(this.imageURL); }
}).mount('#app');
