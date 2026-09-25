---
trigger: always_on
---

Series Taekwondo Management System (STMS)
Architecture Specification & Engineering Roadmap
---
1. System Overview & Tech Stack
Series Taekwondo Management System (STMS) is an internal club and dojang operations platform built for fast, reactive, low-overhead floor management. It is designed to run reliably on front-desk tablets, mobile devices used by coaches, and desktop workstations.
Core Stack
Language/Runtime: Go (Golang 1.22+)
Routing & HTTP: `net/http` standard library router (Go 1.22 enhanced routing) or `chi`
Frontend Interactivity: HTMX (Server-Driven Hypermedia)
Styling: Tailwind CSS (served via CLI standalone build or CDN for early MVP)
Visualizations: Chart.js or Alpine.js + inline SVG for radar ability charts
Database: PostgreSQL with `pgx` driver / raw SQL migrations
Templating: Go `html/template` or `templ` (type-safe Go templates)
Authentication: HTTP-only secure cookie sessions (Argon2id password hashing)
---
2. Core Domain Requirements & Features
2.1 Student Management
Basic Details: Full name, date of birth, gender, contact number, registration date, status (`active`, `inactive`, `on-leave`).
Safety & Medical: Emergency contact (name, phone, relationship), medical clearance flags, allergies, and injury logs.
Belt Progression: Current Kup/Dan rank, date of last promotion, next eligible test date.
Ability Chart: Six-factor athletic radar metric:
Flexibility
Stamina / Cardio
Power / Impact
Technique & Forms (Poomsae)
Sparring IQ / Timing (Kyorugi)
Discipline & Etiquette
2.2 Coach & Staff Directory
Profile: Basic info, Dan rank, assigned roles (`coach`, `head_instructor`, `admin`).
Safety Verification: Red Cross / BLS First Aid readiness status, certification issuance, and auto-flagging for expired credentials.
Specialties: Tagged expertise (`Sparring/Kyorugi`, `Forms/Poomsae`, `Demo Team`, `Conditioning`, `Cadets/Kids`).
Payroll & Billing: Base rate per session, compensation calculation based on verified session logs.
History: Direct aggregate query of all classes conducted and students supervised.
2.3 Packages & Billing
Package Templates: Pre-configured passes (e.g., 12-Session Sparring Card, Monthly Unlimited, Pre-Cadet Fundamentals).
Student Packages: Active packages tied to student records, tracking:
Total purchased sessions
Remaining sessions
Expiration date
Payment status (`paid`, `partial`, `pending`)
Auto-Deduction: Check-ins automatically decrement remaining credits and validate expiration.
2.4 Training Sessions & Attendance Floor Check-in
Session Attributes: Scheduled date/time, training category (`Poomsae`, `Sparring`, `Conditioning`, `Promotion Prep`), lead coach, supervising admin, session remarks.
Floor Check-In Screen: Quick-search or QR-ready input interface powered by HTMX:
Instant feedback on package status.
Rejection/warning for expired or zero-credit packages.
One-click manual override for authorized admins.
2.5 Promotion Readiness Insights
Weighted rules engine that dynamically displays a student's readiness badge:
Attendance Count: Has the student logged the minimum required sessions for their current rank? (e.g., 24 sessions from White to Yellow).
Time-in-Rank: Has the required calendar interval passed since `last_promotion_date`?
Curriculum Diversity: Has the student fulfilled required ratios of sparring vs. poomsae classes?
Ability Score Threshold: Does the latest coach evaluation meet the minimum benchmark (e.g., no attribute below 6/10)?
Output Badges:
🟢 `READY`: All criteria met; ready for promotion testing.
🟡 `PRE-TEST ELIGIBLE`: Attendance met; pending formal coach evaluation.
🔴 `DEVELOPING`: Insufficient hours or tenure.
---
3. Database Schema (PostgreSQL DDL)
```sql
-- Coaches & Staff
CREATE TABLE coaches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(120) NOT NULL,
    email VARCHAR(120) UNIQUE NOT NULL,
    phone VARCHAR(30) NOT NULL,
    belt_rank VARCHAR(50) NOT NULL,
    rate_per_session NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    first_aid_certified BOOLEAN NOT NULL DEFAULT FALSE,
    first_aid_expiry DATE,
    specialties TEXT[],
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Students
CREATE TABLE students (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(120) NOT NULL,
    dob DATE NOT NULL,
    gender VARCHAR(10),
    phone VARCHAR(30),
    current_belt VARCHAR(50) NOT NULL DEFAULT 'White',
    last_promotion_date DATE NOT NULL DEFAULT CURRENT_DATE,
    emergency_name VARCHAR(120) NOT NULL,
    emergency_phone VARCHAR(30) NOT NULL,
    emergency_relation VARCHAR(50) NOT NULL,
    medical_notes TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Packages & Membership Templates
CREATE TABLE package_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(100) NOT NULL,
    session_count INT, -- NULL signifies unlimited
    validity_days INT NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

-- Purchased Student Packages
CREATE TABLE student_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    template_id UUID NOT NULL REFERENCES package_templates(id),
    total_sessions INT,
    remaining_sessions INT,
    purchase_date DATE NOT NULL DEFAULT CURRENT_DATE,
    expiry_date DATE NOT NULL,
    payment_status VARCHAR(20) NOT NULL DEFAULT 'paid', -- 'paid', 'unpaid', 'refunded'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Training Sessions (Floor Log)
CREATE TABLE training_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_date DATE NOT NULL DEFAULT CURRENT_DATE,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    coach_id UUID NOT NULL REFERENCES coaches(id),
    admin_id UUID REFERENCES coaches(id),
    training_type VARCHAR(50) NOT NULL, -- 'Poomsae', 'Sparring', 'Conditioning', etc.
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Attendance Records
CREATE TABLE attendance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES training_sessions(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    student_package_id UUID REFERENCES student_packages(id),
    checked_in_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_student_session UNIQUE (session_id, student_id)
);

-- Student Ability Evaluations (Radar Chart Source)
CREATE TABLE student_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    coach_id UUID NOT NULL REFERENCES coaches(id),
    evaluation_date DATE NOT NULL DEFAULT CURRENT_DATE,
    flexibility INT CHECK (flexibility BETWEEN 1 AND 10),
    stamina INT CHECK (stamina BETWEEN 1 AND 10),
    power INT CHECK (power BETWEEN 1 AND 10),
    technique INT CHECK (technique BETWEEN 1 AND 10),
    sparring_iq INT CHECK (sparring_iq BETWEEN 1 AND 10),
    discipline INT CHECK (discipline BETWEEN 1 AND 10),
    coach_remarks TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
---
4. Go + HTMX Architecture & Project Layout
```
series-taekwondo/
├── cmd/
│   └── server/
│       └── main.go                 # Server bootstrapper & configuration
├── internal/
│   ├── database/
│   │   ├── db.go                   # Connection pooling (pgx)
│   │   └── migrations/             # SQL migration files
│   ├── models/                     # Core domain structs
│   │   ├── student.go
│   │   ├── coach.go
│   │   ├── session.go
│   │   └── package.go
│   ├── services/                   # Business logic (e.g. Promotion readiness)
│   │   ├── promotion_service.go
│   │   └── payroll_service.go
│   └── handlers/                   # HTTP / HTMX handlers returning HTML partials
│       ├── attendance_handler.go
│       ├── student_handler.go
│       ├── coach_handler.go
│       └── session_handler.go
├── web/
│   ├── static/                     # CSS, JS, brand assets
│   │   ├── img/series-logo.png
│   │   └── css/tailwind.css
│   └── templates/
│       ├── layout.html             # Base shell (Nav, Header, htmx.js)
│       ├── pages/                  # Full page layouts
│       │   ├── dashboard.html
│       │   ├── students.html
│       │   └── sessions.html
│       └── partials/               # HTMX targeted fragments
│           ├── attendance_list.html
│           ├── checkin_row.html
│           ├── readiness_badge.html
│           └── ability_radar.html
├── Makefile
└── go.mod
```
---
5. Key HTMX Interactive Flows
5.1 Real-Time Attendance Check-In
The floor coach opens `/sessions/{id}/live`.
As students arrive, typing into an input fires an HTMX search request:
```html
   <input type="search"
          name="query"
          placeholder="Scan QR or search student..."
          hx-post="/sessions/{{.SessionID}}/search-student"
          hx-trigger="keyup changed delay:250ms"
          hx-target="#search-results" />
   ```
Clicking "Check-In" performs:
```html
   <button hx-post="/sessions/{{.SessionID}}/checkin/{{.StudentID}}"
           hx-target="#attendance-roster"
           hx-swap="afterbegin">
       Admit Student
   </button>
   ```
The Go server:
Begins a database transaction.
Deducts 1 session from the oldest valid package.
Inserts an `attendance` record.
Returns a rendered HTML snippet `checkin_row.html` with current package status and readiness badge.
5.2 Dynamic Ability Chart Rendering
Instead of relying on heavy JavaScript frameworks, the Go handler returns either:
An inline SVG polygon dynamically generated by Go templates based on the 6 evaluation coordinates:
$$\text{Coord}_i = \left(R \cdot \frac{\text{score}_i}{10} \cdot \cos(\theta_i), R \cdot \frac{\text{score}_i}{10} \cdot \sin(\theta_i)\right)$$
Or an HTMX partial loading a simple Chart.js canvas with JSON dataset attributes embedded via `data-*` tags.
---
6. Implementation Milestones
Milestone 1: Database & Core Entities
Spin up PostgreSQL schema.
Implement Go structs, migrations, and CRUD operations for Students, Coaches, and Package Templates.
Milestone 2: Session Floor Operations & Packages
Build the active session view.
Connect package deduction logic with HTMX real-time search and check-in.
Milestone 3: Ability Matrix & Promotion Insights
Implement `student_evaluations` input screen for coaches.
Embed SVG/Chart.js radar visualization.
Build the promotion readiness evaluation pipeline.
Milestone 4: Coach Session Auditing & Payroll
Build coach profile dashboard showing session histories and calculated pay amounts based on rates.