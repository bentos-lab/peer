package github

import (
	"fmt"
	"net/url"
	"strings"
)

func buildAuthenticatedCloneURL(rawCloneURL string, installationToken string) (string, error) {
	installationToken = strings.TrimSpace(installationToken)
	if installationToken == "" {
		return "", fmt.Errorf("installation token is required")
	}
	cloneURL, err := url.Parse(strings.TrimSpace(rawCloneURL))
	if err != nil {
		return "", err
	}
	if cloneURL.Scheme != "http" && cloneURL.Scheme != "https" {
		return "", fmt.Errorf("clone URL must use http or https")
	}
	cloneURL.User = url.UserPassword("x-access-token", installationToken)
	return cloneURL.String(), nil
}
