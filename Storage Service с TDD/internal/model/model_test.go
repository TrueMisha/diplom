package model_test

import (
	"testing"

	"github.com/brandmon/storage-service/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestScrapeJob_Validate_Valid(t *testing.T) {
	job := model.ScrapeJob{
		Brand:      "сбер",
		URL:        "https://banki.ru",
		SourceType: model.SourceTypeHTML,
	}
	assert.NoError(t, job.Validate())
}

func TestScrapeJob_Validate_MissingBrand(t *testing.T) {
	job := model.ScrapeJob{URL: "https://banki.ru", SourceType: model.SourceTypeHTML}
	err := job.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "brand")
}

func TestScrapeJob_Validate_MissingURL(t *testing.T) {
	job := model.ScrapeJob{Brand: "сбер", SourceType: model.SourceTypeHTML}
	err := job.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "url")
}

func TestScrapeJob_Validate_InvalidSourceType(t *testing.T) {
	job := model.ScrapeJob{Brand: "сбер", URL: "https://banki.ru", SourceType: "ftp"}
	err := job.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source_type")
}

func TestRawPage_Validate_Valid(t *testing.T) {
	page := model.RawPage{Brand: "сбер", SourceURL: "https://banki.ru"}
	assert.NoError(t, page.Validate())
}

func TestRawPage_Validate_MissingFields(t *testing.T) {
	assert.Error(t, (&model.RawPage{SourceURL: "https://x.com"}).Validate())
	assert.Error(t, (&model.RawPage{Brand: "сбер"}).Validate())
}
