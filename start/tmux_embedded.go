package start

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type tmuxEmbeddedClient struct {
	processes []*process
	paneIDs   []string
	root      string
}

func newTmuxEmbeddedClient(root string) *tmuxEmbeddedClient {
	return &tmuxEmbeddedClient{
		processes: make([]*process, 0),
		root:      root,
	}
}

func (t *tmuxEmbeddedClient) Start() error {
	for _, p := range t.processes {
		paneID, err := t.createPane(p)
		if err != nil {
			return fmt.Errorf("failed to create pane for %s: %w", p.Name, err)
		}

		t.paneIDs = append(t.paneIDs, paneID)
		p.paneID = paneID

		pid, err := t.panePID(paneID)
		if err == nil && pid > 0 {
			p.pid = pid
		}
	}

	if len(t.paneIDs) > 0 {
		t.setRemainOnExit()
		t.setPaneTitles()
		exec.Command("tmux", "set-option", "-p", "pane-border-status", "top").Run()
		exec.Command("tmux", "select-layout", "tiled").Run()
	}

	return nil
}

func (t *tmuxEmbeddedClient) createPane(p *process) (string, error) {
	args := []string{
		"split-window", "-d",
		"-c", t.root,
		"-P", "-F", "#{pane_id}",
		p.Command,
	}

	cmd := exec.Command("tmux", args...)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	paneID := strings.TrimPrefix(strings.TrimSpace(out.String()), "%")
	return paneID, nil
}

func (t *tmuxEmbeddedClient) panePID(paneID string) (int, error) {
	for i := 0; i < 20; i++ {
		cmd := exec.Command("tmux", "display-message", "-p", "-t", "%"+paneID, "#{pane_pid}")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr
		cmd.Run()

		pidStr := strings.TrimSpace(out.String())
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
			return pid, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return 0, fmt.Errorf("could not get PID for pane %s", paneID)
}

func (t *tmuxEmbeddedClient) Shutdown() {
	for _, paneID := range t.paneIDs {
		exec.Command("tmux", "kill-pane", "-t", "%"+paneID).Run()
	}
}

func (t *tmuxEmbeddedClient) AddProcess(p *process) {
	t.processes = append(t.processes, p)
}

func (t *tmuxEmbeddedClient) RespawnProcess(p *process) {
	cmd := exec.Command("tmux", "respawn-pane", "-k", "-t", "%"+p.paneID, p.Command)
	cmd.Dir = t.root
	cmd.Run()

	if pid, err := t.panePID(p.paneID); err == nil && pid > 0 {
		p.pid = pid
	}
}

func (t *tmuxEmbeddedClient) ExitCode() (status int) {
	for _, paneID := range t.paneIDs {
		if s := t.PaneExitCode(paneID); s > status {
			status = s
		}
	}
	return
}

func (t *tmuxEmbeddedClient) PaneExitCode(paneID string) int {
	cmd := exec.Command("tmux", "display-message", "-p", "-t", "%"+paneID, "#{pane_dead_status}")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	cmd.Run()

	status, _ := strconv.Atoi(strings.TrimSpace(out.String()))
	return status
}

func (t *tmuxEmbeddedClient) SocketName() string {
	tmuxEnv := os.Getenv("TMUX")
	parts := strings.Split(tmuxEnv, ",")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func (t *tmuxEmbeddedClient) SessionName() string {
	cmd := exec.Command("tmux", "display-message", "-p", "#{session_name}")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()
	return strings.TrimSpace(out.String())
}

func (t *tmuxEmbeddedClient) IsEmbedded() bool {
	return true
}

func (t *tmuxEmbeddedClient) setPaneTitles() {
	for i, p := range t.processes {
		title := p.Name
		exec.Command("tmux", "select-pane", "-t", "%"+t.paneIDs[i], "-T", title).Run()
	}
}

func (t *tmuxEmbeddedClient) setRemainOnExit() {
	for _, paneID := range t.paneIDs {
		exec.Command("tmux", "set-option", "-p", "-t", "%"+paneID, "remain-on-exit", "on").Run()
	}
}
