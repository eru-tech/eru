package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DefaultRecordDir is where a live run is recorded and where a replay looks for
// it. One constant for both: they were two literals once, they disagreed, and a
// recording made by the live suite was invisible to the replay that existed to
// score it.
//
// It resolves to eru-ai/local-testdata/trajectories, matching where the page
// eval keeps its own fixtures. That folder is gitignored, because a recording
// carries a real workspace's entity and query names.
const DefaultRecordDir = "../../local-testdata/trajectories"

// Save writes a trajectory to disk.
//
// A live run costs minutes and a real workspace; a recorded one costs nothing.
// Recording every live run means a defect found once can be replayed against
// every later change without running the stack again - which is the difference
// between a suite that gets used and one that does not.
func (t Trajectory) Save(dir, name string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body, err := t.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".json"), body, 0o644)
}

// Load reads a recorded trajectory back.
func Load(path string) (Trajectory, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Trajectory{}, err
	}
	var traj Trajectory
	if err := json.Unmarshal(body, &traj); err != nil {
		return Trajectory{}, err
	}
	return traj, nil
}

// ScoreRecorded scores every recording in a directory against the suite it came
// from, so a change to an expectation can be tested against real runs offline.
func ScoreRecorded(dir string, suite Suite) ([]Result, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(".json")]
		fixture, found := suite.ByName(name)
		if !found {
			continue
		}
		traj, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		results = append(results, fixture.Score(traj))
	}
	return results, nil
}
