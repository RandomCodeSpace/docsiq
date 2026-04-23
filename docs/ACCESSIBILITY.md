# Accessibility

## Commitment

docsiq targets **WCAG 2.1 Level AA** compliance for the embedded React SPA
served by `docsiq serve`. This document describes the current stance and
the practices enforced during development.

## Colour and contrast

- All text and interactive elements meet a contrast ratio of at least **4.5:1**
  against their background (AA normal text) and **3:1** for large text and
  UI components.
- Colour is never the sole means of conveying information (e.g. error states
  use both colour and an icon or label).
- Dark mode is the default; a light theme is available. Both palettes are
  tested for contrast compliance.

## Keyboard navigation

- All interactive elements (buttons, links, inputs, modals) are reachable and
  operable via keyboard alone.
- Focus order follows the logical reading order of the page.
- Focus indicators are always visible; the default browser outline is not
  suppressed without a higher-contrast replacement.
- The command palette (`Cmd/Ctrl+K`) is keyboard-first and fully navigable
  without a mouse.

## Motion

- Non-essential animations respect `prefers-reduced-motion`. When the user
  has opted out of motion, transitions are replaced with instant state changes.
- No animations trigger automatically on page load for more than 5 seconds
  without a pause/stop control.

## Screen readers

- Semantic HTML elements are used throughout (`<nav>`, `<main>`, `<button>`,
  `<dialog>`, etc.) rather than `<div>` with ARIA role overrides.
- ARIA attributes are added only where native semantics are insufficient.
- Dynamic content updates (search results, loading states) use `aria-live`
  regions with appropriate politeness levels.
- All images and icons that convey meaning carry descriptive `alt` text;
  decorative images use `alt=""`.

## Forms and inputs

- Every form input has an associated `<label>` (visible or visually hidden).
- Error messages are associated with their input via `aria-describedby`.
- Required fields are marked with `aria-required` in addition to visual cues.

## Known limitations

The SPA is pre-1.0. A full third-party accessibility audit has not yet been
performed. Issues can be reported via
[GitHub Issues](https://github.com/RandomCodeSpace/docsiq/issues) with the
`accessibility` label.

## Testing

Accessibility is checked during development using:

- **axe-core** browser extension for automated rule violations
- Manual keyboard-only navigation testing
- Screen reader spot-checks (VoiceOver on macOS, NVDA on Windows)
