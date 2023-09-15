package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/exp/slog"

	"github.com/thde/zenduty-calendar/internal/zenduty"
)

var (
	username = os.Getenv("ZENDUTY_USERNAME")
	password = os.Getenv("ZENDUTY_PASSWORD")
	port = os.Getenv("PORT")
)

func run(stdout io.Writer) error {
	logger := slog.New(slog.NewTextHandler(stdout, nil))

	if len(port) == 0 {
		port = "3000"
	}

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	u, err := url.Parse("https://www.zenduty.com")
	if err != nil {
		return err
	}

	z := zenduty.Client{
		BaseURL:    u,
		HTTPCLient: httpClient,
		Logger:     logger.With("pkg", "zenduty"),
	}

	err = z.Login(username, password)
	if err != nil {
		return err
	}

	router := httprouter.New()
	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("content-type", "text/text; charset=utf-8")
		fmt.Fprint(w, "/calendar/:team/:schedule/:member")
	})
	router.GET("/calendar/:team/:schedule/:member", byAtendeeHandler("team", "schedule", "member", z.GetSchedule))

	server := http.Server{
		Addr:         fmt.Sprintf(":%v", port),
		Handler:      withHttpLog(logger.WithGroup("http"))(router),
		ReadTimeout:  8 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	logger.Info(fmt.Sprintf("listening on %s", server.Addr))

	return server.ListenAndServe()
}

func byAtendeeHandler(teamKey, scheduleKey, memberKey string, getSchedule func(teamID string, scheduleID string, months int) (io.ReadCloser, error))  httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		schedule, err := getSchedule(ps.ByName(teamKey), ps.ByName(scheduleKey), 12)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		defer schedule.Close()

		cal, err := ics.ParseCalendar(schedule)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("content-type", "text/calendar; charset=utf-8")
		w.Header().Set("cache-control", fmt.Sprintf("max-age=%d, public", 5*60))
		filter(cal, ics.ComponentPropertyAttendee, ps.ByName(memberKey)).SerializeTo(w)
	}
}

func filter(in *ics.Calendar, prop ics.ComponentProperty, substr string) *ics.Calendar {
	out := *in
	out.Components = []ics.Component{}

	substr = strings.ToLower(substr)

	for _, event := range in.Events() {
		property := event.GetProperty(prop).Value

		if contains(property, substr) {
			out.AddVEvent(event)
		}
	}

	return &out
}

func withHttpLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				logger.Info("access", "Method", r.Method, "Path", r.URL.Path, "RemoteAddr", r.RemoteAddr, "UserAgent", r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func contains(s string, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

func main() {
	if err := run(os.Stdout); err != nil {
		slog.Error("Error", "err", err)
		os.Exit(1)
	}
}
