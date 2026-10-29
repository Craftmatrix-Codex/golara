package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/config"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/database"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

type DatabaseConnections struct {
	Primary *sql.DB
	Router  *database.Router
	all     []*sql.DB
}

func OpenProjectDatabases(ctx context.Context, cfg config.Config, projects []project.Project) (*DatabaseConnections, error) {
	ids := make([]string, 0, len(projects))
	for _, value := range projects {
		if value.ID == "" {
			return nil, errors.New("project registry contains an empty project ID")
		}
		ids = append(ids, value.ID)
	}
	urls, err := cfg.ResolveProjectDatabaseURLs(ids)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 && cfg.DatabaseURL == "" {
		return nil, nil
	}

	connections := &DatabaseConnections{Router: database.NewRouter(nil)}
	byURL := make(map[string]*sql.DB, len(urls))
	for _, value := range projects {
		url := urls[value.ID]
		connection := byURL[url]
		if connection == nil {
			connection, err = sql.Open("pgx", url)
			if err != nil {
				connections.Close()
				return nil, fmt.Errorf("open database for project %q: %w", value.ID, err)
			}
			configurePool(connection, cfg.DatabasePoolSize)
			pingContext, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = connection.PingContext(pingContext)
			cancel()
			if err != nil {
				_ = connection.Close()
				connections.Close()
				return nil, fmt.Errorf("connect database for project %q: %w", value.ID, err)
			}
			byURL[url] = connection
			connections.all = append(connections.all, connection)
			if connections.Primary == nil {
				connections.Primary = connection
			}
		}
		if err := connections.Router.Register(value.ID, connection); err != nil {
			connections.Close()
			return nil, fmt.Errorf("register database for project %q: %w", value.ID, err)
		}
	}

	if connections.Primary == nil && cfg.DatabaseURL != "" {
		connection, openErr := sql.Open("pgx", cfg.DatabaseURL)
		if openErr != nil {
			return nil, fmt.Errorf("open default database: %w", openErr)
		}
		configurePool(connection, cfg.DatabasePoolSize)
		pingContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		pingErr := connection.PingContext(pingContext)
		cancel()
		if pingErr != nil {
			_ = connection.Close()
			return nil, fmt.Errorf("connect default database: %w", pingErr)
		}
		connections.Primary = connection
		connections.all = append(connections.all, connection)
	}
	return connections, nil
}

func configurePool(connection *sql.DB, size int) {
	if size <= 0 {
		size = 10
	}
	connection.SetMaxOpenConns(size)
	connection.SetMaxIdleConns(size)
	connection.SetConnMaxIdleTime(5 * time.Minute)
	connection.SetConnMaxLifetime(30 * time.Minute)
}

func (c *DatabaseConnections) Close() {
	if c == nil {
		return
	}
	for _, connection := range c.all {
		_ = connection.Close()
	}
}
