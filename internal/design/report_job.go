package design

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

/* Final documents are generated in the background, like the AI prefill: the AI call takes
   longer than an HTTP request may last. POST …/generate-pdf starts the job and returns;
   the progress and the finished report are in Status.ReportJob (GET …/design). */

// ReportJobState is the last document generation of a design.
type ReportJobState struct {
	JobID      string     `json:"jobId"`
	Format     string     `json:"format"`
	State      string     `json:"state"` // running | done | failed (same values as the prefill)
	Error      string     `json:"error,omitempty"`
	Report     *Report    `json:"report,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// ReportTimeout bounds the generation of one document.
const ReportTimeout = 15 * time.Minute

type reportRunner struct {
	reports *ReportService
	locks   *ownerLocks
	logger  *slog.Logger
	timeout time.Duration

	mu      sync.Mutex
	running map[string]runningJob
}

func newReportRunner(reports *ReportService, locks *ownerLocks) *reportRunner {
	return &reportRunner{
		reports: reports,
		locks:   locks,
		logger:  slog.Default(),
		timeout: ReportTimeout,
		running: map[string]runningJob{},
	}
}

func (r *reportRunner) isRunning(o owner, id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.running[ownerKey(o)]
	return ok && job.id == id
}

// prepare registers a new job; call it with the owner lock held, then store the returned
// state and start run.
func (r *reportRunner) prepare(o owner, format string) (*ReportJobState, context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	id := uuid.NewString()
	r.mu.Lock()
	if previous, ok := r.running[ownerKey(o)]; ok {
		previous.cancel()
	}
	r.running[ownerKey(o)] = runningJob{id: id, cancel: cancel}
	r.mu.Unlock()
	return &ReportJobState{JobID: id, Format: format, State: PrefillStateRunning, StartedAt: time.Now().UTC()}, ctx
}

func (r *reportRunner) release(o owner, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job, ok := r.running[ownerKey(o)]; ok && job.id == id {
		job.cancel()
		delete(r.running, ownerKey(o))
	}
}

// run generates the document from the design state at the start of the job.
func (r *reportRunner) run(ctx context.Context, o owner, job ReportJobState, status Status) {
	defer r.release(o, job.JobID)

	report, err := r.reports.Generate(ctx, o.kind, o.id, status, job.Format)
	if err != nil {
		r.logger.Error("document generation failed", "owner", ownerKey(o), "format", job.Format, "error", err)
	}

	unlock := r.locks.lock(ownerKey(o))
	defer unlock()
	saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	current, getErr := o.states.Get(saveCtx, o.id)
	if getErr != nil || current.ReportJob == nil || current.ReportJob.JobID != job.JobID {
		return // replaced by a newer request
	}
	now := time.Now().UTC()
	current.ReportJob.FinishedAt = &now
	if err != nil {
		current.ReportJob.State = PrefillStateFailed
		current.ReportJob.Error = "the document could not be generated"
	} else {
		current.ReportJob.State = PrefillStateDone
		current.ReportJob.Report = &report
		if o.kind == "sessions" {
			current.Reports = append(current.Reports, report)
		}
	}
	if err := o.states.Save(saveCtx, o.id, current); err != nil {
		r.logger.Error("document job: save state failed", "owner", ownerKey(o), "error", err)
	}
}

// withLiveness marks as failed a job stored as running that no longer runs here.
func (r *reportRunner) withLiveness(o owner, status Status) Status {
	if status.ReportJob == nil || status.ReportJob.State != PrefillStateRunning || r.isRunning(o, status.ReportJob.JobID) {
		return status
	}
	job := *status.ReportJob
	job.State = PrefillStateFailed
	job.Error = "interrupted"
	status.ReportJob = &job
	return status
}
