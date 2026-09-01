package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/antispam"
	"github.com/NerdsWhoFish/bookings/internal/auth"
	"github.com/NerdsWhoFish/bookings/internal/booking"
	calendarprovider "github.com/NerdsWhoFish/bookings/internal/calendar"
	"github.com/NerdsWhoFish/bookings/internal/config"
	"github.com/NerdsWhoFish/bookings/internal/domain"
	"github.com/NerdsWhoFish/bookings/internal/securetoken"
	"github.com/NerdsWhoFish/bookings/internal/store"
	"github.com/NerdsWhoFish/bookings/internal/tokencrypto"
	"github.com/NerdsWhoFish/bookings/internal/webui"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
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
	mux.HandleFunc("GET /api/public/meeting-types/{slug}", server.meetingType)
	mux.HandleFunc("GET /api/public/meeting-types/{slug}/availability", server.availability)
	mux.HandleFunc("POST /api/public/calendar-invitations/start", server.calendarInvitationStart)
	mux.HandleFunc("POST /api/public/bookings", server.createBooking)
	mux.HandleFunc("POST /api/public/bookings/{id}/cancel", server.cancelBooking)
	mux.HandleFunc("PUT /api/external/blocks/{id}", server.external(server.putExternalBlock))
	mux.HandleFunc("DELETE /api/external/blocks/{id}", server.external(server.deleteExternalBlock))
	mux.HandleFunc("GET /api/admin/session", server.admin(server.adminSession))
	mux.HandleFunc("GET /api/admin/connections", server.admin(server.connections))
	mux.HandleFunc("GET /api/admin/calendar-invitations", server.admin(server.calendarInvitations))
	mux.HandleFunc("POST /api/admin/calendar-invitations", server.admin(server.createCalendarInvitation))
	mux.HandleFunc("DELETE /api/admin/calendar-invitations/{id}", server.admin(server.deleteCalendarInvitation))
	mux.HandleFunc("GET /api/admin/meeting-types", server.admin(server.adminMeetingTypes))
	mux.HandleFunc("GET /api/admin/connections/{id}/calendars", server.admin(server.connectionCalendars))
	mux.HandleFunc("PUT /api/admin/connections/{id}", server.admin(server.putConnection))
	mux.HandleFunc("GET /api/admin/google/start", server.googleStart)
	mux.HandleFunc("GET /api/admin/google/callback", server.googleCallback)
	mux.HandleFunc("PUT /api/admin/meeting-types/{id}", server.admin(server.putMeetingType))
	mux.HandleFunc("DELETE /api/admin/meeting-types/{id}", server.admin(server.deleteMeetingType))
	mux.Handle("/", webui.Handler())
	return securityHeaders(otelhttp.NewHandler(server.requestLog(mux), "http.request"))
}

type responseCapture struct {
	http.ResponseWriter
	status int
}

func (response *responseCapture) WriteHeader(status int) {
	if response.status != 0 {
		return
	}
	response.status = status
	response.ResponseWriter.WriteHeader(status)
}

func (response *responseCapture) Write(body []byte) (int, error) {
	if response.status == 0 {
		response.WriteHeader(http.StatusOK)
	}
	return response.ResponseWriter.Write(body)
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		captured := &responseCapture{ResponseWriter: response}
		next.ServeHTTP(captured, request)
		if request.Pattern == "GET /healthz" {
			return
		}
		status := captured.status
		if status == 0 {
			status = http.StatusOK
		}
		attributes := []any{
			"http.request.method", request.Method,
			"http.route", request.Pattern,
			"http.response.status_code", status,
			"duration_ms", time.Since(started).Milliseconds(),
		}
		span := trace.SpanContextFromContext(request.Context())
		if span.IsValid() {
			attributes = append(attributes, "trace_id", span.TraceID().String(), "span_id", span.SpanID().String())
		}
		s.logger.InfoContext(request.Context(), "request completed", attributes...)
	})
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
	normalizeMeetingTypes(meetings)
	sanitizePublicMeetingTypes(meetings)
	writeJSON(response, http.StatusOK, meetings)
}

func (s *Server) meetingType(response http.ResponseWriter, request *http.Request) {
	meeting, err := s.store.GetMeetingType(request.Context(), request.PathValue("slug"))
	if err != nil {
		s.problem(response, request, statusFor(err), "Meeting type not found", err)
		return
	}
	meeting.AttendeeEmails = []string{}
	meeting.BlockerEmails = []string{}
	writeJSON(response, http.StatusOK, meeting)
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

func (s *Server) external(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if s.config.ExternalBlockToken == "" {
			s.problem(response, request, http.StatusNotFound, "External block API is disabled", errors.New("external block token is not configured"))
			return
		}
		expected := sha256.Sum256([]byte("Bearer " + s.config.ExternalBlockToken))
		provided := sha256.Sum256([]byte(request.Header.Get("Authorization")))
		if !hmac.Equal(expected[:], provided[:]) {
			response.Header().Set("WWW-Authenticate", "Bearer")
			s.problem(response, request, http.StatusUnauthorized, "Authentication required", errors.New("invalid external block token"))
			return
		}
		next(response, request)
	}
}

func (s *Server) putExternalBlock(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if !validExternalBlockID(id) {
		s.problem(response, request, http.StatusUnprocessableEntity, "Invalid external block ID", errors.New("external block id is outside supported bounds"))
		return
	}
	var body struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	}
	if err := decodeJSON(request, &body); err != nil || body.Start.IsZero() || !body.Start.Before(body.End) || body.End.Sub(body.Start) > 366*24*time.Hour {
		s.problem(response, request, http.StatusUnprocessableEntity, "Start and end must describe a valid block", errors.New("external block interval is outside supported bounds"))
		return
	}
	now := time.Now().UTC()
	block := domain.ExternalBlock{ID: id, Start: body.Start, End: body.End, CreatedAt: now, UpdatedAt: now}
	if err := s.store.PutExternalBlock(request.Context(), block); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not save external block", err)
		return
	}
	s.logger.InfoContext(request.Context(), "external busy block saved")
	response.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteExternalBlock(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if !validExternalBlockID(id) {
		s.problem(response, request, http.StatusUnprocessableEntity, "Invalid external block ID", errors.New("external block id is outside supported bounds"))
		return
	}
	if err := s.store.DeleteExternalBlock(request.Context(), id); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not delete external block", err)
		return
	}
	s.logger.InfoContext(request.Context(), "external busy block deleted")
	response.WriteHeader(http.StatusNoContent)
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

func (s *Server) calendarInvitations(response http.ResponseWriter, request *http.Request) {
	invitations, err := s.store.ListCalendarInvitations(request.Context())
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not load calendar invitations", err)
		return
	}
	sort.Slice(invitations, func(i, j int) bool { return invitations[i].CreatedAt.After(invitations[j].CreatedAt) })
	writeJSON(response, http.StatusOK, invitations)
}

func (s *Server) createCalendarInvitation(response http.ResponseWriter, request *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(request, &body); err != nil {
		s.problem(response, request, http.StatusBadRequest, "Invalid calendar invitation", err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		s.problem(response, request, http.StatusUnprocessableEntity, "Enter a valid email address", errors.New("invalid invitation email"))
		return
	}
	if _, err := s.store.GetConnection(request.Context(), connectionID(email)); err == nil {
		s.problem(response, request, http.StatusConflict, "That Google account is already connected", errors.New("connection already exists"))
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		s.problem(response, request, http.StatusInternalServerError, "Could not check connected accounts", err)
		return
	}
	id, err := securetoken.RandomURL(16)
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not create calendar invitation", err)
		return
	}
	secret, err := securetoken.RandomURL(32)
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not create calendar invitation", err)
		return
	}
	now := time.Now().UTC()
	invitation := domain.CalendarInvitation{
		ID: id, Email: email, TokenHash: securetoken.Hash(secret),
		ExpiresAt: now.Add(7 * 24 * time.Hour), CreatedAt: now,
	}
	if err := s.store.PutCalendarInvitation(request.Context(), invitation); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not save calendar invitation", err)
		return
	}
	s.logger.InfoContext(request.Context(), "calendar invitation created", "invitation_id", id)
	writeJSON(response, http.StatusCreated, map[string]any{
		"invitation": invitation,
		"url":        strings.TrimRight(s.config.PublicURL, "/") + "/connect#" + id + "." + secret,
	})
}

func (s *Server) deleteCalendarInvitation(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if err := s.store.DeleteCalendarInvitation(request.Context(), id); err != nil {
		s.problem(response, request, statusFor(err), "Calendar invitation not found", err)
		return
	}
	s.logger.InfoContext(request.Context(), "calendar invitation revoked", "invitation_id", id)
	response.WriteHeader(http.StatusNoContent)
}

func (s *Server) calendarInvitationStart(response http.ResponseWriter, request *http.Request) {
	if s.oauth == nil {
		s.problem(response, request, http.StatusNotImplemented, "Google OAuth is disabled in development", errors.New("OAuth not configured"))
		return
	}
	if !sameOrigin(request, s.config.PublicURL) {
		s.problem(response, request, http.StatusForbidden, "Origin rejected", errors.New("cross-origin invitation request"))
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(request, &body); err != nil {
		s.problem(response, request, http.StatusBadRequest, "Invalid calendar invitation", err)
		return
	}
	id, secret, ok := strings.Cut(body.Token, ".")
	if !ok || id == "" || secret == "" {
		s.problem(response, request, http.StatusGone, "This calendar invitation is invalid", errors.New("malformed invitation token"))
		return
	}
	invitation, err := s.store.GetCalendarInvitation(request.Context(), id)
	if err != nil {
		s.problem(response, request, invitationStatus(err), "This calendar invitation is no longer available", err)
		return
	}
	if err := validateCalendarInvitation(invitation, secret, time.Now()); err != nil {
		s.problem(response, request, invitationStatus(err), "This calendar invitation is no longer available", err)
		return
	}
	if err := s.sessions.IssueCalendarInviteBridge(response, invitation.ID); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not start Google connection", err)
		return
	}
	state, err := s.sessions.NewState(response)
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not start Google connection", err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"url": s.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)})
}

func (s *Server) adminMeetingTypes(response http.ResponseWriter, request *http.Request) {
	meetings, err := s.store.ListAllMeetingTypes(request.Context())
	if err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not load meeting types", err)
		return
	}
	normalizeMeetingTypes(meetings)
	writeJSON(response, http.StatusOK, meetings)
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
	s.sessions.ClearCalendarInviteBridge(response)
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
	bridge, bridgeErr := s.sessions.ReadCalendarInviteBridge(request)
	adminSession, sessionErr := s.sessions.Read(request)
	if bridgeErr != nil && sessionErr != nil && !s.config.AdminEmails[strings.ToLower(email)] {
		s.problem(response, request, http.StatusForbidden, "This account is not an administrator", errors.New("email is not allowed"))
		return
	}
	if bridgeErr == nil {
		invitation, err := s.store.GetCalendarInvitation(request.Context(), bridge.InvitationID)
		if err == nil && invitation.UsedAt != nil {
			err = store.ErrAlreadyUsed
		}
		if err == nil && !time.Now().Before(invitation.ExpiresAt) {
			err = store.ErrExpired
		}
		if err == nil && !strings.EqualFold(invitation.Email, email) {
			err = store.ErrEmailMismatch
		}
		if err != nil {
			s.sessions.ClearCalendarInviteBridge(response)
			s.problem(response, request, invitationStatus(err), "This Google account does not match the calendar invitation", err)
			return
		}
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
	if bridgeErr == nil {
		if err := s.store.UseCalendarInvitation(request.Context(), bridge.InvitationID, email, time.Now()); err != nil {
			s.sessions.ClearCalendarInviteBridge(response)
			s.problem(response, request, invitationStatus(err), "Could not finish calendar invitation", err)
			return
		}
		s.sessions.ClearCalendarInviteBridge(response)
		s.logger.InfoContext(request.Context(), "calendar invitation accepted", "invitation_id", bridge.InvitationID, "connection_id", id)
		http.Redirect(response, request, "/connect?status=connected", http.StatusFound)
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
	var err error
	meeting.AttendeeEmails, err = normalizeEmailList(meeting.AttendeeEmails, 50)
	if err == nil {
		meeting.BlockerEmails, err = normalizeEmailList(meeting.BlockerEmails, 20)
	}
	if err != nil {
		s.problem(response, request, http.StatusUnprocessableEntity, "Invalid meeting type", err)
		return
	}
	if err := validateMeetingType(meeting); err != nil {
		s.problem(response, request, http.StatusUnprocessableEntity, "Invalid meeting type", err)
		return
	}
	if !s.config.DevMode {
		if meeting.DestinationConnectionID == "" || meeting.DestinationCalendarID == "" {
			s.problem(response, request, http.StatusUnprocessableEntity, "Choose a destination calendar", errors.New("destination connection and calendar are required"))
			return
		}
		connection, err := s.store.GetConnection(request.Context(), meeting.DestinationConnectionID)
		if err != nil {
			s.problem(response, request, http.StatusUnprocessableEntity, "Destination account not found", err)
			return
		}
		calendars, err := s.calendar.Calendars(request.Context(), connection)
		if err != nil {
			s.problem(response, request, http.StatusBadGateway, "Could not validate destination calendar", err)
			return
		}
		known := false
		for _, calendar := range calendars {
			known = known || calendar.ID == meeting.DestinationCalendarID
		}
		if !known {
			s.problem(response, request, http.StatusUnprocessableEntity, "Destination calendar not found", errors.New("unknown destination calendar"))
			return
		}
		connections, err := s.store.ListConnections(request.Context())
		if err != nil {
			s.problem(response, request, http.StatusInternalServerError, "Could not validate meeting attendees", err)
			return
		}
		connected := make(map[string]bool, len(connections))
		for _, item := range connections {
			connected[strings.ToLower(item.Email)] = true
		}
		attendees := make(map[string]bool, len(meeting.AttendeeEmails))
		for _, email := range meeting.AttendeeEmails {
			if !connected[email] {
				s.problem(response, request, http.StatusUnprocessableEntity, "Choose connected accounts as attendees", errors.New("unknown attendee"))
				return
			}
			attendees[email] = true
		}
		for _, email := range meeting.BlockerEmails {
			if connected[email] || attendees[email] {
				s.problem(response, request, http.StatusUnprocessableEntity, "Private blocker addresses must not be connected attendees", errors.New("blocker address overlaps a connected account"))
				return
			}
		}
	}
	if err := s.store.PutMeetingType(request.Context(), meeting); err != nil {
		s.problem(response, request, http.StatusInternalServerError, "Could not save meeting type", err)
		return
	}
	writeJSON(response, http.StatusOK, meeting)
}

func (s *Server) deleteMeetingType(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if err := s.store.DeleteMeetingType(request.Context(), id); err != nil {
		s.problem(response, request, statusFor(err), "Meeting type not found", err)
		return
	}
	s.logger.InfoContext(request.Context(), "meeting type deleted", "meeting_type_id", id)
	response.WriteHeader(http.StatusNoContent)
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
	if len(meeting.Name) > 120 || len(meeting.Description) > 500 || !validSlug(meeting.Slug) {
		return errors.New("name, description, or slug is outside supported bounds")
	}
	if meeting.BufferBeforeMinutes < 0 || meeting.BufferAfterMinutes < 0 || meeting.MinimumNoticeMinutes < 0 || meeting.SlotIntervalMinutes > 120 {
		return errors.New("buffers, notice, and slot interval are outside supported bounds")
	}
	if len(meeting.Availability) == 0 || len(meeting.Availability) > 7 {
		return errors.New("at least one weekly availability window is required")
	}
	if _, err := normalizeEmailList(meeting.AttendeeEmails, 50); err != nil {
		return errors.New("meeting attendees must be unique email addresses")
	}
	if _, err := normalizeEmailList(meeting.BlockerEmails, 20); err != nil {
		return errors.New("private blocker addresses must be unique email addresses")
	}
	seen := make(map[int]bool)
	for _, hours := range meeting.Availability {
		start, startErr := time.Parse("15:04", hours.Start)
		end, endErr := time.Parse("15:04", hours.End)
		if hours.Weekday < 0 || hours.Weekday > 6 || seen[hours.Weekday] || startErr != nil || endErr != nil || !start.Before(end) {
			return errors.New("weekly availability must have one valid start and end per weekday")
		}
		seen[hours.Weekday] = true
	}
	return nil
}

func validSlug(value string) bool {
	if len(value) > 80 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func validExternalBlockID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && !strings.ContainsRune("._:-", character) {
			return false
		}
	}
	return true
}

func normalizeEmailList(values []string, limit int) ([]string, error) {
	if len(values) > limit {
		return nil, errors.New("too many email addresses")
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		email := strings.ToLower(strings.TrimSpace(value))
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || seen[email] {
			return nil, errors.New("email addresses must be valid and unique")
		}
		seen[email] = true
		result = append(result, email)
	}
	return result, nil
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

func invitationStatus(err error) int {
	if errors.Is(err, store.ErrEmailMismatch) {
		return http.StatusForbidden
	}
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrExpired) || errors.Is(err, store.ErrAlreadyUsed) {
		return http.StatusGone
	}
	return http.StatusBadRequest
}

func validateCalendarInvitation(invitation domain.CalendarInvitation, secret string, now time.Time) error {
	if invitation.UsedAt != nil {
		return store.ErrAlreadyUsed
	}
	if !now.Before(invitation.ExpiresAt) {
		return store.ErrExpired
	}
	if !securetoken.EqualHash(secret, invitation.TokenHash) {
		return store.ErrNotFound
	}
	return nil
}

func normalizeMeetingTypes(meetings []domain.MeetingType) {
	for index := range meetings {
		if meetings[index].AttendeeEmails == nil {
			meetings[index].AttendeeEmails = []string{}
		}
		if meetings[index].BlockerEmails == nil {
			meetings[index].BlockerEmails = []string{}
		}
	}
}

func sanitizePublicMeetingTypes(meetings []domain.MeetingType) {
	for index := range meetings {
		meetings[index].AttendeeEmails = []string{}
		meetings[index].BlockerEmails = []string{}
	}
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
