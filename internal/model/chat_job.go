package model

import "time"

const (
	ChatJobStatusPending = "pending"
	ChatJobStatusRunning = "running"
	ChatJobStatusSuccess = "success"
	ChatJobStatusFailed  = "failed"
)

type ChatJob struct {
	ID                 string     `json:"id" gorm:"primaryKey;size:64"`
	UserID             string     `json:"user_id" gorm:"size:64;not null;index:idx_user_job_created,priority:1"`
	SessionID          string     `json:"session_id" gorm:"size:64;not null;index:idx_session_job_created,priority:1"`
	Model              string     `json:"model" gorm:"size:128;not null"`
	Status             string     `json:"status" gorm:"size:32;not null;index"`
	RequestText        string     `json:"request_text" gorm:"type:text;not null"`
	UserMessageID      string     `json:"user_message_id,omitempty" gorm:"size:64"`
	AnswerText         string     `json:"answer_text,omitempty" gorm:"type:text"`
	ErrorMessage       string     `json:"error_message,omitempty" gorm:"type:text"`
	RetryCount         int        `json:"retry_count" gorm:"not null;default:0"`
	AssistantMessageID string     `json:"assistant_message_id,omitempty" gorm:"size:64"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at" gorm:"index:idx_user_job_created,priority:2;index:idx_session_job_created,priority:2"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
