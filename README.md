# zenduty-calendar

Exposes a Zenduty schedule as an ICS calendar:

```
/calendar/:team/:schedule/:member
```

## Configuration

The following environment variables can be configured:

* PORT: Port for the application to listen on
* ZENDUTY_USERNAME: Zenduty username (ics is only supported with user credentials)
* ZENDUTY_PASSWORD: Zenduty password
