// Package sse holds the server-sent-events plumbing every clicky server needs,
// so handlers stop hand-writing it:
//
//   - Writer frames events onto an http.ResponseWriter (lazy headers, multi-line
//     data, comment pings) and is what rpc's streaming operations write through.
//   - Snapshot streams a value that is re-loaded on a tick or a wake, sending it
//     only when it changed (optionally ignoring volatile fields such as a rate
//     limit or a sample counter) and pinging otherwise.
//   - Notifier is a fan-out wake-up: one Notify wakes every subscribed stream,
//     coalescing bursts, unlike a shared channel where streams steal each
//     other's signals.
//   - Hub multiplexes every stream of a browser tab over one connection, since a
//     browser allows only six HTTP/1.1 connections per host across all its tabs,
//     and Guard refuses the direct per-topic EventSource connections a stale page
//     would otherwise hold open.
//
// A minimal server wiring all four:
//
//	mux := http.NewServeMux()
//	var changed sse.Notifier // call changed.Notify() whenever the data changes
//	mux.HandleFunc("GET /api/items/stream", func(w http.ResponseWriter, r *http.Request) {
//		wake, cancel := changed.Subscribe()
//		defer cancel()
//		err := sse.ServeSnapshot(w, r, sse.SnapshotOptions{
//			Load:     func(ctx context.Context) (any, error) { return loadItems(ctx) },
//			Interval: 5 * time.Second,
//			Wake:     wake,
//			Exclude:  []string{"fetchedAt"},
//		})
//		if err != nil {
//			logger.Debugf("items stream: %v", err)
//		}
//	})
//	hub := sse.NewHub(sse.HubOptions{Build: buildID})
//	root := hub.Guard(mux)
//	hub.Register(mux, root)
//	http.ListenAndServe(":8080", root)
//
// A browser then opens GET /api/events once and subscribes to
// /api/items/stream over it (clicky-ui's createEventHub does this).
package sse
