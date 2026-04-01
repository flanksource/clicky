package taskui

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/flanksource/clicky/task"
)

// Handler returns an http.Handler that serves the task progress UI.
// It mounts:
//
//	GET /                 → HTML page with embedded JS
//	GET /api/tasks        → JSON snapshot of all task groups
//	GET /api/tasks/stream → SSE stream of task updates
func Handler(taskIDs ...string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/tasks", task.JSONHandler(taskIDs...))
	mux.Handle("/api/tasks/stream", task.SSEHandler(taskIDs...))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, pageHTML())
	})
	return mux
}

func pageHTML() string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Task Progress</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://code.iconify.design/iconify-icon/2.0.0/iconify-icon.min.js"></script>
</head>
<body>
    <div id="root"></div>
    <script>`)
	b.WriteString(bundleJS)
	b.WriteString(`</script>
</body>
</html>`)
	return b.String()
}
