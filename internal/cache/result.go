package cache

type TaskResult struct {
	Logs   string
	Failed bool
}

func NewTaskResult(logs string, failed bool) TaskResult {
	return TaskResult{
		Logs:   logs,
		Failed: failed,
	}
}
