package handler

import (
	"net/http"

	"github.com/HeartBtz/tempest/internal/client"
)

type ProfileHandler struct{}

func NewProfileHandler() *ProfileHandler {
	return &ProfileHandler{}
}

func (h *ProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	type ProfileInfo struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Version   string `json:"version"`
		UserAgent string `json:"user_agent"`
	}

	names := client.ListProfileNames()
	profiles := make([]ProfileInfo, 0, len(names))

	for _, name := range names {
		p, _ := client.GetProfile(name)
		profiles = append(profiles, ProfileInfo{
			ID:        name,
			Name:      p.Name,
			Version:   p.Version,
			UserAgent: p.UserAgent,
		})
	}

	writeJSON(w, http.StatusOK, profiles)
}
