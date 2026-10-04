package jobs

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

type SchedulerOptions struct {
	PollInterval time.Duration
	Now          func() time.Time
}

type Scheduler struct {
	db           *sql.DB
	pollInterval time.Duration
	now          func() time.Time
}

type scheduledJob struct {
	id       int64
	schedule string
	command  string
	last     sql.NullTime
}

func NewScheduler(db *sql.DB, options SchedulerOptions) *Scheduler {
	interval := options.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Scheduler{db: db, pollInterval: interval, now: now}
}

func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || s.db == nil {
		return
	}
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.RunOnce(ctx)
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT jobid, schedule, command, last_scheduled_at FROM cron.job WHERE active = true ORDER BY jobid`)
	if err != nil {
		return err
	}
	var jobs []scheduledJob
	for rows.Next() {
		var job scheduledJob
		if err := rows.Scan(&job.id, &job.schedule, &job.command, &job.last); err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	now := s.now().UTC()
	for _, job := range jobs {
		last := time.Time{}
		var claimLast any
		if job.last.Valid {
			last = job.last.Time.UTC()
			claimLast = job.last.Time
		}
		if !scheduleIsDue(job.schedule, last, now) {
			continue
		}
		claimed, err := s.db.ExecContext(ctx, `UPDATE cron.job SET last_scheduled_at = $2 WHERE jobid = $1 AND last_scheduled_at IS NOT DISTINCT FROM $3`, job.id, now, claimLast)
		if err != nil {
			return err
		}
		count, err := claimed.RowsAffected()
		if err != nil || count != 1 {
			continue
		}
		started := s.now().UTC()
		_, commandErr := s.db.ExecContext(ctx, job.command)
		ended := s.now().UTC()
		status := "succeeded"
		message := "command completed"
		if commandErr != nil {
			status = "failed"
			message = commandErr.Error()
			if len(message) > 2000 {
				message = message[:2000]
			}
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO cron.job_run_details (jobid, database, username, command, status, return_message, start_time, end_time) VALUES ($1, current_database(), current_user, $2, $3, $4, $5, $6)`, job.id, job.command, status, message, started, ended); err != nil {
			return err
		}
	}
	return nil
}

func scheduleIsDue(schedule string, last, now time.Time) bool {
	parts := strings.Fields(strings.TrimSpace(schedule))
	if len(parts) == 2 && (parts[1] == "second" || parts[1] == "seconds") {
		seconds, err := strconv.Atoi(parts[0])
		return err == nil && seconds > 0 && (last.IsZero() || !last.Add(time.Duration(seconds)*time.Second).After(now))
	}
	if len(parts) != 5 || (!last.IsZero() && !last.Truncate(time.Minute).Before(now.Truncate(time.Minute))) {
		return false
	}
	values := []int{now.Minute(), now.Hour(), now.Day(), int(now.Month()), int(now.Weekday())}
	bounds := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	for index, expression := range parts {
		if !cronFieldMatches(expression, values[index], bounds[index][0], bounds[index][1], index == 4 && values[index] == 0) {
			return false
		}
	}
	return true
}

func cronFieldMatches(expression string, value, minimum, maximum int, sunday bool) bool {
	for _, term := range strings.Split(expression, ",") {
		step := 1
		base := term
		if slash := strings.IndexByte(term, '/'); slash >= 0 {
			base = term[:slash]
			parsed, err := strconv.Atoi(term[slash+1:])
			if err != nil || parsed <= 0 {
				continue
			}
			step = parsed
		}
		start, end := minimum, maximum
		switch {
		case base == "*":
		case strings.Contains(base, "-"):
			pieces := strings.SplitN(base, "-", 2)
			var err error
			start, err = strconv.Atoi(pieces[0])
			if err != nil {
				continue
			}
			end, err = strconv.Atoi(pieces[1])
			if err != nil {
				continue
			}
		default:
			parsed, err := strconv.Atoi(base)
			if err != nil {
				continue
			}
			start, end = parsed, parsed
		}
		candidate := value
		if sunday && start == 7 {
			candidate = 7
		}
		if start < minimum || end > maximum || candidate < start || candidate > end {
			continue
		}
		if (candidate-start)%step == 0 {
			return true
		}
	}
	return false
}
