### sulane [![sulane](https://github.com/lafin/sulane/actions/workflows/app.yml/badge.svg)](https://github.com/lafin/sulane/actions/workflows/app.yml)

> The idea is to provide a tool to generate a report with all repositories and display the status of actions for that repository (passed or failed)

![](assets/image.png)

### How to use

```sh
$ go install github.com/lafin/sulane@latest
$ sulane -h
  Usage of ./sulane:
  -config string
    	path to config file (default "config.yaml")
  -verbose
    	verbose mode (default true)
```

### Config file

```yaml
github_login: "your-github-login"
access_token: "your-access-token"
# should_restart_failed: false
# should_reactivate_suspended_workflows: true
last: "7d"
# skip_archive: true
# do_merge_one_pr_per_day_if_no_action_today: false
```
