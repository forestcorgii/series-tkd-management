## 2026-10-15 - Password Visibility Toggle Input Pattern
**Learning:** For absolute-positioned input action buttons (like password show/hide toggles), setting right padding `pr-11` on the full-width input container prevents typed text from rendering beneath the toggle button. In addition, dynamically updating `aria-label`, `title`, and `aria-pressed` states on the toggle button maintains full screen reader accessibility.
**Action:** Reuse the relative wrapper + `pr-11` input + absolute icon button pattern with synchronized `aria-pressed` attributes for form input actions.
