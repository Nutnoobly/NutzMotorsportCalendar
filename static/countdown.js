// Live Telemetry Countdown Engine for NutzMotorsportCalendar
(function () {
  function initCountdown() {
    const hud = document.getElementById('telemetry-hud');
    if (!hud) return;

    const targetISO = hud.dataset.targetTime;
    if (!targetISO) return;

    const targetDate = new Date(targetISO);
    if (isNaN(targetDate.getTime())) return;

    const daysEl = document.getElementById('hud-days');
    const hoursEl = document.getElementById('hud-hours');
    const minutesEl = document.getElementById('hud-minutes');
    const secondsEl = document.getElementById('hud-seconds');
    const statusEl = document.getElementById('hud-status');

    function update() {
      const now = new Date();
      const diffMs = targetDate.getTime() - now.getTime();

      if (diffMs <= 0) {
        if (daysEl) daysEl.textContent = '00';
        if (hoursEl) hoursEl.textContent = '00';
        if (minutesEl) minutesEl.textContent = '00';
        if (secondsEl) secondsEl.textContent = '00';
        if (statusEl) {
          statusEl.className = 'inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-mono font-bold bg-red-950/80 text-red-400 border border-red-800/80';
          statusEl.innerHTML = '<span class="w-2 h-2 rounded-full bg-red-500 animate-live-led"></span> SESSION LIVE';
        }
        return;
      }

      const totalSeconds = Math.floor(diffMs / 1000);
      const days = Math.floor(totalSeconds / 86400);
      const hours = Math.floor((totalSeconds % 86400) / 3600);
      const minutes = Math.floor((totalSeconds % 3600) / 60);
      const seconds = totalSeconds % 60;

      if (daysEl) daysEl.textContent = String(days).padStart(2, '0');
      if (hoursEl) hoursEl.textContent = String(hours).padStart(2, '0');
      if (minutesEl) minutesEl.textContent = String(minutes).padStart(2, '0');
      if (secondsEl) secondsEl.textContent = String(seconds).padStart(2, '0');
    }

    update();
    if (window._countdownInterval) clearInterval(window._countdownInterval);
    window._countdownInterval = setInterval(update, 1000);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initCountdown);
  } else {
    initCountdown();
  }

  document.body.addEventListener('htmx:afterSwap', function (evt) {
    if (evt.detail.target && evt.detail.target.id === 'calendar-section') {
      // Retain or re-init countdown if refreshed
      initCountdown();
    }
  });
})();
