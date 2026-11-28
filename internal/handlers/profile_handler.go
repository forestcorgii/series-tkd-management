package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

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

	// Cap total request body to 6MB to prevent disk/memory spill DoS
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)

	// Parse multipart form up to 5MB to handle photo uploads
	if err := r.ParseMultipartForm(5 << 20); err != nil && err != http.ErrNotMultipart {
		data.ErrorMessage = "Failed to process form or upload exceeded 5MB limit."
		a.RenderPage(w, "profile.html", data)
		return
	}

	// Handle Avatar Removal
	if r.FormValue("remove_avatar") == "1" || r.FormValue("remove_avatar") == "true" {
		if user.ProfilePictureURL != "" {
			oldURL := user.ProfilePictureURL
			user.ProfilePictureURL = ""
			if err := a.store.UpdateUser(user); err != nil {
				data.ErrorMessage = "Failed to remove profile photo: " + err.Error()
				a.RenderPage(w, "profile.html", data)
				return
			}
			if a.storage != nil {
				_ = a.storage.Delete(r.Context(), oldURL)
			}
			a.LogAction(r, "PROFILE_AVATAR_REMOVE", models.AuditCategorySettings, "User", user.ID.String(), user.DisplayName, "User removed their profile picture")
		}
	}

	// Handle Avatar Upload
	avatarFile, avatarHeader, err := r.FormFile("avatar")
	if err == nil && avatarFile != nil {
		defer avatarFile.Close()

		if avatarHeader.Size > 5<<20 {
			data.ErrorMessage = "Profile photo exceeds maximum limit of 5MB."
			a.RenderPage(w, "profile.html", data)
			return
		}

		ext := strings.ToLower(filepath.Ext(avatarHeader.Filename))
		allowedExts := map[string]string{
			".jpg":  "image/jpeg",
			".jpeg": "image/jpeg",
			".png":  "image/png",
			".webp": "image/webp",
			".gif":  "image/gif",
		}
		contentType, valid := allowedExts[ext]
		if !valid {
			data.ErrorMessage = "Invalid image file type. Please upload a JPG, PNG, WEBP, or GIF image."
			a.RenderPage(w, "profile.html", data)
			return
		}

		// Sniff magic bytes to prevent extension spoofing
		head := make([]byte, 512)
		n, readErr := io.ReadFull(avatarFile, head)
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			data.ErrorMessage = "Failed to read uploaded image."
			a.RenderPage(w, "profile.html", data)
			return
		}
		detectedType := http.DetectContentType(head[:n])
		if detectedType != "image/jpeg" && detectedType != "image/png" && detectedType != "image/webp" && detectedType != "image/gif" {
			data.ErrorMessage = "Uploaded file content is not a valid JPG, PNG, WEBP, or GIF image."
			a.RenderPage(w, "profile.html", data)
			return
		}
		contentType = detectedType
		uploadReader := io.MultiReader(bytes.NewReader(head[:n]), avatarFile)

		if a.storage != nil {
			oldURL := user.ProfilePictureURL
			key := fmt.Sprintf("avatars/user-%s-%d%s", user.ID.String(), time.Now().Unix(), ext)
			uploadURL, err := a.storage.Upload(r.Context(), key, uploadReader, contentType)
			if err != nil {
				data.ErrorMessage = "Failed to upload profile picture."
				a.RenderPage(w, "profile.html", data)
				return
			}
			user.ProfilePictureURL = uploadURL
			if err := a.store.UpdateUser(user); err != nil {
				_ = a.storage.Delete(r.Context(), uploadURL)
				data.ErrorMessage = "Failed to save profile picture URL."
				a.RenderPage(w, "profile.html", data)
				return
			}
			if oldURL != "" && oldURL != uploadURL {
				_ = a.storage.Delete(r.Context(), oldURL)
			}
			a.LogAction(r, "PROFILE_AVATAR_UPLOAD", models.AuditCategorySettings, "User", user.ID.String(), user.DisplayName, "User uploaded a new profile picture")
		}
	}

	// Process updates based on user role
	currentPassword := r.FormValue("current_password")
	newPassword := strings.TrimSpace(r.FormValue("new_password"))
	confirmPassword := strings.TrimSpace(r.FormValue("confirm_password"))
	if newPassword != "" {
		if currentPassword == "" || !user.CheckPassword(currentPassword) {
			data.ErrorMessage = "Current password is incorrect."
			a.RenderPage(w, "profile.html", data)
			return
		}
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
			data.ErrorMessage = "Failed to hash new password."
			a.RenderPage(w, "profile.html", data)
			return
		}
		if err := a.store.UpdateUser(user); err != nil {
			data.ErrorMessage = "Failed to update account credentials."
			a.RenderPage(w, "profile.html", data)
			return
		}
		// Revoke all other sessions and issue a fresh token for the current session
		_ = a.store.DeleteUserSessions(user.ID)
		if newToken, tokErr := a.authSvc.CreateSessionForUser(user.ID); tokErr == nil && newToken != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieKey,
				Value:    newToken,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   IsHTTPS(r),
				MaxAge:   7 * 24 * 60 * 60,
			})
		}
		a.LogAction(r, "PROFILE_PASSWORD_CHANGE", models.AuditCategorySettings, "User", user.ID.String(), user.DisplayName, "User updated their account password")
	}

	userUpdated := false
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
		if displayName != "" && displayName != user.DisplayName {
			user.DisplayName = displayName
			userUpdated = true
		}
	}

	// Process username update (available to all roles)
	if _, hasUsername := r.Form["username"]; hasUsername || r.FormValue("username") != "" {
		reqUsername := strings.ToLower(strings.TrimSpace(r.FormValue("username")))
		if reqUsername != strings.ToLower(strings.TrimSpace(user.Username)) {
			if reqUsername != "" {
				if len(reqUsername) < 3 || len(reqUsername) > 30 {
					data.ErrorMessage = "Username must be between 3 and 30 characters long."
					a.RenderPage(w, "profile.html", data)
					return
				}
				for _, ch := range reqUsername {
					if !(ch >= 'a' && ch <= 'z') && !(ch >= '0' && ch <= '9') && ch != '.' && ch != '_' && ch != '-' {
						data.ErrorMessage = "Username can only contain letters, numbers, dots, hyphens, and underscores."
						a.RenderPage(w, "profile.html", data)
						return
					}
				}
				existing, _ := a.store.GetUserByUsername(reqUsername)
				if existing != nil && existing.ID != user.ID {
					data.ErrorMessage = fmt.Sprintf("Username '%s' is already taken. Please choose another.", reqUsername)
					a.RenderPage(w, "profile.html", data)
					return
				}
			}
			user.Username = reqUsername
			userUpdated = true
			a.LogAction(r, "PROFILE_USERNAME_CHANGE", models.AuditCategorySettings, "User", user.ID.String(), user.DisplayName, "User updated their username to "+reqUsername)
		}
	}

	if userUpdated {
		if err := a.store.UpdateUser(user); err != nil {
			data.ErrorMessage = "Failed to update user profile: " + err.Error()
			a.RenderPage(w, "profile.html", data)
			return
		}
	}

	// Update personal notification preferences if present in form
	if r.FormValue("has_notif_prefs") == "1" {
		isPrefChecked := func(fieldName string) bool {
			val := strings.TrimSpace(r.FormValue(fieldName))
			return val == "on" || val == "true" || val == "1"
		}
		data.Preferences.EmailEnabled = isPrefChecked("pref_email")
		_ = a.notifSvc.UpdateUserPreferences(data.Preferences)
	}

	data.SuccessNotice = "Profile updated successfully!"
	data.CurrentUser = user
	a.LogAction(r, "PROFILE_UPDATE", models.AuditCategorySettings, "User", user.ID.String(), user.DisplayName, "User updated their profile information")
	a.RenderPage(w, "profile.html", data)
}
