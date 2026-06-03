package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/smerud/family-cal/internal/api"
	"github.com/smerud/family-cal/internal/auth"
	"github.com/smerud/family-cal/internal/calendar"
	"github.com/smerud/family-cal/internal/db"
	"github.com/smerud/family-cal/internal/notification"
	"github.com/smerud/family-cal/internal/user"
)

//go:embed web
var webFS embed.FS

func main() {
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := api.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	authSvc := auth.NewService(database, cfg.JWTSecret)
	userSvc := user.NewService(database)
	calSvc := calendar.NewService(database)

	// Print setup token if no users exist yet
	if token, err := authSvc.SetupToken(); err == nil && token != "" {
		fmt.Printf("\n=== FIRST RUN SETUP ===\nSetup token: %s\nPOST /api/auth/setup with {\"token\":\"%s\",\"email\":\"...\",\"name\":\"...\",\"password\":\"...\"}\n======================\n\n", token, token)
	}

	notifScheduler := notification.NewScheduler(database, notification.Config{
		SMTPHost: cfg.SMTPHost,
		SMTPPort: cfg.SMTPPort,
		SMTPUser: cfg.SMTPUser,
		SMTPPass: cfg.SMTPPass,
		SMTPFrom: cfg.SMTPFrom,
	})
	notifScheduler.Start()

	router := api.NewRouter(authSvc, userSvc, calSvc)

	// Serve embedded frontend
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("web fs: %v", err)
	}
	router.Handle("/*", http.FileServer(http.FS(webContent)))

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
