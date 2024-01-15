# Deprecated

Zenduty added support for [subscribing to an ICS calendar](https://docs.zenduty.com/docs/my-on-call#apple-calender).

# zenduty-calendar

Exposes a Zenduty schedule as an ICS calendar. Multiple endpoints are possible:

```
/calendar/:teamUUID/:scheduleUUID/:memberEmail
```

Return the specific zenduty team schedule as an ICS calendar. Only keep events
where the given member is part of. The ":team" and ":schedule" parts need to be
UUIDs of the corresponding zenduty team and schedule. The passed ":member" needs
to be an email address of the zenduty user for whom events should be retained.

```
/myschedule
```

Returns a combined ICS calendar from all zenduty schedules where the logged in
user (`ZENDUTY_USERNAME` env variable) is part of. Only events for the logged in
user will be retained.

```
/myschedule/:member
```

Similar to the above endpoint, but the the combined schedule will be created for
the given ":member" which needs to be an email address of a zenduty user.

## Configuration

The following environment variables can be configured:

* `PORT`: Port for the application to listen on
* `ZENDUTY_USERNAME`: Zenduty username (ics is only supported with user credentials)
* `ZENDUTY_PASSWORD`: Zenduty password
