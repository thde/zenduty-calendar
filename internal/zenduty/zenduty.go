package zenduty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
	"golang.org/x/net/publicsuffix"
)

const (
	defaultTimeout = 5 * time.Second
	amountMonths   = 12
)

type Client struct {
	credentials func() (username string, password string)
	http        *http.Client
	baseURL     *url.URL
	logger      *slog.Logger
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Success bool `json:"success"`
}

type scheduleICSResponse struct {
	URL string `json:"url"`
}

func defaultBaseURL() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   "www.zenduty.com",
	}
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: defaultTimeout,
	}
}

type LoggerOptions struct {
	slog.HandlerOptions
	Out io.Writer
}

// NewLogger returns a new logger instance which logs to the given output
func NewLogger(options LoggerOptions) *slog.Logger {
	var output io.Writer = os.Stdout
	if options.Out != nil {
		output = options.Out
	}
	return slog.New(slog.NewTextHandler(output, &options.HandlerOptions)).With("pkg", "zenduty")
}

// NewClient returns a new zenduty client which can be modified by passing
// options
func NewClient(credentials func() (string, string), opts ...ClientOption) *Client {
	c := &Client{
		credentials: credentials,
		baseURL:     defaultBaseURL(),
		http:        defaultHTTPClient(),
		logger:      NewLogger(LoggerOptions{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type ClientOption func(c *Client)

// BaseURL sets the base URL of the Zenduty client
func BaseURL(u *url.URL) ClientOption {
	return func(c *Client) {
		c.baseURL = u
	}
}

// Logger sets the logger of the Zenduty client
func Logger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// Login executes a login with the given username and password
func (c *Client) Login() error {
	if c.http.Jar == nil {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			return fmt.Errorf("jar init error: %w", err)
		}

		c.http.Jar = jar
	}

	res, err := c.http.Get(fmt.Sprintf("%s/login/", c.baseURL))
	if err != nil {
		return fmt.Errorf("error getting login page: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("error getting login page")
	}

	username, password := c.credentials()
	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(loginRequest{Email: username, Password: password}); err != nil {
		return fmt.Errorf("can not encode login body: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/account/loginAjax/", c.baseURL), body)
	if err != nil {
		return fmt.Errorf("error creating login request: %w", err)
	}
	resp := &loginResponse{}
	if err = c.do(req, resp); err != nil {
		return fmt.Errorf("error logging in: %w", err)
	}

	c.logger.Info("cookies", "jar", c.http.Jar)
	return nil
}

func (c *Client) doLoggedIn(req *http.Request, obj interface{}) error {
	if c.http.Jar == nil {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			return fmt.Errorf("jar init error: %w", err)
		}

		c.http.Jar = jar
	}

	if !c.isLoggedIn() {
		err := c.Login()
		if err != nil {
			return err
		}
	}

	return c.do(req, obj)
}

func (c *Client) isLoggedIn() bool {
	for _, cookie := range c.http.Jar.Cookies(c.baseURL) {
		if cookie.Name != "sessionid" {
			continue
		}

		if cookie.Expires.Before(time.Now()) {
			return false
		}

		return true
	}

	return false
}

func (c *Client) do(req *http.Request, obj interface{}) error {
	req.Header.Set("content-type", "application/json")
	for _, cookie := range c.http.Jar.Cookies(c.baseURL) {
		if cookie.Name != "csrftoken" {
			continue
		}
		req.Header.Set("X-CSRFToken", cookie.Value)
		break
	}

	c.logger.Debug("request", "req", req)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("received error code %d from server", res.StatusCode)
	}
	if obj == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(obj)
}

type team struct {
	ID      string          `json:"unique_id"`
	Name    string          `json:"name"`
	Account string          `json:"account"`
	Members []apiTeamMember `json:"members"`
}

func (t team) containsUser(email string) bool {
	for _, member := range t.Members {
		if member.User.Email == email {
			return true
		}
	}
	return false
}

type teamList []team

func (tl teamList) teamsForUser(email string) teamList {
	result := []team{}
	for _, team := range tl {
		if team.containsUser(email) {
			result = append(result, team)
		}
	}
	return result
}

type apiUser struct {
	ID        string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

type apiTeamMember struct {
	ID          string    `json:"unique_id"`
	User        apiUser   `json:"user"`
	JoiningDate time.Time `json:"joining_date"`
}

type apiSchedule struct {
	ID          string `json:"unique_id"`
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

// Schedule wraps a calendar to provide additional functions
type Schedule struct {
	*ics.Calendar
}

// Filter filters the schedule based on the given property and value function
func (s *Schedule) filter(prop ics.ComponentProperty, valueFilter func(value string) bool) *Schedule {
	out := *s.Calendar
	out.Components = []ics.Component{}

	for _, event := range s.Events() {
		if valueFilter(event.GetProperty(prop).Value) {
			out.AddVEvent(event)
		}
	}
	return &Schedule{Calendar: &out}
}

// ContainsEventID returns true if the schedule contains the given event ID.
// Otherwise it returns false.
func (s *Schedule) ContainsEventID(id string) bool {
	ids := make([]string, len(s.Events()))
	for i, event := range s.Events() {
		ids[i] = event.Id()
	}
	return slices.Contains(ids, id)
}

// OnlyAttendees keeps only the events where at least one of the given emails
// is an attendee
func (s *Schedule) OnlyAttendees(emails ...string) *Schedule {
	return s.filter(
		ics.ComponentPropertyAttendee,
		func(value string) bool {
			for _, email := range emails {
				if strings.Contains(value, email) {
					return true
				}
			}
			return false
		},
	)
}

func newScheduleFrom(data io.Reader) (*Schedule, error) {
	cal, err := ics.ParseCalendar(data)
	if err != nil {
		return nil, fmt.Errorf("got error when parsing calendar: %w", err)
	}
	return &Schedule{Calendar: cal}, nil
}

func (c *Client) listTeams() (teamList, error) {
	url := fmt.Sprintf("%s/api/account/teams", c.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("can't create request to list teams: %w", err)
	}
	teamsResp := []team{}
	if err := c.doLoggedIn(req, &teamsResp); err != nil {
		return nil, fmt.Errorf("can't list teams: %w", err)
	}
	return teamsResp, nil
}

func (c *Client) listSchedules(teamID string) ([]apiSchedule, error) {
	url := fmt.Sprintf("%s/api/account/teams/%s/schedules", c.baseURL, teamID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for listing team schedules: %w", err)
	}

	scheduleList := []apiSchedule{}
	if err := c.doLoggedIn(req, &scheduleList); err != nil {
		return nil, fmt.Errorf("error when listing schedules of team with ID %s: %w", teamID, err)
	}
	return scheduleList, nil
}

// CombinedSchedule returns the full combined schedule of all team schedules where the given
// user email is part of
func (c *Client) CombinedSchedule(email string) (*Schedule, error) {
	teams, err := c.listTeams()
	if err != nil {
		return nil, err
	}
	teamsOfUser := teams.teamsForUser(email)
	combined := ics.NewCalendarFor("zenduty-oncall")
	for _, team := range teamsOfUser {
		schedules, err := c.listSchedules(team.ID)
		if err != nil {
			return nil, err
		}
		for _, schedule := range schedules {
			calendar, err := c.GetSchedule(team.ID, schedule.ID, amountMonths)
			if err != nil {
				return nil, err
			}
			c.logger.Debug("got calendar response", "content", string(calendar.Serialize()))
			for _, event := range calendar.Events() {
				event := event
				if event == nil {
					continue
				}
				msg := fmt.Sprintf("on call for team %s", team.Name)
				event.SetSummary(msg)
				event.SetDescription(fmt.Sprintf("%s (schedule: %s)", msg, schedule.Name))
				combined.AddVEvent(event)
			}
		}
	}
	return &Schedule{Calendar: combined}, nil
}

func (c *Client) GetSchedule(teamID, scheduleID string, months int) (*Schedule, error) {
	url := fmt.Sprintf("%s/api/account/teams/%s/schedules/%s/get_schedule_ics/?months=%d&is_team_or_user=1", c.baseURL, teamID, scheduleID, months)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request schedule %q of team %q: %w", scheduleID, teamID, err)
	}
	body := scheduleICSResponse{}
	if err := c.doLoggedIn(req, &body); err != nil {
		return nil, fmt.Errorf("error requesting schedule %q of team %q: %w", scheduleID, teamID, err)
	}
	res, err := c.http.Get(body.URL)
	if err != nil {
		return nil, fmt.Errorf("error requesting ics for schedule %q of team %q: %w", scheduleID, teamID, err)
	}
	return newScheduleFrom(res.Body)
}
