## 2025-05-18 - Accessibility of Icon-Only Close Buttons
**Learning:** Icon-only close buttons utilizing symbols like `&times;` (×) or SVG icons without visible text labels are unannounced or poorly announced by screen readers if they lack an `aria-label` attribute.
**Action:** Always include an explicit `aria-label` attribute (e.g. `aria-label="Close modal"` or `aria-label="Dismiss notice"`) on any close button or icon-only interactive element.
