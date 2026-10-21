# Palette's Journal - UX & Accessibility Learnings

## 2026-10-21 - Header Popover ARIA States & Escape Key
**Learning:** Interactive header popovers (profile menu and notification center) need dynamic `aria-expanded` updates alongside visual toggle so screen readers accurately communicate popover state. Furthermore, keyboard users expect `Escape` key dismiss functionality consistent with slide-out drawers.
**Action:** Always bind `aria-expanded` attributes on popover trigger buttons to their JS visibility toggles and ensure the global `Escape` key handler closes all active overlay popovers.
