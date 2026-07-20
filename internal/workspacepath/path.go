package workspacepath

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// PrivateRoot returns workspace/private/{userID}/{agentSlug} under base.
func PrivateRoot(base, userID, agentSlug string) (string, error) {
	userID = strings.TrimSpace(userID)
	agentSlug = strings.TrimSpace(agentSlug)
	if agentSlug == "" {
		agentSlug = "default"
	}
	if !safeID.MatchString(userID) {
		return "", fmt.Errorf("invalid user id")
	}
	if !safeID.MatchString(agentSlug) {
		return "", fmt.Errorf("invalid agent id")
	}
	return filepath.Join(base, "private", userID, agentSlug), nil
}
