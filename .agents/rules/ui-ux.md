---
trigger: model_decision
description: when working on design tokens, typography rules, layout hierarchy, and HTMX interaction patterns.
---

# Series Taekwondo - UI/UX & Design System Guide (Go + HTMX)

This guide defines the design tokens, typography rules, layout hierarchy, and HTMX interaction patterns for building the Series Taekwondo dojang management system.

---

## 1. Brand Identity & Visual Language

Series Taekwondo blends a sharp, modern athletic aesthetic with martial arts discipline. The UI emphasizes high contrast, geometric precision, bold typography, and clear visual state transitions suitable for floor tablets and desktop admin desks.

### Core Color Palette
* **Series Black (`#000000`)**: Deep neutral used for dark backgrounds, structural headers, high-contrast borders, and primary dark buttons.
* **Series Crimson (`#990303`)**: High-impact brand red. Used for active badges, alerts, primary action buttons, destructive operations, and accent highlights.
* **Warm Cream / Off-White (`#FFFDF4`)**: The signature canvas surface. Used for light-mode view backgrounds, card surfaces, input fills, and contrasting text on dark elements.

### Supporting / Functional Palette
* **Pure White (`#FFFFFF`)**: Modal backdrops, crisp card highlights, and icon fills on dark backgrounds.
* **Border Gray (`#E5E3D8`)**: Subtle borders and table dividers on cream backgrounds.
* **Muted Gray (`#666666` / `#8C8A82`)**: Secondary helper text, timestamps, and placeholder copy.
* **Status Success (`#1B7F43`)**: Package active, attendance logged, promotion ready.
* **Status Warning (`#C98A0C`)**: Package expiring soon, pre-test eligible.

---

## 2. Typography Rules

The visual identity relies on two distinct typefaces:

### Primary Display Font: **Good Timing**
* **Role**: Primary brand identity, major section titles, modal headers, scorecard numbers, and brand hero elements.
* **Characteristics**: Extended, modern, geometric sans-serif with a technical, high-performance edge.
* **Usage Rules**:
  * Headings (`h1`, `h2`), large metric callouts, and dojang branding.
  * Always use uppercase or title case for headers (`text-transform: uppercase; letter-spacing: 0.05em;`).

### Body & Interface Font: **Montserrat**
* **Role**: All operational text, navigation labels, data tables, form inputs, status badges, and metadata.
* **Characteristics**: Clean, geometric, legible grotesque sans-serif with multiple weights.
* **Usage Rules**:
  * Body copy: Regular (`400`) or Medium (`500`).
  * Table headers & Buttons: SemiBold (`600`) or Bold (`700`) with slight letter-spacing.
  * Microcopy & Tags: Medium (`500`) at 11px–13px.

---

## 3. Tailwind CSS Configuration Preset

When compiling Tailwind CSS alongside Go templates, map the brand tokens as follows:

```javascript
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/**/*.html"],
  theme: {
    extend: {
      colors: {
        brand: {
          black: '#000000',
          red: '#990303',
          cream: '#FFFDF4',
          'cream-dark': '#F4F1E4',
          'red-hover': '#7D0202',
        }
      },
      fontFamily: {
        display: ['"Good Timing"', 'sans-serif'],
        sans: ['Montserrat', 'sans-serif'],
      }
    },
  },
  plugins: [],
}