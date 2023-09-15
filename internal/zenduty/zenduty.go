package zenduty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"golang.org/x/exp/slog"
	"golang.org/x/net/publicsuffix"
)

type Client struct {
	BaseURL    *url.URL
	HTTPCLient *http.Client
	Logger     *slog.Logger
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

func (c *Client) Login(username, password string) error {
	if c.HTTPCLient.Jar == nil {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			return fmt.Errorf("jar init error: %w", err)
		}

		c.HTTPCLient.Jar = jar
	}

	res, err := c.HTTPCLient.Get(fmt.Sprintf("%s/login/", c.BaseURL))
	if err != nil {
		return fmt.Errorf("error getting login page: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("error getting login page")
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(loginRequest{Email: username, Password: password})

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/account/loginAjax/", c.BaseURL), body)
	if err != nil {
		return fmt.Errorf("error creating login request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	c.Logger.Info("request", "req", req)

	res, err = c.do(req)
	if err != nil {
		return fmt.Errorf("error logging in: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("error logging in")
	}

	var respBody loginResponse
	err = json.NewDecoder(res.Body).Decode(&respBody)
	if err != nil {
		return fmt.Errorf("error logging in: %w", err)
	}

	c.Logger.Info("cookies", "jar", c.HTTPCLient.Jar)

	return nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	cookies := c.HTTPCLient.Jar.Cookies(c.BaseURL)
	for _, cookie := range cookies {
		if cookie.Name != "csrftoken" {
			continue
		}

		req.Header.Set("X-CSRFToken", cookie.Value)
		break
	}

	return c.HTTPCLient.Do(req)
}

func (c *Client) GetSchedule(teamID, scheduleID string, months int) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/api/account/teams/%s/schedules/%s/get_schedule_ics/?months=%d&is_team_or_user=1", c.BaseURL, teamID, scheduleID, months)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request schedule %s, team %s: %w", scheduleID, teamID, err)
	}
	req.Header.Set("content-type", "application/json")

	res, err := c.HTTPCLient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error requesting schedule %s, team %s: %w", scheduleID, teamID, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("error requesting schedule %s, team %s: status: %s", scheduleID, teamID, res.Status)
	}

	var body scheduleICSResponse
	err = json.NewDecoder(res.Body).Decode(&body)
	if err != nil {
		return nil, fmt.Errorf("error decoding url for schedule %s, team %s: %w", scheduleID, teamID, err)
	}

	res, err = c.HTTPCLient.Get(body.URL)
	if err != nil {
		return nil, fmt.Errorf("error requesting ics for schedule %s, team %s: %w", scheduleID, teamID, err)
	}

	return res.Body, err
}
