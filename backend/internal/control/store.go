package control

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/brdweb/podman-manager/internal/config"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Store struct {
	db      *sql.DB
	dialect string
	logger  *slog.Logger
}

type LinkCategory struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Slug        string `json:"slug" yaml:"slug"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	SortOrder   int    `json:"sort_order" yaml:"sort_order"`
}

type ManagedLink struct {
	ID             string `json:"id" yaml:"id"`
	CategoryID     string `json:"category_id" yaml:"category_id"`
	CategoryName   string `json:"category_name" yaml:"category_name"`
	Title          string `json:"title" yaml:"title"`
	URL            string `json:"url" yaml:"url"`
	Description    string `json:"description,omitempty" yaml:"description,omitempty"`
	Icon           string `json:"icon,omitempty" yaml:"icon,omitempty"`
	IconKind       string `json:"icon_kind" yaml:"icon_kind"`
	Target         string `json:"target" yaml:"target"`
	VisibilityRole string `json:"visibility_role" yaml:"visibility_role"`
	HealthURL      string `json:"health_url,omitempty" yaml:"health_url,omitempty"`
	SortOrder      int    `json:"sort_order" yaml:"sort_order"`
	Status         string `json:"status" yaml:"status"`
	Source         string `json:"source,omitempty" yaml:"source,omitempty"`
}

type LinkGroup struct {
	Category LinkCategory  `json:"category" yaml:"category"`
	Links    []ManagedLink `json:"links" yaml:"links"`
}

type LinksResponse struct {
	Categories []LinkCategory `json:"categories" yaml:"categories"`
	Links      []ManagedLink  `json:"links" yaml:"links"`
	Groups     []LinkGroup    `json:"groups" yaml:"groups"`
}

type LinkMutation struct {
	CategoryID     string `json:"category_id"`
	CategoryName   string `json:"category_name"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	Description    string `json:"description"`
	Icon           string `json:"icon"`
	Target         string `json:"target"`
	VisibilityRole string `json:"visibility_role"`
	HealthURL      string `json:"health_url"`
	SortOrder      int    `json:"sort_order"`
	Status         string `json:"status"`
	Source         string `json:"source"`
}

type AuditEvent struct {
	ID         string         `json:"id"`
	Actor      string         `json:"actor,omitempty"`
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	ResourceID string         `json:"resource_id,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

func NewStore(ctx context.Context, cfg config.StateConfig, configPath string, logger *slog.Logger) (*Store, error) {
	if logger == nil {
		logger = slog.Default()
	}

	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		if strings.TrimSpace(cfg.DSN) != "" {
			driver = "postgres"
		} else {
			driver = "sqlite"
		}
	}

	var db *sql.DB
	var err error
	switch driver {
	case "postgres":
		db, err = sql.Open("pgx", cfg.DSN)
	case "sqlite":
		path := config.ExpandPath(strings.TrimSpace(cfg.SQLitePath))
		if path == "" {
			path = defaultSQLitePath(configPath)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("creating state database directory: %w", err)
		}
		db, err = sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	default:
		return nil, fmt.Errorf("unsupported state driver %q", driver)
	}
	if err != nil {
		return nil, fmt.Errorf("opening state database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to state database: %w", err)
	}

	store := &Store{db: db, dialect: driver, logger: logger}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating state database: %w", err)
	}
	return store, nil
}

func defaultSQLitePath(configPath string) string {
	normalized := strings.ReplaceAll(filepath.Clean(configPath), "\\", "/")
	if configPath == "" || normalized == "/etc/podman-manager/config.yaml" || normalized == "/etc/homelab-control/config.yaml" {
		return "/var/lib/homelab-control/control.db"
	}
	return filepath.Join(filepath.Dir(configPath), "control.db")
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) placeholder(n int) string {
	if s.dialect == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func (s *Store) migrate(ctx context.Context) error {
	schema := `
		CREATE TABLE IF NOT EXISTS link_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS managed_links (
			id TEXT PRIMARY KEY,
			category_id TEXT NOT NULL REFERENCES link_categories(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			icon TEXT NOT NULL DEFAULT '',
			icon_kind TEXT NOT NULL DEFAULT 'favicon',
			target TEXT NOT NULL DEFAULT '_blank',
			visibility_role TEXT NOT NULL DEFAULT 'viewer',
			health_url TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'unknown',
			source TEXT NOT NULL DEFAULT 'manual',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_managed_links_category ON managed_links(category_id, sort_order, title);
		CREATE INDEX IF NOT EXISTS idx_managed_links_url ON managed_links(url);

		CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			actor TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			resource TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			details TEXT NOT NULL DEFAULT '{}',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) ListLinks(ctx context.Context) (LinksResponse, error) {
	categories, err := s.listCategories(ctx)
	if err != nil {
		return LinksResponse{}, err
	}

	query := `
		SELECT l.id, l.category_id, c.name, l.title, l.url, l.description, l.icon, l.icon_kind,
			l.target, l.visibility_role, l.health_url, l.sort_order, l.status, l.source
		FROM managed_links l
		JOIN link_categories c ON c.id = l.category_id
		ORDER BY c.sort_order, c.name, l.sort_order, l.title
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return LinksResponse{}, err
	}
	defer rows.Close()

	links := make([]ManagedLink, 0)
	for rows.Next() {
		var link ManagedLink
		if err := rows.Scan(
			&link.ID,
			&link.CategoryID,
			&link.CategoryName,
			&link.Title,
			&link.URL,
			&link.Description,
			&link.Icon,
			&link.IconKind,
			&link.Target,
			&link.VisibilityRole,
			&link.HealthURL,
			&link.SortOrder,
			&link.Status,
			&link.Source,
		); err != nil {
			return LinksResponse{}, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return LinksResponse{}, err
	}

	return buildLinksResponse(categories, links), nil
}

func (s *Store) listCategories(ctx context.Context) ([]LinkCategory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, slug, description, sort_order
		FROM link_categories
		ORDER BY sort_order, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]LinkCategory, 0)
	for rows.Next() {
		var cat LinkCategory
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.Description, &cat.SortOrder); err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, rows.Err()
}

func buildLinksResponse(categories []LinkCategory, links []ManagedLink) LinksResponse {
	byCategory := make(map[string][]ManagedLink)
	for _, link := range links {
		byCategory[link.CategoryID] = append(byCategory[link.CategoryID], link)
	}

	groups := make([]LinkGroup, 0, len(categories))
	for _, cat := range categories {
		groups = append(groups, LinkGroup{Category: cat, Links: byCategory[cat.ID]})
	}
	return LinksResponse{Categories: categories, Links: links, Groups: groups}
}

func (s *Store) GetLink(ctx context.Context, id string) (ManagedLink, error) {
	query := `
		SELECT l.id, l.category_id, c.name, l.title, l.url, l.description, l.icon, l.icon_kind,
			l.target, l.visibility_role, l.health_url, l.sort_order, l.status, l.source
		FROM managed_links l
		JOIN link_categories c ON c.id = l.category_id
		WHERE l.id = ` + s.placeholder(1)
	var link ManagedLink
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&link.ID,
		&link.CategoryID,
		&link.CategoryName,
		&link.Title,
		&link.URL,
		&link.Description,
		&link.Icon,
		&link.IconKind,
		&link.Target,
		&link.VisibilityRole,
		&link.HealthURL,
		&link.SortOrder,
		&link.Status,
		&link.Source,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ManagedLink{}, ErrNotFound
	}
	return link, err
}

func (s *Store) CreateLink(ctx context.Context, req LinkMutation) (ManagedLink, error) {
	normalized, err := normalizeLinkMutation(req)
	if err != nil {
		return ManagedLink{}, err
	}
	category, err := s.ensureCategory(ctx, normalized.CategoryID, normalized.CategoryName, 100)
	if err != nil {
		return ManagedLink{}, err
	}
	if normalized.SortOrder == 0 {
		normalized.SortOrder = s.nextLinkSortOrder(ctx, category.ID)
	}

	id := uuid.NewString()
	query := `
		INSERT INTO managed_links (
			id, category_id, title, url, description, icon, icon_kind, target,
			visibility_role, health_url, sort_order, status, source
		) VALUES (` + s.placeholder(1) + `, ` + s.placeholder(2) + `, ` + s.placeholder(3) + `, ` + s.placeholder(4) + `, ` + s.placeholder(5) + `, ` + s.placeholder(6) + `, ` + s.placeholder(7) + `, ` + s.placeholder(8) + `, ` + s.placeholder(9) + `, ` + s.placeholder(10) + `, ` + s.placeholder(11) + `, ` + s.placeholder(12) + `, ` + s.placeholder(13) + `)`
	_, err = s.db.ExecContext(ctx, query,
		id,
		category.ID,
		normalized.Title,
		normalized.URL,
		normalized.Description,
		normalized.Icon,
		iconKind(normalized.Icon),
		normalized.Target,
		normalized.VisibilityRole,
		normalized.HealthURL,
		normalized.SortOrder,
		normalized.Status,
		normalized.Source,
	)
	if err != nil {
		return ManagedLink{}, err
	}
	return s.GetLink(ctx, id)
}

func (s *Store) UpdateLink(ctx context.Context, id string, req LinkMutation) (ManagedLink, error) {
	normalized, err := normalizeLinkMutation(req)
	if err != nil {
		return ManagedLink{}, err
	}
	category, err := s.ensureCategory(ctx, normalized.CategoryID, normalized.CategoryName, 100)
	if err != nil {
		return ManagedLink{}, err
	}
	query := `
		UPDATE managed_links
		SET category_id = ` + s.placeholder(1) + `,
			title = ` + s.placeholder(2) + `,
			url = ` + s.placeholder(3) + `,
			description = ` + s.placeholder(4) + `,
			icon = ` + s.placeholder(5) + `,
			icon_kind = ` + s.placeholder(6) + `,
			target = ` + s.placeholder(7) + `,
			visibility_role = ` + s.placeholder(8) + `,
			health_url = ` + s.placeholder(9) + `,
			sort_order = ` + s.placeholder(10) + `,
			status = ` + s.placeholder(11) + `,
			source = ` + s.placeholder(12) + `,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ` + s.placeholder(13)
	result, err := s.db.ExecContext(ctx, query,
		category.ID,
		normalized.Title,
		normalized.URL,
		normalized.Description,
		normalized.Icon,
		iconKind(normalized.Icon),
		normalized.Target,
		normalized.VisibilityRole,
		normalized.HealthURL,
		normalized.SortOrder,
		normalized.Status,
		normalized.Source,
		id,
	)
	if err != nil {
		return ManagedLink{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ManagedLink{}, ErrNotFound
	}
	return s.GetLink(ctx, id)
}

func (s *Store) DeleteLink(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM managed_links WHERE id = `+s.placeholder(1), id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpsertImportedLink(ctx context.Context, req LinkMutation) (ManagedLink, bool, error) {
	normalized, err := normalizeLinkMutation(req)
	if err != nil {
		return ManagedLink{}, false, err
	}

	existingID, err := s.findLinkByTitleURL(ctx, normalized.Title, normalized.URL)
	if err != nil {
		return ManagedLink{}, false, err
	}
	if existingID != "" {
		link, err := s.UpdateLink(ctx, existingID, normalized)
		return link, false, err
	}
	link, err := s.CreateLink(ctx, normalized)
	return link, true, err
}

func (s *Store) findLinkByTitleURL(ctx context.Context, title string, rawURL string) (string, error) {
	query := `SELECT id FROM managed_links WHERE title = ` + s.placeholder(1) + ` AND url = ` + s.placeholder(2) + ` LIMIT 1`
	var id string
	err := s.db.QueryRowContext(ctx, query, title, rawURL).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (s *Store) ensureCategory(ctx context.Context, id string, name string, sortOrder int) (LinkCategory, error) {
	if strings.TrimSpace(id) != "" {
		cat, err := s.getCategory(ctx, id)
		if err == nil {
			return cat, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return LinkCategory{}, err
		}
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = "General"
	}
	if cat, err := s.getCategoryByName(ctx, name); err == nil {
		return cat, nil
	} else if !errors.Is(err, ErrNotFound) {
		return LinkCategory{}, err
	}

	cat := LinkCategory{
		ID:        uuid.NewString(),
		Name:      name,
		Slug:      slugify(name),
		SortOrder: sortOrder,
	}
	if cat.Slug == "" {
		cat.Slug = "category"
	}

	query := `
		INSERT INTO link_categories (id, name, slug, description, sort_order)
		VALUES (` + s.placeholder(1) + `, ` + s.placeholder(2) + `, ` + s.placeholder(3) + `, '', ` + s.placeholder(4) + `)`
	_, err := s.db.ExecContext(ctx, query, cat.ID, cat.Name, cat.Slug, cat.SortOrder)
	if err != nil {
		return LinkCategory{}, err
	}
	return cat, nil
}

func (s *Store) getCategory(ctx context.Context, id string) (LinkCategory, error) {
	query := `SELECT id, name, slug, description, sort_order FROM link_categories WHERE id = ` + s.placeholder(1)
	var cat LinkCategory
	err := s.db.QueryRowContext(ctx, query, id).Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.Description, &cat.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return LinkCategory{}, ErrNotFound
	}
	return cat, err
}

func (s *Store) getCategoryByName(ctx context.Context, name string) (LinkCategory, error) {
	query := `SELECT id, name, slug, description, sort_order FROM link_categories WHERE name = ` + s.placeholder(1)
	var cat LinkCategory
	err := s.db.QueryRowContext(ctx, query, name).Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.Description, &cat.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return LinkCategory{}, ErrNotFound
	}
	return cat, err
}

func (s *Store) nextLinkSortOrder(ctx context.Context, categoryID string) int {
	query := `SELECT COALESCE(MAX(sort_order), 0) + 10 FROM managed_links WHERE category_id = ` + s.placeholder(1)
	var next int
	if err := s.db.QueryRowContext(ctx, query, categoryID).Scan(&next); err != nil {
		return 10
	}
	return next
}

func (s *Store) LogAudit(ctx context.Context, event AuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.Action == "" || event.Resource == "" {
		return fmt.Errorf("audit action and resource are required")
	}
	query := `
		INSERT INTO audit_events (id, actor, action, resource, resource_id, details)
		VALUES (` + s.placeholder(1) + `, ` + s.placeholder(2) + `, ` + s.placeholder(3) + `, ` + s.placeholder(4) + `, ` + s.placeholder(5) + `, ` + s.placeholder(6) + `)`
	_, err := s.db.ExecContext(ctx, query, event.ID, event.Actor, event.Action, event.Resource, event.ResourceID, "{}")
	return err
}

func (s *Store) CountLinks(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM managed_links`).Scan(&count)
	return count, err
}

func normalizeLinkMutation(req LinkMutation) (LinkMutation, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.URL = strings.TrimSpace(req.URL)
	req.Description = strings.TrimSpace(req.Description)
	req.Icon = strings.TrimSpace(req.Icon)
	req.CategoryID = strings.TrimSpace(req.CategoryID)
	req.CategoryName = strings.TrimSpace(req.CategoryName)
	req.HealthURL = strings.TrimSpace(req.HealthURL)
	req.Source = strings.TrimSpace(req.Source)

	if req.Title == "" {
		return LinkMutation{}, fmt.Errorf("title is required")
	}
	if err := validateHTTPURL(req.URL); err != nil {
		return LinkMutation{}, fmt.Errorf("url is invalid: %w", err)
	}
	if req.HealthURL != "" {
		if err := validateHTTPURL(req.HealthURL); err != nil {
			return LinkMutation{}, fmt.Errorf("health_url is invalid: %w", err)
		}
	}
	if req.Target == "" {
		req.Target = "_blank"
	}
	if req.Target != "_blank" && req.Target != "_self" {
		return LinkMutation{}, fmt.Errorf("target must be '_blank' or '_self'")
	}
	if req.VisibilityRole == "" {
		req.VisibilityRole = "viewer"
	}
	switch req.VisibilityRole {
	case "viewer", "operator", "admin":
	default:
		return LinkMutation{}, fmt.Errorf("visibility_role must be viewer, operator, or admin")
	}
	if req.Status == "" {
		req.Status = "unknown"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	if req.CategoryName == "" && req.CategoryID == "" {
		req.CategoryName = "General"
	}

	return req, nil
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("host is required")
	}
	return nil
}

func iconKind(icon string) string {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return "favicon"
	}
	lower := strings.ToLower(icon)
	switch {
	case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
		return "url"
	case strings.HasPrefix(icon, "/"):
		return "upload"
	default:
		return "slug"
	}
}

func slugify(input string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(input) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

var ErrNotFound = errors.New("not found")

func SortMutationsByOrder(links []LinkMutation) {
	sort.SliceStable(links, func(i, j int) bool {
		if links[i].CategoryName == links[j].CategoryName {
			return links[i].SortOrder < links[j].SortOrder
		}
		return links[i].CategoryName < links[j].CategoryName
	})
}
