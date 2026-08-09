package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/application"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/gin-gonic/gin"
)

func (api *API) listDeployments(c *gin.Context) {
	items, next, err := api.app.ListDeployments(c.Request.Context(), queryLimit(c), c.Query("cursor"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	values := make([]gin.H, len(items))
	for index, item := range items {
		values[index] = deploymentDTO(item)
	}
	writePage(c, values, next)
}

func (api *API) listMarketQuotes(c *gin.Context) {
	items, err := api.app.ListMarketQuotes(c.Request.Context(), queryLimit(c), strings.ToUpper(c.Query("symbol")))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	values := make([]gin.H, len(items))
	for index, item := range items {
		values[index] = marketQuoteDTO(item)
	}
	writePage(c, values, "")
}

func (api *API) getWeather(c *gin.Context) {
	value, err := api.app.Weather(c.Request.Context(), c.Query("location"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	if value == nil {
		api.writeProblem(c, domain.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, weatherDTO(*value))
}

type uptimeMonitor struct {
	ID   any     `json:"id"`
	Name string  `json:"name"`
	URL  *string `json:"url"`
}

type uptimeHeartbeat struct {
	Status int      `json:"status"`
	Time   string   `json:"time"`
	Ping   *float64 `json:"ping"`
	Msg    *string  `json:"msg"`
}

type uptimeEventRequest struct {
	Monitor   *uptimeMonitor   `json:"monitor"`
	Heartbeat *uptimeHeartbeat `json:"heartbeat"`
	Msg       string           `json:"msg"`
}

func (api *API) uptimeEvent(c *gin.Context) {
	if !validBearer(c.GetHeader("Authorization"), api.integrations.UptimeToken) {
		api.writeProblem(c, domain.ErrUnauthorized)
		return
	}
	raw, ok := api.readWebhook(c, 1<<20)
	if !ok {
		return
	}
	var request uptimeEventRequest
	if json.Unmarshal(raw, &request) != nil {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	// Uptime Kuma's notification test intentionally has no monitor or heartbeat.
	// Accept it as a connectivity check without creating a delivery or changing state.
	if request.Monitor == nil && request.Heartbeat == nil && strings.TrimSpace(request.Msg) != "" {
		api.logger.Info("uptime notification test accepted")
		c.Status(http.StatusAccepted)
		return
	}
	if request.Monitor == nil || request.Heartbeat == nil || strings.TrimSpace(request.Monitor.Name) == "" || (request.Heartbeat.Status != 0 && request.Heartbeat.Status != 1) {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	target, exists := api.uptimeTarget(request.Monitor.Name)
	if !exists {
		api.logger.Warn("unmapped uptime monitor ignored", "monitor", request.Monitor.Name)
		c.Status(http.StatusAccepted)
		return
	}
	observedAt := parseEventTime(request.Heartbeat.Time)
	message := request.Msg
	if request.Heartbeat.Msg != nil && strings.TrimSpace(*request.Heartbeat.Msg) != "" {
		message = *request.Heartbeat.Msg
	}
	var latency *int
	if request.Heartbeat.Ping != nil {
		value := int(*request.Heartbeat.Ping + 0.5)
		latency = &value
	}
	monitorURL := ""
	if request.Monitor.URL != nil {
		monitorURL = *request.Monitor.URL
	}
	deliveryID := c.GetHeader("Idempotency-Key")
	if deliveryID == "" {
		deliveryID = fmt.Sprintf("%v:%s:%d", request.Monitor.ID, request.Heartbeat.Time, request.Heartbeat.Status)
	}
	err := api.app.HandleUptime(c.Request.Context(), target, application.UptimeInput{DeliveryID: deliveryID, MonitorName: request.Monitor.Name, MonitorURL: monitorURL, Message: message, Status: request.Heartbeat.Status, LatencyMS: latency, ObservedAt: observedAt}, raw)
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

type githubActionRequest struct {
	ProductKey    string     `json:"product_key"`
	Repository    string     `json:"repository"`
	Branch        string     `json:"branch"`
	Workflow      string     `json:"workflow"`
	Status        string     `json:"status"`
	CommitSHA     string     `json:"commit_sha"`
	CommitMessage string     `json:"commit_message"`
	Author        string     `json:"author"`
	OccurredAt    time.Time  `json:"occurred_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	HTMLURL       *string    `json:"html_url"`
}

func (api *API) githubActions(c *gin.Context) {
	if !validBearer(c.GetHeader("Authorization"), api.integrations.GitHubActionsToken) {
		api.writeProblem(c, domain.ErrUnauthorized)
		return
	}
	raw, ok := api.readWebhook(c, 1<<20)
	if !ok {
		return
	}
	var request githubActionRequest
	if json.Unmarshal(raw, &request) != nil || request.Repository == "" || request.Branch == "" || request.Workflow == "" || request.CommitSHA == "" || request.Author == "" || request.OccurredAt.IsZero() {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	status := normalizeDeploymentStatus(request.Status)
	if status == "" {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	productKey := request.ProductKey
	if productKey == "" {
		productKey = repositoryKey(request.Repository)
	}
	value := domain.Deployment{ProductID: api.app.ProductIDByKey(c.Request.Context(), productKey), Provider: "github-actions", ExternalID: c.GetHeader("Idempotency-Key"), Repository: request.Repository, Branch: request.Branch, Workflow: request.Workflow, Status: status, CommitSHA: request.CommitSHA, CommitMessage: request.CommitMessage, Author: request.Author, StartedAt: request.OccurredAt.UTC(), FinishedAt: request.FinishedAt, HTMLURL: request.HTMLURL}
	if value.ExternalID == "" {
		value.ExternalID = request.Repository + ":" + request.Workflow + ":" + request.CommitSHA + ":" + request.Status
	}
	if err := api.app.HandleDeployment(c.Request.Context(), value, value.ExternalID, raw); err != nil {
		api.writeProblem(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

type githubWebhookRequest struct {
	Action     string `json:"action"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
	WorkflowRun struct {
		ID         int64     `json:"id"`
		Name       string    `json:"name"`
		HeadBranch string    `json:"head_branch"`
		HeadSHA    string    `json:"head_sha"`
		Status     string    `json:"status"`
		Conclusion string    `json:"conclusion"`
		RunStarted time.Time `json:"run_started_at"`
		UpdatedAt  time.Time `json:"updated_at"`
		HTMLURL    string    `json:"html_url"`
		HeadCommit struct {
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"head_commit"`
	} `json:"workflow_run"`
}

func (api *API) githubWebhook(c *gin.Context) {
	raw, ok := api.readWebhook(c, 2<<20)
	if !ok {
		return
	}
	if !validGitHubSignature(raw, c.GetHeader("X-Hub-Signature-256"), api.integrations.GitHubWebhookSecret) {
		api.writeProblem(c, domain.ErrUnauthorized)
		return
	}
	if c.GetHeader("X-GitHub-Event") == "ping" {
		c.Status(http.StatusAccepted)
		return
	}
	if c.GetHeader("X-GitHub-Event") != "workflow_run" {
		c.Status(http.StatusAccepted)
		return
	}
	var request githubWebhookRequest
	if json.Unmarshal(raw, &request) != nil || request.WorkflowRun.ID == 0 || request.Repository.FullName == "" {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	status := request.WorkflowRun.Status
	if status == "completed" {
		status = request.WorkflowRun.Conclusion
	}
	status = normalizeDeploymentStatus(status)
	if status == "" {
		status = "in_progress"
	}
	started := request.WorkflowRun.RunStarted
	if started.IsZero() {
		started = time.Now().UTC()
	}
	var finished *time.Time
	if request.WorkflowRun.Status == "completed" && !request.WorkflowRun.UpdatedAt.IsZero() {
		value := request.WorkflowRun.UpdatedAt.UTC()
		finished = &value
	}
	url := request.WorkflowRun.HTMLURL
	author := request.WorkflowRun.HeadCommit.Author.Name
	if author == "" {
		author = request.Sender.Login
	}
	value := domain.Deployment{ProductID: api.app.ProductIDByKey(c.Request.Context(), repositoryKey(request.Repository.FullName)), Provider: "github", ExternalID: strconv.FormatInt(request.WorkflowRun.ID, 10), Repository: request.Repository.FullName, Branch: request.WorkflowRun.HeadBranch, Workflow: request.WorkflowRun.Name, Status: status, CommitSHA: request.WorkflowRun.HeadSHA, CommitMessage: request.WorkflowRun.HeadCommit.Message, Author: author, StartedAt: started.UTC(), FinishedAt: finished, HTMLURL: &url}
	if err := api.app.HandleDeployment(c.Request.Context(), value, c.GetHeader("X-GitHub-Delivery"), raw); err != nil {
		api.writeProblem(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (api *API) readWebhook(c *gin.Context, maximum int64) ([]byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximum)
	value, err := io.ReadAll(c.Request.Body)
	if err != nil || len(value) == 0 {
		api.writeProblem(c, domain.ErrInvalid)
		return nil, false
	}
	return value, true
}

func (api *API) uptimeTarget(name string) (application.MonitorTarget, bool) {
	for configured, target := range api.integrations.UptimeMonitors {
		if strings.EqualFold(strings.TrimSpace(configured), strings.TrimSpace(name)) {
			return target, true
		}
	}
	return application.MonitorTarget{}, false
}

func validBearer(header, expected string) bool {
	const prefix = "Bearer "
	if expected == "" || !strings.HasPrefix(header, prefix) {
		return false
	}
	actual := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return len(actual) == len(expected) && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func validGitHubSignature(payload []byte, signature, secret string) bool {
	if secret == "" || !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return hmac.Equal(provided, mac.Sum(nil))
}

func parseEventTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04:05.000"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func normalizeDeploymentStatus(value string) string {
	switch value {
	case "queued", "in_progress", "success", "failure", "cancelled":
		return value
	case "requested", "waiting", "pending":
		return "queued"
	case "timed_out", "action_required", "startup_failure":
		return "failure"
	case "skipped", "stale":
		return "cancelled"
	default:
		return ""
	}
}

func repositoryKey(repository string) string {
	value := strings.ToLower(path.Base(strings.TrimSpace(repository)))
	var result strings.Builder
	lastDash := false
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			result.WriteRune(character)
			lastDash = false
		} else if !lastDash {
			result.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(result.String(), "-")
}

func deploymentDTO(value domain.Deployment) gin.H {
	return gin.H{"id": value.ID, "product_id": value.ProductID, "repository": value.Repository, "branch": value.Branch, "workflow": value.Workflow, "status": value.Status, "commit_sha": value.CommitSHA, "commit_message": value.CommitMessage, "author": value.Author, "started_at": value.StartedAt, "finished_at": value.FinishedAt, "html_url": value.HTMLURL}
}

func marketQuoteDTO(value domain.MarketQuote) gin.H {
	return gin.H{"provider": value.Provider, "symbol": value.Symbol, "quote_asset": value.QuoteAsset, "bid": value.Bid, "ask": value.Ask, "last": value.Last, "change_percent": value.ChangePercent, "observed_at": value.ObservedAt, "stale": time.Since(value.ObservedAt) > 30*time.Minute}
}

func weatherDTO(value domain.WeatherObservation) gin.H {
	return gin.H{"location": value.LocationName, "temperature_c": value.TemperatureC, "apparent_temperature_c": value.ApparentTemperatureC, "humidity_percent": value.HumidityPercent, "weather_code": value.WeatherCode, "wind_kmh": value.WindKMH, "daily_min_c": value.DailyMinC, "daily_max_c": value.DailyMaxC, "precipitation_probability_percent": value.PrecipitationProbability, "observed_at": value.ObservedAt, "stale": time.Since(value.ObservedAt) > 45*time.Minute}
}
