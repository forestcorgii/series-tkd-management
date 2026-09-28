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


