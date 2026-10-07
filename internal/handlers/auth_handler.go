package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

type contextKey string

const (
	UserContextKey   contextKey = "stms_auth_user"
	userContextKey              = UserContextKey
	sessionCookieKey string     = "stms_session"
)

func GetUserFromContext(ctx context.Context) *models.User {
	if u, ok := ctx.Value(userContextKey).(*models.User); ok {
		return u
	}
	return nil
}

func (a *AppHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieKey)
		if err == nil && cookie != nil && cookie.Value != "" {
			user, err := a.authSvc.ValidateSession(cookie.Value)
			if err == nil && user != nil {
				ctx := context.WithValue(r.Context(), userContextKey, user)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *AppHandler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("HX-Request") == "true" || strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "unauthorized",
					"message": "authentication required",
				})
				return
			}
			redirectURL := "/login"
			if r.URL.Path != "" && r.URL.Path != "/" {
				redirectURL += "?redirect=" + url.QueryEscape(r.URL.RequestURI())
			}
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (a *AppHandler) RequireRole(roles ...models.UserRole) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return a.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil || !user.HasRole(roles...) {
				if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("HX-Request") == "true" || strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":   "forbidden",
						"message": "insufficient permissions for this resource",
					})
					return
				}
				// If browser request, send to their authorized view
				if user != nil {
					switch user.Role {
					case models.RoleOperationManager:
						http.Redirect(w, r, "/", http.StatusSeeOther)
						return
					case models.RoleAdmin:
						http.Redirect(w, r, "/", http.StatusSeeOther)
						return
					case models.RoleCoach:
						http.Redirect(w, r, "/", http.StatusSeeOther)
						return
					case models.RoleStudent:
						http.Redirect(w, r, "/portal/student", http.StatusSeeOther)
						return
					}
				}
				http.Error(w, "Forbidden: Insufficient Permissions", http.StatusForbidden)
				return
			}
			next(w, r)
		})
	}
}

// IsDemoLoginEnabled returns whether quick demo logins should be displayed.
// By default, demo login is enabled in development/local environments and disabled in production.
// It can be explicitly overridden via SHOW_DEMO_LOGIN or ENABLE_DEMO_LOGIN ("true"/"false").
func IsDemoLoginEnabled() bool {
	if val := os.Getenv("SHOW_DEMO_LOGIN"); val != "" {
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "true" || val == "1" || val == "yes"
	}
	if val := os.Getenv("ENABLE_DEMO_LOGIN"); val != "" {
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "true" || val == "1" || val == "yes"
	}

	// Environment variable checks (production vs dev)
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("GO_ENV")))
	}
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	}

	if env == "production" || env == "prod" {
		return false
	}

	// Railway or cloud deployment detection
	if os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("RAILWAY_ENVIRONMENT_NAME") != "" {
		return false
	}

	// In this system, DATABASE_URL with postgres denotes production deployment
	// unless explicitly tagged as development/dev/local.
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" && (strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://")) {
		if env != "development" && env != "dev" && env != "local" {
			return false
		}
	}

	return true
}

func (a *AppHandler) IsDemoLoginEnabled() bool {
	return IsDemoLoginEnabled()
}

type LoginPageData struct {
	CurrentUser   *models.User
	Error         string
	Success       string
	Redirect      string
	ShowDemoLogin bool
}

func (a *AppHandler) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user != nil {
		switch user.Role {
		case models.RoleOperationManager, models.RoleAdmin, models.RoleCoach:
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		case models.RoleStudent:
			http.Redirect(w, r, "/portal/student", http.StatusSeeOther)
			return
		default:
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	data := LoginPageData{
		Error:         r.URL.Query().Get("error"),
		Success:       r.URL.Query().Get("success"),
		Redirect:      r.URL.Query().Get("redirect"),
		ShowDemoLogin: a.IsDemoLoginEnabled(),
	}

	a.RenderPage(w, "login.html", data)
}


func (a *AppHandler) HandleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=Invalid+form+submission", http.StatusSeeOther)
		return
	}

	identifier := strings.TrimSpace(r.FormValue("identifier"))
	if identifier == "" {
		identifier = strings.TrimSpace(r.FormValue("email"))
	}
	if identifier == "" {
		identifier = strings.TrimSpace(r.FormValue("username"))
	}
	password := r.FormValue("password")
	redirectTarget := r.FormValue("redirect")

	user, token, err := a.authSvc.Login(identifier, password)
	if err != nil {
		msg := "Invalid email or password"
		if errors.Is(err, services.ErrAccountPendingApproval) {
			msg = "Your account is pending manager approval. Please wait for an Operations Manager to review and approve your registration."
		} else if errors.Is(err, services.ErrUserInactive) {
			msg = "Account is inactive. Please contact your dojang administrator."
		}
		http.Redirect(w, r, "/login?error="+url.QueryEscape(msg), http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieKey,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})

	a.LogAction(r, "AUTH_LOGIN", models.AuditCategoryAuth, "User", user.ID.String(), user.DisplayName, "User "+user.Email+" signed in successfully")

	if redirectTarget != "" && strings.HasPrefix(redirectTarget, "/") {
		http.Redirect(w, r, redirectTarget, http.StatusSeeOther)
		return
	}

	switch user.Role {
	case models.RoleOperationManager, models.RoleAdmin, models.RoleCoach:
		http.Redirect(w, r, "/", http.StatusSeeOther)
	case models.RoleStudent:
		http.Redirect(w, r, "/portal/student", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (a *AppHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieKey)
	if err == nil && cookie != nil {
		_ = a.authSvc.Logout(cookie.Value)
	}

	a.LogAction(r, "AUTH_LOGOUT", models.AuditCategoryAuth, "User", "", "", "User signed out")

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieKey,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	http.Redirect(w, r, "/login?success="+url.QueryEscape("You have been signed out successfully."), http.StatusSeeOther)
}

// REST API Endpoints
type LoginRequest struct {
	Identifier string `json:"identifier"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type LoginResponse struct {
	Status string       `json:"status"`
	Token  string       `json:"token"`
	User   *models.User `json:"user"`
}

func (a *AppHandler) HandleAPILogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	identifier := req.Identifier
	if identifier == "" {
		identifier = req.Email
	}
	if identifier == "" {
		identifier = req.Username
	}

	user, token, err := a.authSvc.Login(identifier, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieKey,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoginResponse{
		Status: "ok",
		Token:  token,
		User:   user,
	})
}

func (a *AppHandler) HandleAPIMe(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
	})
}

func (a *AppHandler) HandleAPILogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieKey)
	if err == nil && cookie != nil {
		_ = a.authSvc.Logout(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieKey,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "logged out",
	})
}

type RegisterRequest struct {
	Email     string          `json:"email"`
	Username  string          `json:"username"`
	Password  string          `json:"password"`
	Role      models.UserRole `json:"role"`
	StudentID *uuid.UUID      `json:"student_id,omitempty"`
	CoachID   *uuid.UUID      `json:"coach_id,omitempty"`
}

func (a *AppHandler) HandleAPIRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Role == "" {
		req.Role = models.RoleStudent
	}

	user, err := a.authSvc.RegisterUser(req.Email, req.Password, req.Role, req.StudentID, req.CoachID, req.Username)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "created",
		"user":   user,
	})
}

type RegisterPageData struct {
	CurrentUser  *models.User
	Error        string
	Success      string
	SelectedRole string
}

func (a *AppHandler) HandleRegisterPage(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	role := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("role")))
	if role != "ADMIN" {
		role = "COACH"
	}

	data := RegisterPageData{
		CurrentUser:  nil,
		Error:        r.URL.Query().Get("error"),
		Success:      r.URL.Query().Get("success"),
		SelectedRole: role,
	}

	a.RenderPage(w, "register.html", data)
}

func (a *AppHandler) HandleRegisterSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
		strings.HasPrefix(r.URL.Path, "/api/")

	var role, fullName, email, username, password, confirmPassword, phone, beltRank, specialtiesStr string
	var firstAid bool

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			Role              string   `json:"role"`
			FullName          string   `json:"full_name"`
			Email             string   `json:"email"`
			Username          string   `json:"username"`
			Password          string   `json:"password"`
			ConfirmPassword   string   `json:"confirm_password"`
			Phone             string   `json:"phone"`
			BeltRank          string   `json:"belt_rank"`
			Specialties       []string `json:"specialties"`
			FirstAidCertified bool     `json:"first_aid_certified"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if isJSON {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body: " + err.Error()})
				return
			}
			http.Redirect(w, r, "/register?error="+url.QueryEscape("Invalid request payload"), http.StatusSeeOther)
			return
		}
		role = req.Role
		fullName = req.FullName
		email = req.Email
		username = req.Username
		password = req.Password
		confirmPassword = req.ConfirmPassword
		phone = req.Phone
		beltRank = req.BeltRank
		firstAid = req.FirstAidCertified
		specialtiesStr = strings.Join(req.Specialties, ",")
	} else {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/register?error="+url.QueryEscape("Invalid form submission"), http.StatusSeeOther)
			return
		}
		role = r.FormValue("role")
		fullName = r.FormValue("full_name")
		email = r.FormValue("email")
		username = r.FormValue("username")
		password = r.FormValue("password")
		confirmPassword = r.FormValue("confirm_password")
		phone = r.FormValue("phone")
		beltRank = r.FormValue("belt_rank")
		specialtiesStr = r.FormValue("specialties")
		firstAid = r.FormValue("first_aid_certified") == "on" || r.FormValue("first_aid_certified") == "true"
	}

	role = strings.ToUpper(strings.TrimSpace(role))
	if role != "COACH" && role != "ADMIN" {
		role = "COACH"
	}

	fail := func(msg string) {
		if isJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
			return
		}
		http.Redirect(w, r, "/register?error="+url.QueryEscape(msg)+"&role="+url.QueryEscape(role), http.StatusSeeOther)
	}

	if strings.TrimSpace(fullName) == "" {
		fail("Full name is required.")
		return
	}
	if strings.TrimSpace(email) == "" || !strings.Contains(email, "@") {
		fail("A valid email address is required.")
		return
	}
	if len(password) < 6 {
		fail("Password must be at least 6 characters long.")
		return
	}
	if confirmPassword != "" && password != confirmPassword {
		fail("Passwords do not match.")
		return
	}

	var specialties []string
	if specialtiesStr != "" {
		for _, s := range strings.Split(specialtiesStr, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				specialties = append(specialties, trimmed)
			}
		}
	}

	if role == "COACH" {
		if strings.TrimSpace(phone) == "" {
			fail("Phone number is required for coach registration.")
			return
		}
		if strings.TrimSpace(beltRank) == "" {
			beltRank = "1st Dan Black Belt"
		}
		_, _, err := a.authSvc.RegisterPendingCoach(fullName, email, username, password, phone, beltRank, specialties, firstAid)
		if err != nil {
			fail(err.Error())
			return
		}
	} else if role == "ADMIN" {
		_, err := a.authSvc.RegisterPendingAdmin(fullName, email, username, password)
		if err != nil {
			fail(err.Error())
			return
		}
	}

	a.LogAction(r, "AUTH_STAFF_REGISTER", models.AuditCategoryAuth, "User", "", fullName, "New "+role+" account registered (pending approval): "+email)

	successMsg := "Registration submitted successfully! Your account is pending manager approval. You can log in once approved."
	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "pending_approval",
			"message": successMsg,
			"role":    role,
		})
		return
	}

	http.Redirect(w, r, "/login?success="+url.QueryEscape(successMsg), http.StatusSeeOther)
}

// Forgot Password & Reset Password Web Views and APIs

type ForgotPasswordPageData struct {
	CurrentUser *models.User
	Error       string
	Success     string
}

func (a *AppHandler) HandleForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := ForgotPasswordPageData{
		CurrentUser: nil,
		Error:       r.URL.Query().Get("error"),
		Success:     r.URL.Query().Get("success"),
	}

	a.RenderPage(w, "forgot_password.html", data)
}

func (a *AppHandler) HandleForgotPasswordSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/forgot-password?error=Invalid+form+submission", http.StatusSeeOther)
		return
	}

	identifier := strings.TrimSpace(r.FormValue("identifier"))
	if identifier == "" {
		identifier = strings.TrimSpace(r.FormValue("email"))
	}
	if identifier == "" {
		identifier = strings.TrimSpace(r.FormValue("username"))
	}

	if identifier == "" {
		http.Redirect(w, r, "/forgot-password?error="+url.QueryEscape("Please enter your registered email address or username."), http.StatusSeeOther)
		return
	}

	_, _ = a.authSvc.RequestPasswordReset(identifier)

	msg := "If an account with that email or username exists, instructions have been sent to reset your password."
	http.Redirect(w, r, "/forgot-password?success="+url.QueryEscape(msg), http.StatusSeeOther)
}

type ResetPasswordPageData struct {
	CurrentUser *models.User
	Error       string
	Token       string
}

func (a *AppHandler) HandleResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Redirect(w, r, "/forgot-password?error="+url.QueryEscape("Password reset token is missing."), http.StatusSeeOther)
		return
	}

	_, _, err := a.authSvc.ValidatePasswordResetToken(token)
	if err != nil {
		http.Redirect(w, r, "/forgot-password?error="+url.QueryEscape("This password reset link is invalid or has expired. Please request a new one."), http.StatusSeeOther)
		return
	}

	data := ResetPasswordPageData{
		CurrentUser: GetUserFromContext(r.Context()),
		Error:       r.URL.Query().Get("error"),
		Token:       token,
	}

	a.RenderPage(w, "reset_password.html", data)
}

func (a *AppHandler) HandleResetPasswordSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/forgot-password?error=Invalid+form+submission", http.StatusSeeOther)
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	newPassword := strings.TrimSpace(r.FormValue("new_password"))
	confirmPassword := strings.TrimSpace(r.FormValue("confirm_password"))

	if token == "" {
		http.Redirect(w, r, "/forgot-password?error="+url.QueryEscape("Password reset token is missing."), http.StatusSeeOther)
		return
	}

	if len(newPassword) < 6 {
		http.Redirect(w, r, "/reset-password?token="+url.QueryEscape(token)+"&error="+url.QueryEscape("Password must be at least 6 characters long."), http.StatusSeeOther)
		return
	}

	if newPassword != confirmPassword {
		http.Redirect(w, r, "/reset-password?token="+url.QueryEscape(token)+"&error="+url.QueryEscape("Passwords do not match."), http.StatusSeeOther)
		return
	}

	_, err := a.authSvc.ResetPassword(token, newPassword)
	if err != nil {
		http.Redirect(w, r, "/forgot-password?error="+url.QueryEscape("Unable to reset password: "+err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "AUTH_RESET_PASSWORD", models.AuditCategoryAuth, "User", "", "", "Password reset successfully via reset token")

	http.Redirect(w, r, "/login?success="+url.QueryEscape("Your password has been reset successfully! You can now sign in with your new credentials."), http.StatusSeeOther)
}

type ForgotPasswordRequest struct {
	Identifier string `json:"identifier"`
	Email      string `json:"email"`
	Username   string `json:"username"`
}

func (a *AppHandler) HandleAPIForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	id := req.Identifier
	if id == "" {
		id = req.Email
	}
	if id == "" {
		id = req.Username
	}

	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Identifier (email or username) is required"})
		return
	}

	_, _ = a.authSvc.RequestPasswordReset(id)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "If an account exists, instructions have been sent to reset your password.",
	})
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (a *AppHandler) HandleAPIResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if len(req.Password) < 6 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 6 characters long"})
		return
	}

	_, err := a.authSvc.ResetPassword(req.Token, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Password reset successfully",
	})
}
