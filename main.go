package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/exp/slog"

	"github.com/thde/zenduty-calendar/internal/zenduty"
)

var (
	username = os.Getenv("ZENDUTY_USERNAME")
	password = os.Getenv("ZENDUTY_PASSWORD")
	port     = os.Getenv("PORT")
)

func run(out io.Writer) error {
	if len(port) == 0 {
		port = "3000"
	}

	loggerOpts := zenduty.LoggerOptions{}
	loggerOpts.Out = out
	logger := zenduty.NewLogger(loggerOpts)
	z := zenduty.NewClient(zenduty.Logger(logger))
	if err := z.Login(username, password); err != nil {
		return err
	}

	router := httprouter.New()
	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("content-type", "text/text; charset=utf-8")
		fmt.Fprint(w, "/calendar/:team/:schedule/:member")
	})

	// return a specific schedule identified by the given team and schedule
	// UUID. Only events attended by the given member (needs to be an email
	// address) will be kept.
	router.GET("/calendar/:team/:schedule/:member", byAtendeeHandler("team", "schedule", "member", z.GetSchedule))

	// return a combined calendar which contains all schedules of all teams
	// where the user identified by the ZIOS_USERNAME env variable is part
	// of. Only events which contain the user as attendee will be kept.
	router.GET("/calendar/myschedule", myScheduleHandler(z, func(_ httprouter.Params) string { return username }))

	// return a combined calendar which contains all schedules of all teams
	// where the user identified by the "member" parameter (needs to be an
	// email address) is part of. Only events which contain that user as
	// attendee will be kept.
	router.GET("/calendar/myschedule/:member", myScheduleHandler(z, func(ps httprouter.Params) string { return ps.ByName("member") }))

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

func byAtendeeHandler(teamKey, scheduleKey, memberKey string, getSchedule func(teamID string, scheduleID string, months int) (*zenduty.Schedule, error)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		schedule, err := getSchedule(ps.ByName(teamKey), ps.ByName(scheduleKey), 12)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		outputSchedule(w, schedule.OnlyAttendees(ps.ByName(memberKey)))
	}
}

func myScheduleHandler(c *zenduty.Client, forUser func(httprouter.Params) string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		schedule, err := c.CombinedSchedule(forUser(ps))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		outputSchedule(w, schedule.OnlyAttendees(forUser(ps)))
	}
}

func outputSchedule(w http.ResponseWriter, schedule *zenduty.Schedule) {
	w.Header().Set("content-type", "text/calendar; charset=utf-8")
	w.Header().Set("cache-control", fmt.Sprintf("max-age=%d, public", 5*60))
	schedule.SerializeTo(w)
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

func main() {
	if err := run(os.Stdout); err != nil {
		slog.Error("Error", "err", err)
		os.Exit(1)
	}
}
