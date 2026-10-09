package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"series-tkd-management/internal/models"
)

type SQLStore struct {
	db     *sql.DB
	driver string
}

func NewSQLStore(db *sql.DB, driver string) *SQLStore {
	return &SQLStore{
		db:     db,
		driver: driver,
	}
}

func (s *SQLStore) DB() *sql.DB {
	return s.db
}

func (s *SQLStore) Close() error {
	return s.db.Close()
}

// InitDatabase initializes the connection, runs migrations, and seeds if empty.
func InitDatabase(databaseURL string) (RepositoryStore, string, error) {
	var driver, connStr string
	if databaseURL != "" && (strings.HasPrefix(databaseURL, "postgres://") || strings.HasPrefix(databaseURL, "postgresql://")) {
		driver = "pgx"
		connStr = databaseURL
	} else {
		driver = "sqlite"
		if databaseURL != "" && !strings.HasPrefix(databaseURL, "postgres") {
			connStr = databaseURL
		} else {
			connStr = "series_tkd.db"
		}
	}

	db, err := sql.Open(driver, connStr)
	if err != nil {
		return nil, driver, fmt.Errorf("failed to open database (%s): %w", driver, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, driver, fmt.Errorf("failed to ping database (%s): %w", driver, err)
	}

	if driver == "sqlite" {
		// WAL mode for non-blocking concurrent readers & fast writes
		_, _ = db.Exec("PRAGMA journal_mode=WAL;")
		_, _ = db.Exec("PRAGMA synchronous=NORMAL;")
		_, _ = db.Exec("PRAGMA busy_timeout=5000;")
		_, _ = db.Exec("PRAGMA cache_size=-20000;") // 20MB cache
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(30 * time.Minute)
	} else {
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(15 * time.Minute)
		db.SetConnMaxIdleTime(5 * time.Minute)
	}

	store := NewSQLStore(db, driver)
	if err := store.runMigrations(); err != nil {
		db.Close()
		return nil, driver, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Auto-seed if empty
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM students").Scan(&count)
	if count == 0 {
		if err := store.SeedDefaultData(); err != nil {
			// Non-fatal if seeding fails
			fmt.Printf("Warning: failed to seed default data: %v\n", err)
		}
	}

	return store, driver, nil
}

func (s *SQLStore) runMigrations() error {
	var schema string
	if s.driver == "sqlite" {
		schema = `
		CREATE TABLE IF NOT EXISTS coaches (
			id TEXT PRIMARY KEY,
			full_name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			phone TEXT NOT NULL,
			belt_rank TEXT NOT NULL,
			rate_per_session REAL NOT NULL DEFAULT 0.00,
			first_aid_certified INTEGER NOT NULL DEFAULT 0,
			first_aid_expiry TEXT,
			specialties TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS students (
			id TEXT PRIMARY KEY,
			full_name TEXT NOT NULL,
			dob TEXT NOT NULL,
			gender TEXT,
			phone TEXT,
			current_belt TEXT NOT NULL DEFAULT 'White',
			last_promotion_date TEXT NOT NULL,
			emergency_name TEXT NOT NULL,
			emergency_phone TEXT NOT NULL,
			emergency_relation TEXT NOT NULL,
			medical_notes TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS package_templates (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT,
			plan_type TEXT DEFAULT 'standard',
			session_count INTEGER,
			sessions_per_week INTEGER,
			validity_days INTEGER NOT NULL,
			price REAL NOT NULL,
			is_active INTEGER NOT NULL DEFAULT 1
		);

		CREATE TABLE IF NOT EXISTS student_packages (
			id TEXT PRIMARY KEY,
			student_id TEXT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
			template_id TEXT NOT NULL REFERENCES package_templates(id),
			plan_type TEXT DEFAULT 'standard',
			total_sessions INTEGER,
			remaining_sessions INTEGER,
			sessions_per_week INTEGER,
			custom_price REAL,
			notes TEXT,
			purchase_date TEXT NOT NULL,
			expiry_date TEXT NOT NULL,
			payment_status TEXT NOT NULL DEFAULT 'paid',
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS locations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			pin TEXT NOT NULL,
			fixed_rate REAL,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS training_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			color TEXT NOT NULL,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS training_sessions (
			id TEXT PRIMARY KEY,
			session_date TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			coach_id TEXT REFERENCES coaches(id),
			admin_id TEXT,
			location_id TEXT REFERENCES locations(id),
			session_rate REAL,
			training_type TEXT NOT NULL,
			notes TEXT,
			is_cancelled INTEGER NOT NULL DEFAULT 0,
			cancellation_reason TEXT DEFAULT '',
			cancelled_at TEXT,
			entry_type TEXT NOT NULL DEFAULT 'class',
			title TEXT DEFAULT '',
			assigned_staff TEXT DEFAULT '[]',
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS attendance (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES training_sessions(id) ON DELETE CASCADE,
			student_id TEXT,
			student_package_id TEXT REFERENCES student_packages(id),
			location_id TEXT REFERENCES locations(id),
			session_rate REAL,
			checked_in_at TEXT NOT NULL,
			attendee_type TEXT NOT NULL DEFAULT 'student',
			attendee_name TEXT DEFAULT '',
			attendee_role TEXT DEFAULT ''
		);

		CREATE TABLE IF NOT EXISTS student_evaluations (
			id TEXT PRIMARY KEY,
			student_id TEXT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
			coach_id TEXT NOT NULL REFERENCES coaches(id),
			evaluation_date TEXT NOT NULL,
			flexibility INTEGER CHECK (flexibility BETWEEN 1 AND 10),
			stamina INTEGER CHECK (stamina BETWEEN 1 AND 10),
			power INTEGER CHECK (power BETWEEN 1 AND 10),
			technique INTEGER CHECK (technique BETWEEN 1 AND 10),
			sparring_iq INTEGER CHECK (sparring_iq BETWEEN 1 AND 10),
			discipline INTEGER CHECK (discipline BETWEEN 1 AND 10),
			coach_remarks TEXT,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			username TEXT UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'STUDENT',
			display_name TEXT DEFAULT '',
			profile_picture_url TEXT DEFAULT '',
			student_id TEXT REFERENCES students(id) ON DELETE SET NULL,
			coach_id TEXT REFERENCES coaches(id) ON DELETE SET NULL,
			is_active INTEGER NOT NULL DEFAULT 1,
			last_login_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS user_sessions (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token TEXT UNIQUE NOT NULL,
			expires_at TEXT NOT NULL,
			used_at TEXT,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS safety_incidents (
			id TEXT PRIMARY KEY,
			student_id TEXT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
			coach_id TEXT REFERENCES coaches(id) ON DELETE SET NULL,
			incident_type TEXT NOT NULL,
			notes TEXT NOT NULL,
			resolved INTEGER NOT NULL DEFAULT 0,
			resolved_by TEXT,
			resolved_at TEXT,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			actor_name TEXT NOT NULL DEFAULT '',
			actor_email TEXT NOT NULL DEFAULT '',
			actor_role TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			category TEXT NOT NULL,
			target_type TEXT DEFAULT '',
			target_id TEXT DEFAULT '',
			target_name TEXT DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			ip_address TEXT DEFAULT '',
			user_agent TEXT DEFAULT '',
			metadata TEXT DEFAULT '',
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS notification_settings (
			id TEXT PRIMARY KEY,
			push_enabled INTEGER NOT NULL DEFAULT 1,
			sms_enabled INTEGER NOT NULL DEFAULT 1,
			email_enabled INTEGER NOT NULL DEFAULT 1,
			notify_admin_student_admitted INTEGER NOT NULL DEFAULT 1,
			notify_admin_new_class_opened INTEGER NOT NULL DEFAULT 1,
			notify_admin_staff_registered INTEGER NOT NULL DEFAULT 1,
			notify_admin_student_injured INTEGER NOT NULL DEFAULT 1,
			notify_admin_class_cancelled INTEGER NOT NULL DEFAULT 1,
			notify_student_promotion_eligible INTEGER NOT NULL DEFAULT 1,
			notify_student_injured INTEGER NOT NULL DEFAULT 1,
			notify_student_evaluation_logged INTEGER NOT NULL DEFAULT 1,
			notify_student_membership_changed INTEGER NOT NULL DEFAULT 1,
			notify_student_safety_resolved INTEGER NOT NULL DEFAULT 1,
			notify_student_class_cancelled INTEGER NOT NULL DEFAULT 1,
			notify_student_pass_expiring INTEGER NOT NULL DEFAULT 1,
			smtp_host TEXT DEFAULT '',
			smtp_port INTEGER DEFAULT 587,
			smtp_user TEXT DEFAULT '',
			smtp_password TEXT DEFAULT '',
			smtp_from TEXT DEFAULT '',
			sms_provider TEXT DEFAULT 'simulated',
			sms_api_key TEXT DEFAULT '',
			sms_from_number TEXT DEFAULT '',
			web_push_public_key TEXT DEFAULT '',
			web_push_private_key TEXT DEFAULT '',
			web_push_subject TEXT DEFAULT '',
			updated_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS user_notification_preferences (
			user_id TEXT PRIMARY KEY,
			email_enabled INTEGER NOT NULL DEFAULT 1,
			sms_enabled INTEGER NOT NULL DEFAULT 1,
			push_enabled INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			recipient_role TEXT DEFAULT '',
			recipient_type TEXT NOT NULL,
			recipient_name TEXT NOT NULL,
			recipient_contact TEXT DEFAULT '',
			channel TEXT NOT NULL,
			event_type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			metadata TEXT DEFAULT '',
			status TEXT NOT NULL,
			is_read INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);`
	} else {
		schema = `
		CREATE TABLE IF NOT EXISTS coaches (
			id UUID PRIMARY KEY,
			full_name VARCHAR(120) NOT NULL,
			email VARCHAR(120) UNIQUE NOT NULL,
			phone VARCHAR(30) NOT NULL,
			belt_rank VARCHAR(50) NOT NULL,
			rate_per_session NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
			first_aid_certified BOOLEAN NOT NULL DEFAULT FALSE,
			first_aid_expiry DATE,
			specialties TEXT,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS students (
			id UUID PRIMARY KEY,
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

		CREATE TABLE IF NOT EXISTS package_templates (
			id UUID PRIMARY KEY,
			title VARCHAR(100) NOT NULL,
			description TEXT,
			plan_type VARCHAR(50) DEFAULT 'standard',
			session_count INT,
			sessions_per_week INT,
			validity_days INT NOT NULL,
			price NUMERIC(10, 2) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE
		);

		CREATE TABLE IF NOT EXISTS student_packages (
			id UUID PRIMARY KEY,
			student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
			template_id UUID NOT NULL REFERENCES package_templates(id),
			plan_type VARCHAR(50) DEFAULT 'standard',
			total_sessions INT,
			remaining_sessions INT,
			sessions_per_week INT,
			custom_price NUMERIC(10, 2),
			notes TEXT,
			purchase_date DATE NOT NULL DEFAULT CURRENT_DATE,
			expiry_date DATE NOT NULL,
			payment_status VARCHAR(20) NOT NULL DEFAULT 'paid',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS locations (
			id UUID PRIMARY KEY,
			name VARCHAR(150) NOT NULL,
			pin TEXT NOT NULL,
			fixed_rate NUMERIC(10, 2),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS training_categories (
			id UUID PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			color VARCHAR(30) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS training_sessions (
			id UUID PRIMARY KEY,
			session_date DATE NOT NULL DEFAULT CURRENT_DATE,
			start_time VARCHAR(20) NOT NULL,
			end_time VARCHAR(20) NOT NULL,
			coach_id UUID REFERENCES coaches(id),
			admin_id UUID,
			location_id UUID REFERENCES locations(id),
			session_rate NUMERIC(10, 2),
			training_type VARCHAR(50) NOT NULL,
			notes TEXT,
			is_cancelled BOOLEAN NOT NULL DEFAULT FALSE,
			cancellation_reason TEXT DEFAULT '',
			cancelled_at TIMESTAMPTZ,
			entry_type VARCHAR(30) NOT NULL DEFAULT 'class',
			title VARCHAR(200) DEFAULT '',
			assigned_staff TEXT DEFAULT '[]',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS attendance (
			id UUID PRIMARY KEY,
			session_id UUID NOT NULL REFERENCES training_sessions(id) ON DELETE CASCADE,
			student_id UUID REFERENCES students(id) ON DELETE CASCADE,
			student_package_id UUID REFERENCES student_packages(id),
			location_id UUID REFERENCES locations(id),
			session_rate NUMERIC(10, 2),
			checked_in_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			attendee_type VARCHAR(20) NOT NULL DEFAULT 'student',
			attendee_name VARCHAR(120) DEFAULT '',
			attendee_role VARCHAR(50) DEFAULT ''
		);

		CREATE TABLE IF NOT EXISTS student_evaluations (
			id UUID PRIMARY KEY,
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

		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(80) UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL DEFAULT 'STUDENT',
			display_name VARCHAR(120) DEFAULT '',
			profile_picture_url VARCHAR(500) DEFAULT '',
			student_id UUID NULL REFERENCES students(id) ON DELETE SET NULL,
			coach_id UUID NULL REFERENCES coaches(id) ON DELETE SET NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			last_login_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS user_sessions (
			token VARCHAR(255) PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(255) UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS safety_incidents (
			id UUID PRIMARY KEY,
			student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
			coach_id UUID REFERENCES coaches(id) ON DELETE SET NULL,
			incident_type VARCHAR(100) NOT NULL,
			notes TEXT NOT NULL,
			resolved BOOLEAN NOT NULL DEFAULT FALSE,
			resolved_by VARCHAR(255),
			resolved_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY,
			user_id UUID,
			actor_name VARCHAR(150) NOT NULL DEFAULT '',
			actor_email VARCHAR(150) NOT NULL DEFAULT '',
			actor_role VARCHAR(50) NOT NULL DEFAULT '',
			action VARCHAR(100) NOT NULL,
			category VARCHAR(50) NOT NULL,
			target_type VARCHAR(50) DEFAULT '',
			target_id VARCHAR(100) DEFAULT '',
			target_name VARCHAR(150) DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			ip_address VARCHAR(60) DEFAULT '',
			user_agent TEXT DEFAULT '',
			metadata TEXT DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS notification_settings (
			id UUID PRIMARY KEY,
			push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			sms_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_student_admitted BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_new_class_opened BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_staff_registered BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_student_injured BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_class_cancelled BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_promotion_eligible BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_injured BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_evaluation_logged BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_membership_changed BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_safety_resolved BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_class_cancelled BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_pass_expiring BOOLEAN NOT NULL DEFAULT TRUE,
			smtp_host TEXT DEFAULT '',
			smtp_port INT DEFAULT 587,
			smtp_user TEXT DEFAULT '',
			smtp_password TEXT DEFAULT '',
			smtp_from TEXT DEFAULT '',
			sms_provider TEXT DEFAULT 'simulated',
			sms_api_key TEXT DEFAULT '',
			sms_from_number TEXT DEFAULT '',
			web_push_public_key TEXT DEFAULT '',
			web_push_private_key TEXT DEFAULT '',
			web_push_subject TEXT DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS user_notification_preferences (
			user_id UUID PRIMARY KEY,
			email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			sms_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY,
			user_id UUID,
			recipient_role VARCHAR(50) DEFAULT '',
			recipient_type VARCHAR(50) NOT NULL,
			recipient_name VARCHAR(150) NOT NULL,
			recipient_contact VARCHAR(150) DEFAULT '',
			channel VARCHAR(30) NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			title VARCHAR(200) NOT NULL,
			message TEXT NOT NULL,
			metadata TEXT DEFAULT '',
			status VARCHAR(30) NOT NULL,
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`
	}

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Safely guarantee column migrations
	if s.driver == "sqlite" {
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN display_name TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN username TEXT`)
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN profile_picture_url TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username IS NOT NULL AND username != ''`)
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token TEXT UNIQUE NOT NULL,
			expires_at TEXT NOT NULL,
			used_at TEXT,
			created_at TEXT NOT NULL
		)`)
		_, _ = s.db.Exec(`ALTER TABLE students ADD COLUMN has_safety_flag INTEGER DEFAULT 0`)
		_, _ = s.db.Exec(`ALTER TABLE package_templates ADD COLUMN description TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE package_templates ADD COLUMN plan_type TEXT DEFAULT 'standard'`)
		_, _ = s.db.Exec(`ALTER TABLE package_templates ADD COLUMN sessions_per_week INTEGER`)
		_, _ = s.db.Exec(`ALTER TABLE student_packages ADD COLUMN plan_type TEXT DEFAULT 'standard'`)
		_, _ = s.db.Exec(`ALTER TABLE student_packages ADD COLUMN sessions_per_week INTEGER`)
		_, _ = s.db.Exec(`ALTER TABLE student_packages ADD COLUMN custom_price REAL`)
		_, _ = s.db.Exec(`ALTER TABLE student_packages ADD COLUMN notes TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN is_cancelled INTEGER DEFAULT 0`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN cancellation_reason TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN cancelled_at TEXT`)
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS locations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			pin TEXT NOT NULL,
			fixed_rate REAL,
			created_at TEXT NOT NULL
		)`)
		_, _ = s.db.Exec(`ALTER TABLE locations ADD COLUMN fixed_rate REAL`)
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS training_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			color TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN location_id TEXT`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN session_rate REAL`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN entry_type TEXT DEFAULT 'class'`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN title TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN assigned_staff TEXT DEFAULT '[]'`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN location_id TEXT`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN session_rate REAL`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN attendee_type TEXT DEFAULT 'student'`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN attendee_name TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN attendee_role TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			actor_name TEXT NOT NULL DEFAULT '',
			actor_email TEXT NOT NULL DEFAULT '',
			actor_role TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			category TEXT NOT NULL,
			target_type TEXT DEFAULT '',
			target_id TEXT DEFAULT '',
			target_name TEXT DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			ip_address TEXT DEFAULT '',
			user_agent TEXT DEFAULT '',
			metadata TEXT DEFAULT '',
			created_at TEXT NOT NULL
		)`)

		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS notification_settings (
			id TEXT PRIMARY KEY,
			push_enabled INTEGER NOT NULL DEFAULT 1,
			sms_enabled INTEGER NOT NULL DEFAULT 1,
			email_enabled INTEGER NOT NULL DEFAULT 1,
			notify_admin_student_admitted INTEGER NOT NULL DEFAULT 1,
			notify_admin_new_class_opened INTEGER NOT NULL DEFAULT 1,
			notify_admin_staff_registered INTEGER NOT NULL DEFAULT 1,
			notify_admin_student_injured INTEGER NOT NULL DEFAULT 1,
			notify_admin_class_cancelled INTEGER NOT NULL DEFAULT 1,
			notify_student_promotion_eligible INTEGER NOT NULL DEFAULT 1,
			notify_student_injured INTEGER NOT NULL DEFAULT 1,
			notify_student_evaluation_logged INTEGER NOT NULL DEFAULT 1,
			notify_student_membership_changed INTEGER NOT NULL DEFAULT 1,
			notify_student_safety_resolved INTEGER NOT NULL DEFAULT 1,
			notify_student_class_cancelled INTEGER NOT NULL DEFAULT 1,
			notify_student_pass_expiring INTEGER NOT NULL DEFAULT 1,
			smtp_host TEXT DEFAULT '',
			smtp_port INTEGER DEFAULT 587,
			smtp_user TEXT DEFAULT '',
			smtp_password TEXT DEFAULT '',
			smtp_from TEXT DEFAULT '',
			sms_provider TEXT DEFAULT 'simulated',
			sms_api_key TEXT DEFAULT '',
			sms_from_number TEXT DEFAULT '',
			web_push_public_key TEXT DEFAULT '',
			web_push_private_key TEXT DEFAULT '',
			web_push_subject TEXT DEFAULT '',
			updated_at TEXT NOT NULL
		)`)

		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS user_notification_preferences (
			user_id TEXT PRIMARY KEY,
			email_enabled INTEGER NOT NULL DEFAULT 1,
			sms_enabled INTEGER NOT NULL DEFAULT 1,
			push_enabled INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL
		)`)

		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			recipient_role TEXT DEFAULT '',
			recipient_type TEXT NOT NULL,
			recipient_name TEXT NOT NULL,
			recipient_contact TEXT DEFAULT '',
			channel TEXT NOT NULL,
			event_type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			metadata TEXT DEFAULT '',
			status TEXT NOT NULL,
			is_read INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`)

		var notifSettingsCountSQLite int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM notification_settings`).Scan(&notifSettingsCountSQLite); err == nil && notifSettingsCountSQLite == 0 {
			def := models.DefaultNotificationSettings()
			_, _ = s.db.Exec(`INSERT INTO notification_settings (
				id, push_enabled, sms_enabled, email_enabled,
				notify_admin_student_admitted, notify_admin_new_class_opened, notify_admin_staff_registered,
				notify_admin_student_injured, notify_admin_class_cancelled,
				notify_student_promotion_eligible, notify_student_injured, notify_student_evaluation_logged,
				notify_student_membership_changed, notify_student_safety_resolved, notify_student_class_cancelled,
				notify_student_pass_expiring,
				smtp_host, smtp_port, smtp_user, smtp_password, smtp_from,
				sms_provider, sms_api_key, sms_from_number,
				web_push_public_key, web_push_private_key, web_push_subject,
				updated_at
			) VALUES ($1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, '', 587, '', '', '', 'simulated', '', '', '', '', '', $2)`,
				def.ID.String(), formatTimeForDB(time.Now()))
		}

		// Seed default categories if none exist in SQLite
		var catCountSQLite int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM training_categories`).Scan(&catCountSQLite); err == nil && catCountSQLite == 0 {
			now := time.Now()
			defaults := []struct {
				name  string
				color string
			}{
				{"Sparring", "#990303"},
				{"Poomsae", "#4F46E5"},
				{"Conditioning", "#D97706"},
				{"Promotion Prep", "#7C3AED"},
			}
			for _, d := range defaults {
				_, _ = s.db.Exec(`INSERT INTO training_categories (id, name, color, created_at) VALUES ($1, $2, $3, $4)`,
					uuid.New().String(), d.name, d.color, formatTimeForDB(now))
			}
		}

		// Drop NOT NULL on coach_id in training_sessions if table was previously created with NOT NULL
		if rows, err := s.db.Query(`PRAGMA table_info(training_sessions)`); err == nil {
			needsCoachNullable := false
			for rows.Next() {
				var cid, notnull, pk int
				var name, colType string
				var dfltValue sql.NullString
				if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err == nil {
					if name == "coach_id" && notnull == 1 {
						needsCoachNullable = true
					}
				}
			}
			rows.Close()
			if needsCoachNullable {
				migrationSQL := `
					PRAGMA foreign_keys=OFF;
					CREATE TABLE training_sessions_dg_tmp (
						id TEXT PRIMARY KEY,
						session_date TEXT NOT NULL,
						start_time TEXT NOT NULL,
						end_time TEXT NOT NULL,
						coach_id TEXT REFERENCES coaches(id),
						admin_id TEXT,
						location_id TEXT REFERENCES locations(id),
						training_type TEXT NOT NULL,
						notes TEXT,
						is_cancelled INTEGER NOT NULL DEFAULT 0,
						cancellation_reason TEXT DEFAULT '',
						cancelled_at TEXT,
						created_at TEXT NOT NULL
					);
					INSERT INTO training_sessions_dg_tmp SELECT id, session_date, start_time, end_time, coach_id, admin_id, location_id, training_type, notes, is_cancelled, cancellation_reason, cancelled_at, created_at FROM training_sessions;
					DROP TABLE training_sessions;
					ALTER TABLE training_sessions_dg_tmp RENAME TO training_sessions;
					PRAGMA foreign_keys=ON;
				`
				_, _ = s.db.Exec(migrationSQL)
			}
		}
	} else {
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(120) DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS username VARCHAR(80) UNIQUE`)
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture_url VARCHAR(500) DEFAULT ''`)
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(255) UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
		_, _ = s.db.Exec(`ALTER TABLE students ADD COLUMN IF NOT EXISTS has_safety_flag BOOLEAN DEFAULT FALSE`)
		_, _ = s.db.Exec(`ALTER TABLE package_templates ADD COLUMN IF NOT EXISTS description TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE package_templates ADD COLUMN IF NOT EXISTS plan_type VARCHAR(50) DEFAULT 'standard'`)
		_, _ = s.db.Exec(`ALTER TABLE package_templates ADD COLUMN IF NOT EXISTS sessions_per_week INT`)
		_, _ = s.db.Exec(`ALTER TABLE student_packages ADD COLUMN IF NOT EXISTS plan_type VARCHAR(50) DEFAULT 'standard'`)
		_, _ = s.db.Exec(`ALTER TABLE student_packages ADD COLUMN IF NOT EXISTS sessions_per_week INT`)
		_, _ = s.db.Exec(`ALTER TABLE student_packages ADD COLUMN IF NOT EXISTS custom_price NUMERIC(10, 2)`)
		_, _ = s.db.Exec(`ALTER TABLE student_packages ADD COLUMN IF NOT EXISTS notes TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN IF NOT EXISTS is_cancelled BOOLEAN DEFAULT FALSE`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN IF NOT EXISTS cancellation_reason TEXT DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ`)
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS locations (
			id UUID PRIMARY KEY,
			name VARCHAR(150) NOT NULL,
			pin TEXT NOT NULL,
			fixed_rate NUMERIC(10, 2),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
		_, _ = s.db.Exec(`ALTER TABLE locations ADD COLUMN IF NOT EXISTS fixed_rate NUMERIC(10, 2)`)
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS training_categories (
			id UUID PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			color VARCHAR(30) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN IF NOT EXISTS location_id UUID REFERENCES locations(id)`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN IF NOT EXISTS session_rate NUMERIC(10, 2)`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN IF NOT EXISTS entry_type VARCHAR(30) DEFAULT 'class'`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN IF NOT EXISTS title VARCHAR(200) DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ADD COLUMN IF NOT EXISTS assigned_staff TEXT DEFAULT '[]'`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN IF NOT EXISTS location_id UUID REFERENCES locations(id)`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN IF NOT EXISTS session_rate NUMERIC(10, 2)`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ALTER COLUMN student_id DROP NOT NULL`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN IF NOT EXISTS attendee_type VARCHAR(20) DEFAULT 'student'`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN IF NOT EXISTS attendee_name VARCHAR(120) DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE attendance ADD COLUMN IF NOT EXISTS attendee_role VARCHAR(50) DEFAULT ''`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions ALTER COLUMN coach_id DROP NOT NULL`)
		_, _ = s.db.Exec(`ALTER TABLE training_sessions DROP CONSTRAINT IF EXISTS training_sessions_admin_id_fkey`)
		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY,
			user_id UUID,
			actor_name VARCHAR(150) NOT NULL DEFAULT '',
			actor_email VARCHAR(150) NOT NULL DEFAULT '',
			actor_role VARCHAR(50) NOT NULL DEFAULT '',
			action VARCHAR(100) NOT NULL,
			category VARCHAR(50) NOT NULL,
			target_type VARCHAR(50) DEFAULT '',
			target_id VARCHAR(100) DEFAULT '',
			target_name VARCHAR(150) DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			ip_address VARCHAR(60) DEFAULT '',
			user_agent TEXT DEFAULT '',
			metadata TEXT DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)

		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS notification_settings (
			id UUID PRIMARY KEY,
			push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			sms_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_student_admitted BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_new_class_opened BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_staff_registered BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_student_injured BOOLEAN NOT NULL DEFAULT TRUE,
			notify_admin_class_cancelled BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_promotion_eligible BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_injured BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_evaluation_logged BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_membership_changed BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_safety_resolved BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_class_cancelled BOOLEAN NOT NULL DEFAULT TRUE,
			notify_student_pass_expiring BOOLEAN NOT NULL DEFAULT TRUE,
			smtp_host TEXT DEFAULT '',
			smtp_port INT DEFAULT 587,
			smtp_user TEXT DEFAULT '',
			smtp_password TEXT DEFAULT '',
			smtp_from TEXT DEFAULT '',
			sms_provider TEXT DEFAULT 'simulated',
			sms_api_key TEXT DEFAULT '',
			sms_from_number TEXT DEFAULT '',
			web_push_public_key TEXT DEFAULT '',
			web_push_private_key TEXT DEFAULT '',
			web_push_subject TEXT DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)

		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS user_notification_preferences (
			user_id UUID PRIMARY KEY,
			email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			sms_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)

		_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY,
			user_id UUID,
			recipient_role VARCHAR(50) DEFAULT '',
			recipient_type VARCHAR(50) NOT NULL,
			recipient_name VARCHAR(150) NOT NULL,
			recipient_contact VARCHAR(150) DEFAULT '',
			channel VARCHAR(30) NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			title VARCHAR(200) NOT NULL,
			message TEXT NOT NULL,
			metadata TEXT DEFAULT '',
			status VARCHAR(30) NOT NULL,
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)

		var notifSettingsCountPG int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM notification_settings`).Scan(&notifSettingsCountPG); err == nil && notifSettingsCountPG == 0 {
			def := models.DefaultNotificationSettings()
			_, _ = s.db.Exec(`INSERT INTO notification_settings (
				id, push_enabled, sms_enabled, email_enabled,
				notify_admin_student_admitted, notify_admin_new_class_opened, notify_admin_staff_registered,
				notify_admin_student_injured, notify_admin_class_cancelled,
				notify_student_promotion_eligible, notify_student_injured, notify_student_evaluation_logged,
				notify_student_membership_changed, notify_student_safety_resolved, notify_student_class_cancelled,
				notify_student_pass_expiring,
				smtp_host, smtp_port, smtp_user, smtp_password, smtp_from,
				sms_provider, sms_api_key, sms_from_number,
				web_push_public_key, web_push_private_key, web_push_subject,
				updated_at
			) VALUES ($1, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, '', 587, '', '', '', 'simulated', '', '', '', '', '', NOW())`,
				def.ID.String())
		}

		// Seed default categories if none exist in Postgres
		var catCountPG int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM training_categories`).Scan(&catCountPG); err == nil && catCountPG == 0 {
			now := time.Now()
			defaults := []struct {
				name  string
				color string
			}{
				{"Sparring", "#990303"},
				{"Poomsae", "#4F46E5"},
				{"Conditioning", "#D97706"},
				{"Promotion Prep", "#7C3AED"},
			}
			for _, d := range defaults {
				_, _ = s.db.Exec(`INSERT INTO training_categories (id, name, color, created_at) VALUES ($1, $2, $3, $4)`,
					uuid.New().String(), d.name, d.color, formatTimeForDB(now))
			}
		}
	}

	// High-performance operational indexes (SQLite & PostgreSQL compatible)
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_attendance_session_id ON attendance(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_student_id ON attendance(student_id)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_package_id ON attendance(student_package_id)`,
		`CREATE INDEX IF NOT EXISTS idx_student_packages_student_id ON student_packages(student_id)`,
		`CREATE INDEX IF NOT EXISTS idx_training_sessions_date ON training_sessions(session_date)`,
		`CREATE INDEX IF NOT EXISTS idx_training_sessions_coach ON training_sessions(coach_id)`,
		`CREATE INDEX IF NOT EXISTS idx_training_sessions_loc ON training_sessions(location_id)`,
		`CREATE INDEX IF NOT EXISTS idx_evaluations_student_date ON student_evaluations(student_id, evaluation_date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_students_active_name ON students(is_active, full_name)`,
		`CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)`,
		`CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_safety_incidents_resolved ON safety_incidents(resolved)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_category ON audit_logs(category)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)`,
	}
	for _, idx := range indexes {
		_, _ = s.db.Exec(idx)
	}

	return nil
}

// Helpers
func parseTimeFlex(v interface{}) (time.Time, error) {
	if v == nil {
		return time.Time{}, nil
	}
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case string:
		if t == "" {
			return time.Time{}, nil
		}
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05-07:00",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, l := range layouts {
			if parsed, err := time.Parse(l, t); err == nil {
				return parsed, nil
			}
		}
		return time.Time{}, fmt.Errorf("unable to parse time: %s", t)
	default:
		return time.Time{}, fmt.Errorf("unknown time type: %T", v)
	}
}

func formatTimeForDB(t time.Time) string {
	return t.Format(time.RFC3339)
}

func formatDateForDB(t time.Time) string {
	return t.Format("2006-01-02")
}

// Locations
func (s *SQLStore) GetAllLocations() ([]*models.Location, error) {
	query := `SELECT id, name, pin, fixed_rate, created_at FROM locations ORDER BY name ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []*models.Location
	for rows.Next() {
		var idStr, createdStr string
		var nullRate sql.NullFloat64
		loc := &models.Location{}
		if err := rows.Scan(&idStr, &loc.Name, &loc.Pin, &nullRate, &createdStr); err != nil {
			return nil, err
		}
		loc.ID = uuid.Must(uuid.Parse(idStr))
		if nullRate.Valid {
			loc.FixedRate = &nullRate.Float64
		}
		loc.CreatedAt, _ = parseTimeFlex(createdStr)
		locations = append(locations, loc)
	}
	return locations, nil
}

func (s *SQLStore) GetLocationByID(id uuid.UUID) (*models.Location, error) {
	query := `SELECT id, name, pin, fixed_rate, created_at FROM locations WHERE id = $1`
	var idStr, createdStr string
	var nullRate sql.NullFloat64
	loc := &models.Location{}
	err := s.db.QueryRow(query, id.String()).Scan(&idStr, &loc.Name, &loc.Pin, &nullRate, &createdStr)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	loc.ID = uuid.Must(uuid.Parse(idStr))
	if nullRate.Valid {
		loc.FixedRate = &nullRate.Float64
	}
	loc.CreatedAt, _ = parseTimeFlex(createdStr)
	return loc, nil
}

func (s *SQLStore) CreateLocation(loc *models.Location) error {
	if loc.ID == uuid.Nil {
		loc.ID = uuid.New()
	}
	if loc.CreatedAt.IsZero() {
		loc.CreatedAt = time.Now()
	}
	query := `INSERT INTO locations (id, name, pin, fixed_rate, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := s.db.Exec(query, loc.ID.String(), loc.Name, loc.Pin, loc.FixedRate, formatTimeForDB(loc.CreatedAt))
	return err
}

func (s *SQLStore) UpdateLocation(loc *models.Location) error {
	query := `UPDATE locations SET name = $1, pin = $2, fixed_rate = $3 WHERE id = $4`
	res, err := s.db.Exec(query, loc.Name, loc.Pin, loc.FixedRate, loc.ID.String())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) DeleteLocation(id uuid.UUID) error {
	query := `DELETE FROM locations WHERE id = $1`
	res, err := s.db.Exec(query, id.String())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Training Categories
func (s *SQLStore) GetAllTrainingCategories() ([]*models.TrainingCategory, error) {
	query := `SELECT id, name, color, created_at FROM training_categories ORDER BY name ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*models.TrainingCategory
	for rows.Next() {
		var idStr, createdStr string
		cat := &models.TrainingCategory{}
		if err := rows.Scan(&idStr, &cat.Name, &cat.Color, &createdStr); err != nil {
			return nil, err
		}
		cat.ID = uuid.Must(uuid.Parse(idStr))
		cat.CreatedAt, _ = parseTimeFlex(createdStr)
		categories = append(categories, cat)
	}
	return categories, nil
}

func (s *SQLStore) GetTrainingCategoryByID(id uuid.UUID) (*models.TrainingCategory, error) {
	query := `SELECT id, name, color, created_at FROM training_categories WHERE id = $1`
	var idStr, createdStr string
	cat := &models.TrainingCategory{}
	err := s.db.QueryRow(query, id.String()).Scan(&idStr, &cat.Name, &cat.Color, &createdStr)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	cat.ID = uuid.Must(uuid.Parse(idStr))
	cat.CreatedAt, _ = parseTimeFlex(createdStr)
	return cat, nil
}

func (s *SQLStore) GetTrainingCategoryByName(name string) (*models.TrainingCategory, error) {
	query := `SELECT id, name, color, created_at FROM training_categories WHERE LOWER(name) = LOWER($1)`
	var idStr, createdStr string
	cat := &models.TrainingCategory{}
	err := s.db.QueryRow(query, strings.TrimSpace(name)).Scan(&idStr, &cat.Name, &cat.Color, &createdStr)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	cat.ID = uuid.Must(uuid.Parse(idStr))
	cat.CreatedAt, _ = parseTimeFlex(createdStr)
	return cat, nil
}

func (s *SQLStore) CreateTrainingCategory(cat *models.TrainingCategory) error {
	cleanName := strings.TrimSpace(cat.Name)
	if cleanName == "" {
		return errors.New("category name is required")
	}

	var existingCount int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM training_categories WHERE LOWER(name) = LOWER($1)`, cleanName).Scan(&existingCount)
	if err == nil && existingCount > 0 {
		return errors.New("training category with this name already exists")
	}

	if cat.ID == uuid.Nil {
		cat.ID = uuid.New()
	}
	if cat.CreatedAt.IsZero() {
		cat.CreatedAt = time.Now()
	}
	if strings.TrimSpace(cat.Color) == "" {
		cat.Color = "#990303"
	}
	query := `INSERT INTO training_categories (id, name, color, created_at) VALUES ($1, $2, $3, $4)`
	_, err = s.db.Exec(query, cat.ID.String(), cleanName, cat.Color, formatTimeForDB(cat.CreatedAt))
	return err
}

func (s *SQLStore) UpdateTrainingCategory(cat *models.TrainingCategory) error {
	cleanName := strings.TrimSpace(cat.Name)
	if cleanName == "" {
		return errors.New("category name is required")
	}

	// First fetch old name
	var oldName string
	err := s.db.QueryRow(`SELECT name FROM training_categories WHERE id = $1`, cat.ID.String()).Scan(&oldName)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	var dupCount int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM training_categories WHERE LOWER(name) = LOWER($1) AND id != $2`, cleanName, cat.ID.String()).Scan(&dupCount)
	if err == nil && dupCount > 0 {
		return errors.New("training category with this name already exists")
	}

	query := `UPDATE training_categories SET name = $1, color = $2 WHERE id = $3`
	res, err := s.db.Exec(query, cleanName, cat.Color, cat.ID.String())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}

	// Update existing sessions if name changed
	if oldName != cleanName {
		_, _ = s.db.Exec(`UPDATE training_sessions SET training_type = $1 WHERE training_type = $2`, cleanName, oldName)
	}

	return nil
}

func (s *SQLStore) DeleteTrainingCategory(id uuid.UUID) error {
	// Get category name
	var name string
	err := s.db.QueryRow(`SELECT name FROM training_categories WHERE id = $1`, id.String()).Scan(&name)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	// Check if any sessions are using this category
	var count int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM training_sessions WHERE training_type = $1`, name).Scan(&count)
	if err == nil && count > 0 {
		return errors.New("cannot delete training category that is assigned to existing sessions")
	}

	query := `DELETE FROM training_categories WHERE id = $1`
	res, err := s.db.Exec(query, id.String())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Students
func (s *SQLStore) GetAllStudents() ([]*models.Student, error) {
	query := `SELECT id, full_name, dob, gender, phone, current_belt, last_promotion_date,
		emergency_name, emergency_phone, emergency_relation, medical_notes, is_active, created_at,
		COALESCE(has_safety_flag, FALSE)
		FROM students ORDER BY full_name ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []*models.Student
	for rows.Next() {
		var idStr, dobStr, promoStr, createdStr string
		st := &models.Student{}
		var gender, phone, medNotes sql.NullString
		err := rows.Scan(
			&idStr, &st.FullName, &dobStr, &gender, &phone,
			&st.CurrentBelt, &promoStr, &st.EmergencyName, &st.EmergencyPhone,
			&st.EmergencyRelation, &medNotes, &st.IsActive, &createdStr,
			&st.HasSafetyFlag,
		)
		if err != nil {
			return nil, err
		}
		st.ID = uuid.Must(uuid.Parse(idStr))
		st.DOB, _ = parseTimeFlex(dobStr)
		st.LastPromotionDate, _ = parseTimeFlex(promoStr)
		st.CreatedAt, _ = parseTimeFlex(createdStr)
		if gender.Valid {
			st.Gender = gender.String
		}
		if phone.Valid {
			st.Phone = phone.String
		}
		if medNotes.Valid {
			st.MedicalNotes = medNotes.String
		}
		students = append(students, st)
	}
	return students, nil
}

func (s *SQLStore) GetStudentByID(id uuid.UUID) (*models.Student, error) {
	query := `SELECT id, full_name, dob, gender, phone, current_belt, last_promotion_date,
		emergency_name, emergency_phone, emergency_relation, medical_notes, is_active, created_at,
		COALESCE(has_safety_flag, FALSE)
		FROM students WHERE id = $1`
	var idStr, dobStr, promoStr, createdStr string
	st := &models.Student{}
	var gender, phone, medNotes sql.NullString
	err := s.db.QueryRow(query, id.String()).Scan(
		&idStr, &st.FullName, &dobStr, &gender, &phone,
		&st.CurrentBelt, &promoStr, &st.EmergencyName, &st.EmergencyPhone,
		&st.EmergencyRelation, &medNotes, &st.IsActive, &createdStr,
		&st.HasSafetyFlag,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	st.ID = uuid.Must(uuid.Parse(idStr))
	st.DOB, _ = parseTimeFlex(dobStr)
	st.LastPromotionDate, _ = parseTimeFlex(promoStr)
	st.CreatedAt, _ = parseTimeFlex(createdStr)
	if gender.Valid {
		st.Gender = gender.String
	}
	if phone.Valid {
		st.Phone = phone.String
	}
	if medNotes.Valid {
		st.MedicalNotes = medNotes.String
	}
	return st, nil
}

func (s *SQLStore) SearchStudents(query string) ([]*models.Student, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return s.GetAllStudents()
	}
	likePattern := "%" + strings.ToLower(q) + "%"
	sqlQuery := `SELECT id, full_name, dob, gender, phone, current_belt, last_promotion_date,
		emergency_name, emergency_phone, emergency_relation, medical_notes, is_active, created_at,
		COALESCE(has_safety_flag, FALSE)
		FROM students
		WHERE LOWER(full_name) LIKE $1 OR phone LIKE $1 OR LOWER(current_belt) LIKE $1
		ORDER BY full_name ASC`
	rows, err := s.db.Query(sqlQuery, likePattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []*models.Student
	for rows.Next() {
		var idStr, dobStr, promoStr, createdStr string
		st := &models.Student{}
		var gender, phone, medNotes sql.NullString
		err := rows.Scan(
			&idStr, &st.FullName, &dobStr, &gender, &phone,
			&st.CurrentBelt, &promoStr, &st.EmergencyName, &st.EmergencyPhone,
			&st.EmergencyRelation, &medNotes, &st.IsActive, &createdStr,
			&st.HasSafetyFlag,
		)
		if err != nil {
			return nil, err
		}
		st.ID = uuid.Must(uuid.Parse(idStr))
		st.DOB, _ = parseTimeFlex(dobStr)
		st.LastPromotionDate, _ = parseTimeFlex(promoStr)
		st.CreatedAt, _ = parseTimeFlex(createdStr)
		if gender.Valid {
			st.Gender = gender.String
		}
		if phone.Valid {
			st.Phone = phone.String
		}
		if medNotes.Valid {
			st.MedicalNotes = medNotes.String
		}
		students = append(students, st)
	}
	return students, nil
}

func (s *SQLStore) CreateStudent(st *models.Student) error {
	if st.ID == uuid.Nil {
		st.ID = uuid.New()
	}
	if st.CreatedAt.IsZero() {
		st.CreatedAt = time.Now()
	}
	query := `INSERT INTO students (id, full_name, dob, gender, phone, current_belt, last_promotion_date,
		emergency_name, emergency_phone, emergency_relation, medical_notes, is_active, created_at, has_safety_flag)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`
	_, err := s.db.Exec(query,
		st.ID.String(), st.FullName, formatDateForDB(st.DOB), st.Gender, st.Phone,
		string(st.CurrentBelt), formatDateForDB(st.LastPromotionDate), st.EmergencyName,
		st.EmergencyPhone, st.EmergencyRelation, st.MedicalNotes, st.IsActive, formatTimeForDB(st.CreatedAt),
		st.HasSafetyFlag,
	)
	return err
}

func (s *SQLStore) UpdateStudent(st *models.Student) error {
	query := `UPDATE students SET full_name = $1, dob = $2, gender = $3, phone = $4,
		current_belt = $5, last_promotion_date = $6, emergency_name = $7, emergency_phone = $8,
		emergency_relation = $9, medical_notes = $10, is_active = $11, has_safety_flag = $12
		WHERE id = $13`
	res, err := s.db.Exec(query,
		st.FullName, formatDateForDB(st.DOB), st.Gender, st.Phone,
		string(st.CurrentBelt), formatDateForDB(st.LastPromotionDate), st.EmergencyName,
		st.EmergencyPhone, st.EmergencyRelation, st.MedicalNotes, st.IsActive, st.HasSafetyFlag, st.ID.String(),
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) DeleteStudent(studentID uuid.UUID) error {
	var sid string
	err := s.db.QueryRow(`SELECT id FROM students WHERE id = $1`, studentID.String()).Scan(&sid)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Purge associated user sessions, password reset tokens and user records
	rows, err := tx.Query(`SELECT id FROM users WHERE student_id = $1`, studentID.String())
	if err == nil {
		var uids []string
		for rows.Next() {
			var uid string
			if scanErr := rows.Scan(&uid); scanErr == nil {
				uids = append(uids, uid)
			}
		}
		rows.Close()
		for _, uid := range uids {
			_, _ = tx.Exec(`DELETE FROM password_reset_tokens WHERE user_id = $1`, uid)
			_, _ = tx.Exec(`DELETE FROM user_sessions WHERE user_id = $1`, uid)
			_, _ = tx.Exec(`DELETE FROM users WHERE id = $1`, uid)
		}
	}

	// 2. Cascade delete safety incidents
	if _, err := tx.Exec(`DELETE FROM safety_incidents WHERE student_id = $1`, studentID.String()); err != nil {
		return err
	}

	// 3. Cascade delete student evaluations
	if _, err := tx.Exec(`DELETE FROM student_evaluations WHERE student_id = $1`, studentID.String()); err != nil {
		return err
	}

	// 4. Cascade delete attendance
	if _, err := tx.Exec(`DELETE FROM attendance WHERE student_id = $1`, studentID.String()); err != nil {
		return err
	}

	// 5. Cascade delete student packages
	if _, err := tx.Exec(`DELETE FROM student_packages WHERE student_id = $1`, studentID.String()); err != nil {
		return err
	}

	// 6. Delete student record
	res, err := tx.Exec(`DELETE FROM students WHERE id = $1`, studentID.String())
	if err != nil {
		return err
	}
	rAffected, _ := res.RowsAffected()
	if rAffected == 0 {
		return ErrNotFound
	}

	return tx.Commit()
}

// Coaches
func (s *SQLStore) GetAllCoaches() ([]*models.Coach, error) {
	query := `SELECT id, full_name, email, phone, belt_rank, rate_per_session, first_aid_certified,
		first_aid_expiry, specialties, is_active, created_at
		FROM coaches ORDER BY full_name ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var coaches []*models.Coach
	for rows.Next() {
		var idStr, createdStr string
		var expiryStr, specStr sql.NullString
		c := &models.Coach{}
		err := rows.Scan(
			&idStr, &c.FullName, &c.Email, &c.Phone, &c.BeltRank,
			&c.RatePerSession, &c.FirstAidCertified, &expiryStr,
			&specStr, &c.IsActive, &createdStr,
		)
		if err != nil {
			return nil, err
		}
		c.ID = uuid.Must(uuid.Parse(idStr))
		c.CreatedAt, _ = parseTimeFlex(createdStr)
		if expiryStr.Valid && expiryStr.String != "" {
			t, _ := parseTimeFlex(expiryStr.String)
			c.FirstAidExpiry = &t
		}
		if specStr.Valid && specStr.String != "" {
			_ = json.Unmarshal([]byte(specStr.String), &c.Specialties)
		}
		coaches = append(coaches, c)
	}
	return coaches, nil
}

func (s *SQLStore) GetCoachByID(id uuid.UUID) (*models.Coach, error) {
	query := `SELECT id, full_name, email, phone, belt_rank, rate_per_session, first_aid_certified,
		first_aid_expiry, specialties, is_active, created_at
		FROM coaches WHERE id = $1`
	var idStr, createdStr string
	var expiryStr, specStr sql.NullString
	c := &models.Coach{}
	err := s.db.QueryRow(query, id.String()).Scan(
		&idStr, &c.FullName, &c.Email, &c.Phone, &c.BeltRank,
		&c.RatePerSession, &c.FirstAidCertified, &expiryStr,
		&specStr, &c.IsActive, &createdStr,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.ID = uuid.Must(uuid.Parse(idStr))
	c.CreatedAt, _ = parseTimeFlex(createdStr)
	if expiryStr.Valid && expiryStr.String != "" {
		t, _ := parseTimeFlex(expiryStr.String)
		c.FirstAidExpiry = &t
	}
	if specStr.Valid && specStr.String != "" {
		_ = json.Unmarshal([]byte(specStr.String), &c.Specialties)
	}
	return c, nil
}

func (s *SQLStore) CreateCoach(c *models.Coach) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	var expiryVal interface{}
	if c.FirstAidExpiry != nil {
		expiryVal = formatDateForDB(*c.FirstAidExpiry)
	}
	specBytes, _ := json.Marshal(c.Specialties)

	query := `INSERT INTO coaches (id, full_name, email, phone, belt_rank, rate_per_session,
		first_aid_certified, first_aid_expiry, specialties, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := s.db.Exec(query,
		c.ID.String(), c.FullName, c.Email, c.Phone, c.BeltRank, c.RatePerSession,
		c.FirstAidCertified, expiryVal, string(specBytes), c.IsActive, formatTimeForDB(c.CreatedAt),
	)
	return err
}

func (s *SQLStore) UpdateCoach(c *models.Coach) error {
	var expiryVal interface{}
	if c.FirstAidExpiry != nil {
		expiryVal = formatDateForDB(*c.FirstAidExpiry)
	}
	specBytes, _ := json.Marshal(c.Specialties)
	query := `UPDATE coaches SET full_name = $1, phone = $2, belt_rank = $3,
		rate_per_session = $4, first_aid_certified = $5, first_aid_expiry = $6,
		specialties = $7, is_active = $8 WHERE id = $9`
	res, err := s.db.Exec(query,
		c.FullName, c.Phone, c.BeltRank, c.RatePerSession,
		c.FirstAidCertified, expiryVal, string(specBytes), c.IsActive, c.ID.String(),
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) ToggleCoachActive(coachID uuid.UUID, isActive bool) error {
	res, err := s.db.Exec(`UPDATE coaches SET is_active = $1 WHERE id = $2`, isActive, coachID.String())
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	var coachEmail string
	_ = s.db.QueryRow(`SELECT email FROM coaches WHERE id = $1`, coachID.String()).Scan(&coachEmail)

	nowStr := formatTimeForDB(time.Now())
	_, err = s.db.Exec(`UPDATE users SET is_active = $1, updated_at = $2 WHERE coach_id = $3 OR (role = 'COACH' AND LOWER(email) = LOWER($4))`,
		isActive, nowStr, coachID.String(), coachEmail)
	if err != nil {
		return err
	}

	if !isActive {
		query := `DELETE FROM user_sessions WHERE user_id IN (
			SELECT id FROM users WHERE coach_id = $1 OR (role = 'COACH' AND LOWER(email) = LOWER($2))
		)`
		_, _ = s.db.Exec(query, coachID.String(), coachEmail)
	}

	return nil
}

func (s *SQLStore) DeleteCoach(coachID uuid.UUID) error {
	var coachEmail string
	err := s.db.QueryRow(`SELECT email FROM coaches WHERE id = $1`, coachID.String()).Scan(&coachEmail)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Purge associated user sessions and user records
	rows, err := tx.Query(`SELECT id FROM users WHERE coach_id = $1 OR (role = 'COACH' AND LOWER(email) = LOWER($2))`, coachID.String(), coachEmail)
	if err == nil {
		var uids []string
		for rows.Next() {
			var uid string
			if scanErr := rows.Scan(&uid); scanErr == nil {
				uids = append(uids, uid)
			}
		}
		rows.Close()
		for _, uid := range uids {
			_, _ = tx.Exec(`DELETE FROM user_sessions WHERE user_id = $1`, uid)
			_, _ = tx.Exec(`DELETE FROM users WHERE id = $1`, uid)
		}
	}

	// 2. Cascade delete safety incidents logged by or assigned to this coach
	_, _ = tx.Exec(`DELETE FROM safety_incidents WHERE coach_id = $1`, coachID.String())

	// 3. Cascade delete student evaluations authored by this coach
	_, _ = tx.Exec(`DELETE FROM student_evaluations WHERE coach_id = $1`, coachID.String())

	// 4. Nullify supervising admin reference in training sessions
	_, _ = tx.Exec(`UPDATE training_sessions SET admin_id = NULL WHERE admin_id = $1`, coachID.String())

	// 5. Cascade delete attendance for sessions led by this coach, then delete the sessions
	_, _ = tx.Exec(`DELETE FROM attendance WHERE session_id IN (SELECT id FROM training_sessions WHERE coach_id = $1)`, coachID.String())
	_, _ = tx.Exec(`DELETE FROM training_sessions WHERE coach_id = $1`, coachID.String())

	// 6. Delete coach record
	res, err := tx.Exec(`DELETE FROM coaches WHERE id = $1`, coachID.String())
	if err != nil {
		return err
	}
	rAffected, _ := res.RowsAffected()
	if rAffected == 0 {
		return ErrNotFound
	}

	return tx.Commit()
}


// Packages
func (s *SQLStore) GetPackageTemplates() ([]*models.PackageTemplate, error) {
	query := `SELECT id, title, COALESCE(description, ''), COALESCE(plan_type, 'standard'), session_count, sessions_per_week, validity_days, price, is_active
		FROM package_templates ORDER BY is_active DESC, price ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*models.PackageTemplate
	for rows.Next() {
		var idStr, desc, planTypeStr string
		var count, spw sql.NullInt64
		pt := &models.PackageTemplate{}
		err := rows.Scan(&idStr, &pt.Title, &desc, &planTypeStr, &count, &spw, &pt.ValidityDays, &pt.Price, &pt.IsActive)
		if err != nil {
			return nil, err
		}
		pt.ID = uuid.Must(uuid.Parse(idStr))
		pt.Description = desc
		if planTypeStr == "" {
			if count.Valid {
				planTypeStr = "standard"
			} else {
				planTypeStr = "unlimited"
			}
		}
		pt.PlanType = models.PlanType(planTypeStr)
		if count.Valid {
			c := int(count.Int64)
			pt.SessionCount = &c
		}
		if spw.Valid {
			w := int(spw.Int64)
			pt.SessionsPerWeek = &w
		}
		templates = append(templates, pt)
	}
	return templates, nil
}

func (s *SQLStore) GetPackageTemplateByID(id uuid.UUID) (*models.PackageTemplate, error) {
	query := `SELECT id, title, COALESCE(description, ''), COALESCE(plan_type, 'standard'), session_count, sessions_per_week, validity_days, price, is_active
		FROM package_templates WHERE id = $1`
	var idStr, desc, planTypeStr string
	var count, spw sql.NullInt64
	pt := &models.PackageTemplate{}
	err := s.db.QueryRow(query, id.String()).Scan(&idStr, &pt.Title, &desc, &planTypeStr, &count, &spw, &pt.ValidityDays, &pt.Price, &pt.IsActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	pt.ID = uuid.Must(uuid.Parse(idStr))
	pt.Description = desc
	if planTypeStr == "" {
		if count.Valid {
			planTypeStr = "standard"
		} else {
			planTypeStr = "unlimited"
		}
	}
	pt.PlanType = models.PlanType(planTypeStr)
	if count.Valid {
		c := int(count.Int64)
		pt.SessionCount = &c
	}
	if spw.Valid {
		w := int(spw.Int64)
		pt.SessionsPerWeek = &w
	}
	return pt, nil
}

func (s *SQLStore) CreatePackageTemplate(tpl *models.PackageTemplate) error {
	if tpl.ID == uuid.Nil {
		tpl.ID = uuid.New()
	}
	if tpl.PlanType == "" {
		if tpl.SessionCount == nil {
			tpl.PlanType = models.PlanTypeUnlimited
		} else {
			tpl.PlanType = models.PlanTypeStandard
		}
	}
	query := `INSERT INTO package_templates (id, title, description, plan_type, session_count, sessions_per_week, validity_days, price, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := s.db.Exec(query, tpl.ID.String(), tpl.Title, tpl.Description, string(tpl.PlanType), tpl.SessionCount, tpl.SessionsPerWeek, tpl.ValidityDays, tpl.Price, tpl.IsActive)
	return err
}

func (s *SQLStore) UpdatePackageTemplate(tpl *models.PackageTemplate) error {
	if tpl.PlanType == "" {
		if tpl.SessionCount == nil {
			tpl.PlanType = models.PlanTypeUnlimited
		} else {
			tpl.PlanType = models.PlanTypeStandard
		}
	}
	query := `UPDATE package_templates SET title = $1, description = $2, plan_type = $3, session_count = $4, sessions_per_week = $5, validity_days = $6, price = $7, is_active = $8
		WHERE id = $9`
	res, err := s.db.Exec(query, tpl.Title, tpl.Description, string(tpl.PlanType), tpl.SessionCount, tpl.SessionsPerWeek, tpl.ValidityDays, tpl.Price, tpl.IsActive, tpl.ID.String())
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) TogglePackageTemplateStatus(id uuid.UUID, isActive bool) error {
	query := `UPDATE package_templates SET is_active = $1 WHERE id = $2`
	res, err := s.db.Exec(query, isActive, id.String())
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) GetStudentPackages(studentID uuid.UUID) ([]*models.StudentPackage, error) {
	query := `SELECT sp.id, sp.student_id, sp.template_id, pt.title, COALESCE(sp.plan_type, 'standard'), sp.total_sessions,
		sp.remaining_sessions, sp.sessions_per_week, sp.custom_price, COALESCE(sp.notes, ''), sp.purchase_date, sp.expiry_date, sp.payment_status, sp.created_at
		FROM student_packages sp
		LEFT JOIN package_templates pt ON sp.template_id = pt.id
		WHERE sp.student_id = $1
		ORDER BY sp.purchase_date ASC`
	rows, err := s.db.Query(query, studentID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pkgs []*models.StudentPackage
	for rows.Next() {
		var idStr, stIDStr, tmplIDStr, purchStr, expStr, createdStr, notesStr string
		var titleStr, planTypeStr sql.NullString
		var total, rem, spw sql.NullInt64
		var customPrice sql.NullFloat64
		sp := &models.StudentPackage{}
		err := rows.Scan(
			&idStr, &stIDStr, &tmplIDStr, &titleStr, &planTypeStr, &total,
			&rem, &spw, &customPrice, &notesStr, &purchStr, &expStr, &sp.PaymentStatus, &createdStr,
		)
		if err != nil {
			return nil, err
		}
		sp.ID = uuid.Must(uuid.Parse(idStr))
		sp.StudentID = uuid.Must(uuid.Parse(stIDStr))
		sp.TemplateID = uuid.Must(uuid.Parse(tmplIDStr))
		if titleStr.Valid {
			sp.TemplateTitle = titleStr.String
		}
		pType := "standard"
		if planTypeStr.Valid && planTypeStr.String != "" {
			pType = planTypeStr.String
		} else if !total.Valid {
			pType = "unlimited"
		}
		sp.PlanType = models.PlanType(pType)
		sp.Notes = notesStr
		if customPrice.Valid {
			cp := customPrice.Float64
			sp.CustomPrice = &cp
		}
		if total.Valid {
			t := int(total.Int64)
			sp.TotalSessions = &t
		}
		if rem.Valid {
			r := int(rem.Int64)
			sp.RemainingSessions = &r
		}
		if spw.Valid {
			w := int(spw.Int64)
			sp.SessionsPerWeek = &w
		}
		sp.PurchaseDate, _ = parseTimeFlex(purchStr)
		sp.ExpiryDate, _ = parseTimeFlex(expStr)
		sp.CreatedAt, _ = parseTimeFlex(createdStr)
		pkgs = append(pkgs, sp)
	}
	return pkgs, nil
}

func (s *SQLStore) GetAllStudentPackagesGrouped() (map[uuid.UUID][]*models.StudentPackage, error) {
	query := `SELECT sp.id, sp.student_id, sp.template_id, pt.title, COALESCE(sp.plan_type, 'standard'), sp.total_sessions,
		sp.remaining_sessions, sp.sessions_per_week, sp.custom_price, COALESCE(sp.notes, ''), sp.purchase_date, sp.expiry_date, sp.payment_status, sp.created_at
		FROM student_packages sp
		LEFT JOIN package_templates pt ON sp.template_id = pt.id
		ORDER BY sp.purchase_date ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]*models.StudentPackage)
	for rows.Next() {
		var idStr, stIDStr, tmplIDStr, purchStr, expStr, createdStr, notesStr string
		var titleStr, planTypeStr sql.NullString
		var total, rem, spw sql.NullInt64
		var customPrice sql.NullFloat64
		sp := &models.StudentPackage{}
		err := rows.Scan(
			&idStr, &stIDStr, &tmplIDStr, &titleStr, &planTypeStr, &total,
			&rem, &spw, &customPrice, &notesStr, &purchStr, &expStr, &sp.PaymentStatus, &createdStr,
		)
		if err != nil {
			return nil, err
		}
		sp.ID = uuid.Must(uuid.Parse(idStr))
		sp.StudentID = uuid.Must(uuid.Parse(stIDStr))
		sp.TemplateID = uuid.Must(uuid.Parse(tmplIDStr))
		if titleStr.Valid {
			sp.TemplateTitle = titleStr.String
		}
		pType := "standard"
		if planTypeStr.Valid && planTypeStr.String != "" {
			pType = planTypeStr.String
		} else if !total.Valid {
			pType = "unlimited"
		}
		sp.PlanType = models.PlanType(pType)
		sp.Notes = notesStr
		if customPrice.Valid {
			cp := customPrice.Float64
			sp.CustomPrice = &cp
		}
		if total.Valid {
			t := int(total.Int64)
			sp.TotalSessions = &t
		}
		if rem.Valid {
			r := int(rem.Int64)
			sp.RemainingSessions = &r
		}
		if spw.Valid {
			w := int(spw.Int64)
			sp.SessionsPerWeek = &w
		}
		sp.PurchaseDate, _ = parseTimeFlex(purchStr)
		sp.ExpiryDate, _ = parseTimeFlex(expStr)
		sp.CreatedAt, _ = parseTimeFlex(createdStr)
		result[sp.StudentID] = append(result[sp.StudentID], sp)
	}
	return result, nil
}

func (s *SQLStore) AssignPackage(pkg *models.StudentPackage) error {
	if pkg.ID == uuid.Nil {
		pkg.ID = uuid.New()
	}
	if pkg.CreatedAt.IsZero() {
		pkg.CreatedAt = time.Now()
	}
	if pkg.PlanType == "" {
		if pkg.TotalSessions == nil {
			pkg.PlanType = models.PlanTypeUnlimited
		} else {
			pkg.PlanType = models.PlanTypeStandard
		}
	}
	query := `INSERT INTO student_packages (id, student_id, template_id, plan_type, total_sessions,
		remaining_sessions, sessions_per_week, custom_price, notes, purchase_date, expiry_date, payment_status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	_, err := s.db.Exec(query,
		pkg.ID.String(), pkg.StudentID.String(), pkg.TemplateID.String(), string(pkg.PlanType),
		pkg.TotalSessions, pkg.RemainingSessions, pkg.SessionsPerWeek, pkg.CustomPrice, pkg.Notes, formatDateForDB(pkg.PurchaseDate),
		formatDateForDB(pkg.ExpiryDate), pkg.PaymentStatus, formatTimeForDB(pkg.CreatedAt),
	)
	return err
}

func (s *SQLStore) GetStudentPackageByID(id uuid.UUID) (*models.StudentPackage, error) {
	query := `SELECT sp.id, sp.student_id, sp.template_id, pt.title, COALESCE(sp.plan_type, 'standard'), sp.total_sessions,
		sp.remaining_sessions, sp.sessions_per_week, sp.custom_price, COALESCE(sp.notes, ''), sp.purchase_date, sp.expiry_date, sp.payment_status, sp.created_at
		FROM student_packages sp
		LEFT JOIN package_templates pt ON sp.template_id = pt.id
		WHERE sp.id = $1`
	var idStr, stIDStr, tmplIDStr, purchStr, expStr, createdStr, notesStr string
	var titleStr, planTypeStr sql.NullString
	var total, rem, spw sql.NullInt64
	var customPrice sql.NullFloat64
	sp := &models.StudentPackage{}
	err := s.db.QueryRow(query, id.String()).Scan(
		&idStr, &stIDStr, &tmplIDStr, &titleStr, &planTypeStr, &total,
		&rem, &spw, &customPrice, &notesStr, &purchStr, &expStr, &sp.PaymentStatus, &createdStr,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sp.ID = uuid.Must(uuid.Parse(idStr))
	sp.StudentID = uuid.Must(uuid.Parse(stIDStr))
	sp.TemplateID = uuid.Must(uuid.Parse(tmplIDStr))
	if titleStr.Valid {
		sp.TemplateTitle = titleStr.String
	}
	pType := "standard"
	if planTypeStr.Valid && planTypeStr.String != "" {
		pType = planTypeStr.String
	} else if !total.Valid {
		pType = "unlimited"
	}
	sp.PlanType = models.PlanType(pType)
	sp.Notes = notesStr
	if customPrice.Valid {
		cp := customPrice.Float64
		sp.CustomPrice = &cp
	}
	if total.Valid {
		tot := int(total.Int64)
		sp.TotalSessions = &tot
	}
	if rem.Valid {
		r := int(rem.Int64)
		sp.RemainingSessions = &r
	}
	if spw.Valid {
		w := int(spw.Int64)
		sp.SessionsPerWeek = &w
	}
	sp.PurchaseDate, _ = parseTimeFlex(purchStr)
	sp.ExpiryDate, _ = parseTimeFlex(expStr)
	sp.CreatedAt, _ = parseTimeFlex(createdStr)
	return sp, nil
}

func (s *SQLStore) UpdateStudentPackage(pkg *models.StudentPackage) error {
	query := `UPDATE student_packages SET remaining_sessions = $1, payment_status = $2 WHERE id = $3`
	res, err := s.db.Exec(query, pkg.RemainingSessions, pkg.PaymentStatus, pkg.ID.String())
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) RevokeStudentPackage(id uuid.UUID) error {
	query := `UPDATE student_packages SET payment_status = 'revoked' WHERE id = $1`
	res, err := s.db.Exec(query, id.String())
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Training Sessions
func (s *SQLStore) scanSession(scan func(...interface{}) error) (*models.TrainingSession, error) {
	var idStr, sessDateStr, createdStr string
	var coachIDStr, coachName, adminIDStr, notes, cancelReason, cancelledAtStr, adminName sql.NullString
	var locationIDStr, locationName, locationPin sql.NullString
	var isCancelled sql.NullBool
	var sessionRateVal sql.NullFloat64
	var entryTypeStr, titleStr, assignedStaffStr sql.NullString
	ts := &models.TrainingSession{}
	err := scan(
		&idStr, &sessDateStr, &ts.StartTime, &ts.EndTime, &coachIDStr,
		&coachName, &adminIDStr, &ts.TrainingType, &notes, &createdStr,
		&isCancelled, &cancelReason, &cancelledAtStr, &adminName,
		&locationIDStr, &locationName, &locationPin,
		&sessionRateVal,
		&entryTypeStr, &titleStr, &assignedStaffStr,
	)
	if err != nil {
		return nil, err
	}
	ts.ID = uuid.Must(uuid.Parse(idStr))
	if coachIDStr.Valid && coachIDStr.String != "" {
		cid, err := uuid.Parse(coachIDStr.String)
		if err == nil && cid != uuid.Nil {
			ts.CoachID = &cid
		}
	}
	ts.SessionDate, _ = parseTimeFlex(sessDateStr)
	ts.CreatedAt, _ = parseTimeFlex(createdStr)
	if sessionRateVal.Valid {
		r := sessionRateVal.Float64
		ts.SessionRate = &r
	}
	if coachName.Valid {
		ts.CoachName = coachName.String
	}
	if adminIDStr.Valid && adminIDStr.String != "" {
		adm := uuid.Must(uuid.Parse(adminIDStr.String))
		ts.AdminID = &adm
	}
	if adminName.Valid {
		ts.AdminName = adminName.String
	}
	if locationIDStr.Valid && locationIDStr.String != "" {
		locID := uuid.Must(uuid.Parse(locationIDStr.String))
		ts.LocationID = &locID
	}
	if locationName.Valid {
		ts.LocationName = locationName.String
	}
	if locationPin.Valid {
		ts.LocationPin = locationPin.String
	}
	if notes.Valid {
		ts.Notes = notes.String
	}
	if isCancelled.Valid {
		ts.IsCancelled = isCancelled.Bool
	}
	if cancelReason.Valid {
		ts.CancellationReason = cancelReason.String
	}
	if cancelledAtStr.Valid && cancelledAtStr.String != "" {
		t, err := parseTimeFlex(cancelledAtStr.String)
		if err == nil && !t.IsZero() {
			ts.CancelledAt = &t
		}
	}
	if entryTypeStr.Valid && entryTypeStr.String != "" {
		ts.EntryType = models.EntryType(entryTypeStr.String)
	} else {
		ts.EntryType = models.EntryTypeClass
	}
	if titleStr.Valid {
		ts.Title = titleStr.String
	}
	if assignedStaffStr.Valid && assignedStaffStr.String != "" {
		_ = json.Unmarshal([]byte(assignedStaffStr.String), &ts.AssignedStaff)
	}
	return ts, nil
}

func (s *SQLStore) GetAllSessions() ([]*models.TrainingSession, error) {
	return s.GetSessions(SessionFilter{})
}

func (s *SQLStore) GetSessions(filter SessionFilter) ([]*models.TrainingSession, error) {
	query := `SELECT ts.id, ts.session_date, ts.start_time, ts.end_time, ts.coach_id,
		c.full_name, ts.admin_id, ts.training_type, ts.notes, ts.created_at,
		COALESCE(ts.is_cancelled, FALSE), COALESCE(ts.cancellation_reason, ''), ts.cancelled_at,
		COALESCE(u.display_name, u.email, ca.full_name, ''),
		ts.location_id, COALESCE(loc.name, ''), COALESCE(loc.pin, ''),
		ts.session_rate,
		COALESCE(ts.entry_type, 'class'), COALESCE(ts.title, ''), COALESCE(ts.assigned_staff, '[]')
		FROM training_sessions ts
		LEFT JOIN coaches c ON ts.coach_id = c.id
		LEFT JOIN users u ON ts.admin_id = u.id
		LEFT JOIN coaches ca ON ts.admin_id = ca.id
		LEFT JOIN locations loc ON ts.location_id = loc.id
		WHERE 1=1`

	var args []interface{}
	idx := 1

	if filter.CoachID != nil {
		query += fmt.Sprintf(" AND (ts.coach_id = $%d OR ts.assigned_staff LIKE '%%' || $%d || '%%')", idx, idx)
		args = append(args, filter.CoachID.String())
		idx++
	}
	if filter.LocationID != nil {
		query += fmt.Sprintf(" AND ts.location_id = $%d", idx)
		args = append(args, filter.LocationID.String())
		idx++
	}
	if filter.StudentID != nil {
		query += fmt.Sprintf(" AND ts.id IN (SELECT session_id FROM attendance WHERE student_id = $%d)", idx)
		args = append(args, filter.StudentID.String())
		idx++
	}
	if filter.TrainingType != "" {
		query += fmt.Sprintf(" AND ts.training_type = $%d", idx)
		args = append(args, filter.TrainingType)
		idx++
	}
	if filter.EntryType != "" {
		if filter.EntryType == "class" {
			query += " AND (ts.entry_type = 'class' OR ts.entry_type = '' OR ts.entry_type IS NULL)"
		} else {
			query += fmt.Sprintf(" AND ts.entry_type = $%d", idx)
			args = append(args, filter.EntryType)
			idx++
		}
	}
	if filter.Date != "" {
		if s.driver == "sqlite" {
			query += fmt.Sprintf(" AND (DATE(ts.session_date) = $%d OR substr(ts.session_date, 1, 10) = $%d)", idx, idx)
		} else {
			query += fmt.Sprintf(" AND DATE(ts.session_date) = $%d::date", idx)
		}
		args = append(args, filter.Date)
		idx++
	} else {
		if filter.StartDate != "" {
			if s.driver == "sqlite" {
				query += fmt.Sprintf(" AND (DATE(ts.session_date) >= $%d OR substr(ts.session_date, 1, 10) >= $%d)", idx, idx)
			} else {
				query += fmt.Sprintf(" AND DATE(ts.session_date) >= $%d::date", idx)
			}
			args = append(args, filter.StartDate)
			idx++
		}
		if filter.EndDate != "" {
			if s.driver == "sqlite" {
				query += fmt.Sprintf(" AND (DATE(ts.session_date) <= $%d OR substr(ts.session_date, 1, 10) <= $%d)", idx, idx)
			} else {
				query += fmt.Sprintf(" AND DATE(ts.session_date) <= $%d::date", idx)
			}
			args = append(args, filter.EndDate)
			idx++
		}
	}

	query += " ORDER BY ts.session_date ASC, ts.start_time ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.TrainingSession
	for rows.Next() {
		ts, err := s.scanSession(rows.Scan)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, ts)
	}
	return sessions, nil
}

func (s *SQLStore) GetSessionByID(id uuid.UUID) (*models.TrainingSession, error) {
	query := `SELECT ts.id, ts.session_date, ts.start_time, ts.end_time, ts.coach_id,
		c.full_name, ts.admin_id, ts.training_type, ts.notes, ts.created_at,
		COALESCE(ts.is_cancelled, FALSE), COALESCE(ts.cancellation_reason, ''), ts.cancelled_at,
		COALESCE(u.display_name, u.email, ca.full_name, ''),
		ts.location_id, COALESCE(loc.name, ''), COALESCE(loc.pin, ''),
		ts.session_rate,
		COALESCE(ts.entry_type, 'class'), COALESCE(ts.title, ''), COALESCE(ts.assigned_staff, '[]')
		FROM training_sessions ts
		LEFT JOIN coaches c ON ts.coach_id = c.id
		LEFT JOIN users u ON ts.admin_id = u.id
		LEFT JOIN coaches ca ON ts.admin_id = ca.id
		LEFT JOIN locations loc ON ts.location_id = loc.id
		WHERE ts.id = $1`
	ts, err := s.scanSession(s.db.QueryRow(query, id.String()).Scan)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return ts, nil
}

func (s *SQLStore) CreateSession(sess *models.TrainingSession) error {
	if sess.ID == uuid.Nil {
		sess.ID = uuid.New()
	}
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now()
	}
	var coachIDVal interface{}
	if sess.CoachID != nil && *sess.CoachID != uuid.Nil {
		coachIDVal = sess.CoachID.String()
	}
	var adminIDVal interface{}
	if sess.AdminID != nil && *sess.AdminID != uuid.Nil {
		adminIDVal = sess.AdminID.String()
	}
	var locationIDVal interface{}
	if sess.LocationID != nil && *sess.LocationID != uuid.Nil {
		locationIDVal = sess.LocationID.String()
	}
	var sessionRateVal interface{}
	if sess.SessionRate != nil {
		sessionRateVal = *sess.SessionRate
	}
	var cancelledAtVal interface{}
	if sess.CancelledAt != nil {
		cancelledAtVal = formatTimeForDB(*sess.CancelledAt)
	}
	var assignedStaffJSON string
	if len(sess.AssignedStaff) > 0 {
		b, _ := json.Marshal(sess.AssignedStaff)
		assignedStaffJSON = string(b)
	} else {
		assignedStaffJSON = "[]"
	}
	entryTypeVal := sess.EntryTypeVal()

	query := `INSERT INTO training_sessions (id, session_date, start_time, end_time, coach_id,
		admin_id, location_id, session_rate, training_type, notes, is_cancelled, cancellation_reason, cancelled_at, created_at,
		entry_type, title, assigned_staff)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`
	_, err := s.db.Exec(query,
		sess.ID.String(), formatDateForDB(sess.SessionDate), sess.StartTime, sess.EndTime,
		coachIDVal, adminIDVal, locationIDVal, sessionRateVal, string(sess.TrainingType), sess.Notes,
		sess.IsCancelled, sess.CancellationReason, cancelledAtVal, formatTimeForDB(sess.CreatedAt),
		entryTypeVal, sess.Title, assignedStaffJSON,
	)
	return err
}

func (s *SQLStore) UpdateSession(sess *models.TrainingSession) error {
	var coachIDVal interface{}
	if sess.CoachID != nil && *sess.CoachID != uuid.Nil {
		coachIDVal = sess.CoachID.String()
	}
	var adminIDVal interface{}
	if sess.AdminID != nil && *sess.AdminID != uuid.Nil {
		adminIDVal = sess.AdminID.String()
	}
	var locationIDVal interface{}
	if sess.LocationID != nil && *sess.LocationID != uuid.Nil {
		locationIDVal = sess.LocationID.String()
	}
	var sessionRateVal interface{}
	if sess.SessionRate != nil {
		sessionRateVal = *sess.SessionRate
	}
	var assignedStaffJSON string
	if len(sess.AssignedStaff) > 0 {
		b, _ := json.Marshal(sess.AssignedStaff)
		assignedStaffJSON = string(b)
	} else {
		assignedStaffJSON = "[]"
	}
	entryTypeVal := sess.EntryTypeVal()

	query := `UPDATE training_sessions 
		SET session_date = $1, start_time = $2, end_time = $3, coach_id = $4,
			admin_id = $5, location_id = $6, session_rate = $7, training_type = $8, notes = $9,
			entry_type = $10, title = $11, assigned_staff = $12
		WHERE id = $13`
	res, err := s.db.Exec(query,
		formatDateForDB(sess.SessionDate), sess.StartTime, sess.EndTime,
		coachIDVal, adminIDVal, locationIDVal, sessionRateVal, string(sess.TrainingType), sess.Notes,
		entryTypeVal, sess.Title, assignedStaffJSON,
		sess.ID.String(),
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) DeleteSession(sessionID uuid.UUID) error {
	var exists bool
	err := s.db.QueryRow(`SELECT 1 FROM training_sessions WHERE id = $1`, sessionID.String()).Scan(&exists)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Refund any student package credits
	rows, err := tx.Query(`SELECT student_package_id FROM attendance WHERE session_id = $1 AND student_package_id IS NOT NULL`, sessionID.String())
	if err != nil {
		return err
	}
	var pkgIDs []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err == nil && pid != "" {
			pkgIDs = append(pkgIDs, pid)
		}
	}
	rows.Close()

	for _, pid := range pkgIDs {
		_, _ = tx.Exec(`UPDATE student_packages SET remaining_sessions = remaining_sessions + 1 WHERE id = $1 AND remaining_sessions IS NOT NULL`, pid)
	}

	// Delete attendance records
	if _, err := tx.Exec(`DELETE FROM attendance WHERE session_id = $1`, sessionID.String()); err != nil {
		return err
	}

	// Delete training session
	res, err := tx.Exec(`DELETE FROM training_sessions WHERE id = $1`, sessionID.String())
	if err != nil {
		return err
	}
	rAffected, _ := res.RowsAffected()
	if rAffected == 0 {
		return ErrNotFound
	}

	return tx.Commit()
}

func (s *SQLStore) CancelSession(sessionID uuid.UUID, reason string, refundCredits bool) error {
	// 1. Verify session exists and is not already cancelled
	var isCancelled bool
	err := s.db.QueryRow(`SELECT COALESCE(is_cancelled, FALSE) FROM training_sessions WHERE id = $1`, sessionID.String()).Scan(&isCancelled)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if isCancelled {
		return errors.New("class is already cancelled")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if refundCredits {
		// Find attendances that used student package
		rows, err := tx.Query(`SELECT student_package_id FROM attendance WHERE session_id = $1 AND student_package_id IS NOT NULL`, sessionID.String())
		if err != nil {
			return err
		}
		var pkgIDs []string
		for rows.Next() {
			var pkgID string
			if err := rows.Scan(&pkgID); err == nil && pkgID != "" {
				pkgIDs = append(pkgIDs, pkgID)
			}
		}
		rows.Close()

		// Refund 1 session to each student package with limited sessions
		for _, pkgID := range pkgIDs {
			_, err = tx.Exec(`UPDATE student_packages SET remaining_sessions = remaining_sessions + 1 WHERE id = $1 AND remaining_sessions IS NOT NULL`, pkgID)
			if err != nil {
				return err
			}
		}

		// Delete attendance records for this session
		_, err = tx.Exec(`DELETE FROM attendance WHERE session_id = $1`, sessionID.String())
		if err != nil {
			return err
		}
	}

	// Update session cancellation status
	now := time.Now()
	nowDB := formatTimeForDB(now)
	if s.driver == "sqlite" {
		_, err = tx.Exec(`UPDATE training_sessions SET is_cancelled = 1, cancellation_reason = $1, cancelled_at = $2 WHERE id = $3`,
			reason, nowDB, sessionID.String())
	} else {
		_, err = tx.Exec(`UPDATE training_sessions SET is_cancelled = TRUE, cancellation_reason = $1, cancelled_at = $2 WHERE id = $3`,
			reason, nowDB, sessionID.String())
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Attendance & Live Check-In
func (s *SQLStore) scanAttendanceRow(rows *sql.Rows) (*models.Attendance, error) {
	var idStr, sessStr, checkedStr string
	var stStr sql.NullString
	var pkgIDStr, stName, belt, pkgTitle sql.NullString
	var locIDStr, locName, locPin sql.NullString
	var sessionRateVal sql.NullFloat64
	var attendeeType, attendeeName, attendeeRole sql.NullString
	att := &models.Attendance{}
	err := rows.Scan(
		&idStr, &sessStr, &stStr, &pkgIDStr, &checkedStr,
		&stName, &belt, &pkgTitle,
		&locIDStr, &locName, &locPin,
		&sessionRateVal,
		&attendeeType, &attendeeName, &attendeeRole,
	)
	if err != nil {
		return nil, err
	}
	att.ID = uuid.Must(uuid.Parse(idStr))
	att.SessionID = uuid.Must(uuid.Parse(sessStr))
	if stStr.Valid && stStr.String != "" {
		if sID, err := uuid.Parse(stStr.String); err == nil {
			att.StudentID = sID
		}
	}
	att.CheckedInAt, _ = parseTimeFlex(checkedStr)
	if pkgIDStr.Valid && pkgIDStr.String != "" {
		pID := uuid.Must(uuid.Parse(pkgIDStr.String))
		att.StudentPackageID = &pID
	}
	if sessionRateVal.Valid {
		r := sessionRateVal.Float64
		att.SessionRate = &r
	}
	if stName.Valid {
		att.StudentName = stName.String
	}
	if belt.Valid {
		att.StudentBelt = models.BeltRank(belt.String)
	}
	if pkgTitle.Valid {
		att.PackageTitle = pkgTitle.String
	}
	if locIDStr.Valid && locIDStr.String != "" {
		lID := uuid.Must(uuid.Parse(locIDStr.String))
		att.LocationID = &lID
	}
	if locName.Valid {
		att.LocationName = locName.String
	}
	if locPin.Valid {
		att.LocationPin = locPin.String
	}
	if attendeeType.Valid && attendeeType.String != "" {
		att.AttendeeType = attendeeType.String
	} else {
		att.AttendeeType = "student"
	}
	if attendeeName.Valid {
		att.AttendeeName = attendeeName.String
	}
	if attendeeRole.Valid {
		att.AttendeeRole = attendeeRole.String
	}
	return att, nil
}

func (s *SQLStore) GetSessionAttendances(sessionID uuid.UUID) ([]*models.Attendance, error) {
	query := `SELECT a.id, a.session_id, a.student_id, a.student_package_id, a.checked_in_at,
		st.full_name, st.current_belt, pt.title,
		COALESCE(a.location_id, ts.location_id),
		COALESCE(loc.name, loc_ts.name, ''),
		COALESCE(loc.pin, loc_ts.pin, ''),
		a.session_rate,
		COALESCE(a.attendee_type, 'student'),
		COALESCE(a.attendee_name, ''),
		COALESCE(a.attendee_role, '')
		FROM attendance a
		LEFT JOIN students st ON a.student_id = st.id
		LEFT JOIN student_packages sp ON a.student_package_id = sp.id
		LEFT JOIN package_templates pt ON sp.template_id = pt.id
		LEFT JOIN training_sessions ts ON a.session_id = ts.id
		LEFT JOIN locations loc ON a.location_id = loc.id
		LEFT JOIN locations loc_ts ON ts.location_id = loc_ts.id
		WHERE a.session_id = $1
		ORDER BY a.checked_in_at DESC`
	rows, err := s.db.Query(query, sessionID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendances []*models.Attendance
	for rows.Next() {
		att, err := s.scanAttendanceRow(rows)
		if err != nil {
			return nil, err
		}
		attendances = append(attendances, att)
	}
	return attendances, nil
}

func (s *SQLStore) GetAllAttendances() ([]*models.Attendance, error) {
	query := `SELECT a.id, a.session_id, a.student_id, a.student_package_id, a.checked_in_at,
		st.full_name, st.current_belt, pt.title,
		COALESCE(a.location_id, ts.location_id),
		COALESCE(loc.name, loc_ts.name, ''),
		COALESCE(loc.pin, loc_ts.pin, ''),
		a.session_rate,
		COALESCE(a.attendee_type, 'student'),
		COALESCE(a.attendee_name, ''),
		COALESCE(a.attendee_role, '')
		FROM attendance a
		LEFT JOIN students st ON a.student_id = st.id
		LEFT JOIN student_packages sp ON a.student_package_id = sp.id
		LEFT JOIN package_templates pt ON sp.template_id = pt.id
		LEFT JOIN training_sessions ts ON a.session_id = ts.id
		LEFT JOIN locations loc ON a.location_id = loc.id
		LEFT JOIN locations loc_ts ON ts.location_id = loc_ts.id
		ORDER BY a.checked_in_at DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendances []*models.Attendance
	for rows.Next() {
		att, err := s.scanAttendanceRow(rows)
		if err != nil {
			return nil, err
		}
		attendances = append(attendances, att)
	}
	return attendances, nil
}

func (s *SQLStore) GetSessionAttendanceCounts() (map[uuid.UUID]int, error) {
	query := `SELECT session_id, COUNT(*) FROM attendance GROUP BY session_id`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[uuid.UUID]int)
	for rows.Next() {
		var sessIDStr string
		var count int
		if err := rows.Scan(&sessIDStr, &count); err != nil {
			return nil, err
		}
		if sID, err := uuid.Parse(sessIDStr); err == nil {
			counts[sID] = count
		}
	}
	return counts, nil
}

func (s *SQLStore) GetStudentAttendances(studentID uuid.UUID) ([]*models.Attendance, error) {
	query := `SELECT a.id, a.session_id, a.student_id, a.student_package_id, a.checked_in_at,
		st.full_name, st.current_belt, pt.title,
		COALESCE(a.location_id, ts.location_id),
		COALESCE(loc.name, loc_ts.name, ''),
		COALESCE(loc.pin, loc_ts.pin, ''),
		a.session_rate,
		COALESCE(a.attendee_type, 'student'),
		COALESCE(a.attendee_name, ''),
		COALESCE(a.attendee_role, '')
		FROM attendance a
		LEFT JOIN students st ON a.student_id = st.id
		LEFT JOIN student_packages sp ON a.student_package_id = sp.id
		LEFT JOIN package_templates pt ON sp.template_id = pt.id
		LEFT JOIN training_sessions ts ON a.session_id = ts.id
		LEFT JOIN locations loc ON a.location_id = loc.id
		LEFT JOIN locations loc_ts ON ts.location_id = loc_ts.id
		WHERE a.student_id = $1
		ORDER BY a.checked_in_at DESC`
	rows, err := s.db.Query(query, studentID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendances []*models.Attendance
	for rows.Next() {
		att, err := s.scanAttendanceRow(rows)
		if err != nil {
			return nil, err
		}
		attendances = append(attendances, att)
	}
	return attendances, nil
}

func (s *SQLStore) CheckInAttendee(sessionID uuid.UUID, attendeeType string, attendeeID *uuid.UUID, attendeeName, attendeeRole string, packageID *uuid.UUID, sessionRate *float64) (*models.Attendance, error) {
	if attendeeType == "" {
		attendeeType = "student"
	}

	// 1. Check duplicate attendance
	if attendeeType == "student" && attendeeID != nil {
		var existingID string
		err := s.db.QueryRow("SELECT id FROM attendance WHERE session_id = $1 AND student_id = $2",
			sessionID.String(), attendeeID.String()).Scan(&existingID)
		if err == nil {
			return nil, ErrAlreadyInRoster
		}
	} else if attendeeID != nil {
		var existingID string
		err := s.db.QueryRow("SELECT id FROM attendance WHERE session_id = $1 AND (student_id = $2 OR (attendee_name != '' AND attendee_name = $3))",
			sessionID.String(), attendeeID.String(), attendeeName).Scan(&existingID)
		if err == nil {
			return nil, ErrAlreadyInRoster
		}
	} else if attendeeName != "" {
		var existingID string
		err := s.db.QueryRow("SELECT id FROM attendance WHERE session_id = $1 AND attendee_name = $2",
			sessionID.String(), attendeeName).Scan(&existingID)
		if err == nil {
			return nil, ErrAlreadyInRoster
		}
	}

	att := &models.Attendance{
		ID:           uuid.New(),
		SessionID:    sessionID,
		SessionRate:  sessionRate,
		CheckedInAt:  time.Now(),
		AttendeeType: attendeeType,
		AttendeeName: attendeeName,
		AttendeeRole: attendeeRole,
	}

	var studentIDVal interface{}
	if attendeeID != nil {
		att.StudentID = *attendeeID
		studentIDVal = attendeeID.String()
	}

	var pkgIDVal interface{}
	if packageID != nil {
		att.StudentPackageID = packageID
		pkgIDVal = packageID.String()
	}

	var sessionRateVal interface{}
	if sessionRate != nil {
		sessionRateVal = *sessionRate
	}

	// Look up session's location
	var locIDStr sql.NullString
	_ = s.db.QueryRow("SELECT location_id FROM training_sessions WHERE id = $1", sessionID.String()).Scan(&locIDStr)
	var locationID *uuid.UUID
	var locIDVal interface{}
	if locIDStr.Valid && locIDStr.String != "" {
		if lID, err := uuid.Parse(locIDStr.String); err == nil && lID != uuid.Nil {
			locationID = &lID
			locIDVal = lID.String()
		}
	}
	att.LocationID = locationID

	query := `INSERT INTO attendance (id, session_id, student_id, student_package_id, location_id, session_rate, checked_in_at, attendee_type, attendee_name, attendee_role)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := s.db.Exec(query,
		att.ID.String(), att.SessionID.String(), studentIDVal, pkgIDVal, locIDVal, sessionRateVal, formatTimeForDB(att.CheckedInAt),
		att.AttendeeType, att.AttendeeName, att.AttendeeRole,
	)
	if err != nil {
		return nil, err
	}

	// Enrich student info if student
	if attendeeType == "student" && attendeeID != nil {
		var fullName, currentBelt string
		_ = s.db.QueryRow("SELECT full_name, current_belt FROM students WHERE id = $1", attendeeID.String()).
			Scan(&fullName, &currentBelt)
		att.StudentName = fullName
		att.StudentBelt = models.BeltRank(currentBelt)
		if att.AttendeeName == "" {
			att.AttendeeName = fullName
		}
		if att.AttendeeRole == "" {
			att.AttendeeRole = currentBelt
		}

		if packageID != nil {
			var title string
			_ = s.db.QueryRow(`SELECT pt.title FROM student_packages sp
				JOIN package_templates pt ON sp.template_id = pt.id
				WHERE sp.id = $1`, packageID.String()).Scan(&title)
			att.PackageTitle = title
		}
	} else if attendeeType == "coach" && attendeeID != nil {
		var fullName, beltRank string
		_ = s.db.QueryRow("SELECT full_name, belt_rank FROM coaches WHERE id = $1", attendeeID.String()).
			Scan(&fullName, &beltRank)
		if att.AttendeeName == "" {
			att.AttendeeName = fullName
		}
		if att.AttendeeRole == "" {
			att.AttendeeRole = "Coach"
		}
	} else if attendeeType == "admin" && attendeeID != nil {
		var displayName, email, role string
		_ = s.db.QueryRow("SELECT display_name, email, role FROM users WHERE id = $1", attendeeID.String()).
			Scan(&displayName, &email, &role)
		if att.AttendeeName == "" {
			if displayName != "" {
				att.AttendeeName = displayName
			} else {
				att.AttendeeName = email
			}
		}
		if att.AttendeeRole == "" {
			att.AttendeeRole = role
		}
	}

	if locationID != nil {
		var locName, locPin string
		_ = s.db.QueryRow("SELECT name, pin FROM locations WHERE id = $1", locationID.String()).Scan(&locName, &locPin)
		att.LocationName = locName
		att.LocationPin = locPin
	}

	return att, nil
}

func (s *SQLStore) CheckInStudent(sessionID, studentID uuid.UUID, packageID *uuid.UUID, sessionRate *float64) (*models.Attendance, error) {
	return s.CheckInAttendee(sessionID, "student", &studentID, "", "", packageID, sessionRate)
}

func (s *SQLStore) RemoveAttendance(sessionID, studentID uuid.UUID) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var attIDStr string
	var pkgIDStr sql.NullString
	err = tx.QueryRow(`SELECT id, student_package_id FROM attendance WHERE session_id = $1 AND (student_id = $2 OR id = $2)`,
		sessionID.String(), studentID.String()).Scan(&attIDStr, &pkgIDStr)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if pkgIDStr.Valid && pkgIDStr.String != "" {
		_, err = tx.Exec(`UPDATE student_packages SET remaining_sessions = remaining_sessions + 1 WHERE id = $1 AND remaining_sessions IS NOT NULL`, pkgIDStr.String)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`DELETE FROM attendance WHERE id = $1`, attIDStr)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Student Evaluations
func (s *SQLStore) GetLatestEvaluation(studentID uuid.UUID) (*models.StudentEvaluation, error) {
	query := `SELECT se.id, se.student_id, se.coach_id, c.full_name, se.evaluation_date,
		se.flexibility, se.stamina, se.power, se.technique, se.sparring_iq, se.discipline,
		se.coach_remarks, se.created_at
		FROM student_evaluations se
		LEFT JOIN coaches c ON se.coach_id = c.id
		WHERE se.student_id = $1
		ORDER BY se.evaluation_date DESC LIMIT 1`
	var idStr, stIDStr, coachIDStr, evalDateStr, createdStr string
	var coachName, remarks sql.NullString
	eval := &models.StudentEvaluation{}
	err := s.db.QueryRow(query, studentID.String()).Scan(
		&idStr, &stIDStr, &coachIDStr, &coachName, &evalDateStr,
		&eval.Flexibility, &eval.Stamina, &eval.Power, &eval.Technique,
		&eval.SparringIQ, &eval.Discipline, &remarks, &createdStr,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	eval.ID = uuid.Must(uuid.Parse(idStr))
	eval.StudentID = uuid.Must(uuid.Parse(stIDStr))
	eval.CoachID = uuid.Must(uuid.Parse(coachIDStr))
	eval.EvaluationDate, _ = parseTimeFlex(evalDateStr)
	eval.CreatedAt, _ = parseTimeFlex(createdStr)
	eval.Sparring = eval.SparringIQ
	eval.Poomsae = eval.Technique
	if coachName.Valid {
		eval.CoachName = coachName.String
	}
	if remarks.Valid {
		eval.CoachRemarks = remarks.String
	}
	return eval, nil
}

func (s *SQLStore) GetLatestEvaluations() (map[uuid.UUID]*models.StudentEvaluation, error) {
	query := `SELECT ranked.id, ranked.student_id, ranked.coach_id, c.full_name, ranked.evaluation_date,
		ranked.flexibility, ranked.stamina, ranked.power, ranked.technique, ranked.sparring_iq, ranked.discipline,
		ranked.coach_remarks, ranked.created_at
	FROM (
		SELECT id, student_id, coach_id, evaluation_date,
			flexibility, stamina, power, technique, sparring_iq, discipline,
			coach_remarks, created_at,
			ROW_NUMBER() OVER (PARTITION BY student_id ORDER BY evaluation_date DESC, created_at DESC) as rn
		FROM student_evaluations
	) ranked
	LEFT JOIN coaches c ON ranked.coach_id = c.id
	WHERE ranked.rn = 1`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*models.StudentEvaluation)
	for rows.Next() {
		var idStr, stIDStr, coachIDStr, evalDateStr, createdStr string
		var coachName, remarks sql.NullString
		eval := &models.StudentEvaluation{}
		err := rows.Scan(
			&idStr, &stIDStr, &coachIDStr, &coachName, &evalDateStr,
			&eval.Flexibility, &eval.Stamina, &eval.Power, &eval.Technique,
			&eval.SparringIQ, &eval.Discipline, &remarks, &createdStr,
		)
		if err != nil {
			return nil, err
		}
		eval.ID = uuid.Must(uuid.Parse(idStr))
		eval.StudentID = uuid.Must(uuid.Parse(stIDStr))
		eval.CoachID = uuid.Must(uuid.Parse(coachIDStr))
		eval.EvaluationDate, _ = parseTimeFlex(evalDateStr)
		eval.CreatedAt, _ = parseTimeFlex(createdStr)
		eval.Sparring = eval.SparringIQ
		eval.Poomsae = eval.Technique
		if coachName.Valid {
			eval.CoachName = coachName.String
		}
		if remarks.Valid {
			eval.CoachRemarks = remarks.String
		}
		result[eval.StudentID] = eval
	}
	return result, nil
}

func (s *SQLStore) GetStudentEvaluations(studentID uuid.UUID) ([]*models.StudentEvaluation, error) {
	query := `SELECT se.id, se.student_id, se.coach_id, c.full_name, se.evaluation_date,
		se.flexibility, se.stamina, se.power, se.technique, se.sparring_iq, se.discipline,
		se.coach_remarks, se.created_at
		FROM student_evaluations se
		LEFT JOIN coaches c ON se.coach_id = c.id
		WHERE se.student_id = $1
		ORDER BY se.evaluation_date DESC`
	rows, err := s.db.Query(query, studentID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evals []*models.StudentEvaluation
	for rows.Next() {
		var idStr, stIDStr, coachIDStr, evalDateStr, createdStr string
		var coachName, remarks sql.NullString
		eval := &models.StudentEvaluation{}
		err := rows.Scan(
			&idStr, &stIDStr, &coachIDStr, &coachName, &evalDateStr,
			&eval.Flexibility, &eval.Stamina, &eval.Power, &eval.Technique,
			&eval.SparringIQ, &eval.Discipline, &remarks, &createdStr,
		)
		if err != nil {
			return nil, err
		}
		eval.ID = uuid.Must(uuid.Parse(idStr))
		eval.StudentID = uuid.Must(uuid.Parse(stIDStr))
		eval.CoachID = uuid.Must(uuid.Parse(coachIDStr))
		eval.EvaluationDate, _ = parseTimeFlex(evalDateStr)
		eval.CreatedAt, _ = parseTimeFlex(createdStr)
		eval.Sparring = eval.SparringIQ
		eval.Poomsae = eval.Technique
		if coachName.Valid {
			eval.CoachName = coachName.String
		}
		if remarks.Valid {
			eval.CoachRemarks = remarks.String
		}
		evals = append(evals, eval)
	}
	return evals, nil
}

func (s *SQLStore) CreateEvaluation(eval *models.StudentEvaluation) error {
	if eval.ID == uuid.Nil {
		eval.ID = uuid.New()
	}
	if eval.CreatedAt.IsZero() {
		eval.CreatedAt = time.Now()
	}
	eval.SyncLegacyFields()
	query := `INSERT INTO student_evaluations (id, student_id, coach_id, evaluation_date,
		flexibility, stamina, power, technique, sparring_iq, discipline, coach_remarks, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	_, err := s.db.Exec(query,
		eval.ID.String(), eval.StudentID.String(), eval.CoachID.String(), formatDateForDB(eval.EvaluationDate),
		eval.Flexibility, eval.Stamina, eval.Power, eval.Technique, eval.SparringIQ, eval.Discipline,
		eval.CoachRemarks, formatTimeForDB(eval.CreatedAt),
	)
	return err
}

// SeedDefaultData seeds the initial coaches, packages, students, sessions, attendances, and evaluations.
func (s *SQLStore) SeedDefaultData() error {
	now := time.Now()

	// 1. Coaches
	c1ID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	c2ID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	c3ID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	c1Expiry := now.AddDate(1, 0, 0)
	c2Expiry := now.AddDate(0, 0, 10)
	c3Expiry := now.AddDate(0, 0, -15)

	_ = s.CreateCoach(&models.Coach{
		ID: c1ID, FullName: "Master Dae-Hyun Kim", Email: "master.kim@seriestkd.com", Phone: "+1 (555) 019-2831",
		BeltRank: "6th Dan Black Belt", RatePerSession: 85.00, FirstAidCertified: true, FirstAidExpiry: &c1Expiry,
		Specialties: []string{"Forms/Poomsae", "Sparring/Kyorugi", "Demo Team"}, IsActive: true, CreatedAt: now.AddDate(-2, 0, 0),
	})
	_ = s.CreateCoach(&models.Coach{
		ID: c2ID, FullName: "Coach Ji-Woo Park", Email: "jiwoo.park@seriestkd.com", Phone: "+1 (555) 014-9920",
		BeltRank: "4th Dan Black Belt", RatePerSession: 65.00, FirstAidCertified: true, FirstAidExpiry: &c2Expiry,
		Specialties: []string{"Sparring/Kyorugi", "Conditioning"}, IsActive: true, CreatedAt: now.AddDate(-1, 0, 0),
	})
	_ = s.CreateCoach(&models.Coach{
		ID: c3ID, FullName: "Coach Min-Seok Lee", Email: "minseok.lee@seriestkd.com", Phone: "+1 (555) 017-4482",
		BeltRank: "3rd Dan Black Belt", RatePerSession: 55.00, FirstAidCertified: false, FirstAidExpiry: &c3Expiry,
		Specialties: []string{"Cadets/Kids", "Forms/Poomsae"}, IsActive: true, CreatedAt: now.AddDate(0, -6, 0),
	})

	// 2. Package Templates
	t1ID := uuid.MustParse("a1111111-1111-1111-1111-111111111111")
	t2ID := uuid.MustParse("a2222222-2222-2222-2222-222222222222")
	t3ID := uuid.MustParse("a3333333-3333-3333-3333-333333333333")
	t4ID := uuid.MustParse("a4444444-4444-4444-4444-444444444444")
	t5ID := uuid.MustParse("a5555555-5555-5555-5555-555555555555")
	c12 := 12
	c24 := 24
	c4 := 4
	c8 := 8
	spw1 := 1
	spw2 := 2

	_, _ = s.db.Exec(`INSERT INTO package_templates (id, title, description, plan_type, session_count, sessions_per_week, validity_days, price, is_active) VALUES
		($1, '12-Session Sparring & Technical Pass', 'Structured sparring drills, footwork, and tactical timing combinations.', 'standard', $2, NULL, 90, 180.00, 1),
		($3, '24-Session Promotion Prep Pass', 'Comprehensive syllabus coverage, Kup forms, and board breaking preparation.', 'standard', $4, NULL, 180, 320.00, 1),
		($5, 'Monthly Unlimited Athlete Membership', 'Full floor access to all regular classes, poomsae sessions, and open sparring mats.', 'unlimited', NULL, NULL, 30, 220.00, 1),
		($6, '4-Week Fundamental Pass (1x/week)', 'Weekly foundational drills and discipline. Strictly consumable over 4 weeks.', 'four_week', $7, $8, 28, 80.00, 1),
		($9, '4-Week Cadet & Athlete Pass (2x/week)', 'Twice-weekly high performance training. Strictly consumable over 4 weeks.', 'four_week', $10, $11, 28, 140.00, 1)`,
		t1ID.String(), c12, t2ID.String(), c24, t3ID.String(),
		t4ID.String(), c4, spw1, t5ID.String(), c8, spw2,
	)

	// 3. Students
	s1ID := uuid.MustParse("b1111111-1111-1111-1111-111111111111")
	s2ID := uuid.MustParse("b2222222-2222-2222-2222-222222222222")
	s3ID := uuid.MustParse("b3333333-3333-3333-3333-333333333333")
	s4ID := uuid.MustParse("b4444444-4444-4444-4444-444444444444")

	_ = s.CreateStudent(&models.Student{
		ID: s1ID, FullName: "Alex Vance", DOB: now.AddDate(-14, 0, 0), Gender: "Male", Phone: "+1 (555) 234-5678",
		CurrentBelt: models.BeltWhite, LastPromotionDate: now.AddDate(0, 0, -65), EmergencyName: "Sarah Vance",
		EmergencyPhone: "+1 (555) 234-5679", EmergencyRelation: "Mother", MedicalNotes: "Mild asthma, uses inhaler before intense cardio",
		IsActive: true, CreatedAt: now.AddDate(0, -3, 0),
	})
	_ = s.CreateStudent(&models.Student{
		ID: s2ID, FullName: "Chloe Ramirez", DOB: now.AddDate(-16, 0, 0), Gender: "Female", Phone: "+1 (555) 345-6789",
		CurrentBelt: models.BeltHighYellow, LastPromotionDate: now.AddDate(0, 0, -75), EmergencyName: "Carlos Ramirez",
		EmergencyPhone: "+1 (555) 345-6780", EmergencyRelation: "Father", MedicalNotes: "No known allergies or medical restrictions",
		IsActive: true, CreatedAt: now.AddDate(0, -5, 0),
	})
	_ = s.CreateStudent(&models.Student{
		ID: s3ID, FullName: "Marcus Brody", DOB: now.AddDate(-12, 0, 0), Gender: "Male", Phone: "+1 (555) 456-7890",
		CurrentBelt: models.BeltLowBlue, LastPromotionDate: now.AddDate(0, 0, -15), EmergencyName: "Elena Brody",
		EmergencyPhone: "+1 (555) 456-7891", EmergencyRelation: "Mother", MedicalNotes: "Previous wrist sprain, clear for non-contact forms",
		IsActive: true, CreatedAt: now.AddDate(0, -2, 0),
	})
	_ = s.CreateStudent(&models.Student{
		ID: s4ID, FullName: "Sophia Chen", DOB: now.AddDate(-18, 0, 0), Gender: "Female", Phone: "+1 (555) 567-8901",
		CurrentBelt: models.BeltHighRed, LastPromotionDate: now.AddDate(0, 0, -160), EmergencyName: "David Chen",
		EmergencyPhone: "+1 (555) 567-8902", EmergencyRelation: "Father", MedicalNotes: "Full medical clearance",
		IsActive: true, CreatedAt: now.AddDate(-2, 0, 0),
	})

	// 4. Student Packages
	sp1Rem := 8
	sp1ID := uuid.New()
	_ = s.AssignPackage(&models.StudentPackage{
		ID: sp1ID, StudentID: s1ID, TemplateID: t1ID, TotalSessions: &c12, RemainingSessions: &sp1Rem,
		PurchaseDate: now.AddDate(0, 0, -30), ExpiryDate: now.AddDate(0, 0, 60), PaymentStatus: "paid", CreatedAt: now.AddDate(0, 0, -30),
	})
	sp2Rem := 15
	sp2ID := uuid.New()
	_ = s.AssignPackage(&models.StudentPackage{
		ID: sp2ID, StudentID: s2ID, TemplateID: t2ID, TotalSessions: &c24, RemainingSessions: &sp2Rem,
		PurchaseDate: now.AddDate(0, 0, -60), ExpiryDate: now.AddDate(0, 0, 120), PaymentStatus: "paid", CreatedAt: now.AddDate(0, 0, -60),
	})
	sp3Rem := 2
	sp3ID := uuid.New()
	_ = s.AssignPackage(&models.StudentPackage{
		ID: sp3ID, StudentID: s3ID, TemplateID: t1ID, TotalSessions: &c12, RemainingSessions: &sp3Rem,
		PurchaseDate: now.AddDate(0, 0, -10), ExpiryDate: now.AddDate(0, 0, 80), PaymentStatus: "paid", CreatedAt: now.AddDate(0, 0, -10),
	})
	sp4ID := uuid.New()
	_ = s.AssignPackage(&models.StudentPackage{
		ID: sp4ID, StudentID: s4ID, TemplateID: t3ID, TotalSessions: nil, RemainingSessions: nil,
		PurchaseDate: now.AddDate(0, 0, -15), ExpiryDate: now.AddDate(0, 0, 15), PaymentStatus: "paid", CreatedAt: now.AddDate(0, 0, -15),
	})

	// 5. Locations
	loc1ID := uuid.MustParse("d1111111-1111-1111-1111-111111111111")
	loc2ID := uuid.MustParse("d2222222-2222-2222-2222-222222222222")

	_ = s.CreateLocation(&models.Location{
		ID:        loc1ID,
		Name:      "Makati Central Dojang (HQ)",
		Pin:       "https://maps.google.com/?q=Makati+Central+Dojang",
		CreatedAt: now.AddDate(-1, 0, 0),
	})
	_ = s.CreateLocation(&models.Location{
		ID:        loc2ID,
		Name:      "BGC High Street Training Hall",
		Pin:       "https://maps.google.com/?q=BGC+High+Street+Training+Hall",
		CreatedAt: now.AddDate(0, -6, 0),
	})

	// 6. Training Sessions
	sess1ID := uuid.MustParse("c1111111-1111-1111-1111-111111111111")
	sess2ID := uuid.MustParse("c2222222-2222-2222-2222-222222222222")

	_ = s.CreateSession(&models.TrainingSession{
		ID: sess1ID, SessionDate: now, StartTime: "17:00", EndTime: "18:30", CoachID: &c1ID, CoachName: "Master Dae-Hyun Kim",
		AdminID: &c1ID, LocationID: &loc1ID, TrainingType: models.TrainingSparring, Notes: "High intensity floor drills, electronic scoring pad practice",
		CreatedAt: now.Add(-2 * time.Hour),
	})
	_ = s.CreateSession(&models.TrainingSession{
		ID: sess2ID, SessionDate: now.AddDate(0, 0, -1), StartTime: "18:30", EndTime: "20:00", CoachID: &c2ID, CoachName: "Coach Ji-Woo Park",
		AdminID: &c1ID, LocationID: &loc2ID, TrainingType: models.TrainingPoomsae, Notes: "Taegeuk 1 through 8 refinement & stance balance auditing",
		CreatedAt: now.AddDate(0, 0, -1),
	})

	// 6. Historic Attendance logs for promotion readiness
	for i := 0; i < 18; i++ {
		dummyID := uuid.New()
		tType := models.TrainingPoomsae
		if i%2 == 0 {
			tType = models.TrainingSparring
		}
		_ = s.CreateSession(&models.TrainingSession{
			ID: dummyID, SessionDate: now.AddDate(0, 0, -i*2), StartTime: "17:00", EndTime: "18:15",
			CoachID: &c1ID, TrainingType: tType, Notes: "Regular class attendance log",
		})
		_, _ = s.CheckInStudent(dummyID, s1ID, &sp1ID, nil)
	}

	// Also check Alex in to current live sess1
	_, _ = s.CheckInStudent(sess1ID, s1ID, &sp1ID, nil)

	for i := 0; i < 25; i++ {
		dummyID := uuid.New()
		_ = s.CreateSession(&models.TrainingSession{
			ID: dummyID, SessionDate: now.AddDate(0, 0, -i*2), StartTime: "18:30", EndTime: "19:45",
			CoachID: &c2ID, TrainingType: models.TrainingPoomsae, Notes: "Regular class",
		})
		_, _ = s.CheckInStudent(dummyID, s2ID, &sp2ID, nil)
	}

	// 7. Student Evaluation for Alex
	_ = s.CreateEvaluation(&models.StudentEvaluation{
		ID: uuid.New(), StudentID: s1ID, CoachID: c1ID, EvaluationDate: now.AddDate(0, 0, -5),
		Sparring: 8, Flexibility: 8, Poomsae: 8,
		CoachRemarks: "Exceptional discipline and kick height. Clear candidate for Low Yellow promotion testing.",
		CreatedAt:    now.AddDate(0, 0, -5),
	})

	// 8. Default Users for Multi-Role Auth
	uManager := &models.User{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000000"),
		Email:       "manager@seriestkd.com",
		Username:    "manager",
		Role:        models.RoleOperationManager,
		IsActive:    true,
		DisplayName: "Operation Manager",
	}
	_ = uManager.SetPassword("manager123")
	_ = s.CreateUser(uManager)

	uAdmin := &models.User{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Email:       "admin@seriestkd.com",
		Username:    "admin",
		Role:        models.RoleAdmin,
		IsActive:    true,
		DisplayName: "Dojang Administrator",
	}
	_ = uAdmin.SetPassword("admin123")
	_ = s.CreateUser(uAdmin)

	uCoach := &models.User{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Email:       "jiwoo.park@seriestkd.com",
		Username:    "jiwoo.park",
		Role:        models.RoleCoach,
		CoachID:     &c2ID,
		IsActive:    true,
		DisplayName: "Coach Ji-Woo Park",
	}
	_ = uCoach.SetPassword("coach123")
	_ = s.CreateUser(uCoach)

	uStudent1 := &models.User{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		Email:       "alex.vance@seriestkd.com",
		Username:    "alex.vance",
		Role:        models.RoleStudent,
		StudentID:   &s1ID,
		IsActive:    true,
		DisplayName: "Alex Vance",
	}
	_ = uStudent1.SetPassword("student123")
	_ = s.CreateUser(uStudent1)

	uStudent2 := &models.User{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000004"),
		Email:       "chloe.ramirez@seriestkd.com",
		Username:    "chloe.ramirez",
		Role:        models.RoleStudent,
		StudentID:   &s2ID,
		IsActive:    true,
		DisplayName: "Chloe Ramirez",
	}
	_ = uStudent2.SetPassword("student123")
	_ = s.CreateUser(uStudent2)

	// 9. Initial Safety Incident on Marcus Brody (s3)
	incID := uuid.MustParse("00000000-0000-0000-0000-000000000009")
	_ = s.CreateSafetyIncident(&models.SafetyIncident{
		ID:           incID,
		StudentID:    s3ID,
		CoachID:      &c2ID,
		IncidentType: "Wrist Strain / Sprain",
		Notes:        "Slight hyperextension during power break rehearsal. Ice applied. No sparring contact until cleared.",
		Resolved:     false,
		CreatedAt:    now.AddDate(0, 0, -2),
	})

	return nil
}

// Auth & Users
const userSelectFields = `SELECT u.id, u.email, u.username, u.password_hash, u.role, u.student_id, u.coach_id, u.is_active, u.last_login_at, u.created_at, u.updated_at,
	COALESCE(NULLIF(u.display_name, ''), st.full_name, c.full_name, 'Dojang Administrator') as display_name,
	COALESCE(u.profile_picture_url, '') as profile_picture_url
	FROM users u
	LEFT JOIN students st ON u.student_id = st.id
	LEFT JOIN coaches c ON u.coach_id = c.id`

func scanUser(scanner interface{ Scan(dest ...any) error }) (*models.User, error) {
	var idStr, studentIDStr, coachIDStr, lastLoginStr, createdStr, updatedStr, usernameStr sql.NullString
	u := &models.User{}
	var roleStr string
	var picURL sql.NullString
	err := scanner.Scan(
		&idStr, &u.Email, &usernameStr, &u.PasswordHash, &roleStr, &studentIDStr, &coachIDStr,
		&u.IsActive, &lastLoginStr, &createdStr, &updatedStr, &u.DisplayName, &picURL,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.ID = uuid.Must(uuid.Parse(idStr.String))
	if usernameStr.Valid {
		u.Username = usernameStr.String
	}
	u.Role = models.UserRole(roleStr)
	if studentIDStr.Valid && studentIDStr.String != "" {
		sid, _ := uuid.Parse(studentIDStr.String)
		u.StudentID = &sid
	}
	if coachIDStr.Valid && coachIDStr.String != "" {
		cid, _ := uuid.Parse(coachIDStr.String)
		u.CoachID = &cid
	}
	if lastLoginStr.Valid && lastLoginStr.String != "" {
		t, _ := parseTimeFlex(lastLoginStr.String)
		u.LastLoginAt = &t
	}
	u.CreatedAt, _ = parseTimeFlex(createdStr.String)
	u.UpdatedAt, _ = parseTimeFlex(updatedStr.String)
	if picURL.Valid {
		u.ProfilePictureURL = picURL.String
	}
	return u, nil
}

func (s *SQLStore) GetUserByEmail(email string) (*models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	query := userSelectFields + ` WHERE LOWER(u.email) = $1`
	return scanUser(s.db.QueryRow(query, normEmail))
}

func (s *SQLStore) GetUserByUsername(username string) (*models.User, error) {
	normUsername := strings.ToLower(strings.TrimSpace(username))
	if normUsername == "" {
		return nil, ErrNotFound
	}
	query := userSelectFields + ` WHERE LOWER(u.username) = $1`
	return scanUser(s.db.QueryRow(query, normUsername))
}

func (s *SQLStore) GetUserByIdentifier(identifier string) (*models.User, error) {
	norm := strings.ToLower(strings.TrimSpace(identifier))
	if norm == "" {
		return nil, ErrNotFound
	}
	query := userSelectFields + ` WHERE LOWER(u.email) = $1 OR LOWER(u.username) = $1`
	return scanUser(s.db.QueryRow(query, norm))
}

func (s *SQLStore) GetUserByID(id uuid.UUID) (*models.User, error) {
	query := userSelectFields + ` WHERE u.id = $1`
	return scanUser(s.db.QueryRow(query, id.String()))
}

func (s *SQLStore) GetUserByStudentID(studentID uuid.UUID) (*models.User, error) {
	query := userSelectFields + ` WHERE u.student_id = $1`
	return scanUser(s.db.QueryRow(query, studentID.String()))
}

func (s *SQLStore) GetUserByCoachID(coachID uuid.UUID) (*models.User, error) {
	query := userSelectFields + ` WHERE u.coach_id = $1`
	return scanUser(s.db.QueryRow(query, coachID.String()))
}

func (s *SQLStore) CreateUser(u *models.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	normEmail := strings.ToLower(strings.TrimSpace(u.Email))
	u.Email = normEmail

	normUsername := strings.ToLower(strings.TrimSpace(u.Username))
	u.Username = normUsername

	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	var studentID, coachID, usernameCol sql.NullString
	if normUsername != "" {
		usernameCol = sql.NullString{String: normUsername, Valid: true}
	}
	if u.StudentID != nil {
		studentID = sql.NullString{String: u.StudentID.String(), Valid: true}
	}
	if u.CoachID != nil {
		coachID = sql.NullString{String: u.CoachID.String(), Valid: true}
	}

	query := `INSERT INTO users (id, email, username, password_hash, role, display_name, profile_picture_url, student_id, coach_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	_, err := s.db.Exec(query,
		u.ID.String(), u.Email, usernameCol, u.PasswordHash, string(u.Role), u.DisplayName, u.ProfilePictureURL, studentID, coachID,
		u.IsActive, formatTimeForDB(u.CreatedAt), formatTimeForDB(u.UpdatedAt),
	)
	return err
}

func (s *SQLStore) UpdateUser(u *models.User) error {
	now := time.Now()
	normUsername := strings.ToLower(strings.TrimSpace(u.Username))
	u.Username = normUsername
	var usernameCol sql.NullString
	if normUsername != "" {
		usernameCol = sql.NullString{String: normUsername, Valid: true}
	}
	query := `UPDATE users SET username = $1, password_hash = $2, is_active = $3, display_name = $4, profile_picture_url = $5, updated_at = $6 WHERE id = $7`
	res, err := s.db.Exec(query, usernameCol, u.PasswordHash, u.IsActive, u.DisplayName, u.ProfilePictureURL, formatTimeForDB(now), u.ID.String())
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) ToggleUserActive(userID uuid.UUID, isActive bool) error {
	now := time.Now()
	query := `UPDATE users SET is_active = $1, updated_at = $2 WHERE id = $3`
	res, err := s.db.Exec(query, isActive, formatTimeForDB(now), userID.String())
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	if !isActive {
		_, _ = s.db.Exec(`DELETE FROM user_sessions WHERE user_id = $1`, userID.String())
	}
	return nil
}

func (s *SQLStore) DeleteUser(userID uuid.UUID) error {
	_, _ = s.db.Exec(`DELETE FROM user_sessions WHERE user_id = $1`, userID.String())
	_, _ = s.db.Exec(`DELETE FROM password_reset_tokens WHERE user_id = $1`, userID.String())
	_, _ = s.db.Exec(`UPDATE training_sessions SET admin_id = NULL WHERE admin_id = $1`, userID.String())
	res, err := s.db.Exec(`DELETE FROM users WHERE id = $1`, userID.String())
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) GetUsersByRole(role models.UserRole) ([]*models.User, error) {
	query := userSelectFields + ` WHERE u.role = $1 ORDER BY u.created_at DESC`
	rows, err := s.db.Query(query, string(role))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// Password Reset Tokens
func (s *SQLStore) CreatePasswordResetToken(t *models.PasswordResetToken) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	query := `INSERT INTO password_reset_tokens (id, user_id, token, expires_at, used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	var usedAtStr sql.NullString
	if t.UsedAt != nil {
		usedAtStr = sql.NullString{String: formatTimeForDB(*t.UsedAt), Valid: true}
	}
	_, err := s.db.Exec(query, t.ID.String(), t.UserID.String(), t.Token, formatTimeForDB(t.ExpiresAt), usedAtStr, formatTimeForDB(t.CreatedAt))
	return err
}

func (s *SQLStore) GetPasswordResetToken(token string) (*models.PasswordResetToken, error) {
	query := `SELECT id, user_id, token, expires_at, used_at, created_at FROM password_reset_tokens WHERE token = $1`
	var idStr, userIDStr, expStr, usedStr, createdStr sql.NullString
	t := &models.PasswordResetToken{}
	err := s.db.QueryRow(query, token).Scan(&idStr, &userIDStr, &t.Token, &expStr, &usedStr, &createdStr)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.ID = uuid.Must(uuid.Parse(idStr.String))
	t.UserID = uuid.Must(uuid.Parse(userIDStr.String))
	t.ExpiresAt, _ = parseTimeFlex(expStr.String)
	t.CreatedAt, _ = parseTimeFlex(createdStr.String)
	if usedStr.Valid && usedStr.String != "" {
		uAt, _ := parseTimeFlex(usedStr.String)
		t.UsedAt = &uAt
	}
	return t, nil
}

func (s *SQLStore) MarkPasswordResetTokenUsed(token string) error {
	now := time.Now()
	query := `UPDATE password_reset_tokens SET used_at = $1 WHERE token = $2`
	res, err := s.db.Exec(query, formatTimeForDB(now), token)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) UpdateUserLastLogin(id uuid.UUID) error {
	now := time.Now()
	query := `UPDATE users SET last_login_at = $1, updated_at = $2 WHERE id = $3`
	_, err := s.db.Exec(query, formatTimeForDB(now), formatTimeForDB(now), id.String())
	return err
}

func (s *SQLStore) CreateSessionToken(token string, userID uuid.UUID, expiresAt time.Time) error {
	query := `INSERT INTO user_sessions (token, user_id, expires_at, created_at) VALUES ($1, $2, $3, $4)`
	_, err := s.db.Exec(query, token, userID.String(), formatTimeForDB(expiresAt), formatTimeForDB(time.Now()))
	return err
}

func (s *SQLStore) GetUserBySessionToken(token string) (*models.User, error) {
	query := `SELECT u.id, u.email, u.password_hash, u.role, u.student_id, u.coach_id, u.is_active, u.last_login_at, u.created_at, u.updated_at,
		COALESCE(NULLIF(u.display_name, ''), st.full_name, c.full_name, 'Dojang Administrator') as display_name,
		sess.expires_at
		FROM user_sessions sess
		JOIN users u ON sess.user_id = u.id
		LEFT JOIN students st ON u.student_id = st.id
		LEFT JOIN coaches c ON u.coach_id = c.id
		WHERE sess.token = $1`
	var idStr, studentIDStr, coachIDStr, lastLoginStr, createdStr, updatedStr, expiresStr sql.NullString
	u := &models.User{}
	var roleStr string
	err := s.db.QueryRow(query, token).Scan(
		&idStr, &u.Email, &u.PasswordHash, &roleStr, &studentIDStr, &coachIDStr,
		&u.IsActive, &lastLoginStr, &createdStr, &updatedStr, &u.DisplayName, &expiresStr,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if expiresStr.Valid && expiresStr.String != "" {
		exp, _ := parseTimeFlex(expiresStr.String)
		if time.Now().After(exp) {
			_ = s.DeleteSessionToken(token)
			return nil, ErrNotFound
		}
	}
	u.ID = uuid.Must(uuid.Parse(idStr.String))
	u.Role = models.UserRole(roleStr)
	if studentIDStr.Valid && studentIDStr.String != "" {
		sid, _ := uuid.Parse(studentIDStr.String)
		u.StudentID = &sid
	}
	if coachIDStr.Valid && coachIDStr.String != "" {
		cid, _ := uuid.Parse(coachIDStr.String)
		u.CoachID = &cid
	}
	if lastLoginStr.Valid && lastLoginStr.String != "" {
		t, _ := parseTimeFlex(lastLoginStr.String)
		u.LastLoginAt = &t
	}
	u.CreatedAt, _ = parseTimeFlex(createdStr.String)
	u.UpdatedAt, _ = parseTimeFlex(updatedStr.String)
	return u, nil
}

func (s *SQLStore) DeleteSessionToken(token string) error {
	query := `DELETE FROM user_sessions WHERE token = $1`
	_, err := s.db.Exec(query, token)
	return err
}

func (s *SQLStore) CreateSafetyIncident(inc *models.SafetyIncident) error {
	if inc.ID == uuid.Nil {
		inc.ID = uuid.New()
	}
	if inc.CreatedAt.IsZero() {
		inc.CreatedAt = time.Now()
	}
	var coachID sql.NullString
	if inc.CoachID != nil {
		coachID = sql.NullString{String: inc.CoachID.String(), Valid: true}
	}
	query := `INSERT INTO safety_incidents (id, student_id, coach_id, incident_type, notes, resolved, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.db.Exec(query,
		inc.ID.String(), inc.StudentID.String(), coachID, inc.IncidentType, inc.Notes,
		inc.Resolved, formatTimeForDB(inc.CreatedAt),
	)
	if err != nil {
		return err
	}
	_ = s.SetStudentSafetyFlag(inc.StudentID, true)
	return nil
}

func (s *SQLStore) GetSafetyIncidents(resolved *bool) ([]*models.SafetyIncident, error) {
	query := `SELECT inc.id, inc.student_id, inc.coach_id, inc.incident_type, inc.notes,
		inc.resolved, inc.resolved_by, inc.resolved_at, inc.created_at,
		COALESCE(st.full_name, 'Unknown Student') as student_name,
		COALESCE(c.full_name, 'Staff') as coach_name
		FROM safety_incidents inc
		LEFT JOIN students st ON inc.student_id = st.id
		LEFT JOIN coaches c ON inc.coach_id = c.id`

	var rows *sql.Rows
	var err error
	if resolved != nil {
		query += ` WHERE inc.resolved = $1 ORDER BY inc.created_at DESC`
		rows, err = s.db.Query(query, *resolved)
	} else {
		query += ` ORDER BY inc.created_at DESC`
		rows, err = s.db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incidents []*models.SafetyIncident
	for rows.Next() {
		var idStr, stIDStr, coachIDStr, resByStr, resAtStr, createdStr sql.NullString
		inc := &models.SafetyIncident{}
		err := rows.Scan(
			&idStr, &stIDStr, &coachIDStr, &inc.IncidentType, &inc.Notes,
			&inc.Resolved, &resByStr, &resAtStr, &createdStr,
			&inc.StudentName, &inc.CoachName,
		)
		if err != nil {
			return nil, err
		}
		inc.ID = uuid.Must(uuid.Parse(idStr.String))
		inc.StudentID = uuid.Must(uuid.Parse(stIDStr.String))
		if coachIDStr.Valid && coachIDStr.String != "" {
			cid, _ := uuid.Parse(coachIDStr.String)
			inc.CoachID = &cid
		}
		if resByStr.Valid {
			inc.ResolvedBy = resByStr.String
		}
		if resAtStr.Valid && resAtStr.String != "" {
			t, _ := parseTimeFlex(resAtStr.String)
			inc.ResolvedAt = &t
		}
		inc.CreatedAt, _ = parseTimeFlex(createdStr.String)
		incidents = append(incidents, inc)
	}
	return incidents, nil
}

func (s *SQLStore) ResolveSafetyIncident(incidentID uuid.UUID, adminEmail string) error {
	now := time.Now()
	var studentIDStr string
	err := s.db.QueryRow(`SELECT student_id FROM safety_incidents WHERE id = $1`, incidentID.String()).Scan(&studentIDStr)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	query := `UPDATE safety_incidents SET resolved = $1, resolved_by = $2, resolved_at = $3 WHERE id = $4`
	if _, err := s.db.Exec(query, true, adminEmail, formatTimeForDB(now), incidentID.String()); err != nil {
		return err
	}

	// Check if this student still has any unresolved incidents
	var remainingCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM safety_incidents WHERE student_id = $1 AND resolved = $2`, studentIDStr, false).Scan(&remainingCount)
	if remainingCount == 0 {
		stID, _ := uuid.Parse(studentIDStr)
		_ = s.SetStudentSafetyFlag(stID, false)
	}
	return nil
}

func (s *SQLStore) SetStudentSafetyFlag(studentID uuid.UUID, hasSafetyFlag bool) error {
	query := `UPDATE students SET has_safety_flag = $1 WHERE id = $2`
	_, err := s.db.Exec(query, hasSafetyFlag, studentID.String())
	return err
}

func (s *SQLStore) PromoteStudent(studentID uuid.UUID, newBelt models.BeltRank) error {
	now := time.Now()
	query := `UPDATE students SET current_belt = $1, last_promotion_date = $2 WHERE id = $3`
	_, err := s.db.Exec(query, string(newBelt), formatDateForDB(now), studentID.String())
	return err
}

// Audit Logs
func (s *SQLStore) CreateAuditLog(entry *models.AuditLog) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	var userIDStr sql.NullString
	if entry.UserID != nil && *entry.UserID != uuid.Nil {
		userIDStr = sql.NullString{String: entry.UserID.String(), Valid: true}
	}

	query := `INSERT INTO audit_logs (
		id, user_id, actor_name, actor_email, actor_role, action, category,
		target_type, target_id, target_name, description, ip_address, user_agent, metadata, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	var createdAtVal interface{}
	if s.driver == "sqlite" {
		createdAtVal = formatTimeForDB(entry.CreatedAt)
	} else {
		createdAtVal = entry.CreatedAt
	}

	_, err := s.db.Exec(query,
		entry.ID.String(),
		userIDStr,
		entry.ActorName,
		entry.ActorEmail,
		string(entry.ActorRole),
		entry.Action,
		string(entry.Category),
		entry.TargetType,
		entry.TargetID,
		entry.TargetName,
		entry.Description,
		entry.IPAddress,
		entry.UserAgent,
		entry.Metadata,
		createdAtVal,
	)
	return err
}

func (s *SQLStore) GetAuditLogs(filter models.AuditLogFilter) ([]*models.AuditLog, int, error) {
	var whereClauses []string
	var args []interface{}
	argIdx := 1

	if filter.Category != "" && strings.ToUpper(filter.Category) != "ALL" {
		whereClauses = append(whereClauses, fmt.Sprintf("UPPER(category) = UPPER($%d)", argIdx))
		args = append(args, filter.Category)
		argIdx++
	}

	if filter.Role != "" && strings.ToUpper(filter.Role) != "ALL" {
		whereClauses = append(whereClauses, fmt.Sprintf("UPPER(actor_role) = UPPER($%d)", argIdx))
		args = append(args, filter.Role)
		argIdx++
	}

	if filter.Action != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("UPPER(action) LIKE UPPER($%d)", argIdx))
		args = append(args, "%"+filter.Action+"%")
		argIdx++
	}

	if filter.StartDate != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, filter.StartDate+" 00:00:00")
		argIdx++
	}

	if filter.EndDate != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, filter.EndDate+" 23:59:59")
		argIdx++
	}

	if filter.Search != "" {
		searchPattern := "%" + strings.TrimSpace(filter.Search) + "%"
		clause := fmt.Sprintf("(actor_name LIKE $%d OR actor_email LIKE $%d OR action LIKE $%d OR target_name LIKE $%d OR description LIKE $%d OR ip_address LIKE $%d)",
			argIdx, argIdx, argIdx, argIdx, argIdx, argIdx)
		if s.driver != "sqlite" {
			clause = fmt.Sprintf("(actor_name ILIKE $%d OR actor_email ILIKE $%d OR action ILIKE $%d OR target_name ILIKE $%d OR description ILIKE $%d OR ip_address ILIKE $%d)",
				argIdx, argIdx, argIdx, argIdx, argIdx, argIdx)
		}
		whereClauses = append(whereClauses, clause)
		args = append(args, searchPattern)
		argIdx++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 1. Get total matching count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs %s", whereSQL)
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// 2. Query paginated results
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`SELECT id, user_id, actor_name, actor_email, actor_role, action, category,
		target_type, target_id, target_name, description, ip_address, user_agent, metadata, created_at
		FROM audit_logs %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		var idStr string
		var nullUserID sql.NullString
		var actRole, catStr string
		var createdVal interface{}
		entry := &models.AuditLog{}

		if err := rows.Scan(
			&idStr,
			&nullUserID,
			&entry.ActorName,
			&entry.ActorEmail,
			&actRole,
			&entry.Action,
			&catStr,
			&entry.TargetType,
			&entry.TargetID,
			&entry.TargetName,
			&entry.Description,
			&entry.IPAddress,
			&entry.UserAgent,
			&entry.Metadata,
			&createdVal,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}

		entry.ID = uuid.Must(uuid.Parse(idStr))
		if nullUserID.Valid && nullUserID.String != "" {
			uid, _ := uuid.Parse(nullUserID.String)
			entry.UserID = &uid
		}
		entry.ActorRole = models.UserRole(actRole)
		entry.Category = models.AuditCategory(catStr)
		entry.CreatedAt, _ = parseTimeFlex(createdVal)

		logs = append(logs, entry)
	}

	return logs, total, nil
}

func (s *SQLStore) GetAuditTelemetry() (*models.AuditTelemetry, error) {
	// Total logs
	var total int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM audit_logs").Scan(&total)

	// Today logs
	todayDate := time.Now().Format("2006-01-02")
	var todayCount int
	var todayQuery string
	if s.driver == "sqlite" {
		todayQuery = "SELECT COUNT(*) FROM audit_logs WHERE created_at LIKE $1"
		_ = s.db.QueryRow(todayQuery, todayDate+"%").Scan(&todayCount)
	} else {
		todayQuery = "SELECT COUNT(*) FROM audit_logs WHERE created_at::date = CURRENT_DATE"
		_ = s.db.QueryRow(todayQuery).Scan(&todayCount)
	}

	// Active users today
	var activeUsers int
	if s.driver == "sqlite" {
		_ = s.db.QueryRow("SELECT COUNT(DISTINCT CASE WHEN actor_email != '' THEN actor_email ELSE user_id END) FROM audit_logs WHERE created_at LIKE $1", todayDate+"%").Scan(&activeUsers)
	} else {
		_ = s.db.QueryRow("SELECT COUNT(DISTINCT CASE WHEN actor_email != '' THEN actor_email ELSE CAST(user_id AS TEXT) END) FROM audit_logs WHERE created_at::date = CURRENT_DATE").Scan(&activeUsers)
	}

	// Security / Critical Actions
	var secCount int
	secQuery := `SELECT COUNT(*) FROM audit_logs WHERE category IN ('AUTH', 'SAFETY')
		OR UPPER(action) LIKE '%DELETE%'
		OR UPPER(action) LIKE '%RESET%'
		OR UPPER(action) LIKE '%REVOKE%'
		OR UPPER(action) LIKE '%DEACTIVATE%'`
	_ = s.db.QueryRow(secQuery).Scan(&secCount)

	return &models.AuditTelemetry{
		TotalLogs:       total,
		TodayLogs:       todayCount,
		ActiveUsers:     activeUsers,
		SecurityActions: secCount,
	}, nil
}

func (s *SQLStore) GetNotificationSettings() (*models.NotificationSettings, error) {
	query := `SELECT id, push_enabled, sms_enabled, email_enabled,
		notify_admin_student_admitted, notify_admin_new_class_opened, notify_admin_staff_registered,
		notify_admin_student_injured, notify_admin_class_cancelled,
		notify_student_promotion_eligible, notify_student_injured, notify_student_evaluation_logged,
		notify_student_membership_changed, notify_student_safety_resolved, notify_student_class_cancelled,
		notify_student_pass_expiring,
		COALESCE(smtp_host, ''), COALESCE(smtp_port, 587), COALESCE(smtp_user, ''), COALESCE(smtp_password, ''), COALESCE(smtp_from, ''),
		COALESCE(sms_provider, 'simulated'), COALESCE(sms_api_key, ''), COALESCE(sms_from_number, ''),
		COALESCE(web_push_public_key, ''), COALESCE(web_push_private_key, ''), COALESCE(web_push_subject, ''),
		updated_at
		FROM notification_settings LIMIT 1`

	row := s.db.QueryRow(query)
	var idStr string
	var updatedVal interface{}
	settings := &models.NotificationSettings{}

	err := row.Scan(
		&idStr,
		&settings.PushEnabled,
		&settings.SMSEnabled,
		&settings.EmailEnabled,
		&settings.NotifyAdminStudentAdmitted,
		&settings.NotifyAdminNewClassOpened,
		&settings.NotifyAdminStaffRegistered,
		&settings.NotifyAdminStudentInjured,
		&settings.NotifyAdminClassCancelled,
		&settings.NotifyStudentPromotionEligible,
		&settings.NotifyStudentInjured,
		&settings.NotifyStudentEvaluationLogged,
		&settings.NotifyStudentMembershipChanged,
		&settings.NotifyStudentSafetyResolved,
		&settings.NotifyStudentClassCancelled,
		&settings.NotifyStudentPassExpiring,
		&settings.SMTPHost,
		&settings.SMTPPort,
		&settings.SMTPUser,
		&settings.SMTPPassword,
		&settings.SMTPFrom,
		&settings.SMSProvider,
		&settings.SMSApiKey,
		&settings.SMSFromNumber,
		&settings.WebPushPublicKey,
		&settings.WebPushPrivateKey,
		&settings.WebPushSubject,
		&updatedVal,
	)
	if err == sql.ErrNoRows {
		def := models.DefaultNotificationSettings()
		_ = s.UpdateNotificationSettings(def)
		return def, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query notification settings: %w", err)
	}

	settings.ID = uuid.Must(uuid.Parse(idStr))
	settings.UpdatedAt, _ = parseTimeFlex(updatedVal)
	return settings, nil
}

func (s *SQLStore) UpdateNotificationSettings(settings *models.NotificationSettings) error {
	if settings == nil {
		return errors.New("notification settings cannot be nil")
	}
	if settings.ID == uuid.Nil {
		settings.ID = uuid.New()
	}
	settings.UpdatedAt = time.Now()

	var updatedVal interface{}
	if s.driver == "sqlite" {
		updatedVal = formatTimeForDB(settings.UpdatedAt)
	} else {
		updatedVal = settings.UpdatedAt
	}

	query := `UPDATE notification_settings SET
		push_enabled = $1, sms_enabled = $2, email_enabled = $3,
		notify_admin_student_admitted = $4, notify_admin_new_class_opened = $5, notify_admin_staff_registered = $6,
		notify_admin_student_injured = $7, notify_admin_class_cancelled = $8,
		notify_student_promotion_eligible = $9, notify_student_injured = $10, notify_student_evaluation_logged = $11,
		notify_student_membership_changed = $12, notify_student_safety_resolved = $13, notify_student_class_cancelled = $14,
		notify_student_pass_expiring = $15,
		smtp_host = $16, smtp_port = $17, smtp_user = $18, smtp_password = $19, smtp_from = $20,
		sms_provider = $21, sms_api_key = $22, sms_from_number = $23,
		web_push_public_key = $24, web_push_private_key = $25, web_push_subject = $26,
		updated_at = $27
		WHERE id = $28`

	res, err := s.db.Exec(query,
		settings.PushEnabled, settings.SMSEnabled, settings.EmailEnabled,
		settings.NotifyAdminStudentAdmitted, settings.NotifyAdminNewClassOpened, settings.NotifyAdminStaffRegistered,
		settings.NotifyAdminStudentInjured, settings.NotifyAdminClassCancelled,
		settings.NotifyStudentPromotionEligible, settings.NotifyStudentInjured, settings.NotifyStudentEvaluationLogged,
		settings.NotifyStudentMembershipChanged, settings.NotifyStudentSafetyResolved, settings.NotifyStudentClassCancelled,
		settings.NotifyStudentPassExpiring,
		settings.SMTPHost, settings.SMTPPort, settings.SMTPUser, settings.SMTPPassword, settings.SMTPFrom,
		settings.SMSProvider, settings.SMSApiKey, settings.SMSFromNumber,
		settings.WebPushPublicKey, settings.WebPushPrivateKey, settings.WebPushSubject,
		updatedVal, settings.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update notification settings: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		insertQuery := `INSERT INTO notification_settings (
			id, push_enabled, sms_enabled, email_enabled,
			notify_admin_student_admitted, notify_admin_new_class_opened, notify_admin_staff_registered,
			notify_admin_student_injured, notify_admin_class_cancelled,
			notify_student_promotion_eligible, notify_student_injured, notify_student_evaluation_logged,
			notify_student_membership_changed, notify_student_safety_resolved, notify_student_class_cancelled,
			notify_student_pass_expiring,
			smtp_host, smtp_port, smtp_user, smtp_password, smtp_from,
			sms_provider, sms_api_key, sms_from_number,
			web_push_public_key, web_push_private_key, web_push_subject,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28
		)`
		_, err = s.db.Exec(insertQuery,
			settings.ID.String(),
			settings.PushEnabled, settings.SMSEnabled, settings.EmailEnabled,
			settings.NotifyAdminStudentAdmitted, settings.NotifyAdminNewClassOpened, settings.NotifyAdminStaffRegistered,
			settings.NotifyAdminStudentInjured, settings.NotifyAdminClassCancelled,
			settings.NotifyStudentPromotionEligible, settings.NotifyStudentInjured, settings.NotifyStudentEvaluationLogged,
			settings.NotifyStudentMembershipChanged, settings.NotifyStudentSafetyResolved, settings.NotifyStudentClassCancelled,
			settings.NotifyStudentPassExpiring,
			settings.SMTPHost, settings.SMTPPort, settings.SMTPUser, settings.SMTPPassword, settings.SMTPFrom,
			settings.SMSProvider, settings.SMSApiKey, settings.SMSFromNumber,
			settings.WebPushPublicKey, settings.WebPushPrivateKey, settings.WebPushSubject,
			updatedVal,
		)
		if err != nil {
			return fmt.Errorf("failed to insert notification settings: %w", err)
		}
	}
	return nil
}

func (s *SQLStore) GetUserNotificationPreferences(userID uuid.UUID) (*models.UserNotificationPreferences, error) {
	query := `SELECT user_id, email_enabled, sms_enabled, push_enabled, updated_at
		FROM user_notification_preferences WHERE user_id = $1`

	row := s.db.QueryRow(query, userID.String())
	var idStr string
	var updatedVal interface{}
	prefs := &models.UserNotificationPreferences{}

	err := row.Scan(&idStr, &prefs.EmailEnabled, &prefs.SMSEnabled, &prefs.PushEnabled, &updatedVal)
	if err == sql.ErrNoRows {
		return models.DefaultUserPreferences(userID), nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user notification preferences: %w", err)
	}

	prefs.UserID = uuid.Must(uuid.Parse(idStr))
	prefs.UpdatedAt, _ = parseTimeFlex(updatedVal)
	return prefs, nil
}

func (s *SQLStore) UpdateUserNotificationPreferences(prefs *models.UserNotificationPreferences) error {
	if prefs == nil {
		return errors.New("user preferences cannot be nil")
	}
	prefs.UpdatedAt = time.Now()

	var updatedVal interface{}
	if s.driver == "sqlite" {
		updatedVal = formatTimeForDB(prefs.UpdatedAt)
	} else {
		updatedVal = prefs.UpdatedAt
	}

	query := `INSERT INTO user_notification_preferences (user_id, email_enabled, sms_enabled, push_enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT(user_id) DO UPDATE SET
		email_enabled = EXCLUDED.email_enabled,
		sms_enabled = EXCLUDED.sms_enabled,
		push_enabled = EXCLUDED.push_enabled,
		updated_at = EXCLUDED.updated_at`

	_, err := s.db.Exec(query, prefs.UserID.String(), prefs.EmailEnabled, prefs.SMSEnabled, prefs.PushEnabled, updatedVal)
	if err != nil {
		return fmt.Errorf("failed to upsert user notification preferences: %w", err)
	}
	return nil
}

func (s *SQLStore) CreateNotification(n *models.Notification) error {
	if n == nil {
		return errors.New("notification cannot be nil")
	}
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}

	var userIDStr sql.NullString
	if n.UserID != nil && *n.UserID != uuid.Nil {
		userIDStr = sql.NullString{String: n.UserID.String(), Valid: true}
	}

	var createdAtVal interface{}
	if s.driver == "sqlite" {
		createdAtVal = formatTimeForDB(n.CreatedAt)
	} else {
		createdAtVal = n.CreatedAt
	}

	query := `INSERT INTO notifications (
		id, user_id, recipient_role, recipient_type, recipient_name, recipient_contact,
		channel, event_type, title, message, metadata, status, is_read, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := s.db.Exec(query,
		n.ID.String(),
		userIDStr,
		string(n.RecipientRole),
		n.RecipientType,
		n.RecipientName,
		n.RecipientContact,
		string(n.Channel),
		string(n.EventType),
		n.Title,
		n.Message,
		n.Metadata,
		string(n.Status),
		n.IsRead,
		createdAtVal,
	)
	if err != nil {
		return fmt.Errorf("failed to insert notification: %w", err)
	}
	return nil
}

func (s *SQLStore) GetNotifications(filter models.NotificationFilter) ([]*models.Notification, int, error) {
	var whereClauses []string
	var args []interface{}
	argIdx := 1

	if filter.UserID != nil && *filter.UserID != uuid.Nil {
		if filter.Role != nil && *filter.Role != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("(user_id = $%d OR (user_id IS NULL AND recipient_role = $%d))", argIdx, argIdx+1))
			args = append(args, filter.UserID.String(), string(*filter.Role))
			argIdx += 2
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("user_id = $%d", argIdx))
			args = append(args, filter.UserID.String())
			argIdx++
		}
	} else if filter.Role != nil && *filter.Role != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("recipient_role = $%d", argIdx))
		args = append(args, string(*filter.Role))
		argIdx++
	}

	if filter.Channel != nil && *filter.Channel != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("channel = $%d", argIdx))
		args = append(args, string(*filter.Channel))
		argIdx++
	}

	if filter.EventType != nil && *filter.EventType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("event_type = $%d", argIdx))
		args = append(args, string(*filter.EventType))
		argIdx++
	}

	if filter.UnreadOnly {
		if s.driver == "sqlite" {
			whereClauses = append(whereClauses, "is_read = 0")
		} else {
			whereClauses = append(whereClauses, "is_read = FALSE")
		}
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM notifications %s", whereSQL)
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`SELECT id, user_id, recipient_role, recipient_type, recipient_name, recipient_contact,
		channel, event_type, title, message, metadata, status, is_read, created_at
		FROM notifications %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	var notifs []*models.Notification
	for rows.Next() {
		var idStr string
		var nullUserID sql.NullString
		var roleStr, chanStr, evStr, statStr string
		var createdVal interface{}
		n := &models.Notification{}

		if err := rows.Scan(
			&idStr,
			&nullUserID,
			&roleStr,
			&n.RecipientType,
			&n.RecipientName,
			&n.RecipientContact,
			&chanStr,
			&evStr,
			&n.Title,
			&n.Message,
			&n.Metadata,
			&statStr,
			&n.IsRead,
			&createdVal,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan notification: %w", err)
		}

		n.ID = uuid.Must(uuid.Parse(idStr))
		if nullUserID.Valid && nullUserID.String != "" {
			uid, _ := uuid.Parse(nullUserID.String)
			n.UserID = &uid
		}
		n.RecipientRole = models.UserRole(roleStr)
		n.Channel = models.NotificationChannel(chanStr)
		n.EventType = models.NotificationEventType(evStr)
		n.Status = models.NotificationStatus(statStr)
		n.CreatedAt, _ = parseTimeFlex(createdVal)

		notifs = append(notifs, n)
	}
	return notifs, total, nil
}

func (s *SQLStore) GetUnreadNotificationCount(userID *uuid.UUID, role *models.UserRole) (int, error) {
	var whereClauses []string
	var args []interface{}
	argIdx := 1

	if s.driver == "sqlite" {
		whereClauses = append(whereClauses, "is_read = 0")
	} else {
		whereClauses = append(whereClauses, "is_read = FALSE")
	}

	if userID != nil && *userID != uuid.Nil {
		if role != nil && *role != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("(user_id = $%d OR (user_id IS NULL AND recipient_role = $%d))", argIdx, argIdx+1))
			args = append(args, userID.String(), string(*role))
			argIdx += 2
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("user_id = $%d", argIdx))
			args = append(args, userID.String())
			argIdx++
		}
	} else if role != nil && *role != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("recipient_role = $%d", argIdx))
		args = append(args, string(*role))
		argIdx++
	}

	whereSQL := "WHERE " + strings.Join(whereClauses, " AND ")
	query := fmt.Sprintf("SELECT COUNT(*) FROM notifications %s", whereSQL)

	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}
	return count, nil
}

func (s *SQLStore) MarkNotificationRead(id uuid.UUID) error {
	var query string
	if s.driver == "sqlite" {
		query = "UPDATE notifications SET is_read = 1 WHERE id = $1"
	} else {
		query = "UPDATE notifications SET is_read = TRUE WHERE id = $1"
	}

	res, err := s.db.Exec(query, id.String())
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) MarkAllNotificationsRead(userID *uuid.UUID, role *models.UserRole) error {
	var whereClauses []string
	var args []interface{}
	argIdx := 1

	if userID != nil && *userID != uuid.Nil {
		if role != nil && *role != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("(user_id = $%d OR (user_id IS NULL AND recipient_role = $%d))", argIdx, argIdx+1))
			args = append(args, userID.String(), string(*role))
			argIdx += 2
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("user_id = $%d", argIdx))
			args = append(args, userID.String())
			argIdx++
		}
	} else if role != nil && *role != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("recipient_role = $%d", argIdx))
		args = append(args, string(*role))
		argIdx++
	}

	setSQL := "is_read = 1"
	if s.driver != "sqlite" {
		setSQL = "is_read = TRUE"
	}

	query := fmt.Sprintf("UPDATE notifications SET %s", setSQL)
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications read: %w", err)
	}
	return nil
}
