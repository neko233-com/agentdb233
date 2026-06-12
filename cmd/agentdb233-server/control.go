package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/agentdb233/internal/server"
	"github.com/neko233-com/agentdb233/internal/version"
)

type runtimeConfig struct {
	Addr string `json:"addr"`
}

type runtimeState struct {
	PID          int    `json:"pid"`
	Addr         string `json:"addr"`
	DataDir      string `json:"data_dir"`
	ControlToken string `json:"control_token"`
	StartedAt    string `json:"started_at"`
	Version      string `json:"version"`
}

func serve(listen, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	if err := saveRuntimeConfig(dataDir, runtimeConfig{Addr: listen}); err != nil {
		return err
	}
	token, err := randomHex(16)
	if err != nil {
		return err
	}
	state := runtimeState{
		PID:          os.Getpid(),
		Addr:         listen,
		DataDir:      dataDir,
		ControlToken: token,
		StartedAt:    time.Now().Format(time.RFC3339),
		Version:      version.String("agentdb233-server"),
	}
	if err := saveRuntimeState(dataDir, state); err != nil {
		return err
	}
	defer cleanupRuntimeState(dataDir, state.PID)

	srv := &http.Server{Addr: listen}
	mux := http.NewServeMux()
	mux.HandleFunc("/__admin/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if !authorizeControlRequest(r, token) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		}()
	})
	mux.Handle("/", server.New(dataDir).Router())
	srv.Handler = mux

	fmt.Printf("agentdb233-server listening addr=%s url=%s data=%s\n", listen, browserURL(listen), dataDir)
	err = srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func resolveDataDir(flagValue string, flagSet bool) string {
	if flagSet && strings.TrimSpace(flagValue) != "" {
		return flagValue
	}
	if d := os.Getenv("AGENTDB233_DATA"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agentdb233-server")
}

func resolveListenAddr(dataDir, flagValue string, flagSet bool) (string, error) {
	if flagSet && strings.TrimSpace(flagValue) != "" {
		return flagValue, nil
	}
	if a := os.Getenv("AGENTDB233_ADDR"); a != "" {
		return a, nil
	}
	cfg, err := loadRuntimeConfig(dataDir)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.Addr) != "" {
		return cfg.Addr, nil
	}
	return defaultListenAddr(), nil
}

func defaultListenAddr() string { return "127.0.0.1:23390" }
func defaultPort() string       { return "23390" }

func normalizeListenAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func browserURL(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + listen
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func runtimeDir(dataDir string) string { return filepath.Join(dataDir, "run") }
func serverRuntimeConfigPath(dataDir string) string {
	return filepath.Join(runtimeDir(dataDir), "server-runtime.json")
}
func runtimeStatePath(dataDir string) string {
	return filepath.Join(runtimeDir(dataDir), "server-state.json")
}
func runtimePIDPath(dataDir string) string { return filepath.Join(runtimeDir(dataDir), "server.pid") }
func runtimeLogPath(dataDir string) string { return filepath.Join(runtimeDir(dataDir), "server.log") }

func loadRuntimeConfig(dataDir string) (runtimeConfig, error) {
	var cfg runtimeConfig
	b, err := os.ReadFile(serverRuntimeConfigPath(dataDir))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(b, &cfg)
}

func saveRuntimeConfig(dataDir string, cfg runtimeConfig) error {
	if err := os.MkdirAll(runtimeDir(dataDir), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(serverRuntimeConfigPath(dataDir), append(b, '\n'), 0o644)
}

func saveRuntimeState(dataDir string, st runtimeState) error {
	if err := os.MkdirAll(runtimeDir(dataDir), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(runtimeStatePath(dataDir), append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(runtimePIDPath(dataDir), []byte(strconv.Itoa(st.PID)+"\n"), 0o644)
}

func loadRuntimeState(dataDir string) (runtimeState, string, bool, error) {
	path := runtimeStatePath(dataDir)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return runtimeState{}, path, false, nil
	}
	if err != nil {
		return runtimeState{}, path, false, err
	}
	var st runtimeState
	if err := json.Unmarshal(b, &st); err != nil {
		return runtimeState{}, path, false, err
	}
	if strings.TrimSpace(st.Addr) == "" || !healthzOK(st.Addr) {
		return st, path, false, nil
	}
	return st, path, true, nil
}

func cleanupRuntimeState(dataDir string, pid int) {
	b, err := os.ReadFile(runtimePIDPath(dataDir))
	if err == nil && strings.TrimSpace(string(b)) != strconv.Itoa(pid) {
		return
	}
	_ = os.Remove(runtimeStatePath(dataDir))
	_ = os.Remove(runtimePIDPath(dataDir))
}

func healthzOK(addr string) bool {
	client := &http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(browserURL(addr) + "/healthz")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok"
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func authorizeControlRequest(r *http.Request, token string) bool {
	if r.Method != http.MethodPost {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host != "127.0.0.1" && host != "::1" && host != "localhost" {
		return false
	}
	return r.Header.Get("X-AgentDB233-Control-Token") == token
}

func startDetachedServer(dataDir, listen string) (*runtimeState, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(runtimeDir(dataDir), 0o755); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(runtimeLogPath(dataDir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	defer func() { _ = logFile.Close() }()
	cmd := exec.Command(exe, "serve", "-data", dataDir, "-addr", listen)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	_ = cmd.Process.Release()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st, _, ok, err := loadRuntimeState(dataDir)
		if err == nil && ok {
			return &st, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("server started but runtime state not ready; url=%s log=%s", browserURL(listen), runtimeLogPath(dataDir))
}

func stopServer(st runtimeState) error {
	req, err := http.NewRequest(http.MethodPost, browserURL(st.Addr)+"/__admin/shutdown", bytes.NewReader(nil))
	if err != nil {
		return err
	}
	req.Header.Set("X-AgentDB233-Control-Token", st.ControlToken)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("control request failed: %s", strings.TrimSpace(string(body)))
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !healthzOK(st.Addr) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for shutdown")
}

func normalizePortValue(currentAddr, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("port value is required")
	}
	if strings.Contains(value, ":") {
		return normalizeListenAddr(value), nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 || port > 65535 {
		return "", fmt.Errorf("invalid port %q", value)
	}
	host, _, err := net.SplitHostPort(normalizeListenAddr(currentAddr))
	if err != nil || strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
