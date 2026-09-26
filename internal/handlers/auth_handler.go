package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

type contextKey string

const (
	userContextKey   contextKey = "stms_auth_user"
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
			if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("HX-Request") == "true" {
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
				if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("HX-Request") == "true" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":   "forbidden",
						"message": "insufficient permissions for this resource",
					})
					return
				}
				// If browser request, send to their own portal
				if user != nil {
					switch user.Role {
					case models.RoleAdmin:
						http.Redirect(w, r, "/portal/admin", http.StatusSeeOther)
						return
					case models.RoleCoach:
						http.Redirect(w, r, "/portal/coach", http.StatusSeeOther)
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

type LoginPageData struct {
	CurrentUser *models.User
	Error       string
	Success     string
	Redirect    string
}

func (a *AppHandler) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user != nil {
		switch user.Role {
		case models.RoleAdmin:
			http.Redirect(w, r, "/portal/admin", http.StatusSeeOther)
			return
		case models.RoleCoach:
			http.Redirect(w, r, "/portal/coach", http.StatusSeeOther)
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
		Error:    r.URL.Query().Get("error"),
		Success:  r.URL.Query().Get("success"),
		Redirect: r.URL.Query().Get("redirect"),
	}

	a.RenderPage(w, "login.html", data)
}


func (a *AppHandler) HandleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=Invalid+form+submission", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	redirectTarget := r.FormValue("redirect")

	user, token, err := a.authSvc.Login(email, password)
	if err != nil {
		msg := "Invalid email or password"
		if errors.Is(err, services.ErrUserInactive) {
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

	if redirectTarget != "" && strings.HasPrefix(redirectTarget, "/") {
		http.Redirect(w, r, redirectTarget, http.StatusSeeOther)
		return
	}

	switch user.Role {
	case models.RoleAdmin:
		http.Redirect(w, r, "/portal/admin", http.StatusSeeOther)
	case models.RoleCoach:
		http.Redirect(w, r, "/portal/coach", http.StatusSeeOther)
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
	Email    string `json:"email"`
	Password string `json:"password"`
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

	user, token, err := a.authSvc.Login(req.Email, req.Password)
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

	user, err := a.authSvc.RegisterUser(req.Email, req.Password, req.Role, req.StudentID, req.CoachID)
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
