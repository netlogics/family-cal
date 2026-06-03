package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/smerud/family-cal/internal/auth"
	"github.com/smerud/family-cal/internal/calendar"
	"github.com/smerud/family-cal/internal/user"
)

func NewRouter(authSvc *auth.Service, userSvc *user.Service, calSvc *calendar.Service) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/api/auth/login", authSvc.HandleLogin)
	r.Post("/api/auth/logout", authSvc.HandleLogout)
	r.Post("/api/auth/setup", authSvc.HandleSetup)

	r.Group(func(r chi.Router) {
		r.Use(authSvc.Middleware)

		r.Get("/api/auth/me", authSvc.HandleMe)

		r.Get("/api/calendars", calSvc.HandleListCalendars)
		r.Post("/api/calendars", auth.AdminOnly(http.HandlerFunc(calSvc.HandleCreateCalendar)).ServeHTTP)
		r.Put("/api/calendars/{id}", auth.AdminOnly(http.HandlerFunc(calSvc.HandleUpdateCalendar)).ServeHTTP)
		r.Delete("/api/calendars/{id}", auth.AdminOnly(http.HandlerFunc(calSvc.HandleDeleteCalendar)).ServeHTTP)

		r.Get("/api/events", calSvc.HandleList)
		r.Post("/api/events", calSvc.HandleCreate)
		r.Get("/api/events/{id}", calSvc.HandleGet)
		r.Put("/api/events/{id}", calSvc.HandleUpdate)
		r.Delete("/api/events/{id}", calSvc.HandleDelete)

		r.Get("/api/users", auth.AdminOnly(http.HandlerFunc(userSvc.HandleList)).ServeHTTP)
		r.Post("/api/users", auth.AdminOnly(http.HandlerFunc(userSvc.HandleCreate)).ServeHTTP)
		r.Put("/api/users/{id}", userSvc.HandleUpdate)
		r.Delete("/api/users/{id}", auth.AdminOnly(http.HandlerFunc(userSvc.HandleDelete)).ServeHTTP)
		r.Put("/api/users/me/preferences", userSvc.HandleUpdatePreferences)
		r.Put("/api/users/me/password", userSvc.HandleChangePassword)
	})

	return r
}
