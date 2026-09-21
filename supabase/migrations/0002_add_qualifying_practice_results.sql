-- Migration 0002: Allow qualifying and practice in RESULTS session_type
ALTER TABLE RESULTS DROP CONSTRAINT IF EXISTS results_session_type_check;

ALTER TABLE RESULTS ADD CONSTRAINT results_session_type_check 
    CHECK (session_type IN ('race', 'sprint', 'qualifying', 'practice'));

-- Add index on session_type for fast timetable lookup
CREATE INDEX IF NOT EXISTS idx_results_event_type_pos 
    ON RESULTS(event_id, session_type, result_position);
