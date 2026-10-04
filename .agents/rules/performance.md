---
description: Behavioral rules for prioritizing performance, minimizing latency, and verifying performance improvements across Go, PostgreSQL, and HTMX
---

# Performance Prioritization & Continuous Verification

1. **Proactive Performance Evaluation**
   - For every architectural proposal, database query, and endpoint implementation, explicitly analyze and communicate the performance profile (latency, memory footprint, query complexity).
   - Prioritize low-latency execution paths on critical workflows (e.g., student floor check-in, attendance search, package deduction).

2. **Database & Query Efficiency**
   - **Eliminate N+1 Queries**: Never execute queries inside loops. Use single batch queries, joins, or aggregation subqueries.
   - **Index Awareness**: Ensure foreign keys, lookup fields, and filtered columns (e.g., `student_id`, `session_date`, `is_active`) utilize covering or selective indexes.
   - **Fetch Only What Is Needed**: Select explicit columns rather than unbounded wildcards; limit search result sets.
   - **Atomic Transactions**: Keep transaction boundaries tight to prevent lock contention on high-frequency tables (`attendance`, `student_packages`).

3. **Backend Go Performance Invariants**
   - **Minimize Unnecessary Allocations**: In high-throughput handlers, reuse buffers (`sync.Pool` or `bytes.Buffer`) and avoid superfluous interface conversions.
   - **Parse Templates Once**: Pre-parse Go HTML templates at application boot; never re-parse template sets per request in production paths.
   - **Connection Pool Tuning**: Configure `pgxpool` max connections, idle timeouts, and connection reuse parameters mindfully.

4. **Frontend & HTMX Network Efficiency**
   - **Minimal Payloads**: Return minimal HTML fragments strictly covering the swap target rather than whole-page re-renders.
   - **Debouncing & Throttling**: Always specify appropriate debouncing (e.g., `delay:250ms`) on live-search inputs to avoid flooding the server with search requests.
   - **Lightweight Visualizations**: Keep radar charts and SVG visual metrics lightweight and cleanly computed on the server.

5. **Verification Requirement**
   - Before closing any optimization or feature task, verify performance implications:
     - Check query count per HTTP interaction (target: 1-2 queries per check-in / search).
     - Run benchmarks (`go test -bench=. -benchmem`) for critical domain calculations or hot paths when applicable.
