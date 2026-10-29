package runtime

import (
	"context"
	"os"
	"sync"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/config"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func TestDatabasePoolBoundsAndRetainsConcurrentConnections(t *testing.T) {
	dsn := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not configured")
	}
	connections, err := OpenProjectDatabases(context.Background(), config.Config{DatabaseURL: dsn}, []project.Project{{ID: "default"}})
	if err != nil {
		t.Fatal(err)
	}
	defer connections.Close()
	start := make(chan struct{})
	failures := make(chan error, 24)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := connections.Primary.ExecContext(context.Background(), "SELECT pg_sleep(0.05)")
			if err != nil {
				failures <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	stats := connections.Primary.Stats()
	if stats.WaitCount == 0 {
		t.Fatal("Concurrent requests did not queue behind a bounded database pool")
	}
	if stats.OpenConnections > 10 {
		t.Fatalf("Pool retained %d connections, want at most10", stats.OpenConnections)
	}
	if stats.MaxIdleClosed != 0 {
		t.Fatalf("Pool discarded %d reusable connections after a short burst", stats.MaxIdleClosed)
	}
}
