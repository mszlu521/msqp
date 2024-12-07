package logs

import (
	"github.com/charmbracelet/log"
	"os"
	"testing"
)

func TestError(t *testing.T) {
	logger = log.New(os.Stderr)
	Error("test:%v", 10)
}
