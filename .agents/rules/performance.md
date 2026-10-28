---
trigger: model_decision
description: When working on performance optimization, database queries, template rendering, or HTMX network latency.
---

# Performance Prioritization & Continuous Verification

For historical optimizations and HTMX boost invariants, consult [`references/performance.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/performance.md).

1. **Eliminate N+1 Queries**:
   - Never execute repository queries inside loops (`for _, s := range ...`).
   - Use batch repository methods: `GetAllAttendances()`, `GetSessionAttendanceCounts()`, `GetLatestEvaluations()`, `GetAllStudentPackagesGrouped()`, and `GetAllUsers()`.
2. **Database & Index Efficiency**:
   - Keep attendance check-in and package deduction inside a single atomic transaction.
   - Ensure foreign keys and filter columns (`student_id`, `coach_id`, `session_id`, `session_date`) are indexed in both PostgreSQL and SQLite migrations.
3. **Go Template & Buffer Efficiency**:
   - Pre-parse templates at startup (`baseTmpl.Clone()` per page) and render via pooled `bytes.Buffer` (`internal/handlers`) with `GzipMiddleware`.
4. **HTMX Boost & Partial Swap Invariant**:
   - Because `<body hx-boost="true">` is enabled globally, boosted page navigations send both `HX-Request: true` and `HX-Boosted: true`.
   - Handlers serving both full pages and filter partials (`HandleStudents`, `HandleSessions`) MUST check `r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true"` before returning a partial snippet.
   - Dynamic modal forms whose `action` is set by JavaScript must include `hx-boost="false"` and `hx-disable="true"`.
