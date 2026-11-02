# Go HTML Templates

### Context: Layout Block Isolation Across Multi-Page Apps

* **Problem:** In Go's `html/template`, when multiple template files define blocks with the same name (such as `{{define "content"}}` or `{{define "title"}}`), parsing them all into a single `*template.Template` instance causes each subsequent file to overwrite prior definitions in the shared namespace. The last file parsed alphabetically (e.g., `students.html`) overwrote the `"content"` and `"title"` blocks across all routes, causing the student directory and "+ Register New Student" modal to display on every tab, which then failed with template evaluation errors on incompatible data models (e.g. `DashboardViewData`).
* **Enforced Solution:**
  1. **Base Clone Pattern:** Parse `layout.html` and `partials/*.html` into a shared `baseTmpl`.
  2. **Page Isolation:** For each page in `pages/*.html`, clone `baseTmpl` using `baseTmpl.Clone()` and parse only the specific page file into that clone. Store each page template in `map[string]*template.Template`.
  3. **Buffered Execution:** Render templates into a `bytes.Buffer` before writing to `http.ResponseWriter`. This guarantees that if a template execution error occurs, an HTTP 500 header can be sent cleanly rather than a corrupt partial 200 OK stream.

### Context: Partial Template Define Blocks & HTMX Target Swapping

* **Problem:** In Go's `html/template`, when rendering partial fragments via `RenderPartial(w, "partial_name.html", data)`, if the partial file does not explicitly declare `{{define "partial_name.html"}}...{{end}}`, executing `a.partialTemplates.ExecuteTemplate(&buf, "partial_name.html", data)` throws an empty or undefined template error.
* **Enforced Solution:**
  1. Every partial template located in `web/templates/partials/*.html` must enclose its content inside `{{define "<filename>.html"}}` and conclude with `{{end}}`.
  2. This guarantees that `RenderPartial` invoked during HTMX requests (e.g. keyup search or category dropdown filters) executes and returns the exact HTML fragment targeted for swapping.

### Context: Pointer Float Formatting in Templates (%!f Pointer Leak)

* **Problem:** When domain model fields are pointer types (e.g., `CustomPrice *float64`), calling `{{printf "%.2f" .Package.CustomPrice}}` passes the pointer address to `fmt.Sprintf`. Because Go's `%f` verb does not automatically dereference pointer types, templates emit pointer formatting artifacts such as `%!f(*float64=0x340607ab8d60)`.
* **Enforced Solution:**
  1. **`formatCurrency` FuncMap Helper:** Register a generic `formatCurrency` function in the template `funcMap` using reflection/type-switching to dereference pointers (`*float64`, `*int`, etc.) and return a formatted 2-decimal string (`%.2f`) or empty string if `nil`.
  2. **Domain Helper Method:** Add `CustomPriceVal() float64` to `models.StudentPackage` for nil-safe domain dereferencing.
  3. **Template Standard:** Replace `{{printf "%.2f" ...}}` with `{{formatCurrency ...}}` in templates (`packages.html`, `student_detail.html`) when rendering pointer rates and prices.



### Context: Universal 12-Hour Time Presentation Standard

* **Problem:** Session times (`StartTime`, `EndTime`) are stored in 24-hour machine format (`"17:00"`, `"18:30"`) to comply with standard HTML5 `<input type="time">` submission payloads. Displaying raw strings in UI templates resulted in inconsistent 24-hour presentation (`17:00 - 18:30`) across landing schedules, session calendar cards, live attendance headers, and dashboards.
* **Enforced Solution:**
  1. **Domain Format Helpers:** Added pure domain formatters [`models.FormatTime12`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go) and [`models.FormatTimeRange12`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go), with convenience methods [`TrainingSession.TimeRange()`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go), [`FormattedStartTime()`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go), and [`FormattedEndTime()`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go).
  2. **Template FuncMap Helpers:** Registered `formatTime` and `formatTimeRange` in `funcMap` inside [`internal/handlers/app_handler.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/app_handler.go) supporting `time.Time`, `*time.Time`, and time strings.
  3. **Universal 12-Hour Presentation:** Updated all user-facing template views (`landing_calendar_grid.html`, `session_cards.html`, `admin_dashboard.html`, `coach_dashboard.html`, `coach_portal.html`, `live_checkin.html`, `student_session_item.html`, and `admin_portal.html`) to display times in 12-hour format (`5:00 PM - 6:30 PM`, `8:00 AM`), while preserving 24-hour values on form input attributes (`value="17:00"`).
