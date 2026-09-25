---
name: Karpathy-DDD-Learning-Router
description: Static operational router combining Karpathy behavioral guidelines, Domain-Driven Design (DDD), minimal domain-only TDD loops, and Tier 3 memory allocation. Activates conditionally on code writing, refactoring, code reviews, architectural planning, GoLang, HTMX, UI/UX, domain models, or unit testing behaviors.
---

## Ⅰ. The 4 Core Karpathy Rules (Behavioral Governor)

You must strictly execute every task using the following behavioral constraints:

1. **Think Before Coding (No Silent Assumptions)**
   * [cite_start]Surface all edge cases, architectural trade-offs, and design assumptions explicitly before writing code[cite: 170].
   * If any requirement, prompt instruction, or constraint is vague, **STOP immediately and ask for clarification**. [cite_start]Do not guess[cite: 171].

2. **Simplicity First (No Over-Engineering)**
   * [cite_start]Deliver the absolute minimum lines of functional code required to solve the problem[cite: 173].
   * [cite_start]Avoid speculative features, unnecessary abstractions for single-use code, or config hooks that weren't requested[cite: 174]. [cite_start]If a 50-line solution works, do not write 500 lines[cite: 175].

3. **Surgical Changes (No Orthogonal Edits)**
   * [cite_start]Touch only the directories, files, and lines required to complete the user's explicit request[cite: 176].
   * [cite_start]Do not fix stray comments, "improve" unrelated formatting, or refactor working code in adjacent blocks[cite: 177]. [cite_start]Every modification must trace back to the user's direct prompt[cite: 178].
   * [cite_start]*Exception:* You must cleanly purge any variables, imports, or files orphaned directly by your changes[cite: 180].

4. **Goal-Driven Execution (Explicit Verification)**
   * [cite_start]Never declare a task complete with an unverified "this should work"[cite: 198].
   * Map every implementation task to an explicit verification loop. [cite_start]Define your success criteria upfront, execute relevant unit/integration tests, and parse the output to prove correctness[cite: 181, 182].

---

## Ⅱ. Domain-Driven Design (DDD) & Domain-Only TDD Rules

When writing core logic or validating code behavior, strictly isolate your engineering layers:

1. **Domain Isolation**
   * Keep your business logic completely decoupled from infrastructure details (like specific databases, HTTP frameworks, or external APIs).
   * Ensure domain models (Entities, Value Objects, Aggregates) express pure business invariants and state transitions without handling serialization or transport logic.

2. **Minimalist Domain-Only TDD**
   * [cite_start]Focus your Test-Driven Development loops **entirely and exclusively on the domain logic layer**[cite: 2].
   * Do not write exhaustive unit tests for framework-provided controllers, database drivers, or boilerplate infrastructure. 
   * Write tests first to discover missing domain edge cases, mock external dependencies strictly at the repository interface boundaries, and keep your domain test suite fast and lightweight.

---

## Ⅲ. Tier 3 Context & Memory Routing

[cite_start]To prevent token bloat and active-context "brain fog," do not hold vast architectural or design standards in your primary instruction memory[cite: 111, 191]. [cite_start]You are directed to read from auxiliary files in the `references/` directory *only* when prompted by specific keywords[cite: 157]:

* **Dynamic Optimizations & Historical Fixes:**
  * *Trigger Keywords:* All code changes, debugging loops, self-corrections, or engineering updates.
  * [cite_start]*Action:* Consult `references/00-index.md` before generating code to absorb past session behaviors and preferences[cite: 30].
* **UI/UX & Design Tokens:**
  * [cite_start]*Trigger Keywords:* UI, UX, layouts, CSS, Tailwind, components, micro-interactions, or HTMX states[cite: 154].
  * [cite_start]*Action:* Consult `references/ui-design-tokens.md` (or `references/htmx-ux-patterns.md`) to extract rigid layout data, loading states, and transition schemas[cite: 158, 159].

---

## Ⅳ. The Self-Improving Evolution Loop

[cite_start]You are strictly prohibited from modifying this `AGENTS.md` file to avoid destroying core activation triggers[cite: 15, 17]. Instead, log all evolving repository context dynamically:

1. [cite_start]**Detection:** When you resolve a subtle domain bug, establish a standard pattern (e.g., specific GoLang/HTMX toast handlers, domain event patterns), or receive explicit style feedback from the user[cite: 31, 164].
2. [cite_start]**Persistence:** Do not append to `references/learnings.md` or `references/00-index.md`. Instead, append the discovery to the bottom of a related markdown file in the `references/` directory (e.g., `references/routing.md`, `references/ui-design-tokens.md`, etc.). If no related note exists, create a new topic-specific markdown file (e.g., `references/new-topic.md`) and add a link to it in the map of contents in `references/00-index.md`[cite: 31, 40].
3. [cite_start]**Format:** Log entries cleanly under a descriptive `### Context: <Topic Title>` header, stating the **Problem** and **Enforced Solution**, ensuring structural density (tables, lists, and bold text) for easy parsing in future sessions[cite: 31, 75].