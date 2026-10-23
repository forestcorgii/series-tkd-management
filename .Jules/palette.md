## 2026-10-23 - Dynamic ARIA Expansion and Keyboard Dismissal for Top Navigation Dropdowns
**Learning:** Header disclosure buttons (like profile menu and notification bell) in Go/HTMX apps must dynamically toggle `aria-expanded` ("true"/"false") and participate in a global Escape key event handler to ensure screen reader state accuracy and keyboard navigation usability.
**Action:** Always sync `aria-expanded` attributes on disclosure buttons when toggling popovers/dropdowns and include dropdown close logic in global keydown Escape listeners.
