package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/antispam"
	"github.com/NerdsWhoFish/bookings/internal/auth"
	"github.com/NerdsWhoFish/bookings/internal/booking"
	calendarprovider "github.com/NerdsWhoFish/bookings/internal/calendar"
	"github.com/NerdsWhoFish/bookings/internal/config"
	"github.com/NerdsWhoFish/bookings/internal/domain"
	"github.com/NerdsWhoFish/bookings/internal/store"
	"github.com/NerdsWhoFish/bookings/internal/tokencrypto"
	"github.com/NerdsWhoFish/bookings/internal/webui"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
)

type Server struct {
	config   config.Config
	store    store.Store
	bookings *booking.Service
	calendar calendarprovider.Provider
	spam     antispam.Verifier
	sessions *auth.Manager
	oauth    *oauth2.Config
	cipher   tokencrypto.Cipher
	logger   *slog.Logger
}

func New(cfg config.Config, data store.Store, bookings *booking.Service, calendar calendarprovider.Provider, spam antispam.Verifier, sessions *auth.Manager, oauth *oauth2.Config, cipher tokencrypto.Cipher, logger *slog.Logger) http.Handler {
	server := &Server{config: cfg, store: data, bookings: bookings, calendar: calendar, spam: spam, sessions: sessions, oauth: oauth, cipher: cipher, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /api/public/config", server.publicConfig)
	mux.HandleFunc("GET /api/public/meeting-types", server.meetingTypes)
	mux.HandleFunc("GET /api/public/meeting-types/{slug}/availability", server.availability)
	mux.HandleFunc("POST /api/public/bookings", server.createBooking)
	mux.HandleFunc("POST /api/public/bookings/{id}/cancel", server.cancelBooking)
	mux.HandleFunc("GET /api/admin/session", server.admin(server.adminSession))
	mux.HandleFunc("GET /api/admin/connections", server.admin(server.connections))
	mux.HandleFunc("GET /api/admin/connections/{id}/calendars", server.admin(server.connectionCalendars))
	mux.HandleFunc("PUT /api/admin/connections/{id}", server.admin(server.putConnection))
	mux.HandleFunc("GET /api/admin/google/start", server.googleStart)
	mux.HandleFunc("GET /api/admin/google/callback", server.googleCallback)
	mux.HandleFunc("PUT /api/admin/meeting-types/{id}", server.admin(server.putMeetingType))
	mux.Handle("/", webui.Handler())
	return securityHeaders(otelhttp.NewHandler(mux, "http.request"))
}

func (s *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) publicConfig(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{
		"theme":            s.config.Theme,
		"turnstileSiteKey": s.config.TurnstileSiteKey,
		"faroURL":          s.config.FaroURL,
		"faroAppName":      s.config.FaroAppName,
	})
}

func (s *Server) meetingTypes(response http.ResponseWriter, request *http.Request) {
	meetings, err := s.store.ListMeetingTypes(request.Context())
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not load meeting types", err)
		return
	}
	writeJSON(response, http.StatusOK, meetings)
}

func (s *Server) availability(response http.ResponseWriter, request *http.Request) {
	meeting, err := s.store.GetMeetingType(request.Context(), request.PathValue("slug"))
	if err != nil {
		s.problem(response, request, statusFor(err), "Meeting type not found", err)
		return
	}
	from := time.Now()
	if value := request.URL.Query().Get("from"); value != "" {
		from, err = time.Parse(time.RFC3339, value)
		if err != nil {
			s.problem(response, request, http.StatusBadRequest, "Invalid start date", err)
			return
		}
	}
	slots, err := s.bookings.Availability(request.Context(), meeting, from)
	if err != nil {
		s.problem(response, request, http.StatusBadGateway, "Could not check calendars", err)
		return
	}
	writeJSON(response, http.StatusOK, slots)
}

func (s *Server) createBooking(response http.ResponseWriter, request *http.Request) {
	var body struct {
		booking.Request
		TurnstileToken string `json:"turnstileToken"`
	}
	if err := decodeJSON(request, &body); err != nil {
		s.problem(response, request, http.StatusBadRequest, "Invalid booking", err)
		return
	}
	host, _, _ := net.SplitHostPort(request.RemoteAddr)
	if err := s.spam.Verify(request.Context(), body.TurnstileToken, host); err != nil {
		s.problem(response, request, http.StatusForbidden, "Verification failed", err)
		return
	}
	confirmation, err := s.bookings.Create(request.Context(), body.Request)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, store.ErrConflict) {
			status = http.StatusConflict
		}
		s.problem(response, request, status, "That time is no longer available", err)
		return
	}
	writeJSON(response, http.StatusCreated, confirmation)
}

func (s *Server) cancelBooking(response http.ResponseWriter, request *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(request, &body); err != nil {
		s.problem(response, request, http.StatusBadRequest, "Invalid cancellation", err)
		return
	}
	if err := s.bookings.Cancel(request.Context(), request.PathValue("id"), body.Token); err != nil {
		s.problem(response, request, statusFor(err), "Could not cancel booking", err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if s.config.DevMode {
			next(response, request)
			return
		}
		if _, err := s.sessions.Read(request); err != nil {
			s.problem(response, request, http.StatusUnauthorized, "Sign in required", err)
			return
		}
		if request.Method != http.MethodGet && !sameOrigin(request, s.config.PublicURL) {
			s.problem(response, request, http.StatusForbidden, "Origin rejected", errors.New("cross-origin admin request"))
			return
		}
		next(response, request)
	}
}

func (s *Server) adminSession(response http.ResponseWriter, request *http.Request) {
	if s.config.DevMode {
		writeJSON(response, http.StatusOK, auth.Session{Email: "developer@localhost", ExpiresAt: time.Now().Add(24 * time.Hour).Unix()})
		return
	}
	session, _ := s.sessions.Read(request)
	writeJSON(response, http.StatusOK, session)
}

func (s *Server) connections(response http.ResponseWriter, request *http.Request) {
	connections, err := s.store.ListConnections(request.Context())
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not load connections", err)
		return
	}
	writeJSON(response, http.StatusOK, connections)
}

func (s *Server) connectionCalendars(response http.ResponseWriter, request *http.Request) {
	connection, err := s.store.GetConnection(request.Context(), request.PathValue("id"))
	if err != nil {
		s.problem(response, request, statusFor(err), "Google account not found", err)
		return
	}
	calendars, err := s.calendar.Calendars(request.Context(), connection)
	if err != nil {
		s.problem(response, request, http.StatusBadGateway, "Could not list Google calendars", err)
		return
	}
	writeJSON(response, http.StatusOK, calendars)
}

func (s *Server) putConnection(response http.ResponseWriter, request *http.Request) {
	connection, err := s.store.GetConnection(request.Context(), request.PathValue("id"))
	if err != nil {
		s.problem(response, request, statusFor(err), "Google account not found", err)
		return
	}
	var body struct {
		BusyCalendarIDs []string `json:"busyCalendarIds"`
	}
	if err := decodeJSON(request, &body); err != nil || len(body.BusyCalendarIDs) == 0 || len(body.BusyCalendarIDs) > 50 {
		s.problem(response, request, http.StatusUnprocessableEntity, "Choose between 1 and 50 calendars", errors.New("invalid busy calendar selection"))
		return
	}
	available, err := s.calendar.Calendars(request.Context(), connection)
	if err != nil {
		s.problem(response, request, http.StatusBadGateway, "Could not validate Google calendars", err)
		return
	}
	allowed := make(map[string]bool, len(available))
	for _, item := range available {
		allowed[item.ID] = true
	}
	for _, id := range body.BusyCalendarIDs {
		if !allowed[id] {
			s.problem(response, request, http.StatusUnprocessableEntity, "Unknown Google calendar", errors.New("calendar selection contains unknown id"))
			return
		}
	}
	connection.BusyCalendarIDs = body.BusyCalendarIDs
	connection.UpdatedAt = time.Now().UTC()
	if err := s.store.PutConnection(request.Context(), connection); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not save calendar selection", err)
		return
	}
	writeJSON(response, http.StatusOK, connection)
}

func (s *Server) googleStart(response http.ResponseWriter, request *http.Request) {
	if s.oauth == nil {
		s.problem(response, request, http.StatusNotImplemented, "Google OAuth is disabled in development", errors.New("OAuth not configured"))
		return
	}
	state, err := s.sessions.NewState(response)
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not start sign in", err)
		return
	}
	http.Redirect(response, request, s.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce), http.StatusFound)
}

func (s *Server) googleCallback(response http.ResponseWriter, request *http.Request) {
	if s.oauth == nil || s.sessions.VerifyState(request, request.URL.Query().Get("state")) != nil {
		s.problem(response, request, http.StatusBadRequest, "Invalid sign in response", errors.New("OAuth state rejected"))
		return
	}
	token, err := s.oauth.Exchange(request.Context(), request.URL.Query().Get("code"))
	if err != nil {
		s.problem(response, request, http.StatusBadGateway, "Google sign in failed", err)
		return
	}
	email, err := googleEmail(request.Context(), s.oauth.Client(request.Context(), token))
	if err != nil {
		s.problem(response, request, http.StatusBadGateway, "Could not read Google account", err)
		return
	}
	adminSession, sessionErr := s.sessions.Read(request)
	if sessionErr != nil && !s.config.AdminEmails[strings.ToLower(email)] {
		s.problem(response, request, http.StatusForbidden, "This account is not an administrator", errors.New("email is not allowed"))
		return
	}
	encoded, err := json.Marshal(token)
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not save Google account", err)
		return
	}
	id := connectionID(email)
	encrypted, err := s.cipher.Encrypt(request.Context(), id, encoded)
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not protect Google credentials", err)
		return
	}
	now := time.Now().UTC()
	connection := domain.CalendarConnection{
		ID: id, Email: strings.ToLower(email), EncryptedToken: encrypted,
		BusyCalendarIDs: []string{"primary"}, CreatedAt: now, UpdatedAt: now,
	}
	if existing, getErr := s.store.GetConnection(request.Context(), id); getErr == nil {
		connection.CreatedAt = existing.CreatedAt
		connection.BusyCalendarIDs = existing.BusyCalendarIDs
	}
	if err := s.store.PutConnection(request.Context(), connection); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not save Google account", err)
		return
	}
	adminEmail := email
	if sessionErr == nil {
		adminEmail = adminSession.Email
	}
	if err := s.sessions.Issue(response, adminEmail); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not create administrator session", err)
		return
	}
	http.Redirect(response, request, "/admin?connected="+url.QueryEscape(email), http.StatusFound)
}

func (s *Server) putMeetingType(response http.ResponseWriter, request *http.Request) {
	var meeting domain.MeetingType
	if err := decodeJSON(request, &meeting); err != nil {
		s.problem(response, request, http.StatusBadRequest, "Invalid meeting type", err)
		return
	}
	meeting.ID = request.PathValue("id")
	if err := validateMeetingType(meeting); err != nil {
		s.problem(response, request, http.StatusUnprocessableEntity, "Invalid meeting type", err)
		return
	}
	if err := s.store.PutMeetingType(request.Context(), meeting); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not save meeting type", err)
		return
	}
	writeJSON(response, http.StatusOK, meeting)
}

func (s *Server) problem(response http.ResponseWriter, request *http.Request, status int, title string, err error) {
	s.logger.ErrorContext(request.Context(), "request failed", "status", status, "title", title, "error", err)
	writeJSONStatus(response, status, map[string]any{"title": title, "status": status})
}

func googleEmail(ctx context.Context, client *http.Client) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo returned %s", response.Status)
	}
	var result struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.EmailVerified || result.Email == "" {
		return "", errors.New("Google email is not verified")
	}
	return result.Email, nil
}

func validateMeetingType(meeting domain.MeetingType) error {
	if meeting.ID == "" || meeting.Slug == "" || meeting.Name == "" || meeting.TimeZone == "" || meeting.DurationMinutes < 5 || meeting.DurationMinutes > 480 {
		return errors.New("id, slug, name, time zone, and a duration from 5 to 480 minutes are required")
	}
	if meeting.SlotIntervalMinutes < 5 || meeting.BookingWindowDays < 1 || meeting.BookingWindowDays > 365 {
		return errors.New("slot interval and booking window are outside supported bounds")
	}
	if _, err := time.LoadLocation(meeting.TimeZone); err != nil {
		return errors.New("unknown time zone")
	}
	return nil
}

func decodeJSON(request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(nil, request.Body, 32<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	writeJSONStatus(response, status, value)
}

func writeJSONStatus(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func statusFor(err error) int {
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

func connectionID(email string) string {
	value := sha256.Sum256([]byte(strings.ToLower(email)))
	return hex.EncodeToString(value[:12])
}

func sameOrigin(request *http.Request, publicURL string) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	expected, err := url.Parse(publicURL)
	return err == nil && origin == expected.Scheme+"://"+expected.Host
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://challenges.cloudflare.com; frame-src https://challenges.cloudflare.com; connect-src 'self' https://*.grafana.net https://*.grafana.com; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		response.Header().Set("Referrer-Policy", "same-origin")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(response, request)
	})
}
