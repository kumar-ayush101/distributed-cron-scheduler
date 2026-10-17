package models

import (
	"errors"
	"testing"
	"time"
)

func TestCreateJobInput_Validate(t *testing.T) {
	long := make([]byte, 121)
	for i := range long {
		long[i] = 'a'
	}

	tests := []struct {
		name    string
		in      CreateJobInput
		wantErr error
	}{
		{"valid", CreateJobInput{Name: "backup", CronSchedule: "*/5 * * * *"}, nil},
		{"empty name", CreateJobInput{Name: "   ", CronSchedule: "* * * * *"}, ErrEmptyName},
		{"name too long", CreateJobInput{Name: string(long), CronSchedule: "* * * * *"}, ErrNameTooLong},
		{"empty cron", CreateJobInput{Name: "job", CronSchedule: ""}, ErrEmptyCron},
		{"invalid cron", CreateJobInput{Name: "job", CronSchedule: "not a cron"}, ErrInvalidCron},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateJobInput_Validate(t *testing.T) {
	bad := JobStatus("weird")
	good := StatusPaused
	badCron := "xyz"
	goodCron := "0 0 * * *"
	emptyName := "  "

	tests := []struct {
		name    string
		in      UpdateJobInput
		wantErr error
	}{
		{"empty", UpdateJobInput{}, nil},
		{"valid status", UpdateJobInput{Status: &good}, nil},
		{"invalid status", UpdateJobInput{Status: &bad}, ErrInvalidState},
		{"invalid cron", UpdateJobInput{CronSchedule: &badCron}, ErrInvalidCron},
		{"valid cron", UpdateJobInput{CronSchedule: &goodCron}, nil},
		{"empty name", UpdateJobInput{Name: &emptyName}, ErrEmptyName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Validate(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobStatus_Valid(t *testing.T) {
	for _, s := range []JobStatus{StatusActive, StatusPaused, StatusDisabled} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if JobStatus("bogus").Valid() {
		t.Error("bogus should be invalid")
	}
}

func TestNextRun(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	next, err := NextRun("*/5 * * * *", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := base.Add(5 * time.Minute)
	if !next.Equal(want) {
		t.Fatalf("NextRun = %v, want %v", next, want)
	}

	if _, err := NextRun("garbage", base); err == nil {
		t.Fatal("expected error for invalid cron")
	}
}
