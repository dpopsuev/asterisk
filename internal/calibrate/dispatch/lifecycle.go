package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// SpawnResponder builds and launches cmd/mock-calibration-agent as a child
// process watching the given directory.
func SpawnResponder(watchDir string, debug bool) (*os.Process, error) {
	args := []string{"run", "./cmd/mock-calibration-agent"}
	if debug {
		args = append(args, "--debug")
	}
	args = append(args, watchDir)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("[lifecycle] spawning mock-calibration-agent (watch=%s, debug=%v)\n", watchDir, debug)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mock-calibration-agent: %w", err)
	}
	fmt.Printf("[lifecycle] mock-calibration-agent started (pid=%d)\n", cmd.Process.Pid)

	time.Sleep(500 * time.Millisecond)

	return cmd.Process, nil
}

// StopResponder sends SIGTERM to the responder process, waits briefly for
// graceful shutdown, then SIGKILL if still alive.
func StopResponder(proc *os.Process) {
	if proc == nil {
		return
	}

	pid := proc.Pid
	fmt.Printf("[lifecycle] stopping mock-calibration-agent (pid=%d)\n", pid)

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("[lifecycle] mock-calibration-agent already exited (pid=%d): %v\n", pid, err)
		proc.Release()
		return
	}

	done := make(chan error, 1)
	go func() {
		_, err := proc.Wait()
		done <- err
	}()

	select {
	case <-done:
		fmt.Printf("[lifecycle] mock-calibration-agent exited gracefully (pid=%d)\n", pid)
	case <-time.After(3 * time.Second):
		fmt.Printf("[lifecycle] mock-calibration-agent didn't exit in 3s, sending SIGKILL (pid=%d)\n", pid)
		_ = proc.Kill()
		<-done
		fmt.Printf("[lifecycle] mock-calibration-agent killed (pid=%d)\n", pid)
	}
}

// ForwardSignals installs a handler for SIGINT and SIGTERM that kills the
// responder process before the main process exits.
func ForwardSignals(proc *os.Process) {
	if proc == nil {
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Printf("\n[lifecycle] received %s, stopping mock-calibration-agent before exit\n", sig)
		StopResponder(proc)
		signal.Stop(sigCh)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(sig)
	}()
}

// FinalizeSignals walks the calibration directory and sets every signal.json
// to status "complete".
func FinalizeSignals(dir string) {
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || info.Name() != "signal.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var sig SignalFile
		if err := json.Unmarshal(data, &sig); err != nil {
			return nil
		}

		if sig.Status == "complete" {
			return nil
		}

		sig.Status = "complete"
		sig.Timestamp = time.Now().UTC().Format(time.RFC3339)
		if err := WriteSignal(path, &sig); err != nil {
			fmt.Fprintf(os.Stderr, "[lifecycle] finalize %s: %v\n", path, err)
			return nil
		}
		count++
		return nil
	})

	if count > 0 {
		fmt.Printf("[lifecycle] finalized %d signal(s) to status=complete\n", count)
	}
}
