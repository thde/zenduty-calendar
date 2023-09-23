package zenduty

import (
	"os"
	"testing"

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
	z := NewClient(Logger(logger))
	require.NoError(t, z.Login(username, password))

	calendar, err := z.CombinedSchedule(username)
	require.NoError(t, err)
	require.True(t, len(calendar.Events()) > 0)
	logger.Debug("my schedule", "content", calendar.Serialize())
}
