---
trigger: model_decision
description: When working on UI, UX, design tokens, typography rules, layout hierarchy, Tailwind CSS, and HTMX interaction patterns.
---

# Series Taekwondo — UI/UX & Design System Quick Rules

For detailed component classes and layout patterns, consult [`references/ui-design-tokens.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/ui-design-tokens.md) and [`references/htmx-ux-patterns.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/htmx-ux-patterns.md).

## 1. Brand Identity & Color Tokens
- **Series Black (`#000000`)**: Dark backgrounds, structural headers, high-contrast borders.
- **Series Crimson (`#990303` light / `#DC2626` dark)**: Primary CTAs (`.btn-primary`), active states, focus rings. Hover: `#7D0202` (light) / `#B91C1C` (dark).
- **Warm Cream (`#FFFDF4` / `#F4F1E4`)**: Light-mode canvas and card surfaces; borders use `#E5E3D8`.
- **Light Mode Contrast Rule**: Never use `text-slate-400` or `text-slate-500` for body/helper text on light backgrounds; use `text-slate-600 dark:text-slate-400` for secondary text and `text-slate-700 dark:text-slate-400` for labels/headers.

## 2. Typography & Input Rules
- **Display Font (`Good Timing` / `font-display`)**: Headings, KPI callouts, buttons (`uppercase tracking-wider`).
- **Body Font (`Montserrat` / `font-sans`)**: Operational text, tables, inputs, badges.
- **Global Uppercase Input Engine**: General text inputs and textareas automatically transform to uppercase via `layout.html`. Always add `.no-uppercase`, `data-no-uppercase="true"`, and `autocapitalize="none"` to case-sensitive fields (usernames, identifiers, passwords, tokens, keys, SMTP/VAPID settings).
- **Phone Inputs**: 11-digit Philippine mobile format (`09XXXXXXXXX`, `pattern="09[0-9]{9}"`, `maxlength="11"`, `inputmode="numeric"`).

## 3. Compact Layout & Component Standards
- **No Visible Page `<h1>` Banners**: Operational pages use `<h1 class="sr-only">Title</h1>` to preserve vertical viewport space on floor tablets.
- **KPI Strips & Compact Tables**: Use `.kpi-strip` + `.kpi-tile` for metrics and `.table-compact` inside `overflow-x-auto` containers with `min-w-[700px]` guardrails.
- **Scrollable Modals**: Use `.modal-dialog-scrollable` with `.modal-header` (`shrink-0`), `.modal-body` (`overflow-y-auto flex-1`), and sticky `.modal-footer` (`shrink-0`).
- **Tailwind CSS Build**: After editing `web/static/css/input.css` or adding new Tailwind classes in templates, compile via `npm.cmd run build:css`.
