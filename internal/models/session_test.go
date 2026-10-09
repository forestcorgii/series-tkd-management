package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTrainingSession_IsDone(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)

	tests := []struct {
		name        string
		session     TrainingSession
		refTime     time.Time
		wantDone    bool
		wantPastEnd bool
	}{
		{
			name: "Yesterday session is always done",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: yesterday,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:     now,
			wantDone:    true,
			wantPastEnd: true,
		},
		{
			name: "Tomorrow session is never done",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: tomorrow,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:     now,
			wantDone:    false,
			wantPastEnd: false,
		},
		{
			name: "Today session before end time",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: today,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:     time.Date(today.Year(), today.Month(), today.Day(), 17, 30, 0, 0, today.Location()),
			wantDone:    false,
			wantPastEnd: false,
		},
		{
			name: "Today session exactly at end time",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: today,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:     time.Date(today.Year(), today.Month(), today.Day(), 18, 30, 0, 0, today.Location()),
			wantDone:    true,
			wantPastEnd: true,
		},
		{
			name: "Today session after end time",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: today,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:     time.Date(today.Year(), today.Month(), today.Day(), 19, 00, 0, 0, today.Location()),
			wantDone:    true,
			wantPastEnd: true,
		},
		{
			name: "Cancelled session is not considered done (it is cancelled)",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: yesterday,
				StartTime:   "17:00",
				EndTime:     "18:30",
				IsCancelled: true,
			},
			refTime:     now,
			wantDone:    false,
			wantPastEnd: true,
		},
		{
			name: "12-hour AM/PM format support",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: today,
				StartTime:   "5:00 PM",
				EndTime:     "6:30 PM",
			},
			refTime:     time.Date(today.Year(), today.Month(), today.Day(), 18, 45, 0, 0, today.Location()),
			wantDone:    true,
			wantPastEnd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPast := tt.session.IsPastEndTimeAt(tt.refTime)
			if gotPast != tt.wantPastEnd {
				t.Errorf("IsPastEndTimeAt() = %v, want %v", gotPast, tt.wantPastEnd)
			}

			// For IsDone with reference time
			isDone := !tt.session.IsCancelled && gotPast
			if isDone != tt.wantDone {
				t.Errorf("IsDone() = %v, want %v", isDone, tt.wantDone)
			}
		})
	}
}

func TestTrainingSession_IsOpen(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)

	tests := []struct {
		name     string
		session  TrainingSession
		refTime  time.Time
		wantOpen bool
	}{
		{
			name: "Yesterday session is not open",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: yesterday,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:  now,
			wantOpen: false,
		},
		{
			name: "Tomorrow session is open",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: tomorrow,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:  now,
			wantOpen: true,
		},
		{
			name: "Today session before end time is open",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: today,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:  time.Date(today.Year(), today.Month(), today.Day(), 17, 30, 0, 0, today.Location()),
			wantOpen: true,
		},
		{
			name: "Today session at end time is not open",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: today,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:  time.Date(today.Year(), today.Month(), today.Day(), 18, 30, 0, 0, today.Location()),
			wantOpen: false,
		},
		{
			name: "Today session after end time is not open",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: today,
				StartTime:   "17:00",
				EndTime:     "18:30",
			},
			refTime:  time.Date(today.Year(), today.Month(), today.Day(), 19, 0, 0, 0, today.Location()),
			wantOpen: false,
		},
		{
			name: "Cancelled session is never open",
			session: TrainingSession{
				ID:          uuid.New(),
				SessionDate: tomorrow,
				StartTime:   "17:00",
				EndTime:     "18:30",
				IsCancelled: true,
			},
			refTime:  now,
			wantOpen: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOpen := tt.session.IsOpenAt(tt.refTime)
			if gotOpen != tt.wantOpen {
				t.Errorf("IsOpenAt() = %v, want %v", gotOpen, tt.wantOpen)
			}
		})
	}
}

func TestTrainingSession_EntryTypesAndStaff(t *testing.T) {
	classSess := TrainingSession{
		EntryType:    EntryTypeClass,
		TrainingType: TrainingSparring,
	}
	if !classSess.IsClass() || classSess.IsEvent() || classSess.IsDuty() || classSess.IsOpenSession() {
		t.Errorf("expected class session, got: %+v", classSess)
	}
	if classSess.DisplayTitle() != "Sparring" {
		t.Errorf("expected Sparring, got: %s", classSess.DisplayTitle())
	}

	eventSess := TrainingSession{
		EntryType: EntryTypeEvent,
		Title:     "Belt Promotion Exam",
	}
	if !eventSess.IsEvent() || eventSess.IsClass() {
		t.Errorf("expected event session, got: %+v", eventSess)
	}
	if eventSess.DisplayTitle() != "Belt Promotion Exam" {
		t.Errorf("expected 'Belt Promotion Exam', got: %s", eventSess.DisplayTitle())
	}

	dutySess := TrainingSession{
		EntryType: EntryTypeDuty,
		Title:     "Floor Supervision",
		AssignedStaff: []*SessionStaff{
			{ID: "staff-1", Name: "Coach Dan", Role: "Coach"},
			{ID: "staff-2", Name: "Admin Lisa", Role: "Admin"},
		},
	}
	if !dutySess.IsDuty() {
		t.Errorf("expected duty session, got: %+v", dutySess)
	}
	if !dutySess.HasAssignedStaffID("staff-1") || !dutySess.HasAssignedStaffID("staff-2") || dutySess.HasAssignedStaffID("unknown") {
		t.Errorf("HasAssignedStaffID check failed")
	}
	if dutySess.AssignedStaffNames() != "Coach Dan, Admin Lisa" {
		t.Errorf("expected 'Coach Dan, Admin Lisa', got: %s", dutySess.AssignedStaffNames())
	}

	openSess := TrainingSession{
		EntryType: EntryTypeOpenSession,
	}
	if !openSess.IsOpenSession() {
		t.Errorf("expected open session, got: %+v", openSess)
	}
	if openSess.DisplayTitle() != "Free / Open Session" {
		t.Errorf("expected 'Free / Open Session', got: %s", openSess.DisplayTitle())
	}
}

func TestAttendance_AttendeeHelpers(t *testing.T) {
	guestAtt := Attendance{
		AttendeeType: "guest",
		AttendeeName: "John Doe",
	}
	if !guestAtt.IsGuest() || guestAtt.IsStaff() {
		t.Errorf("expected guest attendee")
	}
	if guestAtt.DisplayName() != "John Doe" || guestAtt.DisplayRole() != "Guest" {
		t.Errorf("guest display mismatch: %s / %s", guestAtt.DisplayName(), guestAtt.DisplayRole())
	}

	coachAtt := Attendance{
		AttendeeType: "coach",
		AttendeeName: "Coach Dan",
		AttendeeRole: "Lead Instructor",
	}
	if !coachAtt.IsStaff() || coachAtt.IsGuest() {
		t.Errorf("expected staff attendee")
	}
	if coachAtt.DisplayName() != "Coach Dan" || coachAtt.DisplayRole() != "Lead Instructor" {
		t.Errorf("coach display mismatch: %s / %s", coachAtt.DisplayName(), coachAtt.DisplayRole())
	}

	studentAtt := Attendance{
		StudentID:   uuid.New(),
		StudentName: "Alice Smith",
		StudentBelt: BeltLowYellow,
	}
	if studentAtt.IsGuest() || studentAtt.IsStaff() {
		t.Errorf("expected regular student attendee")
	}
	if studentAtt.DisplayName() != "Alice Smith" || studentAtt.DisplayRole() != "Low Yellow" {
		t.Errorf("student display mismatch: %s / %s", studentAtt.DisplayName(), studentAtt.DisplayRole())
	}
}
