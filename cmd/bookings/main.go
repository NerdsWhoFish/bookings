package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"cloud.google.com/go/firestore"
	kms "cloud.google.com/go/kms/apiv1"
	"github.com/NerdsWhoFish/bookings/internal/antispam"
	"github.com/NerdsWhoFish/bookings/internal/auth"
	"github.com/NerdsWhoFish/bookings/internal/booking"
	calendarprovider "github.com/NerdsWhoFish/bookings/internal/calendar"
	"github.com/NerdsWhoFish/bookings/internal/config"
	"github.com/NerdsWhoFish/bookings/internal/domain"
	"github.com/NerdsWhoFish/bookings/internal/httpapi"
	"github.com/NerdsWhoFish/bookings/internal/store"
	"github.com/NerdsWhoFish/bookings/internal/telemetry"
	"github.com/NerdsWhoFish/bookings/internal/tokencrypto"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

func main() {
	stdout := slog.NewJSONHandler(os.Stdout, nil)
	logger := slog.New(stdout)
	slog.SetDefault(logger)
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger, shutdownTelemetry, err := telemetry.Start(ctx, stdout)
	if err != nil {
		logger.Error("telemetry setup failed", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)
	defer func() { _ = shutdownTelemetry(context.Background()) }()

	data, provider, cipher, oauth, spam, closeClients, err := dependencies(ctx, cfg)
	if err != nil {
		logger.Error("dependency setup failed", "error", err)
		os.Exit(1)
	}
	defer closeClients()
	service := booking.NewService(data, provider)
	sessions := auth.NewManager(sessionKey(cfg), !cfg.DevMode)
	handler := httpapi.New(cfg, data, service, provider, spam, sessions, oauth, cipher, logger)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("bookings listening", "address", server.Addr, "development", cfg.DevMode)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP shutdown failed", "error", err)
	}
}

func dependencies(ctx context.Context, cfg config.Config) (store.Store, calendarprovider.Provider, tokencrypto.Cipher, *oauth2.Config, antispam.Verifier, func(), error) {
	if cfg.DevMode {
		data := store.NewMemory(seedMeetingTypes())
		return data, calendarprovider.Mock{}, tokencrypto.Plaintext{}, nil, antispam.Disabled{}, func() {}, nil
	}
	firestoreClient, err := firestore.NewClientWithDatabase(ctx, cfg.ProjectID, cfg.FirestoreDatabase)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	kmsClient, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		_ = firestoreClient.Close()
		return nil, nil, nil, nil, nil, nil, err
	}
	cipher := tokencrypto.NewKMS(kmsClient, cfg.KMSKeyName)
	oauth := &oauth2.Config{
		ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleClientSecret,
		Endpoint: google.Endpoint, RedirectURL: cfg.PublicURL + "/api/admin/google/callback",
		Scopes: []string{"openid", "email", calendar.CalendarFreebusyScope, calendar.CalendarEventsScope, calendar.CalendarCalendarlistReadonlyScope},
	}
	closeClients := func() {
		_ = firestoreClient.Close()
		_ = kmsClient.Close()
	}
	return store.NewFirestore(firestoreClient), calendarprovider.NewGoogle(oauth, cipher), cipher, oauth, antispam.NewTurnstile(cfg.TurnstileSecret, &http.Client{Timeout: 5 * time.Second}), closeClients, nil
}

func sessionKey(cfg config.Config) string {
	if cfg.DevMode {
		return "development-only-session-key-000000000000"
	}
	return cfg.SessionKey
}

func seedMeetingTypes() []domain.MeetingType {
	availability := []domain.WeekdayHours{
		{Weekday: 1, Start: "09:00", End: "17:00"},
		{Weekday: 2, Start: "09:00", End: "17:00"},
		{Weekday: 3, Start: "09:00", End: "17:00"},
		{Weekday: 4, Start: "09:00", End: "17:00"},
		{Weekday: 5, Start: "09:00", End: "15:00"},
	}
	return []domain.MeetingType{
		{ID: "quick-cast", Slug: "quick-cast", Name: "Quick cast", Description: "A focused check-in for one clear question.", DurationMinutes: 20, BufferAfterMinutes: 10, MinimumNoticeMinutes: 120, BookingWindowDays: 30, SlotIntervalMinutes: 30, TimeZone: "America/New_York", Location: "Google Meet", Availability: availability, Active: true},
		{ID: "deep-dive", Slug: "deep-dive", Name: "Deep dive", Description: "Enough room to unpack a technical problem together.", DurationMinutes: 45, BufferBeforeMinutes: 10, BufferAfterMinutes: 15, MinimumNoticeMinutes: 240, BookingWindowDays: 45, SlotIntervalMinutes: 30, TimeZone: "America/New_York", Location: "Google Meet", Availability: availability, Active: true},
		{ID: "expedition", Slug: "expedition", Name: "Expedition", Description: "A working session for architecture, planning, or review.", DurationMinutes: 75, BufferBeforeMinutes: 15, BufferAfterMinutes: 15, MinimumNoticeMinutes: 1440, BookingWindowDays: 60, SlotIntervalMinutes: 30, TimeZone: "America/New_York", Location: "Google Meet", Availability: availability, Active: true},
	}
}
