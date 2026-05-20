package control

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ImportRequest struct {
	BookmarksPath string `json:"bookmarks_path"`
	ServicesPath  string `json:"services_path"`
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Created  int      `json:"created"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

func (s *Store) ImportHomepage(ctx context.Context, req ImportRequest) (ImportResult, error) {
	result := ImportResult{}
	links := make([]LinkMutation, 0)

	if strings.TrimSpace(req.BookmarksPath) != "" {
		parsed, err := parseHomepageBookmarks(req.BookmarksPath)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		} else {
			links = append(links, parsed...)
		}
	}

	if strings.TrimSpace(req.ServicesPath) != "" {
		parsed, err := parseHomepageServices(req.ServicesPath)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		} else {
			links = append(links, parsed...)
		}
	}

	SortMutationsByOrder(links)
	for _, link := range links {
		_, created, err := s.UpsertImportedLink(ctx, link)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", link.Title, err))
			continue
		}
		result.Imported++
		if created {
			result.Created++
		} else {
			result.Updated++
		}
	}

	if len(links) == 0 && len(result.Errors) > 0 {
		return result, fmt.Errorf("homepage import failed")
	}
	return result, nil
}

func parseHomepageBookmarks(path string) ([]LinkMutation, error) {
	root, err := readYAMLDocument(path)
	if err != nil {
		return nil, err
	}
	if root.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: expected top-level sequence", path)
	}

	links := make([]LinkMutation, 0)
	for groupIndex, groupNode := range root.Content {
		if groupNode.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(groupNode.Content); i += 2 {
			groupName := groupNode.Content[i].Value
			items := groupNode.Content[i+1]
			if items.Kind != yaml.SequenceNode {
				continue
			}
			for itemIndex, itemNode := range items.Content {
				if itemNode.Kind != yaml.MappingNode {
					continue
				}
				for j := 0; j+1 < len(itemNode.Content); j += 2 {
					title := itemNode.Content[j].Value
					props := firstMapping(itemNode.Content[j+1])
					href := mappingScalar(props, "href")
					if href == "" {
						continue
					}
					links = append(links, LinkMutation{
						CategoryName:   groupName,
						Title:          title,
						URL:            href,
						Icon:           mappingScalar(props, "icon"),
						Target:         "_blank",
						VisibilityRole: "viewer",
						SortOrder:      (groupIndex+1)*1000 + (itemIndex+1)*10,
						Source:         "homepage:bookmarks",
					})
				}
			}
		}
	}
	return links, nil
}

func parseHomepageServices(path string) ([]LinkMutation, error) {
	root, err := readYAMLDocument(path)
	if err != nil {
		return nil, err
	}
	if root.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: expected top-level sequence", path)
	}

	links := make([]LinkMutation, 0)
	for groupIndex, groupNode := range root.Content {
		if groupNode.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(groupNode.Content); i += 2 {
			groupName := groupNode.Content[i].Value
			services := groupNode.Content[i+1]
			if services.Kind != yaml.SequenceNode {
				continue
			}
			for serviceIndex, serviceNode := range services.Content {
				if serviceNode.Kind != yaml.MappingNode {
					continue
				}
				for j := 0; j+1 < len(serviceNode.Content); j += 2 {
					title := serviceNode.Content[j].Value
					props := serviceNode.Content[j+1]
					href := mappingScalar(props, "href")
					if href == "" {
						continue
					}
					links = append(links, LinkMutation{
						CategoryName:   groupName,
						Title:          title,
						URL:            href,
						Description:    mappingScalar(props, "description"),
						Icon:           mappingScalar(props, "icon"),
						Target:         "_blank",
						VisibilityRole: "viewer",
						SortOrder:      (groupIndex+1)*1000 + (serviceIndex+1)*10,
						Source:         "homepage:services",
					})
				}
			}
		}
	}
	return links, nil
}

func readYAMLDocument(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("%s: empty YAML document", path)
	}
	return doc.Content[0], nil
}

func firstMapping(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		return node
	}
	if node.Kind == yaml.SequenceNode && len(node.Content) > 0 && node.Content[0].Kind == yaml.MappingNode {
		return node.Content[0]
	}
	return nil
}

func mappingScalar(node *yaml.Node, key string) string {
	if node == nil || node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return strings.TrimSpace(node.Content[i+1].Value)
		}
	}
	return ""
}
