# UI Design Tokens & Contrast Patterns

### Context: Light Mode Text Contrast & Avatar Icon Layout

**Problem:**
1. Using `text-slate-400` or `text-slate-500` for secondary text or labels on white/cream backgrounds in light mode produces low-contrast, washed-out text. In inverted templates (`text-slate-400 dark:text-slate-500`), light mode becomes even fainter than dark mode.
2. Full coach and student names placed inside fixed avatar icons (`h-12 w-12` or `h-16 w-16`) with large font classes (`text-lg`, `text-2xl`) overflow and overlap container borders and adjacent text elements.
3. Fixed-height dashboard lists (e.g. Active Floor Sessions) can expand without bounds and deform the page layout when items accumulate.

**Enforced Solution:**
| UI Element | Light Mode Enforced Class | Dark Mode Enforced Class | Notes |
| :--- | :--- | :--- | :--- |
| Secondary / Helper Text | `text-slate-600` | `dark:text-slate-400` | Never use `text-slate-400` on light mode |
| Table Headers & Labels | `text-slate-700` | `dark:text-slate-400` | Crisp legible slate contrast |
| Empty / Placeholder States | `text-slate-600` / `placeholder-slate-500` | `dark:text-slate-400` / `dark:placeholder-slate-400` | Maintains WCAG readability |
| Small Avatar Icon (`h-12 w-12`) | `text-[10px] leading-tight text-center px-1 overflow-hidden break-words line-clamp-2` | Same | Prevents text overlap on adjacent elements |
| Large Avatar Icon (`h-16 w-16`) | `text-[11px] leading-tight text-center p-1 overflow-hidden break-words line-clamp-2` | Same | Prevents text overlap on adjacent elements |
| Active Floor Sessions Panel | `max-h-[30rem] overflow-y-auto pr-1` | Same | Ensures scrollability when list grows |

### Context: Primary Action Buttons & Brand Crimson Palette

**Problem:**
1. Using emerald/green styling (`bg-emerald-500 hover:bg-emerald-400 text-slate-950`) for primary action buttons conflicts with the Series Taekwondo brand identity where Series Crimson (`#990303`) is the signature brand color.
2. Inconsistent button styling across views causes visual disorientation between auth/portals (red) and legacy CRUD/floor views (green).

**Enforced Solution:**
| UI Element | Light Mode Enforced Class | Dark Mode Enforced Class | Notes |
| :--- | :--- | :--- | :--- |
| Primary CTA Button | `bg-[#990303] hover:bg-[#7D0202] text-white font-display uppercase tracking-wider` | `dark:bg-[#DC2626] dark:hover:bg-[#B91C1C]` | Deep Crimson (#990303) in light; Vibrant Scarlet (#DC2626) in dark for high contrast against black |
| Primary Action Link/Card | `bg-[#990303] hover:bg-[#7D0202] text-white font-display uppercase tracking-wider` | `dark:bg-[#DC2626] dark:hover:bg-[#B91C1C]` | Same contrast pairing for active floor cards and links |
| Secondary / Cancel Button | `bg-slate-200 hover:bg-slate-300 text-slate-700` | `dark:bg-slate-800 dark:hover:bg-slate-700 dark:text-slate-300` | Neutral slate tone preserves visual hierarchy |

### Context: Centralized Component Utilities Architecture

**Problem:**
Repeated inline utility classes across 11 pages and 3 partials led to discrepancies in button padding, corner radiuses (`rounded-xl` vs `rounded-lg`), form focus ring colors (emerald vs brand crimson), and modal structures.

**Enforced Solution:**
Components are centralized in `web/templates/layout.html` within `<style type="text/tailwindcss">` using `@layer components`:

| Component Class | Description | Standard Usage |
| :--- | :--- | :--- |
| `.btn` | Base flex, uppercase font-display, active scale, transition | Never use alone; combine with variant |
| `.btn-primary` | Brand crimson (`#990303` light / `#DC2626` dark) with shadow | Primary CTAs (Save, Register, Schedule, Sign In) |
| `.btn-secondary` | Neutral slate (`bg-slate-200` / `dark:bg-slate-800`) | Cancel, dismiss, navigate back |
| `.btn-outline` | Crimson border with hover fill | Grade, secondary quick actions |
| `.btn-ghost` | Slate hover with transparent background | Icon buttons, theme toggle |
| `.btn-danger` | Rose red (`bg-rose-600` / `dark:bg-rose-700`) | Safety hold flags, delete actions |
| `.btn-success` | Emerald green (`bg-emerald-600`) | Mat admission, belt promotion, safety clearance |
| `.btn-warning` | Amber tone (`bg-amber-500` / `dark:bg-amber-500`) | Manual overrides, warning bypasses |
| `.btn-xs`, `.btn-sm`, `.btn-md`, `.btn-lg` | Explicit sizing scales from 10px to 14px | Table inline actions to hero CTAs |
| `.form-input` | Consistent border, bg, text, and crimson focus ring | Text, date, email, password, search inputs |
| `.form-select` | Consistent select dropdown with crimson focus ring | Form selects |
| `.form-textarea` | Consistent textarea with crimson focus ring | Form remarks, notes |
| `.form-label` | Uppercase, tracking-wider, legible slate | Form field labels |
| `.badge-belt` | Slate pill with border | Kup and Dan belt ranks |
| `.badge-ready` | Emerald chip with shadow | 🟢 READY status |
| `.badge-pretest` | Amber chip with shadow | 🟡 PRE-TEST status |
| `.badge-developing`| Rose chip with shadow | 🔴 DEVELOPING status |
| `.modal-backdrop` | Fixed inset-0 with backdrop blur and centered flex | Modal overlays |
| `.modal-dialog` | Glass panel max-w-lg or max-w-md with 2xl shadow | Modal container |
| `.modal-header` | Flex space-between with border-b | Modal title bar |
| `.modal-close` | Subtle close button with hover state | `&times;` dismissal |
| `.modal-footer` | Flex justify-end with top border | Action buttons bar |



