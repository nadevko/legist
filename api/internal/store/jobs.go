package store

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type JobStore struct{ db *sqlx.DB }

func NewJobStore(db *sqlx.DB) *JobStore { return &JobStore{db} }

func (s *JobStore) Create(j *Job) error {
	_, err := s.db.NamedExec(
		`INSERT INTO jobs (id, user_id, type, status, expires_at)
		 VALUES (:id, :user_id, :type, :status, :expires_at)`, j,
	)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (s *JobStore) GetByID(id string) (*Job, error) {
	var j Job
	if err := s.db.Get(&j, `SELECT * FROM jobs WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return &j, nil
}

func (s *JobStore) UpdateStatus(id, status string) error {
	if _, err := s.db.Exec(
		`UPDATE jobs SET status = ? WHERE id = ?`, status, id,
	); err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

func (s *JobStore) AddFile(jf *JobFile) error {
	_, err := s.db.NamedExec(
		`INSERT INTO job_files (job_id, file_id, status)
		 VALUES (:job_id, :file_id, :status)`, jf,
	)
	if err != nil {
		return fmt.Errorf("add job file: %w", err)
	}
	return nil
}

func (s *JobStore) ListFiles(jobID string) ([]JobFile, error) {
	var jfs []JobFile
	if err := s.db.Select(&jfs,
		`SELECT * FROM job_files WHERE job_id = ?`, jobID,
	); err != nil {
		return nil, fmt.Errorf("list job files: %w", err)
	}
	return jfs, nil
}

func (s *JobStore) UpdateFileStatus(jobID, fileID, status string, errMsg *string) error {
	if _, err := s.db.Exec(
		`UPDATE job_files SET status = ?, error = ? WHERE job_id = ? AND file_id = ?`,
		status, errMsg, jobID, fileID,
	); err != nil {
		return fmt.Errorf("update job file status: %w", err)
	}
	return nil
}
