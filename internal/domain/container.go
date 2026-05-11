package domain

import "time"

// Mount describes a container mount in a Docker-SDK independent shape.
type Mount struct {
	Type        string
	Name        string
	Source      string
	Destination string
	ReadOnly    bool
}

// Container is the application-level model used by filtering and the TUI.
type Container struct {
	ID                 string
	ShortID            string
	Names              []string
	Name               string
	Image              string
	Command            string
	State              string
	Status             string
	Created            time.Time
	Labels             map[string]string
	Mounts             []Mount
	IsDevcontainer     bool
	DevcontainerPath   string
	DevcontainerSource string
}

// DisplayName returns the best human-readable name for a container.
func (c Container) DisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	if len(c.Names) > 0 && c.Names[0] != "" {
		return c.Names[0]
	}
	if c.ShortID != "" {
		return c.ShortID
	}
	return c.ID
}
