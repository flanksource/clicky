package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/flanksource/clicky/route"
)

// RegisterRoutes mounts the generated-operation schedule API. The handlers are
// written by hand rather than generated, so each declares what it does: a
// schedule that fires an operation is itself a change worth recording.
func (s *OperationScheduleService) RegisterRoutes(router *route.Router) {
	router.RawFunc("GET /api/v1/schedules", s.handleListSchedules,
		route.Meta{Entity: "schedule", Verb: "list", ReadOnly: true})
	router.RawFunc("POST /api/v1/schedules", s.handleCreateSchedule,
		route.Meta{Entity: "schedule", Verb: "create"})
	router.RawFunc("PUT /api/v1/schedules/{id}", s.handleUpdateSchedule,
		route.Meta{Entity: "schedule", Verb: "update", IDParam: "id"})
	router.RawFunc("DELETE /api/v1/schedules/{id}", s.handleDeleteSchedule,
		route.Meta{Entity: "schedule", Verb: "delete", IDParam: "id"})
	router.RawFunc("POST /api/v1/schedules/{id}/run", s.handleRunSchedule,
		route.Meta{Entity: "schedule", Verb: "run", IDParam: "id"})
	router.RawFunc("POST /api/v1/schedules/run-now", s.handleRunOperationNow,
		route.Meta{Entity: "schedule", Verb: "run-now"})
}

func (s *OperationScheduleService) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := s.List(r.Context())
	if err != nil {
		writeOperationScheduleError(w, err)
		return
	}
	writeOperationScheduleJSON(w, http.StatusOK, schedules)
}

func (s *OperationScheduleService) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var input OperationScheduleInput
	if err := decodeOperationScheduleJSON(r, &input); err != nil {
		writeOperationScheduleError(w, fmt.Errorf("%w: %v", ErrOperationScheduleInvalid, err))
		return
	}
	schedule, err := s.Create(r.Context(), input)
	if err != nil {
		writeOperationScheduleError(w, err)
		return
	}
	writeOperationScheduleJSON(w, http.StatusCreated, schedule)
}

func (s *OperationScheduleService) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	var input OperationScheduleInput
	if err := decodeOperationScheduleJSON(r, &input); err != nil {
		writeOperationScheduleError(w, fmt.Errorf("%w: %v", ErrOperationScheduleInvalid, err))
		return
	}
	schedule, err := s.Update(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeOperationScheduleError(w, err)
		return
	}
	writeOperationScheduleJSON(w, http.StatusOK, schedule)
}

func (s *OperationScheduleService) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if err := s.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeOperationScheduleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *OperationScheduleService) handleRunSchedule(w http.ResponseWriter, r *http.Request) {
	run, err := s.Run(r.Context(), r.PathValue("id"))
	if err != nil {
		writeOperationScheduleError(w, err)
		return
	}
	writeOperationScheduleJSON(w, http.StatusAccepted, run)
}

func (s *OperationScheduleService) handleRunOperationNow(w http.ResponseWriter, r *http.Request) {
	var input OperationRunInput
	if err := decodeOperationScheduleJSON(r, &input); err != nil {
		writeOperationScheduleError(w, fmt.Errorf("%w: %v", ErrOperationScheduleInvalid, err))
		return
	}
	run, err := s.RunNow(r.Context(), input)
	if err != nil {
		writeOperationScheduleError(w, err)
		return
	}
	writeOperationScheduleJSON(w, http.StatusAccepted, run)
}

func decodeOperationScheduleJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request body must contain one JSON object")
	}
	return nil
}

func writeOperationScheduleError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrOperationScheduleNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrOperationScheduleUnauthorized):
		status = http.StatusForbidden
	case errors.Is(err, ErrOperationNotFound), errors.Is(err, ErrOperationNotSchedulable), errors.Is(err, ErrOperationScheduleInvalid):
		status = http.StatusBadRequest
	}
	writeOperationScheduleJSON(w, status, map[string]string{"error": err.Error()})
}

func writeOperationScheduleJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(fmt.Sprintf("encode operation schedule response: %v", err))
	}
}
