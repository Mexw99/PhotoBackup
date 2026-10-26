package analysis

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// BLIPAnalyzer keeps one caption worker alive and sends one JSON request per image.
type BLIPAnalyzer struct {
	mu                            sync.Mutex
	cmd                           *exec.Cmd
	stdin                         io.WriteCloser
	stdout                        *bufio.Scanner
	python, script, model, worker string
}

func NewBLIPAnalyzerFromEnv() *BLIPAnalyzer {
	loadPortableConfig()
	worker := findCaptionFile("BLIP_WORKER", captionWorkerFileName())
	if runtime.GOOS != "windows" && strings.HasSuffix(strings.ToLower(worker), ".exe") {
		worker = ""
	}
	return &BLIPAnalyzer{python: firstNonEmpty(os.Getenv("BLIP_PYTHON"), findBLIPPython()), script: findCaptionFile("BLIP_SCRIPT", filepath.Join("tools", "caption_worker.py")), model: findCaptionModel(), worker: worker}
}

func (a *BLIPAnalyzer) Analyze(path string) (AIResult, error) {
	if a == nil {
		return AIResult{}, errors.New("BLIP analyzer is nil")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.startLocked(); err != nil {
		return AIResult{}, err
	}
	request, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return AIResult{}, err
	}
	if _, err = a.stdin.Write(append(request, '\n')); err != nil {
		a.stopLocked()
		return AIResult{}, fmt.Errorf("ส่งรูปให้ BLIP ไม่ได้: %w", err)
	}
	if !a.stdout.Scan() {
		err = a.stdout.Err()
		a.stopLocked()
		if err == nil {
			err = errors.New("BLIP worker ปิดตัวก่อนส่งผลลัพธ์")
		}
		return AIResult{}, err
	}
	var response struct {
		Description string `json:"description"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal([]byte(a.stdout.Text()), &response); err != nil {
		return AIResult{}, fmt.Errorf("อ่านผล BLIP ไม่ได้: %w", err)
	}
	if response.Error != "" {
		return AIResult{}, fmt.Errorf("BLIP วิเคราะห์ไม่ได้: %s", response.Error)
	}
	return AIResult{Description: strings.TrimSpace(response.Description)}, nil
}

func (a *BLIPAnalyzer) startLocked() error {
	if a.cmd != nil {
		return nil
	}
	if strings.TrimSpace(a.worker) == "" && (strings.TrimSpace(a.script) == "" || strings.TrimSpace(a.python) == "") {
		return errors.New("ไม่พบ BLIP worker; ตั้งค่า BLIP_WORKER หรือ BLIP_PYTHON/BLIP_SCRIPT")
	}
	if strings.TrimSpace(a.model) == "" {
		return errors.New("ไม่พบ BLIP model directory")
	}
	var command *exec.Cmd
	if strings.TrimSpace(a.worker) != "" {
		command = exec.Command(a.worker)
	} else {
		command = exec.Command(a.python, a.script)
	}
	command.Env = append(os.Environ(), "BLIP_MODEL_PATH="+a.model)
	stdin, err := command.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("เริ่ม BLIP worker ไม่ได้: %w", err)
	}
	a.cmd, a.stdin, a.stdout = command, stdin, bufio.NewScanner(stdout)
	return nil
}

func (a *BLIPAnalyzer) stopLocked() {
	if a.stdin != nil {
		_ = a.stdin.Close()
	}
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_, _ = a.cmd.Process.Wait()
	}
	a.cmd, a.stdin, a.stdout = nil, nil, nil
}
func (a *BLIPAnalyzer) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
	return nil
}
func (a *BLIPAnalyzer) Configured() bool {
	return a != nil && strings.TrimSpace(a.model) != "" && (strings.TrimSpace(a.worker) != "" || (strings.TrimSpace(a.python) != "" && strings.TrimSpace(a.script) != ""))
}

func findBLIPPython() string {
	if value := strings.TrimSpace(os.Getenv("BLIP_PYTHON")); value != "" {
		return value
	}
	for _, root := range yoloRoots() {
		if python := findYOLOPythonIn(root); python != "" {
			return python
		}
	}
	return findLocalPython()
}
func findCaptionModel() string {
	if value := strings.TrimSpace(os.Getenv("BLIP_MODEL_PATH")); value != "" {
		return value
	}
	for _, root := range yoloRoots() {
		for _, candidate := range []string{filepath.Join(root, "models", "blip-image-captioning-base"), filepath.Join(root, "blip-image-captioning-base")} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}
func findCaptionFile(envName, defaultName string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		if filepath.IsAbs(value) && fileExists(value) {
			return value
		}
		for _, root := range yoloRoots() {
			candidate := filepath.Join(root, value)
			if fileExists(candidate) {
				return candidate
			}
		}
		return value
	}
	for _, root := range yoloRoots() {
		candidates := []string{filepath.Join(root, defaultName)}
		if defaultName == captionWorkerFileName() {
			candidates = append(candidates, filepath.Join(root, "tools", "bin", runtime.GOOS, defaultName))
		}
		for _, candidate := range candidates {
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}
func captionWorkerFileName() string {
	if runtime.GOOS == "windows" {
		return "caption_worker.exe"
	}
	return "caption_worker"
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
