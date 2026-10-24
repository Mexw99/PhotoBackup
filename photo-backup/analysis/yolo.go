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

// YOLOAnalyzer keeps one Python process alive and talks to it with JSON Lines.
// This avoids loading the YOLO model once per image.
type YOLOAnalyzer struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	python string
	script string
	model  string
	worker string
}

func NewYOLOAnalyzerFromEnv() *YOLOAnalyzer {
	script := findYOLOFile("YOLO_SCRIPT", filepath.Join("tools", "yolo_worker.py"))
	model := findYOLOFile("YOLO_MODEL_PATH", "yolo11n.pt")
	// Some Wails launchers and `go test` use a working directory below the
	// project root. Once the script is found, use its project root to locate
	// the virtual environment and model as well.
	projectRoot := ""
	if script != "" {
		projectRoot = filepath.Dir(filepath.Dir(script))
	}
	if projectRoot != "" {
		if strings.TrimSpace(model) == "" {
			candidate := filepath.Join(projectRoot, "yolo11n.pt")
			if fileExists(candidate) {
				model = candidate
			}
		}
		if python := findYOLOPythonIn(projectRoot); python != "" {
			return &YOLOAnalyzer{python: python, script: script, model: model, worker: findYOLOFile("YOLO_WORKER", workerFileName())}
		}
	}
	return &YOLOAnalyzer{
		python: findYOLOPython(),
		script: script,
		model:  model,
		worker: findYOLOFile("YOLO_WORKER", workerFileName()),
	}
}

func (a *YOLOAnalyzer) Analyze(path string) (AIResult, error) {
	if a == nil {
		return AIResult{}, errors.New("YOLO analyzer is nil")
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
	if _, err := a.stdin.Write(append(request, '\n')); err != nil {
		a.stopLocked()
		return AIResult{}, fmt.Errorf("ส่งรูปให้ YOLO ไม่ได้: %w", err)
	}
	if !a.stdout.Scan() {
		err := a.stdout.Err()
		a.stopLocked()
		if err == nil {
			err = errors.New("YOLO worker ปิดตัวก่อนส่งผลลัพธ์")
		}
		return AIResult{}, err
	}
	var response struct {
		Tags  []string `json:"tags"`
		Error string   `json:"error"`
	}
	if err := json.Unmarshal([]byte(a.stdout.Text()), &response); err != nil {
		return AIResult{}, fmt.Errorf("อ่านผล YOLO ไม่ได้: %w", err)
	}
	if response.Error != "" {
		return AIResult{}, fmt.Errorf("YOLO วิเคราะห์ไม่ได้: %s", response.Error)
	}
	return AIResult{Tags: cleanTags(response.Tags)}, nil
}

func (a *YOLOAnalyzer) startLocked() error {
	if a.cmd != nil {
		return nil
	}
	if strings.TrimSpace(a.worker) == "" && strings.TrimSpace(a.script) == "" {
		return errors.New("ไม่พบ tools/yolo_worker.py")
	}
	if strings.TrimSpace(a.model) == "" {
		return errors.New("ไม่พบ yolo11n.pt; ตั้งค่า YOLO_MODEL_PATH ให้ชี้ไปที่โมเดล")
	}
	// Keep the caller's working directory so relative tools/ and model paths
	// resolve together when running from the project directory.
	var command *exec.Cmd
	if strings.TrimSpace(a.worker) != "" {
		command = exec.Command(a.worker)
	} else {
		command = exec.Command(a.python, a.script)
	}
	command.Env = append(os.Environ(), "YOLO_MODEL_PATH="+a.model)
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
		return fmt.Errorf("เริ่ม YOLO worker ไม่ได้: %w", err)
	}
	a.cmd, a.stdin, a.stdout = command, stdin, bufio.NewScanner(stdout)
	return nil
}

func (a *YOLOAnalyzer) stopLocked() {
	if a.stdin != nil {
		_ = a.stdin.Close()
	}
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_, _ = a.cmd.Process.Wait()
	}
	a.cmd, a.stdin, a.stdout = nil, nil, nil
}

func (a *YOLOAnalyzer) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
	return nil
}

// Configured reports whether the local worker can be started without loading
// the model. It is used by the UI health check and never calls an AI service.
func (a *YOLOAnalyzer) Configured() bool {
	return a != nil && (strings.TrimSpace(a.worker) != "" || strings.TrimSpace(a.script) != "") && strings.TrimSpace(a.model) != "" && strings.TrimSpace(a.python) != ""
}

func findYOLOPython() string {
	if value := strings.TrimSpace(os.Getenv("YOLO_PYTHON")); value != "" {
		return value
	}
	for _, root := range yoloRoots() {
		if python := findYOLOPythonIn(root); python != "" {
			return python
		}
	}
	return "python"
}

func findYOLOPythonIn(root string) string {
	if runtime.GOOS == "windows" {
		path := filepath.Join(root, ".venv-yolo", "Scripts", "python.exe")
		if fileExists(path) {
			return path
		}
		return ""
	}
	path := filepath.Join(root, ".venv-yolo", "bin", "python")
	if fileExists(path) {
		return path
	}
	return ""
}

func workerFileName() string {
	if runtime.GOOS == "windows" {
		return "yolo_worker.exe"
	}
	return "yolo_worker"
}

func findYOLOFile(envName, defaultName string) string {
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
		if defaultName == "yolo11n.pt" {
			candidates = append(candidates, filepath.Join(root, "models", defaultName))
		}
		if defaultName == workerFileName() {
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

func yoloRoots() []string {
	roots := make([]string, 0, 8)
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for index := 0; index < 4 && root != filepath.Dir(root); index++ {
			roots = append(roots, root)
			root = filepath.Dir(root)
		}
	}
	return uniquePaths(roots)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err == nil && !seen[absolute] {
			seen[absolute] = true
			result = append(result, absolute)
		}
	}
	return result
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
