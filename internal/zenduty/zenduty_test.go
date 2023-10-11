package zenduty

import (
	"os"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slog"
)

func TestICS(t *testing.T) {
	username, password := os.Getenv("ZENDUTY_USERNAME"), os.Getenv("ZENDUTY_PASSWORD")
	if username == "" || password == "" {
		t.Skip("no ZENDUTY_USERNAME or ZENDUTY_PASSWORD env variable set")
	}

	options := LoggerOptions{}
	options.Level = slog.LevelInfo
	// use the debug level to see the downloaded calendars and the combined
	// one
	// options.Level = slog.LevelDebug
	logger := NewLogger(options)
	z := NewClient(
		func() (string, string) { return username, password },
		Logger(logger),
	)
	require.NoError(t, z.Login())

	calendar, err := z.CombinedSchedule(username)
	require.NoError(t, err)
	calendar = calendar.OnlyAttendees(username)
	require.True(t, len(calendar.Events()) > 0)
	logger.Debug("my schedule", "content", calendar.Serialize())
}

func TestOnlyAttendees(t *testing.T) {
	for name, testCase := range map[string]struct {
		schedule    *Schedule
		emails      []string
		expectedIDs []string
	}{
		"filter single email": {
			schedule: newTestSchedule(
				[]event{
					{
						id:        "one",
						startTime: time.Now(),
						summary:   "event1",
						attendeeEmails: []string{
							"user1@example.com",
						},
					},
					{
						id:        "two",
						startTime: time.Now(),
						summary:   "event2",
						attendeeEmails: []string{
							"user1@example.com",
							"user2@example.com",
						},
					},
					{
						id:        "three",
						startTime: time.Now(),
						summary:   "event3",
						attendeeEmails: []string{
							"user3@example.com",
						},
					},
				},
			),
			emails:      []string{"user1@example.com"},
			expectedIDs: []string{"one", "two"},
		},
		"filter multiple emails": {
			schedule: newTestSchedule(
				[]event{
					{
						id:        "one",
						startTime: time.Now(),
						summary:   "event1",
						attendeeEmails: []string{
							"user1@example.com",
						},
					},
					{
						id:        "two",
						startTime: time.Now(),
						summary:   "event2",
						attendeeEmails: []string{
							"user1@example.com",
							"user2@example.com",
						},
					},
					{
						id:        "three",
						startTime: time.Now(),
						summary:   "event3",
						attendeeEmails: []string{
							"user3@example.com",
						},
					},
				},
			),
			emails:      []string{"user1@example.com", "user3@example.com"},
			expectedIDs: []string{"one", "two", "three"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			testCase := testCase
			schedule := testCase.schedule.OnlyAttendees(testCase.emails...)
			require.Len(t, schedule.Events(), len(testCase.expectedIDs))
			for _, id := range testCase.expectedIDs {
				require.True(t, schedule.ContainsEventID(id), "did not find ID %s in events", id)
			}
		})
	}
}

type event struct {
	id             string
	summary        string
	description    string
	startTime      time.Time
	attendeeEmails []string
}

func newTestSchedule(events []event) *Schedule {
	schedule := ics.NewCalendarFor("test")
	for _, event := range events {
		added := schedule.AddEvent(event.id)
		added.SetStartAt(event.startTime)
		added.SetEndAt(event.startTime.Add(24 * time.Hour))
		added.SetSummary(event.summary)
		desc := event.description
		if desc == "" {
			desc = event.summary
		}
		added.SetDescription(desc)
		for _, email := range event.attendeeEmails {
			added.AddAttendee(email)
		}
	}
	return &Schedule{Calendar: schedule}
}
