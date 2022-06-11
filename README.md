### github-action-status (gas) [![github-action-status](https://github.com/lafin/github-action-status/actions/workflows/app.yml/badge.svg)](https://github.com/lafin/github-action-status/actions/workflows/app.yml)

> The idea is to provide a tool to generate a report with all repositories and display the status of actions for that repository (passed or failed)

![](assets/image.png)

### How to use

```sh
$ go install github.com/lafin/github-action-status
$ github-action-status -h
  Usage of ./github-action-status:
  -last string
    	get the results of actions for the last days (default "30d")
  -login string
    	github login
  -restart
    	should restarted failed (default: false)
  -skipArchive
    	skip archived (default: true) (default true)
  -token string
    	github token
  -verbose
    	verbose mode (default: true) (default true)
```
