package adkinteractions

import (
	"encoding/json"
	"net/http"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func NewHandler(appName, userID string, runnerCfg runner.Config) http.Handler {
	return &handler{
		appID:        appName,
		userID:       userID,
		runnerConfig: runnerCfg,
	}
}

type handler struct {
	appID        string
	userID       string
	runnerConfig runner.Config
}

type interactionRequest struct {
	PreviousInteractionID string `json:"previous_interaction_id"`
	Input                 string `json:"input"` // TODO: Support []Content
	Background            bool   `json:"background,omitempty"`
}

type Content struct {
	Text string `json:"text"`
}

type interactionResponse struct {
	ID      string    `json:"id"`
	Outputs []Content `json:"outputs"`
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req interactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Background {
		http.Error(w, "Background interactions not supported yet", http.StatusNotImplemented)
		return
	}

	interactionRunner, err := runner.New(h.runnerConfig)
	if err != nil {
		http.Error(w, "Failed to create runner", http.StatusInternalServerError)
		return
	}

	// Create a new session that will contain the previous interactions.
	cResp, err := h.runnerConfig.SessionService.Create(r.Context(), &session.CreateRequest{
		AppName: h.appID,
		UserID:  h.userID,
	})
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	if req.PreviousInteractionID != "" {
		// TODO: Retrieve and copy previous interactions to the new session.
	}

	it := interactionRunner.Run(r.Context(), h.userID, cResp.Session.ID(), &genai.Content{
		Parts: []*genai.Part{{Text: req.Input}},
	}, agent.RunConfig{})

	for event, err := range it {
		if err != nil {
			http.Error(w, "Failed to run runner", http.StatusInternalServerError)
			return
		}
		_ = event // Handle events to generate outputs.
	}

	resp := interactionResponse{
		ID:      "???",                            // TODO: Align with Interactions ID format.
		Outputs: []Content{{Text: "some output"}}, // TODO: Populate with actual outputs.
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}
