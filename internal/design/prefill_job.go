package design

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

/* The AI prefill runs in the background: analysis + one call per section takes longer than
   an HTTP request may last. PUT …/spark starts it and returns at once; the progress is in
   Status.Prefill (read with GET …/design) and every section is stored as soon as it is
   ready, so the interface can show it filling up.

   Every state change of a design (sections, Spark, PDF, images, prefill) goes through
   ownerLocks, so a section saved by the user while the job runs is never overwritten.
   Locks and running jobs are per process: with several backend instances a job keeps
   running on the instance that started it. */

// PrefillTimeout is the default bound of a whole prefill job (analysis + all sections);
// see Handler.WithPrefillTimeout. Slow (e.g. free, reasoning) models need ~1–2 min per call.
const PrefillTimeout = 20 * time.Minute

// Prefill modes: one AI call per section (progress section by section, a failure costs one
// section) or a single call for everything (one request: for free tiers with a daily limit).
const (
	PrefillModeSections = "sections"
	PrefillModeSingle   = "single"
)

type prefillRunner struct {
	prefiller *Prefiller
	service   *Service
	locks     *ownerLocks
	logger    *slog.Logger
	timeout   time.Duration
	mode      string

	mu      sync.Mutex
	running map[string]runningJob // by owner key
}

type runningJob struct {
	id     string
	cancel context.CancelFunc
}

func newPrefillRunner(prefiller *Prefiller, service *Service, locks *ownerLocks) *prefillRunner {
	return &prefillRunner{
		prefiller: prefiller,
		service:   service,
		locks:     locks,
		logger:    slog.Default(),
		timeout:   PrefillTimeout,
		mode:      PrefillModeSections,
		running:   map[string]runningJob{},
	}
}

func ownerKey(o owner) string {
	return o.kind + ":" + o.id
}

// isRunning reports whether job id is running in this process.
func (r *prefillRunner) isRunning(o owner, id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.running[ownerKey(o)]
	return ok && job.id == id
}

// prepare registers a new job for the owner, cancelling the one still running, and returns
// the state to store before the job starts. Call it with the owner lock held.
func (r *prefillRunner) prepare(o owner) (*PrefillState, context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	id := uuid.NewString()

	r.mu.Lock()
	if previous, ok := r.running[ownerKey(o)]; ok {
		previous.cancel()
	}
	r.running[ownerKey(o)] = runningJob{id: id, cancel: cancel}
	r.mu.Unlock()

	return &PrefillState{
		JobID:     id,
		State:     PrefillStateRunning,
		StartedAt: time.Now().UTC(),
		Notes:     map[string][]FieldNote{},
	}, ctx
}

// release forgets a job that will not run (e.g. its state could not be stored).
func (r *prefillRunner) release(o owner, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job, ok := r.running[ownerKey(o)]; ok && job.id == id {
		job.cancel()
		delete(r.running, ownerKey(o))
	}
}

// run fills the empty sections from the Spark. It must be started (go) after prepare.
func (r *prefillRunner) run(ctx context.Context, o owner, jobID string, spark string) {
	defer r.release(o, jobID)
	logger := r.logger.With("owner", ownerKey(o), "prefill_job", jobID, "mode", r.mode)

	if r.mode == PrefillModeSingle {
		r.runSingle(ctx, o, jobID, spark, logger)
		return
	}
	r.runSections(ctx, o, jobID, spark, logger)
}

// runSingle: analysis and every empty section in one AI call.
func (r *prefillRunner) runSingle(ctx context.Context, o owner, jobID string, spark string, logger *slog.Logger) {
	status, ok := r.current(o, jobID)
	if !ok {
		return
	}
	existing := map[int]json.RawMessage{}
	var empty, skipped []int
	for number := 1; number <= SectionCount; number++ {
		if content := status.Section(number).Content; len(content) > 0 {
			existing[number] = content
			skipped = append(skipped, number)
		} else {
			empty = append(empty, number)
		}
	}
	r.update(o, jobID, func(status *Status) { status.Prefill.Skipped = skipped })

	analysis, drafts, err := r.prefiller.DraftAll(ctx, spark, existing, empty)
	if err == nil && !anyContent(drafts) && ctx.Err() == nil {
		// Free models sometimes answer with a well-formed but empty draft: one more try.
		logger.Warn("prefill draft empty, retrying once")
		analysis, drafts, err = r.prefiller.DraftAll(ctx, spark, existing, empty)
	}
	if err != nil {
		logger.Error("prefill draft failed", "error", err)
		r.update(o, jobID, func(status *Status) { status.Prefill.Failed = empty })
		if ctx.Err() != nil {
			r.finish(o, jobID, PrefillStateFailed, "interrupted")
		} else {
			r.finish(o, jobID, PrefillStateFailed, "the AI could not draft the design")
		}
		return
	}

	filled := 0
	stored := r.update(o, jobID, func(status *Status) {
		status.Prefill.Analysis = &analysis
		for _, number := range empty {
			draft := drafts[number]
			r.applyDraft(status, number, draft, logger)
			if len(draft.Content) == 0 {
				// Nothing usable for this section: failed, so the interface offers a retry.
				status.Prefill.Failed = sortedInts(append(status.Prefill.Failed, number))
				continue
			}
			status.Prefill.Completed = sortedInts(append(status.Prefill.Completed, number))
			filled++
		}
	})
	switch {
	case !stored:
		return
	case filled == 0:
		r.finish(o, jobID, PrefillStateFailed, "the AI returned an empty draft")
	default:
		r.finish(o, jobID, PrefillStateDone, "")
	}
}

func anyContent(drafts map[int]SectionDraft) bool {
	for _, draft := range drafts {
		if len(draft.Content) > 0 {
			return true
		}
	}
	return false
}

// applyDraft stores a section draft unless the user filled the section meanwhile (their
// content wins), with its notes; problems of the answer are logged for diagnosis.
func (r *prefillRunner) applyDraft(status *Status, number int, draft SectionDraft, logger *slog.Logger) {
	if len(draft.Dropped) > 0 || len(draft.MissingRequired) > 0 {
		logger.Warn("prefill section incomplete",
			"section", number, "dropped", draft.Dropped, "missing_required", draft.MissingRequired)
	}
	if len(draft.Content) == 0 {
		return
	}
	*status = r.service.ApplyPrefill(*status, map[string]json.RawMessage{SectionKey(number): draft.Content})
	if string(status.Section(number).Content) == string(draft.Content) && len(draft.Notes) > 0 {
		status.Prefill.Notes[SectionKey(number)] = draft.Notes
	}
}

// runSections: one call for the analysis, then one per section, in order.
func (r *prefillRunner) runSections(ctx context.Context, o owner, jobID string, spark string, logger *slog.Logger) {
	analysis, err := r.prefiller.Analyze(ctx, spark)
	if err != nil {
		logger.Error("prefill analysis failed", "error", err)
		r.finish(o, jobID, PrefillStateFailed, "analysis failed")
		return
	}
	if !r.update(o, jobID, func(status *Status) {
		status.Prefill.Analysis = &analysis
		status.Prefill.Current = 1
	}) {
		return
	}

	sections := map[int]json.RawMessage{} // content of the sections so far, as context
	filled := 0
	for number := 1; number <= SectionCount; number++ {
		if ctx.Err() != nil {
			break
		}

		status, ok := r.current(o, jobID)
		if !ok {
			return
		}
		if existing := status.Section(number).Content; len(existing) > 0 {
			// Already filled by the user: kept, and used as context for the next sections.
			sections[number] = existing
			r.update(o, jobID, func(status *Status) {
				status.Prefill.Skipped = sortedInts(append(status.Prefill.Skipped, number))
				status.Prefill.Current = number + 1
			})
			continue
		}

		draft, err := r.prefiller.FillSection(ctx, spark, analysis, sections, number)
		if err != nil {
			logger.Error("prefill section failed", "section", number, "error", err)
			r.update(o, jobID, func(status *Status) {
				status.Prefill.Failed = sortedInts(append(status.Prefill.Failed, number))
				status.Prefill.Current = number + 1
			})
			continue
		}

		stored := r.update(o, jobID, func(status *Status) {
			r.applyDraft(status, number, draft, logger)
			if content := status.Section(number).Content; len(content) > 0 {
				sections[number] = content
			}
			if len(draft.Content) == 0 {
				status.Prefill.Failed = sortedInts(append(status.Prefill.Failed, number))
			} else {
				status.Prefill.Completed = sortedInts(append(status.Prefill.Completed, number))
			}
			status.Prefill.Current = number + 1
		})
		if !stored {
			return
		}
		if len(draft.Content) > 0 {
			filled++
		}
	}

	switch {
	case ctx.Err() != nil:
		logger.Error("prefill interrupted", "error", ctx.Err())
		r.finish(o, jobID, PrefillStateFailed, "interrupted")
	case filled == 0:
		r.finish(o, jobID, PrefillStateFailed, "no section could be filled")
	default:
		r.finish(o, jobID, PrefillStateDone, "")
	}
}

func (r *prefillRunner) finish(o owner, jobID string, state string, message string) {
	r.update(o, jobID, func(status *Status) {
		now := time.Now().UTC()
		status.Prefill.State = state
		status.Prefill.Error = message
		status.Prefill.FinishedAt = &now
		status.Prefill.Current = 0
	})
}

// current reads the state; false if the job has been replaced or the state is unreadable.
func (r *prefillRunner) current(o owner, jobID string) (Status, bool) {
	unlock := r.locks.lock(ownerKey(o))
	defer unlock()
	status, err := o.states.Get(context.Background(), o.id)
	if err != nil || status.Prefill == nil || status.Prefill.JobID != jobID {
		return Status{}, false
	}
	return status, true
}

// update applies change to the stored state if job is still the owner's current job.
func (r *prefillRunner) update(o owner, jobID string, change func(status *Status)) bool {
	unlock := r.locks.lock(ownerKey(o))
	defer unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := o.states.Get(ctx, o.id)
	if err != nil {
		r.logger.Error("prefill: read state failed", "owner", ownerKey(o), "error", err)
		return false
	}
	if status.Prefill == nil || status.Prefill.JobID != jobID {
		return false // a newer Spark replaced this job
	}
	if status.Prefill.Notes == nil {
		status.Prefill.Notes = map[string][]FieldNote{}
	}
	change(&status)
	if err := o.states.Save(ctx, o.id, status); err != nil {
		r.logger.Error("prefill: save state failed", "owner", ownerKey(o), "error", err)
		return false
	}
	return true
}

// withLiveness marks as failed a job stored as running that no longer runs in this process
// (backend restarted, design imported from a session…), so clients stop waiting for it.
func (r *prefillRunner) withLiveness(o owner, status Status) Status {
	if status.Prefill == nil || status.Prefill.State != PrefillStateRunning {
		return status
	}
	if r.isRunning(o, status.Prefill.JobID) {
		return status
	}
	prefill := *status.Prefill
	prefill.State = PrefillStateFailed
	prefill.Error = "interrupted"
	prefill.Current = 0
	status.Prefill = &prefill
	return status
}

// ── Per-design locks ────────────────────────────────────────────────────────

type ownerLocks struct {
	mu    sync.Mutex
	locks map[string]*refMutex
}

type refMutex struct {
	sync.Mutex
	refs int
}

func newOwnerLocks() *ownerLocks {
	return &ownerLocks{locks: map[string]*refMutex{}}
}

// lock locks the design of key and returns the function that unlocks it.
func (l *ownerLocks) lock(key string) func() {
	l.mu.Lock()
	m, ok := l.locks[key]
	if !ok {
		m = &refMutex{}
		l.locks[key] = m
	}
	m.refs++
	l.mu.Unlock()

	m.Lock()
	return func() {
		m.Unlock()
		l.mu.Lock()
		m.refs--
		if m.refs == 0 {
			delete(l.locks, key)
		}
		l.mu.Unlock()
	}
}
