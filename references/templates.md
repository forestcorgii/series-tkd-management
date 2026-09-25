# Go HTML Templates

### Context: Layout Block Isolation Across Multi-Page Apps

* **Problem:** In Go's `html/template`, when multiple template files define blocks with the same name (such as `{{define "content"}}` or `{{define "title"}}`), parsing them all into a single `*template.Template` instance causes each subsequent file to overwrite prior definitions in the shared namespace. The last file parsed alphabetically (e.g., `students.html`) overwrote the `"content"` and `"title"` blocks across all routes, causing the student directory and "+ Register New Student" modal to display on every tab, which then failed with template evaluation errors on incompatible data models (e.g. `DashboardViewData`).
* **Enforced Solution:**
  1. **Base Clone Pattern:** Parse `layout.html` and `partials/*.html` into a shared `baseTmpl`.
  2. **Page Isolation:** For each page in `pages/*.html`, clone `baseTmpl` using `baseTmpl.Clone()` and parse only the specific page file into that clone. Store each page template in `map[string]*template.Template`.
  3. **Buffered Execution:** Render templates into a `bytes.Buffer` before writing to `http.ResponseWriter`. This guarantees that if a template execution error occurs, an HTTP 500 header can be sent cleanly rather than a corrupt partial 200 OK stream.
