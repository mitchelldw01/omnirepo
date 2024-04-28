package log

import (
	"fmt"
	"math"
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
		prefix = fmt.Sprintf("%s%serror:%s", Red, Bold, Reset)
	}
	fmt.Fprint(os.Stderr, prefix)
	for _, item := range v {
		fmt.Fprintf(os.Stderr, " %v", item)
	}
	fmt.Fprintln(os.Stderr)
}

func TaskOutput(id, out string) {
	mutex.Lock()
	colorCode := codes[index]
	index = (index + 1) % len(codes)

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if NoColor {
			fmt.Printf("%s: %s\n", id, line)
			continue
		}
		fmt.Printf("%s%s:%s %s\n", colorCode, id, Reset, line)
	}

	mutex.Unlock()
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

	fmt.Printf("Duration:    %s\n", formatDuration(duration))
}

func metricsColor(hits, total, failed int, duration time.Duration) {
	taskTxt := fmt.Sprintf("%s%s%d passed%s", Green, Bold, total, Reset)
	if failed > 0 {
		taskTxt = fmt.Sprintf("%s%s%d failed%s", Red, Bold, failed, Reset)
	}
	fmt.Printf("%sTasks:%s       %s, %d total\n", Bold, Reset, taskTxt, total)

	hitsTxt := fmt.Sprintf("%d hits, %d total", hits, total)
	if hits == total {
		hitsTxt += fmt.Sprintf(" %s%s» 100%%%s", Green, Bold, Reset)
	}
	fmt.Printf("%sCache Hits:%s  %s\n", Bold, Reset, hitsTxt)

	durationTxt := formatDuration(duration)
	if hits == total {
		durationTxt += " 🔥"
	}
	fmt.Printf("%sDuration:%s    %s\n", Bold, Reset, durationTxt)
}

func formatDuration(duration time.Duration) string {
	totalSeconds := duration.Seconds()
	totalMinutes := int(totalSeconds / 60)
	seconds := math.Mod(totalSeconds, 60)
	milliseconds := duration.Milliseconds()

	if totalMinutes >= 1 {
		return fmt.Sprintf("%d:%02d min", totalMinutes, int(seconds))
	}

	if totalSeconds < 1 {
		return fmt.Sprintf("%d ms", milliseconds)
	}

	return fmt.Sprintf("%.3f sec", totalSeconds)
}
