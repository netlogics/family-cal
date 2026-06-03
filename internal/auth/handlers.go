package auth

import (
	"encoding/json"
	"net/http"
)

func (s *Service) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, token, err := s.Login(req.Email, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	s.SetCookie(w, token)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userResponse(u))
}

func (s *Service) HandleLogout(w http.ResponseWriter, r *http.Request) {
	s.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) HandleMe(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userResponse(u))
}

func (s *Service) HandleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, err := s.CreateAdmin(req.Token, req.Email, req.Name, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	token, err := s.issueToken(u.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.SetCookie(w, token)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(userResponse(u))
}

func userResponse(u *User) map[string]interface{} {
	return map[string]interface{}{
		"id":        u.ID,
		"email":     u.Email,
		"name":      u.Name,
		"color":     u.Color,
		"is_admin":  u.IsAdmin,
		"dark_mode": u.DarkMode,
	}
}
