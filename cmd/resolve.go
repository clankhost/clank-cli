package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/clankhost/clank-cli/internal/api"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool {
	return uuidRegex.MatchString(strings.TrimSpace(s))
}

// resolveProjectID resolves a project UUID, slug, name, or unique ID prefix to a project ID.
func resolveProjectID(client *api.Client, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("project ID or slug is required")
	}
	if isUUID(ref) {
		return ref, nil
	}

	projects, err := api.ListProjects(client)
	if err != nil {
		return "", err
	}

	var matches []api.Project
	for _, p := range projects {
		if p.ID == ref || strings.EqualFold(p.Slug, ref) || strings.EqualFold(p.Name, ref) {
			return p.ID, nil
		}
		if strings.HasPrefix(p.ID, ref) {
			matches = append(matches, p)
		}
	}

	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous project %q — matches multiple projects; specify the full ID", ref)
	}
	return "", fmt.Errorf("project %q not found", ref)
}

// resolveServiceID resolves a service UUID, slug, name, or unique ID prefix to a service ID.
func resolveServiceID(client *api.Client, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("service ID or name is required")
	}
	if isUUID(ref) {
		return ref, nil
	}

	projects, err := api.ListProjects(client)
	if err != nil {
		return "", err
	}

	var exactMatches []api.Service
	var prefixMatches []api.Service
	for _, p := range projects {
		services, err := api.ListServices(client, p.ID)
		if err != nil {
			return "", err
		}
		for _, s := range services {
			if s.ID == ref || strings.EqualFold(s.Slug, ref) || strings.EqualFold(s.Name, ref) {
				exactMatches = append(exactMatches, s)
			} else if strings.HasPrefix(s.ID, ref) {
				prefixMatches = append(prefixMatches, s)
			}
		}
	}

	if len(exactMatches) == 1 {
		return exactMatches[0].ID, nil
	}
	if len(exactMatches) > 1 {
		return "", fmt.Errorf("ambiguous service %q — matches %d services across projects; specify the full ID", ref, len(exactMatches))
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0].ID, nil
	}
	if len(prefixMatches) > 1 {
		return "", fmt.Errorf("ambiguous service %q — matches multiple services; specify the full ID", ref)
	}
	return "", fmt.Errorf("service %q not found", ref)
}
