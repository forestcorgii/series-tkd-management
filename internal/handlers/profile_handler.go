package handlers

import (
	"net/http"
	"strings"

	"series-tkd-management/internal/models"
)

type ProfileViewData struct {
	CurrentUser   *models.User
	Student       *models.Student
	Coach         *models.Coach
	Preferences   *models.UserNotificationPreferences
	SuccessNotice string
	ErrorMessage  string
}

func (a *AppHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var student *models.Student
	var coach *models.Coach

	if user.IsStudent() && user.StudentID != nil {
		student, _ = a.store.GetStudentByID(*user.StudentID)
	} else if user.IsCoach() && user.CoachID != nil {
		coach, _ = a.store.GetCoachByID(*user.CoachID)
	}

	prefs, _ := a.notifSvc.GetUserPreferences(user.ID)
	if prefs == nil {
		prefs = models.DefaultUserPreferences(user.ID)
	}

	data := ProfileViewData{
		CurrentUser: user,
		Student:     student,
		Coach:       coach,
		Preferences: prefs,
	}

	if r.Method == http.MethodGet {
		a.RenderPage(w, "profile.html", data)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Process updates based on user role
	newPassword := strings.TrimSpace(r.FormValue("new_password"))
	confirmPassword := strings.TrimSpace(r.FormValue("confirm_password"))
	if newPassword != "" {
		if len(newPassword) < 6 {
			data.ErrorMessage = "Password must be at least 6 characters long."
			a.RenderPage(w, "profile.html", data)
			return
		}
		if newPassword != confirmPassword {
			data.ErrorMessage = "Password confirmation does not match."
			a.RenderPage(w, "profile.html", data)
			return
		}
		if err := user.SetPassword(newPassword); err != nil {
			data.ErrorMessage = "Failed to hash new password: " + err.Error()
			a.RenderPage(w, "profile.html", data)
			return
		}
		if err := a.store.UpdateUser(user); err != nil {
			data.ErrorMessage = "Failed to update account credentials: " + err.Error()
			a.RenderPage(w, "profile.html", data)
			return
		}
		a.LogAction(r, "PROFILE_PASSWORD_CHANGE", models.AuditCategorySettings, "User", user.ID.String(), user.DisplayName, "User updated their account password")
	}

	if user.IsStudent() && student != nil {
		phone := strings.TrimSpace(r.FormValue("phone"))
		gender := strings.TrimSpace(r.FormValue("gender"))
		emergencyName := strings.TrimSpace(r.FormValue("emergency_name"))
		emergencyPhone := strings.TrimSpace(r.FormValue("emergency_phone"))
		emergencyRelation := strings.TrimSpace(r.FormValue("emergency_relation"))
		medicalNotes := strings.TrimSpace(r.FormValue("medical_notes"))

		if emergencyName == "" || emergencyPhone == "" {
			data.ErrorMessage = "Emergency contact name and phone number are required."
			a.RenderPage(w, "profile.html", data)
			return
		}

		student.Phone = phone
		if gender != "" {
			student.Gender = gender
		}
		student.EmergencyName = emergencyName
		student.EmergencyPhone = emergencyPhone
		student.EmergencyRelation = emergencyRelation
		student.MedicalNotes = medicalNotes

		if err := a.store.UpdateStudent(student); err != nil {
			data.ErrorMessage = "Failed to update student profile: " + err.Error()
			a.RenderPage(w, "profile.html", data)
			return
		}
		data.Student = student
	} else if user.IsCoach() && coach != nil {
		phone := strings.TrimSpace(r.FormValue("phone"))
		specsRaw := r.FormValue("specialties")

		var specs []string
		for _, s := range strings.Split(specsRaw, ",") {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				specs = append(specs, trimmed)
			}
		}

		coach.Phone = phone
		coach.Specialties = specs

		if err := a.store.UpdateCoach(coach); err != nil {
			data.ErrorMessage = "Failed to update coach profile: " + err.Error()
			a.RenderPage(w, "profile.html", data)
			return
		}
		data.Coach = coach
	} else if user.IsAdmin() || user.IsOperationManager() {
		displayName := strings.TrimSpace(r.FormValue("display_name"))
		if displayName != "" {
			user.DisplayName = displayName
			if err := a.store.UpdateUser(user); err != nil {
				data.ErrorMessage = "Failed to update display name: " + err.Error()
				a.RenderPage(w, "profile.html", data)
				return
			}
		}
	}

	// Update personal notification preferences if present in form
	if r.FormValue("has_notif_prefs") == "1" {
		isPrefChecked := func(fieldName string) bool {
			val := strings.TrimSpace(r.FormValue(fieldName))
			return val == "on" || val == "true" || val == "1"
		}
		data.Preferences.EmailEnabled = isPrefChecked("pref_email")
		data.Preferences.SMSEnabled = isPrefChecked("pref_sms")
		data.Preferences.PushEnabled = isPrefChecked("pref_push")
		_ = a.notifSvc.UpdateUserPreferences(data.Preferences)
	}

	data.SuccessNotice = "Profile updated successfully!"
	data.CurrentUser = user
	a.LogAction(r, "PROFILE_UPDATE", models.AuditCategorySettings, "User", user.ID.String(), user.DisplayName, "User updated their profile information")
	a.RenderPage(w, "profile.html", data)
}
