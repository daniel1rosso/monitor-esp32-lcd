// Package mqtt implements the event publisher and Mosquitto Dynamic Security adapter.
package mqtt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
	paho "github.com/eclipse/paho.mqtt.golang"
)

type Config struct {
	BrokerURL, Host                                                                       string
	Port                                                                                  int
	AdminUsername, AdminPasswordFile, BackendUsername, BackendPasswordFile, ControlBinary string
}

func ConfigFromEnvironment() Config {
	return Config{BrokerURL: env("MQTT_URL", "tcp://mosquitto:1883"), Host: env("MQTT_HOST", "mosquitto"), Port: envInt("MQTT_PORT", 1883), AdminUsername: env("MQTT_ADMIN_USERNAME", "admin"), AdminPasswordFile: env("MQTT_ADMIN_PASSWORD_FILE", "/run/mosquitto/admin-password"), BackendUsername: env("MQTT_BACKEND_USERNAME", "desk-backend"), BackendPasswordFile: env("MQTT_BACKEND_PASSWORD_FILE", "/run/mosquitto/backend-password"), ControlBinary: env("MOSQUITTO_CTRL_PATH", "/usr/bin/mosquitto_ctrl")}
}

type DynamicSecurity struct {
	config        Config
	adminPassword string
}

func NewDynamicSecurity(config Config) (*DynamicSecurity, error) {
	password, err := readSecret(config.AdminPasswordFile)
	if err != nil {
		return nil, fmt.Errorf("read MQTT admin password: %w", err)
	}
	return &DynamicSecurity{config: config, adminPassword: password}, nil
}
func (manager *DynamicSecurity) EnsureBackend(ctx context.Context) error {
	password, err := readSecret(manager.config.BackendPasswordFile)
	if err != nil {
		return fmt.Errorf("read MQTT backend password: %w", err)
	}
	created, err := manager.ensureRole(ctx, "desk-backend")
	if err != nil {
		return err
	}
	if created {
		for _, acl := range [][]string{{"publishClientSend", "desk/#", "100", "allow"}, {"publishClientReceive", "desk/#", "100", "allow"}, {"subscribePattern", "desk/device/+/status", "100", "allow"}, {"subscribePattern", "desk/device/+/telemetry", "100", "allow"}, {"subscribePattern", "desk/device/+/acks", "100", "allow"}} {
			if err := manager.run(ctx, append([]string{"addRoleACL", "desk-backend"}, acl...)...); err != nil {
				return err
			}
		}
	}
	exists, err := manager.exists(ctx, "getClient", manager.config.BackendUsername)
	if err != nil {
		return err
	}
	if !exists {
		if err := manager.run(ctx, "createClient", manager.config.BackendUsername, "-i", manager.config.BackendUsername, "-p", password); err != nil {
			return err
		}
		if err := manager.run(ctx, "addClientRole", manager.config.BackendUsername, "desk-backend", "100"); err != nil {
			return err
		}
	}
	return nil
}
func (manager *DynamicSecurity) ProvisionDevice(ctx context.Context, deviceID, username, password string) error {
	role := "device-" + deviceID
	created, err := manager.ensureRole(ctx, role)
	if err != nil {
		return err
	}
	if created {
		read := []string{"alerts", "notifications", "commands"}
		for _, suffix := range read {
			topic := "desk/device/" + deviceID + "/" + suffix
			for _, kind := range []string{"subscribeLiteral", "publishClientReceive"} {
				if err := manager.run(ctx, "addRoleACL", role, kind, topic, "100", "allow"); err != nil {
					return err
				}
			}
		}
		for _, kind := range []string{"subscribeLiteral", "publishClientReceive"} {
			if err := manager.run(ctx, "addRoleACL", role, kind, "desk/global/events", "100", "allow"); err != nil {
				return err
			}
		}
		for _, suffix := range []string{"status", "telemetry", "acks"} {
			if err := manager.run(ctx, "addRoleACL", role, "publishClientSend", "desk/device/"+deviceID+"/"+suffix, "100", "allow"); err != nil {
				return err
			}
		}
	}
	exists, err := manager.exists(ctx, "getClient", username)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("MQTT client %s already exists", username)
	}
	if err := manager.run(ctx, "createClient", username, "-i", deviceID, "-p", password); err != nil {
		return err
	}
	if err := manager.run(ctx, "addClientRole", username, role, "100"); err != nil {
		_ = manager.run(ctx, "deleteClient", username)
		return err
	}
	return nil
}
func (manager *DynamicSecurity) RevokeDevice(ctx context.Context, deviceID, username string, removeRole bool) error {
	exists, err := manager.exists(ctx, "getClient", username)
	if err != nil {
		return err
	}
	if exists {
		if err := manager.run(ctx, "deleteClient", username); err != nil {
			return err
		}
	}
	if removeRole {
		role := "device-" + deviceID
		exists, err := manager.exists(ctx, "getRole", role)
		if err != nil {
			return err
		}
		if exists {
			return manager.run(ctx, "deleteRole", role)
		}
	}
	return nil
}
func (manager *DynamicSecurity) ensureRole(ctx context.Context, role string) (bool, error) {
	exists, err := manager.exists(ctx, "getRole", role)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	return true, manager.run(ctx, "createRole", role)
}
func (manager *DynamicSecurity) exists(ctx context.Context, command, name string) (bool, error) {
	err := manager.run(ctx, command, name)
	if err == nil {
		return true, nil
	}
	var commandError *commandError
	if errors.As(err, &commandError) && strings.Contains(strings.ToLower(commandError.output), "not found") {
		return false, nil
	}
	return false, err
}

type commandError struct {
	output string
	cause  error
}

func (err *commandError) Error() string {
	detail := strings.TrimSpace(err.output)
	if detail == "" && err.cause != nil {
		detail = err.cause.Error()
	}
	return "mosquitto dynamic security command failed: " + detail
}
func (manager *DynamicSecurity) run(ctx context.Context, args ...string) error {
	options, err := os.CreateTemp("", "desk-mosquitto-ctrl-*")
	if err != nil {
		return fmt.Errorf("create mosquitto control options: %w", err)
	}
	optionsPath := options.Name()
	defer os.Remove(optionsPath)
	if err := options.Chmod(0600); err != nil {
		options.Close()
		return fmt.Errorf("secure mosquitto control options: %w", err)
	}
	_, writeErr := fmt.Fprintf(options, "-h %s\n-p %d\n-u %s\n-P %s\n", manager.config.Host, manager.config.Port, manager.config.AdminUsername, manager.adminPassword)
	closeErr := options.Close()
	if writeErr != nil || closeErr != nil {
		return fmt.Errorf("write mosquitto control options: %w", errors.Join(writeErr, closeErr))
	}
	base := []string{"-o", optionsPath, "dynsec"}
	command := exec.CommandContext(ctx, manager.config.ControlBinary, append(base, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return &commandError{output: string(output), cause: err}
	}
	if outputHasError(output) {
		return &commandError{output: string(output)}
	}
	return nil
}

func outputHasError(output []byte) bool {
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(strings.ToLower(line), "error:") {
			return true
		}
	}
	return false
}

type Publisher struct{ client paho.Client }

func NewPublisher(config Config) (*Publisher, error) {
	password, err := readSecret(config.BackendPasswordFile)
	if err != nil {
		return nil, err
	}
	options := paho.NewClientOptions().AddBroker(config.BrokerURL).SetClientID(config.BackendUsername).SetUsername(config.BackendUsername).SetPassword(password).SetAutoReconnect(true).SetConnectRetry(false).SetOrderMatters(false)
	client := paho.NewClient(options)
	token := client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return nil, fmt.Errorf("MQTT connect timeout")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("MQTT connect: %w", err)
	}
	return &Publisher{client: client}, nil
}
func (publisher *Publisher) Publish(ctx context.Context, topic string, payload []byte) error {
	token := publisher.client.Publish(topic, 1, false, payload)
	done := make(chan struct{})
	go func() { token.Wait(); close(done) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return token.Error()
	case <-time.After(10 * time.Second):
		return fmt.Errorf("MQTT publish timeout")
	}
}
func (publisher *Publisher) Close() { publisher.client.Disconnect(500) }
func readSecret(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", fmt.Errorf("secret file %s is empty", path)
	}
	return value, nil
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

var _ ports.MQTTCredentialManager = (*DynamicSecurity)(nil)
var _ ports.EventPublisher = (*Publisher)(nil)
