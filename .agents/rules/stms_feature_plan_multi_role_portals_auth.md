---
trigger: model_decision
description: When working on auth related tasks
---

# Series Taekwondo Management System (STMS)
## Feature Specification: Multi-Role Portals & RBAC Engine
*Document Target: Antigravity Product Documentation / Architecture RFC*

---

## 1. System Context & Overview

The **Series Taekwondo Management System (STMS)** is an operational dojang floor management and athletic matrix platform deployed in production:
* **Production Deployment:** `https://series-tkd-management-production.up.railway.app/`
* **Operational Status:** Floor Online / Live Operations Active
* **Brand Foundation:**
  * **Color Palette:** Dojang Black (`#000000`), Martial Red (`#990303`), Dobok Off-White (`#FFFDF4`)
  * **Typography:** `Good Timing` (Display / System headers), `Montserrat` (Data rows / Operational UI)

### 1.1 Objective
Expand the platform from an open floor console into a secure, multi-tier system with role-based authentication separating **Student**, **Coach**, and **Admin** experiences.

---

## 2. Authentication & Authorization Architecture

### 2.1 Role-Based Access Control (RBAC) Taxonomy
The system identifies three distinct operational actors:
1. **`STUDENT`:** Martial arts practitioners tracking their personal promotion pipeline, package deductions, and scheduled training blocks.
2. **`COACH`:** Dan-certified instructors managing on-floor operations, conducting rapid check-ins, logging incident flags, and submitting athletic radar evaluations.
3. **`ADMIN`:** Dojang directors and administrative staff with complete oversight of schedule generation, curriculum milestones, rank approvals, user management, and safety flag clearances.

```
                               +----------------------------+
                               |     /api/auth/login        |
                               | (JWT / Secure Cookie Auth) |
                               +----------------------------+
                                             |
                   +-------------------------+-------------------------+
                   |                         |                         |
                   v                         v                         v
          Role: STUDENT                 Role: COACH               Role: ADMIN
          /portal/student              /portal/coach              /portal/admin
    +---------------------------+ +------------------------+ +------------------------+
    | - Personal Readiness Bar  | | - Rapid Floor Check-In | | - Master Operations    |
    | - Athletic Spider Chart   | | - Radar Score Form     | | - Dan Roster Control   |
    | - Package Credit Balance  | | - Active Floor Roster  | | - Promotion Overrides  |
    | - Schedule & QR Check-in  | | - Floor Incident Form  | | - Schedule Generator   |
    | - Personal Safety Status  | | - Readiness Priority Q | | - Safety Log Dismissal |
    +---------------------------+ +------------------------+ +------------------------+
```

### 2.2 Relational Data Models

#### Identity & Authentication Schema (`users`)
```sql
CREATE TYPE user_role AS ENUM ('STUDENT', 'COACH', 'ADMIN');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role user_role NOT NULL DEFAULT 'STUDENT',
    student_id UUID NULL REFERENCES students(id) ON DELETE SET NULL,
    coach_id UUID NULL REFERENCES coaches(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMP WITH TIME ZONE NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

#### Student Profile Linkage (`students`)
```sql
CREATE TYPE readiness_badge AS ENUM ('READY', 'PRE_TEST_ELIGIBLE', 'DEVELOPING');

CREATE TABLE students (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(255) NOT NULL,
    belt_rank VARCHAR(50) NOT NULL,
    sessions_attended INT NOT NULL DEFAULT 0,
    sessions_required INT NOT NULL DEFAULT 16,
    tenure_days INT NOT NULL DEFAULT 0,
    tenure_required INT NOT NULL DEFAULT 45,
    package_credits_remaining INT NOT NULL DEFAULT 0,
    readiness_status readiness_badge NOT NULL DEFAULT 'DEVELOPING',
    has_safety_flag BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

#### Floor Session Schedule (`floor_sessions`)
```sql
CREATE TYPE martial_discipline AS ENUM ('Poomsae', 'Sparring');

CREATE TABLE floor_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discipline martial_discipline NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    lead_instructor VARCHAR(255) NOT NULL,
    capacity INT NOT NULL DEFAULT 25,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
```

---

## 3. Surface Specifications & User Views

### 3.1 Student Portal (`/portal/student`)
**Purpose:** Self-service progression tracking, personal schedule viewer, and prepaid package monitor.

* **Readiness Progress HUD:**
  * Displays current belt rank and badge (`READY` 🟢, `PRE-TEST ELIGIBLE` 🟡, `DEVELOPING` 🔴).
  * Real-time progress bars comparing attended sessions and tenure against curriculum thresholds:
    $$\text{Session Completion} = \frac{\text{Sessions Attended}}{\text{Sessions Required}} \times 100\%$$
    $$\text{Tenure Completion} = \frac{\text{Tenure Days}}{\text{Tenure Required}} \times 100\%$$
* **Athletic Radar Matrix:**
  * Visualization across 5 athletic pillars: *Technique, Power, Agility, Flexibility, Tactical Sparring/Poomsae*.
* **Package Balance & Auto-Deduction Wallet:**
  * Displays remaining prepaid floor credits.
  * Audit log showing past class check-ins and package decrements.
* **Class Schedule & Self Check-in:**
  * Upcoming dojang sessions (e.g., 17:00 Sparring, 18:30 Poomsae).
  * In-app QR generator or "Mat Check-In" action button enabled within 15 minutes of class start.
* **Safety & First Aid Notice:**
  * Notice banner if flagged for First Aid review with instructions on required clearance before stepping on the mat.

---

### 3.2 Coach Portal (`/portal/coach`)
**Purpose:** Floor execution tool optimized for mobile/tablet use during training.

* **Live Floor Session View:**
  * Displays assigned session slots (e.g., Coach Ji-Woo Park, Master Dae-Hyun Kim).
  * Roster of currently checked-in practitioners.
* **Rapid Mat Check-In:**
  * Tap-to-check-in functionality triggering immediate package decrement.
* **Athletic Evaluation Quick-Entry:**
  * Post-sparring/forms assessment tool to input score adjustments (1–10 scale) feeding the promotion readiness engine.
* **Promotion Review Queue:**
  * Highlights students currently on the floor tagged as `READY` (e.g., Alex Vance) or `PRE-TEST ELIGIBLE` (e.g., Chloe Ramirez) for grading observation.
* **First Aid / Safety Flag Logger:**
  * Emergency quick-entry form to record floor incidents (e.g., joint strain, contusion), instantly marking `has_safety_flag = TRUE`.

---

### 3.3 Admin Portal (`/portal/admin`)
**Purpose:** Full operational and business management of the dojang.

* **Floor Telemetry Operations Dashboard:**
  * Live counters: Active students, floor sessions conducted, Dan-certified coaches active, open safety incidents.
* **Curriculum & Schedule Manager:**
  * Form to add, update, and assign floor slots:
    * 17:00 – 18:15 (Sparring / Poomsae)
    * 17:00 – 18:30 (Sparring Extended)
    * 18:30 – 19:45 (Poomsae / Sparring)
    * 18:30 – 20:00 (Poomsae Extended)
* **Promotion Pipeline Oversight & Rank Advancements:**
  * Table sorting all students by readiness status.
  * Single-click promotion advancement: resets tenure days and attendance counts to 0 and increments belt grade.
* **Safety & Incident Clearance:**
  * Administrative queue to resolve First Aid tickets and clear flags.
* **Package Ledger & Account Top-ups:**
  * Billing and prepaid credit administration.

---

## 4. API Surface & Security Endpoints

| Route | Method | Access Level | Description |
| :--- | :--- | :--- | :--- |
| `/api/auth/register` | `POST` | Admin | Provision new user credentials |
| `/api/auth/login` | `POST` | Public | Authenticate user and issue session token |
| `/api/auth/me` | `GET` | Authenticated | Retrieve authenticated user profile and permissions |
| `/api/student/readiness` | `GET` | Student | Return personal readiness metrics and radar data |
| `/api/coach/sessions/live` | `GET` | Coach, Admin | Fetch active floor session and checked-in roster |
| `/api/coach/check-in` | `POST` | Coach, Admin | Confirm student arrival & decrement package balance |
| `/api/coach/evaluate` | `POST` | Coach, Admin | Submit athletic radar scores |
| `/api/safety/flag` | `POST` | Coach, Admin | Log a First Aid incident |
| `/api/safety/resolve` | `PATCH` | Admin | Clear student safety hold |
| `/api/admin/schedule` | `POST`/`PUT` | Admin | Create or modify scheduled floor sessions |
| `/api/admin/promote` | `POST` | Admin | Advance student belt rank and reset cycle counters |

---

## 5. Implementation Roadmap

```
+-----------------------------------------------------------------------+
| Milestone 1: Core RBAC & Auth Guard Engine                           |
| - Provision users table & password hashing (Argon2id/bcrypt)          |
| - Deploy JWT / HTTP-only cookie session handling                      |
| - Introduce RBAC Route Guards across frontend and backend endpoints    |
+-----------------------------------------------------------------------+
                                  |
                                  v
+-----------------------------------------------------------------------+
| Milestone 2: Coach Mat Tools & Check-In Workflow                      |
| - Mobile-first mat dashboard                                          |
| - Single-tap attendance check-in linked to package deductions         |
| - On-floor quick evaluation interface                                 |
+-----------------------------------------------------------------------+
                                  |
                                  v
+-----------------------------------------------------------------------+
| Milestone 3: Student Experience & Self-Service                        |
| - Belt progress visualizer & readiness badges                         |
| - Credit balance wallet & session schedule views                      |
+-----------------------------------------------------------------------+
                                  |
                                  v
+-----------------------------------------------------------------------+
| Milestone 4: Admin Governance & Incident Resolution                   |
| - Rank promotion reset triggers                                       |
| - First Aid clearance module                                          |
| - Master timetable scheduling system                                  |
+-----------------------------------------------------------------------+
```