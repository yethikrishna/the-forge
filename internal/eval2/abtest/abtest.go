package abtest

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Variant struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

type Result struct {
	Variant   string  `json:"variant"`
	Score     float64 `json:"score"`
	LatencyMS int     `json:"latency_ms"`
	CostUSD   float64 `json:"cost_usd"`
	Success   bool    `json:"success"`
	Output    string  `json:"output,omitempty"`
}

type Experiment struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Prompt     string    `json:"prompt"`
	Variants   []Variant `json:"variants"`
	Results    []Result  `json:"results"`
	Status     string    `json:"status"`
	SampleSize int       `json:"sample_size"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Analysis struct {
	ExperimentID   string         `json:"experiment_id"`
	ExperimentName string         `json:"experiment_name"`
	VariantStats   []VariantStats `json:"variant_stats"`
	Winner         string         `json:"winner,omitempty"`
	Confidence     float64        `json:"confidence"`
	Significant    bool           `json:"significant"`
	Recommendation string         `json:"recommendation"`
}

type VariantStats struct {
	Name        string  `json:"name"`
	Model       string  `json:"model"`
	N           int     `json:"n"`
	MeanScore   float64 `json:"mean_score"`
	MeanCost    float64 `json:"mean_cost"`
	MeanLatency float64 `json:"mean_latency_ms"`
	SuccessRate float64 `json:"success_rate"`
	StdDev      float64 `json:"std_dev"`
}

type Store struct {
	mu  sync.RWMutex
	dir string
}

func NewStore(dir string) *Store {
	os.MkdirAll(dir, 0o755)
	return &Store{dir: dir}
}

func (s *Store) Create(name, prompt string, variants []Variant, sampleSize int) (*Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sampleSize <= 0 {
		sampleSize = 30
	}
	exp := &Experiment{
		ID: genID("exp"), Name: name, Prompt: prompt,
		Variants: variants, Results: []Result{},
		Status: "draft", SampleSize: sampleSize,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return exp, s.save(exp)
}

func (s *Store) Get(id string) (*Experiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.get(id)
}

func (s *Store) List() ([]*Experiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, nil
	}
	var exps []*Experiment
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var exp Experiment
		if err := json.Unmarshal(data, &exp); err != nil {
			continue
		}
		exps = append(exps, &exp)
	}
	sort.Slice(exps, func(i, j int) bool {
		return exps[i].CreatedAt.After(exps[j].CreatedAt)
	})
	return exps, nil
}

func (s *Store) Start(id string) (*Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, err := s.get(id)
	if err != nil {
		return nil, err
	}
	if exp.Status != "draft" {
		return nil, fmt.Errorf("can only start draft experiments, current: %s", exp.Status)
	}
	exp.Status = "running"
	exp.UpdatedAt = time.Now()
	return exp, s.save(exp)
}

func (s *Store) RecordResult(id string, result Result) (*Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, err := s.get(id)
	if err != nil {
		return nil, err
	}
	if exp.Status != "running" {
		return nil, fmt.Errorf("can only record for running experiments, current: %s", exp.Status)
	}
	valid := false
	for _, v := range exp.Variants {
		if v.Name == result.Variant {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("variant %q not found", result.Variant)
	}
	exp.Results = append(exp.Results, result)
	exp.UpdatedAt = time.Now()
	if s.isComplete(exp) {
		exp.Status = "completed"
	}
	return exp, s.save(exp)
}

func (s *Store) Complete(id string) (*Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, err := s.get(id)
	if err != nil {
		return nil, err
	}
	exp.Status = "completed"
	exp.UpdatedAt = time.Now()
	return exp, s.save(exp)
}

func (s *Store) Cancel(id string) (*Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, err := s.get(id)
	if err != nil {
		return nil, err
	}
	exp.Status = "cancelled"
	exp.UpdatedAt = time.Now()
	return exp, s.save(exp)
}

func Analyze(exp *Experiment) *Analysis {
	a := &Analysis{ExperimentID: exp.ID, ExperimentName: exp.Name}
	vr := make(map[string][]Result)
	for _, r := range exp.Results {
		vr[r.Variant] = append(vr[r.Variant], r)
	}
	for _, v := range exp.Variants {
		results := vr[v.Name]
		vs := VariantStats{Name: v.Name, Model: v.Model, N: len(results)}
		if len(results) > 0 {
			ts, tc, tl := 0.0, 0.0, 0.0
			succ := 0
			scores := make([]float64, len(results))
			for i, r := range results {
				ts += r.Score
				tc += r.CostUSD
				tl += float64(r.LatencyMS)
				if r.Success {
					succ++
				}
				scores[i] = r.Score
			}
			n := float64(len(results))
			vs.MeanScore = ts / n
			vs.MeanCost = tc / n
			vs.MeanLatency = tl / n
			vs.SuccessRate = float64(succ) / n
			if len(scores) > 1 {
				variance := 0.0
				for _, sc := range scores {
					d := sc - vs.MeanScore
					variance += d * d
				}
				vs.StdDev = math.Sqrt(variance / float64(len(scores)-1))
			}
		}
		a.VariantStats = append(a.VariantStats, vs)
	}
	if len(a.VariantStats) >= 2 {
		best := a.VariantStats[0]
		for _, vs := range a.VariantStats[1:] {
			if vs.MeanScore > best.MeanScore {
				best = vs
			}
		}
		if best.N >= 5 {
			a.Winner = best.Name
			second := a.VariantStats[0]
			if second.Name == best.Name && len(a.VariantStats) > 1 {
				second = a.VariantStats[1]
			}
			for _, vs := range a.VariantStats {
				if vs.Name != best.Name && vs.MeanScore > second.MeanScore {
					second = vs
				}
			}
			diff := best.MeanScore - second.MeanScore
			pooledSD := math.Sqrt((best.StdDev*best.StdDev + second.StdDev*second.StdDev) / 2)
			if pooledSD > 0 && diff > 2*pooledSD {
				a.Significant = true
				a.Confidence = math.Min(0.99, diff/(3*pooledSD))
			} else {
				a.Confidence = 0.5
			}
			a.Recommendation = fmt.Sprintf("Variant %s (%s) has highest mean score (%.3f).", best.Name, best.Model, best.MeanScore)
		}
	}
	return a
}

func FormatAnalysis(a *Analysis) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Analysis: %s (%s)\n", a.ExperimentName, a.ExperimentID))
	sb.WriteString(fmt.Sprintf("  Significant: %v\n", a.Significant))
	if a.Winner != "" {
		sb.WriteString(fmt.Sprintf("  Winner:      %s (confidence: %.0f%%)\n", a.Winner, a.Confidence*100))
	}
	sb.WriteString("\n  Variants:\n")
	for _, vs := range a.VariantStats {
		sb.WriteString(fmt.Sprintf("    %s (%s):\n", vs.Name, vs.Model))
		sb.WriteString(fmt.Sprintf("      Samples:      %d\n", vs.N))
		sb.WriteString(fmt.Sprintf("      Mean Score:   %.3f\n", vs.MeanScore))
		sb.WriteString(fmt.Sprintf("      Std Dev:      %.3f\n", vs.StdDev))
		sb.WriteString(fmt.Sprintf("      Mean Cost:    $%.4f\n", vs.MeanCost))
		sb.WriteString(fmt.Sprintf("      Mean Latency: %.0fms\n", vs.MeanLatency))
		sb.WriteString(fmt.Sprintf("      Success Rate: %.0f%%\n", vs.SuccessRate*100))
	}
	if a.Recommendation != "" {
		sb.WriteString(fmt.Sprintf("\n  Recommendation: %s\n", a.Recommendation))
	}
	return sb.String()
}

func (s *Store) get(id string) (*Experiment, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("experiment %q not found", id)
	}
	var exp Experiment
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, err
	}
	return &exp, nil
}

func (s *Store) save(exp *Experiment) error {
	data, err := json.MarshalIndent(exp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, exp.ID+".json"), data, 0o644)
}

func (s *Store) isComplete(exp *Experiment) bool {
	vr := make(map[string]int)
	for _, r := range exp.Results {
		vr[r.Variant]++
	}
	for _, v := range exp.Variants {
		if vr[v.Name] < exp.SampleSize {
			return false
		}
	}
	return true
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("%s-%x", prefix, b)
}
