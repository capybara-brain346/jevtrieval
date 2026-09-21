package store

import "context"

func (s *Store) EventsAfter(ctx context.Context, runID, after int64) ([]RunEvent, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, event_type, payload FROM run_events WHERE run_id=$1 AND id>$2 ORDER BY id`, runID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []RunEvent
	for rows.Next() {
		var event RunEvent
		if err := rows.Scan(&event.ID, &event.EventType, &event.Payload); err != nil {
			return nil, err
		}
		event.RunID = runID
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) LastEventID(ctx context.Context, runID int64) (int64, error) {
	var id int64
	if err := s.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(id),0) FROM run_events WHERE run_id=$1`, runID).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) IsTerminal(ctx context.Context, runID int64) (bool, error) {
	var status string
	if err := s.Pool.QueryRow(ctx, `SELECT status FROM runs WHERE id=$1`, runID).Scan(&status); err != nil {
		return false, err
	}
	return status == "completed" || status == "failed", nil
}
