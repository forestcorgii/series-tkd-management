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
