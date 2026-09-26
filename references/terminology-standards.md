# UI & Operations Terminology Standards

This document specifies the canonical terminology standard for user-facing UI labels, navigation menus, badges, buttons, and customer-facing reports across the Series Taekwondo Management System (STMS).

---

## 1. Core Terminology Mappings

To maintain operational clarity for students, parents, coaches, and front-desk staff while ensuring 100% backward-compatibility with backend databases, APIs, and Go domain models, the following presentation-layer mapping is enforced:

| Operational Concept | Canonical UI Term | Prohibited / Legacy UI Terms | Underlying Backend / DB Mapping (Intact) |
| :--- | :--- | :--- | :--- |
| **Scheduled Class** | **Class / Training Class** | Session, Training Session, Floor Block | `training_sessions` table, `models.Session`, `/sessions` routes |
| **Attendance Check-In** | **Attendance / Open Attendance** | Floor Check-In, Mat Check-In, Admittance | `attendance` table, `/sessions/{id}/checkin` endpoints |
| **Passes & Passes Roster**| **Memberships / Membership Plans**| Package, Pass, Passes & Billing, Package Pass | `package_templates`, `student_packages` tables, `models.Package` |
| **Session Units** | **Classes / Remaining Classes** | Sessions, Credits, Remaining Sessions | `student_packages.remaining_sessions`, `student_packages.total_sessions` |
| **Live Roster Screen** | **Live Attendance** | Live Floor Check-In, Floor Standby | `/sessions/{id}/live` view handler |
| **Currency Display** | **Philippine Peso (₱ / PHP)** | USD, US Dollar ($) | Float64 rates & prices (`RatePerSession`, `Price`, `CustomPrice`) |

---

## 2. Page & Navigation Conventions

- **Global Navigation (Header):**
  - Desktop: `Dashboard`, `Students`, `Attendance`, `Coaches`, `Memberships`
  - Mobile: `Dashboard`, `Students`, `Attendance`, `Coaches`, `Memberships`
- **Dashboard CTA & Stats:**
  - Hero Button: `⚡ Open Attendance`
  - Counter Metric: `Total Classes`
  - Floor Quick-Access: `Active Classes`
- **Memberships Screen:**
  - Header: `Memberships`
  - Primary Action: `💳 Assign Membership to Student`
  - Template Badge: `Membership Template`
  - Roster Table: `Active Student Memberships Roster`
- **Classes Screen:**
  - Header: `Training Classes & Floor Attendance Log`
  - Scheduling Modal: `Schedule Training Class`
  - Date Field: `Class Date`

---

## 3. Belt Progression Standards

### Context: PTA Belt Progression Hierarchy
- **Problem:** Belt ranking previously contained non-standard "Tag" suffixes and omitted brown belts, which conflicted with the official Philippine Taekwondo Association (PTA) curriculum.
- **Enforced Solution:** Use 12 sequential belt stages from White to 3rd Dan Black Belt.

| Progression Order | Belt Rank Name | Backend Constant | Default Session Req | Default Tenure Days |
| :--- | :--- | :--- | :--- | :--- |
| 1 | **White** | `BeltWhite` | 16 | 45 |
| 2 | **Low Yellow** | `BeltLowYellow` | 20 | 60 |
| 3 | **High Yellow** | `BeltHighYellow` | 24 | 60 |
| 4 | **Low Blue** | `BeltLowBlue` | 28 | 75 |
| 5 | **High Blue** | `BeltHighBlue` | 32 | 90 |
| 6 | **Low Red** | `BeltLowRed` | 36 | 105 |
| 7 | **High Red** | `BeltHighRed` | 40 | 120 |
| 8 | **Low Brown** | `BeltLowBrown` | 44 | 135 |
| 9 | **High Brown** | `BeltHighBrown` | 48 | 150 |
| 10 | **1st Dan Black** | `BeltBlack1stDan` | 60 | 180 |
| 11 | **2nd Dan Black** | `BeltBlack2ndDan` | 72 | 240 |
| 12 | **3rd Dan Black** | `BeltBlack3rdDan` | 84 | 365 |

