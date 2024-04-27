package log

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	Red       = "\x1b[31m"
	Green     = "\x1b[32m"
	Yellow    = "\x1b[33m"
	Blue      = "\x1b[34m"
	Magenta   = "\x1b[35m"
	Cyan      = "\x1b[36m"
	Bold      = "\x1b[1m"
	Underline = "\x1b[4m"
	Reset     = "\x1b[0m"
)

var (
	NoColor = false
	codes   = [4]string{Yellow, Blue, Magenta, Cyan}
	index   = 0
	mutex   = sync.Mutex{}
)

func Fatal(v ...any) {
	Error(v...)
	os.Exit(1)
}

func Error(v ...any) {
	prefix := "error: "
	if !NoColor {
		prefix = fmt.Sprintf("%s%serror:%s ", Red, Bold, Reset)
	}
	fmt.Fprintf(os.Stderr, "%s %v", prefix, v)
}

func TaskOutput(id, out string) {
	mutex.Lock()
	defer mutex.Unlock()

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if NoColor {
			fmt.Printf("%s: %s\n", id, line)
			return
		}

		colorCode := codes[index]
		index = (index + 1) % len(codes)
		fmt.Printf("%s%s:%s %s\n", colorCode, id, Reset, line)
	}
}

func Metrics(hits, total, failed int, duration time.Duration) {
	fmt.Print("\n")
	if NoColor {
		metricsNoColor(hits, total, failed, duration)
		return
	}
	metricsColor(hits, total, failed, duration)
}

func metricsNoColor(hits, total, failed int, duration time.Duration) {
	taskTxt := fmt.Sprintf("%d passed", total)
	if failed > 0 {
		taskTxt = fmt.Sprintf("%d failed", failed)
	}
	fmt.Printf("Tasks:       %s, %d total\n", taskTxt, total)

	hitsTxt := fmt.Sprintf("%d hits, %d total", hits, total)
	if hits == total {
		hitsTxt += " » 100%"
	}
	fmt.Printf("Cache Hits:  %s\n", hitsTxt)

	fmt.Printf("Duration:    %s\n", time.Duration.String(duration))
}

func metricsColor(hits, total, failed int, duration time.Duration) {
	taskTxt := fmt.Sprintf("%s%s%d passed%s", Green, Bold, total, Reset)
	if failed > 0 {
		taskTxt = fmt.Sprintf("%s%s%d failed%s", Red, Bold, failed, Reset)
	}
	fmt.Printf("%sTasks:%s       %s, %d total\n", Bold, Reset, taskTxt, total)

	hitsTxt := fmt.Sprintf("%d hits, %d total", hits, total)
	if hits != total {
		hitsTxt += fmt.Sprintf(" %s%s» 100%%%s", Green, Bold, Reset)
	}
	fmt.Printf("%sCache Hits:%s  %s\n", Bold, Reset, hitsTxt)

	fmt.Printf("%sDuration:%s    %s\n", Bold, Reset, time.Duration.String(duration))
}
