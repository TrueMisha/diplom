package model

import (
	"errors"
	"strings"
	"time"
)

// ─── ScrapeJob ────────────────────────────────────────────────────────────────

type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusFailed  JobStatus = "failed"
)

type SourceType string

const (
	SourceTypeHTML SourceType = "html"
	SourceTypeAPI  SourceType = "api"
	SourceTypeJS   SourceType = "js"
)

type ScrapeJob struct {
	ID         string     `json:"id"`
	Brand      string     `json:"brand"`
	URL        string     `json:"url"`
	SourceType SourceType `json:"source_type"`
	Status     JobStatus  `json:"status"`
	ErrorMsg   string     `json:"error_msg,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func (j *ScrapeJob) Validate() error {
	if strings.TrimSpace(j.Brand) == "" {
		return errors.New("brand is required")
	}
	if strings.TrimSpace(j.URL) == "" {
		return errors.New("url is required")
	}
	switch j.SourceType {
	case SourceTypeHTML, SourceTypeAPI, SourceTypeJS:
	default:
		return errors.New("invalid source_type")
	}
	return nil
}

// ─── RawPage ──────────────────────────────────────────────────────────────────

type RawPage struct {
	ID         string    `json:"id"`
	JobID      string    `json:"job_id,omitempty"`
	Brand      string    `json:"brand"`
	SourceURL  string    `json:"source_url"`
	SourceType string    `json:"source_type"`
	RawHTML    string    `json:"raw_html"`
	ScrapedAt  time.Time `json:"scraped_at"`
}

func (p *RawPage) Validate() error {
	if strings.TrimSpace(p.Brand) == "" {
		return errors.New("brand is required")
	}
	if strings.TrimSpace(p.SourceURL) == "" {
		return errors.New("source_url is required")
	}
	return nil
}

// ─── ParsedWord ───────────────────────────────────────────────────────────────

type ParsedWord struct {
	ID        int64     `json:"id"`
	Brand     string    `json:"brand"`
	SourceURL string    `json:"source_url"`
	Word      string    `json:"word"`
	Frequency int       `json:"frequency"`
	ScrapedAt time.Time `json:"scraped_at"`
}

// ─── WordCooccurrence ─────────────────────────────────────────────────────────

type WordCooccurrence struct {
	ID         int64     `json:"id"`
	Brand      string    `json:"brand"`
	TargetWord string    `json:"target_word"`
	Neighbor   string    `json:"neighbor"`
	Weight     int       `json:"weight"`
	SourceURL  string    `json:"source_url"`
	ScrapedAt  time.Time `json:"scraped_at"`
}

// ─── Query types ──────────────────────────────────────────────────────────────

type TopWordsQuery struct {
	Brand string
	Limit int
}

type CooccurrenceQuery struct {
	Brand      string
	TargetWord string
	Limit      int
}

type WordFreqResult struct {
	Word      string `json:"word"`
	Frequency int    `json:"frequency"`
}

type CooccurrenceResult struct {
	Neighbor string `json:"neighbor"`
	Weight   int    `json:"weight"`
}
