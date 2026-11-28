## 2026-10-15 - Password Visibility Toggle Input Pattern
**Learning:** For absolute-positioned input action buttons (like password show/hide toggles), setting right padding `pr-11` on the full-width input container prevents typed text from rendering beneath the toggle button. In addition, dynamically updating `aria-label`, `title`, and `aria-pressed` states on the toggle button maintains full screen reader accessibility.
**Action:** Reuse the relative wrapper + `pr-11` input + absolute icon button pattern with synchronized `aria-pressed` attributes for form input actions.

## 2026-10-25 - Skip to Content & Dynamic ARIA Expanded States
**Learning:** In Go HTML template layouts with interactive dropdowns and drawer controls, static `aria-expanded="false"` attributes must be dynamically toggled via JS click listeners and click-outside listeners to keep screen reader context accurate.
**Action:** When adding dropdown popovers or slide-out navigation drawers, ensure `aria-haspopup="true"`, `aria-controls="[id]"`, and dynamic `aria-expanded` updates are wired into all open/close/click-outside JS handlers, alongside a `<a href="#main-content">` skip link.

## 2026-10-23 - Dynamic ARIA Expansion and Keyboard Dismissal for Top Navigation Dropdowns
**Learning:** Header disclosure buttons (like profile menu and notification bell) in Go/HTMX apps must dynamically toggle `aria-expanded` ("true"/"false") and participate in a global Escape key event handler to ensure screen reader state accuracy and keyboard navigation usability.
**Action:** Always sync `aria-expanded` attributes on disclosure buttons when toggling popovers/dropdowns and include dropdown close logic in global keydown Escape listeners.
