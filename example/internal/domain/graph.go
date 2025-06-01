package domain

import "time"

type Component struct {
	ID        uint64        `json:"id"`
	Name      string        `json:"name"`
	DependsOn []uint64      `json:"depends_on,omitempty"`
	Status    string        `json:"status"`
	Error     *string       `json:"error,omitempty"`
	Ready     bool          `json:"ready"`
	Delay     time.Duration `json:"delay"`
}

type Graph struct {
	ID         string      `json:"id"`
	Components []Component `json:"components"`
	Status     string      `json:"status"`
}
