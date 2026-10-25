## 2026-10-25 - Skip to Content & Dynamic ARIA Expanded States
**Learning:** In Go HTML template layouts with interactive dropdowns and drawer controls, static `aria-expanded="false"` attributes must be dynamically toggled via JS click listeners and click-outside listeners to keep screen reader context accurate.
**Action:** When adding dropdown popovers or slide-out navigation drawers, ensure `aria-haspopup="true"`, `aria-controls="[id]"`, and dynamic `aria-expanded` updates are wired into all open/close/click-outside JS handlers, alongside a `<a href="#main-content">` skip link.
