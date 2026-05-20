package control

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/brdweb/homelab-control/internal/config"
)

func TestImportHomepagePreservesGroupsAndIcons(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bookmarksPath := filepath.Join(dir, "bookmarks.yaml")
	servicesPath := filepath.Join(dir, "services.yaml")

	if err := os.WriteFile(bookmarksPath, []byte(`
- Productivity:
    - Fastmail:
        - href: https://fastmail.com/
          icon: https://cdn.jsdelivr.net/gh/selfhst/icons/png/fastmail.png
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(servicesPath, []byte(`
- Media & Entertainment:
    - Jellyfin:
        icon: jellyfin.png
        href: https://jellyfin.brdweb.com
        description: Media streaming server
`), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(context.Background(), config.StateConfig{
		Driver:     "sqlite",
		SQLitePath: filepath.Join(dir, "control.db"),
	}, filepath.Join(dir, "config.yaml"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	result, err := store.ImportHomepage(context.Background(), ImportRequest{
		BookmarksPath: bookmarksPath,
		ServicesPath:  servicesPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 2 || result.Created != 2 {
		t.Fatalf("unexpected import result: %+v", result)
	}

	links, err := store.ListLinks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(links.Groups))
	}

	byTitle := map[string]ManagedLink{}
	for _, link := range links.Links {
		byTitle[link.Title] = link
	}

	if byTitle["Fastmail"].IconKind != "url" {
		t.Fatalf("expected Fastmail URL icon, got %q", byTitle["Fastmail"].IconKind)
	}
	if byTitle["Jellyfin"].Icon != "jellyfin.png" || byTitle["Jellyfin"].IconKind != "slug" {
		t.Fatalf("expected Jellyfin slug icon, got %+v", byTitle["Jellyfin"])
	}
	if byTitle["Jellyfin"].CategoryName != "Media & Entertainment" {
		t.Fatalf("expected service group to be preserved, got %q", byTitle["Jellyfin"].CategoryName)
	}
}
