# gha-slack-bot

Slack bot for interacting with GitHub Action Workflows

## Run Locally

```shell
export SLACK_BOT_TOKEN=your_slack_bot_token  
export SLACK_APP_TOKEN=your_slack_app_token
```

```shell
make run
```

#### Test Locally

```shell
http --form POST :8080/trigger text='Hello World' command='test'
```

```shell
http --form POST :8080/trigger text='my-test-workflow' command='run'
```