---
name: stms-ui-htmx-workflow
description: >-
  Step-by-step workflow for creating or modifying Go HTML templates, HTMX partials,
  dynamic modals, searchable dropdowns, and Tailwind CSS builds in STMS.
  Use this skill when working on frontend templates, HTMX endpoints, or styling.
---

# STMS UI, Templates & HTMX Workflow

Before editing templates or CSS, consult:
- [`references/ui-design-tokens.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/ui-design-tokens.md) for Series Crimson (`#990303`) tokens, `.btn-*`, `.kpi-strip`, and `.table-compact` classes.
- [`references/htmx-ux-patterns.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/htmx-ux-patterns.md) for HTMX boost, modal form, and partial swapping invariants.
- [`references/templates.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/templates.md) for Go `html/template` block isolation and FuncMap helpers.

## 1. Go HTML Template Rules
1. **Page Templates (`web/templates/pages/*.html`)**:
   - Cloned individually onto `baseTmpl` (`layout.html` + `partials/*.html`).
   - Define `{{define "title"}}...{{end}}` and `{{define "content"}}...{{end}}`.
   - Use `<h1 class="sr-only">Page Title</h1>` instead of visible bulky page header banners.
   - Access the current user in shared layouts/partials via `{{$user := currentUser .}}`.
   - Format pointer float prices/rates using `{{formatCurrency .CustomPrice}}` (never `{{printf "%.2f" .CustomPrice}}`).
2. **Partial Templates (`web/templates/partials/*.html`)**:
   - **MUST** wrap their entire content in `{{define "<filename>.html"}}...{{end}}` so `a.RenderPartial(w, "<filename>.html", data)` resolves the template block.

## 2. HTMX Interaction Checklist
1. **Boosted Page Navigation vs. Partial Filter Requests**:
   - `<body hx-boost="true">` is enabled globally in `layout.html`.
   - In handlers serving both full pages and HTMX filter partials (`HandleStudents`, `HandleSessions`), check:
     ```go
     isHTMXPartial := r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true"
     ```
2. **Dynamic JS Modal Forms**:
   - Any `<form>` whose `action` attribute is populated dynamically via JavaScript on modal open MUST declare `hx-boost="false" hx-disable="true"` and include a hidden entity ID field (`<input type="hidden" name="session_id" ...>`) to prevent HTMX from submitting to the parent page's creation URL.
3. **Case-Sensitive Inputs**:
   - `layout.html` automatically uppercases text inputs and textareas. Add `class="... no-uppercase" data-no-uppercase="true" autocapitalize="none"` to usernames, emails, passwords, tokens, keys, and URLs.

## 3. Rebuilding Tailwind CSS
Whenever you add new Tailwind utility classes in `.html` or `.go` files, or edit [`web/static/css/input.css`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/input.css), recompile the production stylesheet:

```powershell
npm.cmd run build:css
```

Verify that [`web/static/css/tailwind.css`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/tailwind.css) is updated cleanly.

