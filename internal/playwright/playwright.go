package playwright

import (
	"fmt"
	"os/exec"
)

type RunResult struct {
	Status     string   `json:"status"`
	Screenshots []string `json:"screenshots"`
	Output     string   `json:"output"`
	Error      string   `json:"error,omitempty"`
}

func Run(testPath string) (*RunResult, error) {
	cmd := exec.Command("npx", "playwright", "test", testPath, "--reporter=json")
	out, err := cmd.CombinedOutput()
	res := &RunResult{Status: "passed", Output: string(out)}
	if err != nil {
		res.Status = "failed"
		res.Error = err.Error()
	}
	return res, nil
}

func RunNpx(args ...string) (*RunResult, error) {
	cmd := exec.Command("npx", args...)
	out, err := cmd.CombinedOutput()
	res := &RunResult{Status: "passed", Output: string(out)}
	if err != nil {
		res.Status = "failed"
		res.Error = fmt.Sprintf("%v: %s", err, string(out))
	}
	return res, nil
}
