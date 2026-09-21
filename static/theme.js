// Theme management for Nutz Motorsport Calendar
// Default is dark mode. Supports light/dark switching and localStorage persistence.
(function () {
  function getPreferredTheme() {
    return localStorage.getItem('theme') || 'dark';
  }

  function applyTheme(theme) {
    if (theme === 'light') {
      document.documentElement.classList.remove('dark');
    } else {
      document.documentElement.classList.add('dark');
    }
  }

  window.toggleTheme = function () {
    const isDark = document.documentElement.classList.toggle('dark');
    const theme = isDark ? 'dark' : 'light';
    try {
      localStorage.setItem('theme', theme);
    } catch (e) {
      console.warn('Unable to persist theme to localStorage:', e);
    }
  };

  // Run immediately
  applyTheme(getPreferredTheme());
})();
