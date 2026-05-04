// Theme-flash guard. Applies the persisted theme class before React
// hydrates so there is no FOUC on first paint. Runs synchronously
// from <head> via <script src="/theme-flash.js">. Lives in public/
// (not src/) so it is served as a static asset and the strict CSP
// (script-src 'self') accepts it without an inline-script exception.
// Keep in sync with the Zustand persist key `docsiq-ui` and the
// Providers.tsx effect that toggles `.dark`.
(function () {
  try {
    var raw = localStorage.getItem("docsiq-ui");
    var theme = "system";
    if (raw) {
      var parsed = JSON.parse(raw);
      if (parsed && parsed.state && typeof parsed.state.theme === "string") {
        theme = parsed.state.theme;
      }
    }
    var effective = theme;
    if (theme === "system") {
      effective =
        window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches
          ? "dark"
          : "light";
    }
    var root = document.documentElement;
    root.dataset.theme = effective;
    if (effective === "dark") root.classList.add("dark");
  } catch (e) {
    // localStorage unavailable (privacy mode) — let React decide
    // after hydration; brief FOUC but no crash. Do not log.
  }
})();
