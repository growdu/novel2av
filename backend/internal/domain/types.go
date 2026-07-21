// Package domain holds plain entities + business rules. No external imports
// outside stdlib — these types must be cheap to construct in tests.
package domain

import "time"

type ProjectStatus string

const (
	ProjectCreated   ProjectStatus = "CREATED"
	ProjectSplitting ProjectStatus = "SPLITTING"
	ProjectSplit     ProjectStatus = "SPLIT"
	ProjectExtracting ProjectStatus = "EXTRACTING"
	ProjectReady     ProjectStatus = "READY"
	ProjectRunning   ProjectStatus = "RUNNING"
	ProjectDone      ProjectStatus = "DONE"
	ProjectFailed    ProjectStatus = "FAILED"
)

type Project struct {
	ID         string        `json:"id"`
	UserID     string        `json:"user_id"`
	Title      string        `json:"title"`
	Author     string        `json:"author"`
	SourceKey  string        `json:"source_key"`
	Status     ProjectStatus `json:"status"`
	WordCount  int           `json:"word_count"`
	Config     ProjectConfig `json:"config"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type ProjectConfig struct {
	Aspect string `json:"aspect"` // "9:16" | "16:9"
	Style  string `json:"style"`
	Voice  string `json:"voice"`
}

type Chapter struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Index     int       `json:"index"`
	Title     string    `json:"title"`
	WordCount int       `json:"word_count"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Character struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Aliases     []string  `json:"aliases"`
	Role        string    `json:"role"`
	Appearance  string    `json:"appearance"`
	Personality string    `json:"personality"`
	Voice       string    `json:"voice"`
	RefImageKey string    `json:"ref_image_key"`
	CreatedAt   time.Time `json:"created_at"`
}

type Shot struct {
	ID          string    `json:"id"`
	ChapterID   string    `json:"chapter_id"`
	SceneIdx    int       `json:"scene_idx"`
	ShotIdx     int       `json:"shot_idx"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Narration   string    `json:"narration"`
	Mood        string    `json:"mood"`
	DurationSec float64   `json:"duration_sec"`
	Status      string    `json:"status"`
	ImageKey    string    `json:"image_key"`
	TTSKey      string    `json:"tts_key"`
	BGMKey      string    `json:"bgm_key"`
	SubtitleKey string    `json:"subtitle_key"`
	CreatedAt   time.Time `json:"created_at"`
}

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSuccess   JobStatus = "success"
	JobFailed    JobStatus = "failed"
	JobRetrying  JobStatus = "retrying"
)

type Job struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Type      string    `json:"type"`
	Status    JobStatus `json:"status"`
	Attempts  int       `json:"attempts"`
	Meta      JobMeta   `json:"meta"`
	CreatedAt time.Time `json:"created_at"`
}

type JobMeta struct {
	Step        string `json:"step,omitempty"`
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	CostEstUSD  float64 `json:"cost_est_usd"`
	CostActual  float64 `json:"cost_actual_usd"`
	LastMessage string `json:"last_message,omitempty"`
}
