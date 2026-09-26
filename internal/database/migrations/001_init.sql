-- Coaches & Staff
CREATE TABLE IF NOT EXISTS coaches (
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
CREATE TABLE IF NOT EXISTS students (
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
CREATE TABLE IF NOT EXISTS package_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(100) NOT NULL,
    description TEXT,
    session_count INT, -- NULL signifies unlimited
    validity_days INT NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

-- Purchased Student Packages
CREATE TABLE IF NOT EXISTS student_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    template_id UUID NOT NULL REFERENCES package_templates(id),
    total_sessions INT,
    remaining_sessions INT,
    custom_price NUMERIC(10, 2),
    notes TEXT,
    purchase_date DATE NOT NULL DEFAULT CURRENT_DATE,
    expiry_date DATE NOT NULL,
    payment_status VARCHAR(20) NOT NULL DEFAULT 'paid', -- 'paid', 'unpaid', 'refunded'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Training Sessions (Floor Log)
CREATE TABLE IF NOT EXISTS training_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_date DATE NOT NULL DEFAULT CURRENT_DATE,
    start_time VARCHAR(20) NOT NULL,
    end_time VARCHAR(20) NOT NULL,
    coach_id UUID NOT NULL REFERENCES coaches(id),
    admin_id UUID REFERENCES coaches(id),
    training_type VARCHAR(50) NOT NULL, -- 'Poomsae', 'Sparring', 'Conditioning', etc.
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Attendance Records
CREATE TABLE IF NOT EXISTS attendance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES training_sessions(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    student_package_id UUID REFERENCES student_packages(id),
    checked_in_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_student_session UNIQUE (session_id, student_id)
);

-- Student Ability Evaluations (Radar Chart Source)
CREATE TABLE IF NOT EXISTS student_evaluations (
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
