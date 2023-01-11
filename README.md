### sulane [![sulane](https://github.com/lafin/sulane/actions/workflows/app.yml/badge.svg)](https://github.com/lafin/sulane/actions/workflows/app.yml)

> The idea is to provide a tool to generate a report with all repositories and display the status of actions for that repository (passed or failed)

![](assets/image.png)

### How to use

```sh
$ go install github.com/lafin/sulane
$ sulane -h
  Usage of ./sulane:
  -last string
        get the results of actions for the last days (default "30d")
  -login string
        github login
  -reactivateSuspended
        should reactivate a suspended workflows (default true)
  -restart
        should restarted failed
  -skipArchive
        skip archived (default true)
  -token string
        github token
  -verbose
        verbose mode (default true)
```
