# Performance Standards & Optimization Patterns

### Context: Core Performance Principles in STMS
STMS operates on floor tablets and mobile devices with varying network stability. Performance is treated as an operational requirement.

| Layer | Primary Bottleneck | Optimization Standard |
| :--- | :--- | :--- |
| **Database** | N+1 queries, unindexed filters | Batch queries, indexed lookups, explicit `SELECT` projections |
| **Go Backend** | Template re-parsing, heap allocations | Global pre-compiled templates, efficient slice sizing, pooled buffers |
| **HTMX / Client** | Excessive payload size, chatter | Targeted partial swaps, debounced inputs (`delay:250ms`), lean SVG polygons |

---

### Database Optimization Checklist
1. **Attendance Check-In Hot Path**:
   - Package deduction and attendance insertion must execute in a single atomic transaction.
   - Filter `student_packages` using `WHERE student_id = $1 AND remaining_sessions > 0 AND expiry_date >= CURRENT_DATE ORDER BY purchase_date ASC LIMIT 1 FOR UPDATE`.
2. **Search Autocomplete**:
   - Limit student quick-search results to 10 records.
   - Use trigram or prefix indexes on `students(full_name, phone)`.

---

### Go & Template Optimization Checklist
1. **Pre-Parsed Templates**:
   - Templates stored in a synchronized map or pre-compiled template tree during startup.
2. **Buffer Re-use**:
   - Render templates into a `bytes.Buffer` before writing to `http.ResponseWriter` to handle rendering errors cleanly without partial chunk writes.

---

### Verification Workflow
- Check query counts in test logs for every endpoint.
- Benchmark complex business domain logic (e.g., promotion readiness matrix, radar polygon calculations) using Go's standard benchmark tooling (`go test -bench=.`).
