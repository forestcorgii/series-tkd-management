---
name: asdste100-diagram-communicator
description: >-
  Trigger when the user asks for technical explanations, system designs, runbooks,
  or step-by-step procedures. Formats explanatory responses in 80% ASD-STE100
  Simplified Technical English with a strong preference for visual diagrams.
---

# ASD-STE100 Diagram Communicator

## 1. Scope & Rule Compatibility
- **Applies To**: Technical explanations, architecture walkthroughs, runbooks, and step-by-step procedures.
- **Does Not Override**: Interactive clarification questions (`AGENTS.md` Rule I.1), planning prompts, or code-editing workflows.
- **Domain Terminology**: When explaining STMS features, use the canonical terms defined in [`references/terminology-standards.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/terminology-standards.md) (`Class`, `Attendance`, `Memberships`, `₱`).

## 2. Primary Directive (Visuals First)
Prefer creating a diagram (using Mermaid.js or Markdown tables) instead of writing prose whenever possible to explain concepts, architectures, or sequential flows.

### Mermaid.js Guardrails
- Use only supported diagram types: `flowchart TD`, `flowchart LR`, `sequenceDiagram`, `stateDiagram-v2`, `classDiagram`, `erDiagram`, or `xychart-beta`.
- Quote node labels containing parentheses or brackets: `id["Label (Extra Info)"]`.
- Do not use HTML tags inside node labels.

## 3. Writing Constraints (80% ASD-STE100)
When text must accompany a diagram or list steps, follow these rules:
1. **One Idea Per Sentence**: Place one verifiable fact or one instruction in each sentence. Do not write compound sentences.
2. **Short Sentences**: Cap procedural instructions at 20 words. Cap descriptive sentences at 25 words.
3. **Active Voice**: Use active voice and command verbs. State the entity performing the action.
4. **Consistent Terminology**: Pick one exact term for a concept and use it every time. Do not use synonyms for stylistic variety.
5. **Direct Structure**: Start immediately with the diagram or the first step. Strip out introductory filler, transition sentences, and concluding summaries.
