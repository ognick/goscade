package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ognick/goscade/example/internal/domain"
)

type logger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type usecase interface {
	Graph(ctx context.Context, graphID string) (domain.Graph, error)
	GraphDOT(ctx context.Context, graphID string) string
	StartAll(ctx context.Context, graphID string) error
	StopAll(ctx context.Context, graphID string) error
	UpdateComponent(ctx context.Context, graphID, compID string, delay time.Duration, err *string) error
	KillComponent(ctx context.Context, graphID, compID string) error
}

type handler struct {
	usecase usecase
	log     logger
}

func NewHandler(log logger, usecase usecase) http.Handler {
	h := &handler{
		usecase: usecase,
		log:     log,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/graph", h.handleGraph)
	mux.HandleFunc("/graph/dot", h.handleGraphDOT)
	mux.HandleFunc("/start", h.handleStart)
	mux.HandleFunc("/stop", h.handleStop)
	mux.HandleFunc("/component/update", h.handleUpdateComponent)
	mux.HandleFunc("/component/kill", h.handleKillComponent)
	mux.Handle("/", http.FileServer(http.Dir("web")))

	return mux
}

func (h *handler) getGraphID(r *http.Request) string {
	return r.URL.Query().Get("graph_id")
}

func (h *handler) handleGraph(w http.ResponseWriter, r *http.Request) {
	graph, err := h.usecase.Graph(r.Context(), h.getGraphID(r))
	if err != nil {
		h.log.Errorf("failed to get graph: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(graph); err != nil {
		h.log.Errorf("failed to encode graph: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *handler) handleGraphDOT(w http.ResponseWriter, r *http.Request) {
	dot := h.usecase.GraphDOT(r.Context(), h.getGraphID(r))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(dot)); err != nil {
		h.log.Errorf("failed to write graph: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *handler) handleStart(w http.ResponseWriter, r *http.Request) {
	h.log.Infof("Received start request")
	if err := h.usecase.StartAll(r.Context(), h.getGraphID(r)); err != nil {
		h.log.Errorf("Failed to start Usecase: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *handler) handleStop(w http.ResponseWriter, r *http.Request) {
	h.log.Infof("Received stop request")
	if err := h.usecase.StopAll(r.Context(), h.getGraphID(r)); err != nil {
		h.log.Errorf("Failed to stop Usecase: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type UpdateRequest struct {
	ComponentID string  `json:"component_id"`
	DelayMs     int     `json:"delay_ms"`
	ReadyError  *string `json:"ready_error"`
}

func (h *handler) handleUpdateComponent(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Errorf("Failed to update component: %v", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.usecase.UpdateComponent(
		r.Context(),
		h.getGraphID(r),
		req.ComponentID,
		time.Duration(req.DelayMs)*time.Millisecond,
		req.ReadyError,
	); err != nil {
		h.log.Errorf("Failed to update component: %v", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *handler) handleKillComponent(w http.ResponseWriter, r *http.Request) {
	compID := r.URL.Query().Get("component_id")
	if compID == "" {
		http.Error(w, "missing component_id", http.StatusBadRequest)
		return
	}

	if err := h.usecase.KillComponent(r.Context(), h.getGraphID(r), compID); err != nil {
		h.log.Errorf("Failed to kill component %s: %v", compID, err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
