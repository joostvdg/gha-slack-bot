package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/google/go-github/v68/github"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/slack-go/slack"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type SlackCommand struct {
	Command       string
	Text          string
	TokenizedText []string
}

var (
	githubToken = os.Getenv("GITHUB_TOKEN")
	repoOwner   = os.Getenv("REPO_OWNER")
	repoName    = os.Getenv("REPO_NAME")
	appToken    = os.Getenv("SLACK_APP_TOKEN")
	botToken    = os.Getenv("SLACK_BOT_TOKEN")
)

func main() {
	if appToken == "" {
		panic("SLACK_APP_TOKEN must be set.\n")
	}

	if !strings.HasPrefix(appToken, "xapp-") {
		panic("SLACK_APP_TOKEN must have the prefix \"xapp-\".")
	}

	// When do we need this token?
	if botToken == "" {
		panic("SLACK_BOT_TOKEN must be set.\n")
	}

	if !strings.HasPrefix(botToken, "xoxb-") {
		panic("SLACK_BOT_TOKEN must have the prefix \"xoxb-\".")
	}

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", hello)
	e.POST("/trigger", triggerHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := e.Start(":" + port); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}

}

// Handler
func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

//api := slack.New(
//botToken,
//slack.OptionDebug(true),
//slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
//slack.OptionAppLevelToken(appToken),
//)

func triggerHandler(c echo.Context) error {
	// Log the request body
	slog.Info("Request Body", "body", c.Request().Body)
	params := &slack.Msg{
		Text:         "Hello World!",
		ResponseType: "ephemeral",
	}

	request := c.Request()
	// use a non-deprecated method to read the body
	request.Body = http.MaxBytesReader(c.Response(), request.Body, 1048576)
	slashCommand, err := slack.SlashCommandParse(request)
	if err != nil {
		slog.Error("Error parsing request", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Please provide valid request")
	}

	// print the SlashCommand
	slog.Info("Slash Command", "slashCommand", slashCommand)

	command, tokens := tokenizeSlackCommand(slashCommand.Text)
	slackCommand := SlackCommand{
		Command:       command,
		Text:          slashCommand.Text,
		TokenizedText: tokens,
	}
	// log the command
	slog.Info("Slack Command", "command", slackCommand)

	switch slackCommand.Command {
	case "help":
		params.Text = "Help Command..."
	case "trigger":
		slog.Info("Trigger Command, triggering workflow for owner/repo", "owner", repoOwner, "repo", repoName)
		err := triggerWorkflow(githubToken, repoOwner, repoName, slackCommand)
		if err != nil {
			params.Text = err.Error()
		} else {
			params.Text = "Workflow Triggered Successfully"
		}
	case "list":
		slog.Info("List Command, retrieving workflows for owner/repo", "owner", repoOwner, "repo", repoName)
		params.Text = "List Command..."
		list, err := listWorkflows(githubToken, repoOwner, repoName)
		if err != nil {
			params.Text = err.Error()
		} else {
			params.Text = list
		}

	default:
		params.Text = "Invalid Command..."
	}

	slog.Info("Returning Valid Response (I believe)")
	return c.JSON(http.StatusOK, params)
}

func createGitHubClient(token string) (*github.Client, error) {
	// Load system cert pool
	certPool, err := x509.SystemCertPool()
	if err != nil {
		slog.Error("Failed to load system cert pool", "error", err)
		return nil, err
	}

	// Create a custom HTTP client with the system cert pool
	customTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:            certPool,
			InsecureSkipVerify: true,
		},
	}
	customClient := &http.Client{
		Transport: customTransport,
	}

	return github.NewClient(customClient).WithAuthToken(token), nil
}

func triggerWorkflow(token string, repoOwner string, repoName string, command SlackCommand) error {
	client, err := createGitHubClient(token)
	if err != nil {
		slog.Error("Error creating GitHub client", "error", err)
		return err
	}
	ctx := context.Background()

	workflowFileName := command.TokenizedText[0]
	workflows, _, err2 := client.Actions.ListWorkflows(ctx, repoOwner, repoName, nil)
	if err2 != nil {
		slog.Error("Error listing workflows", "error", err2)
		return err2
	}

	workflowExists := false
	for _, workflow := range workflows.Workflows {
		if workflowFileName == parseWorkflowFilenameFromPath(workflow.GetPath()) {
			workflowExists = true
			break
		}
	}

	if !workflowExists {
		slog.Error("Workflow not found", "workflow", workflowFileName)
		return errors.New("workflow not found")
	}

	slog.Info("Triggering workflow", "workflow", workflowFileName)
	event := github.CreateWorkflowDispatchEventRequest{
		Ref: "main",
	}

	var dispathResponse *github.Response
	dispathResponse, err = client.Actions.CreateWorkflowDispatchEventByFileName(ctx, repoOwner, repoName, workflowFileName+".yml", event)
	if err != nil {
		slog.Error("Error triggering workflow", "error", err)
		return err
	}

	slog.Info("Workflow triggered", "response", dispathResponse)

	return nil

}

func listWorkflows(token string, owner string, repo string) (string, error) {
	client, err := createGitHubClient(token)
	if err != nil {
		slog.Error("Error creating GitHub client", "error", err)
		return "", err
	}

	ctx := context.Background()

	workflows, _, err2 := client.Actions.ListWorkflows(ctx, owner, repo, nil)
	if err2 != nil {
		slog.Error("Error listing workflows", "error", err2)
	}

	var list string
	for _, workflow := range workflows.Workflows {
		list += workflow.GetName() + "(" + parseWorkflowFilenameFromPath(workflow.GetPath()) + "), "
	}
	slog.Info("List of Workflows", "list", list)
	return list, nil
}

func parseWorkflowFilenameFromPath(path string) string {
	// something like .github/workflows/trigger.yml, and we want to return trigger
	numberOfSlashes := strings.Count(path, "/")
	return strings.Split(strings.Split(path, "/")[numberOfSlashes], ".")[0]
}

func tokenizeSlackCommand(text string) (string, []string) {
	tokens := strings.Split(text, " ")
	return tokens[0], tokens[1:]
}
