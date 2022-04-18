package handler

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/ramin0/google-calendar-push/internal/store"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	auth "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var (
	baseURL = func(r *http.Request) string { return fmt.Sprintf("https://%s", r.Host) }
	tmpl    = template.Must(template.ParseFiles(path.Join("static", "calendars.html.tmpl")))
)

type handler struct {
	ClientID     string
	ClientSecret string
	authConfig   func(...string) *oauth2.Config
}

func New(clientID, clientSecret string) *handler {
	return &handler{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		authConfig: func(baseURL ...string) *oauth2.Config {
			var redirectBaseURL string
			if len(baseURL) > 0 {
				redirectBaseURL = baseURL[0]
			}
			return &oauth2.Config{
				Endpoint:     google.Endpoint,
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Scopes: []string{
					auth.UserinfoProfileScope,
					calendar.CalendarReadonlyScope,
					calendar.CalendarEventsScope,
				},
				RedirectURL: redirectBaseURL + "/auth/callback",
			}
		},
	}
}

func (h *handler) Auth(log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "", http.StatusMethodNotAllowed)
			return
		}

		authURL := h.authConfig(baseURL(r)).AuthCodeURL(uuid.NewString(), oauth2.AccessTypeOffline)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

func (h *handler) AuthCallback(log *zap.Logger, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		switch r.Method {
		case http.MethodGet:
			h.authCallbackGET(ctx, log, s)(w, r)
		case http.MethodPost:
			h.authCallbackPOST(ctx, log, s)(w, r)
		default:
			http.Error(w, "", http.StatusMethodNotAllowed)
		}
	}
}

func (h *handler) authCallbackGET(ctx context.Context, log *zap.Logger, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		token, err := h.authConfig(baseURL(r)).Exchange(ctx, code)
		if err != nil {
			log.Error("failed to exchange code", zap.Error(err))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tokenSource := h.authConfig().TokenSource(ctx, token)

		authService, err := auth.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			log.Error("failed to init auth service", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		userInfo, err := authService.Userinfo.Get().Do()
		if err != nil {
			log.Error("failed to get user info", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := s.FindOrCreateUser(ctx, &store.User{
			ID:           userInfo.Id,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			ExpiresAt:    token.Expiry.UTC(),
		}); err != nil {
			log.Error("failed to create user", zap.Error(err))
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}

		calendarService, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			log.Error("failed to init calendar service", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		calendars, err := calendarService.CalendarList.List().Do()
		if err != nil {
			log.Error("failed to list calendars", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, struct {
			State       string
			UserName    string
			AccessToken string
			Calendars   []*calendar.CalendarListEntry
		}{
			State:       "calendars",
			UserName:    userInfo.GivenName,
			AccessToken: token.AccessToken,
			Calendars:   calendars.Items,
		}); err != nil {
			log.Error("failed to render template", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *handler) authCallbackPOST(ctx context.Context, log *zap.Logger, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Error("failed to parse form", zap.Error(err))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tokenSource := h.authConfig().TokenSource(ctx, &oauth2.Token{
			AccessToken: r.FormValue("access_token"),
		})

		authService, err := auth.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			log.Error("failed to init auth service", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		userInfo, err := authService.Userinfo.Get().Do()
		if err != nil {
			log.Error("failed to get user info", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		calendarService, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			log.Error("failed to init calendar service", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		calendarID := r.FormValue("calendar_id")
		channelID := regexp.MustCompile(`[^A-Za-z0-9\-_\+/=]`).ReplaceAllString(
			fmt.Sprintf("%s-%s", userInfo.Id, calendarID), "-")
		for {
			token := uuid.NewString()
			channel, err := calendarService.Events.Watch(calendarID, &calendar.Channel{
				Type:    "webhook",
				Id:      channelID,
				Address: baseURL(r) + "/webhook",
				Token:   token,
			}).Do()
			if err == nil {
				var events *calendar.Events
				for nextPageToken := ""; ; {
					events, err = calendarService.Events.List(calendarID).Do(googleapi.QueryParameter("pageToken", nextPageToken))
					if err != nil {
						log.Error("failed to list events", zap.Error(err))
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					nextPageToken = events.NextPageToken
					if nextPageToken == "" {
						break
					}
				}
				if err := s.FindOrCreateChannel(ctx, &store.Channel{
					ID:            channelID,
					UserID:        userInfo.Id,
					ResourceID:    channel.ResourceId,
					CalendarID:    calendarID,
					Token:         token,
					LastSyncToken: events.NextSyncToken,
				}); err != nil {
					log.Error("failed to create channel", zap.Error(err))
					http.Error(w, err.Error(), http.StatusUnprocessableEntity)
					return
				}

				if err := tmpl.Execute(w, struct {
					State     string
					ChannelID string
				}{
					State:     "watch",
					ChannelID: channel.Id,
				}); err != nil {
					log.Error("failed to render template", zap.Error(err))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				break
			}

			gerr, ok := err.(*googleapi.Error)
			if !ok || len(gerr.Errors) == 1 && gerr.Errors[0].Reason != "channelIdNotUnique" {
				log.Error("failed to watch events", zap.Error(err))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			resourceID, err := func() (string, error) {
				channelID := uuid.NewString()
				channel, err := calendarService.Events.Watch(calendarID, &calendar.Channel{
					Type:    "webhook",
					Id:      channelID,
					Address: baseURL(r) + "/webhook",
				}).Do()
				if err != nil {
					return "", err
				}
				resourceID := channel.ResourceId
				if err := calendarService.Channels.Stop(&calendar.Channel{
					Id:         channelID,
					ResourceId: resourceID,
				}).Do(); err != nil {
					return "", err
				}
				return resourceID, nil
			}()
			if err != nil {
				log.Error("failed to get resource id", zap.Error(err))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if err := calendarService.Channels.Stop(&calendar.Channel{
				Id:         channelID,
				ResourceId: resourceID,
			}).Do(); err != nil {
				log.Error("failed to stop watching events", zap.Error(err))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
}

func (h *handler) Webhook(log *zap.Logger, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		switch r.Method {
		case http.MethodPost:
			h.webhookPOST(ctx, log, s)(w, r)
		default:
			http.Error(w, "", http.StatusMethodNotAllowed)
		}
	}
}

func (h *handler) webhookPOST(ctx context.Context, log *zap.Logger, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			channelID         = r.Header.Get("X-Goog-Channel-Id")
			channelToken      = r.Header.Get("X-Goog-Channel-Token")
			channelResourceID = r.Header.Get("X-Goog-Resource-Id")
		)

		channel, err := s.FindChannel(ctx, channelID, channelToken, channelResourceID)
		if err != nil {
			if err == pgx.ErrNoRows {
				log.Warn("failed to find channel", zap.Error(err))
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			log.Error("failed to find channel", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		user, err := s.FindUser(ctx, channel.UserID)
		if err != nil {
			if err == pgx.ErrNoRows {
				log.Warn("failed to find user", zap.Error(err))
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			log.Error("failed to find user", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tokenSource := h.authConfig().TokenSource(ctx, &oauth2.Token{
			AccessToken:  user.AccessToken,
			RefreshToken: user.RefreshToken,
			Expiry:       user.ExpiresAt,
		})
		token, err := tokenSource.Token()
		if err != nil {
			log.Error("failed to get token", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if user.AccessToken != token.AccessToken ||
			(token.RefreshToken != "" && user.RefreshToken != token.RefreshToken) {
			if err := s.UpdateUser(ctx, &store.User{
				ID:           user.ID,
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				ExpiresAt:    token.Expiry.UTC(),
			}); err != nil {
				log.Error("failed to update user", zap.Error(err))
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
		}

		calendarService, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			log.Error("failed to init calendar service", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var events *calendar.Events
		var newEvents []*calendar.Event
		for nextPageToken := ""; ; {
			events, err = calendarService.Events.List(channel.CalendarID).Do(
				googleapi.QueryParameter("syncToken", channel.LastSyncToken),
				googleapi.QueryParameter("pageToken", nextPageToken))
			if err != nil {
				log.Error("failed to list events", zap.Error(err))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for _, e := range events.Items {
				if e.Summary == "Busy" && e.Reminders.UseDefault {
					newEvents = append(newEvents, e)
				}
			}
			nextPageToken = events.NextPageToken
			if nextPageToken == "" {
				break
			}
		}

		for _, e := range newEvents {
			_, err := calendarService.Events.Patch(channel.CalendarID, e.Id, &calendar.Event{
				Reminders: &calendar.EventReminders{ForceSendFields: []string{"UseDefault"}},
			}).Do()
			if err != nil {
				log.Error("failed to patch event", zap.Error(err))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			log.Info("ok", zap.String("channel", channel.ID), zap.String("event", e.Id))
		}

		if err := s.UpdateChannelLastSyncToken(ctx, channel.ID, events.NextSyncToken); err != nil {
			log.Error("failed to update channel", zap.Error(err))
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}
}
