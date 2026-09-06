package scorecalcworker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"


	"encore.dev/rlog"
	"encore.dev/pubsub"
	"encore.app/scorecalc"
	"encore.app/scorecalcworker/sqlc"
)

// db is the shared Lightwing database (direct reference: Encore tracks
// resources through package-level references only).
var _ = pubsub.NewSubscription(scorecalc.ScoreCalcRequestedTopic, "scorecalc-worker-requests", pubsub.SubscriptionConfig[scorecalc.ScoreCalcRequested]{
	Handler: HandleScoreCalcRequest,
})

// HandleScoreCalcRequest claims a pending job, computes the projection from
// canonical source data, and publishes the completion.
//
// Mirrors ts-legacy/scorecalc-worker/worker.ts handleScoreCalcRequest.
func HandleScoreCalcRequest(ctx context.Context, event scorecalc.ScoreCalcRequested) error {
	jobID, eventID, generation := event.JobID, event.EventID, event.Generation
	rlog.Info(fmt.Sprintf("Received ScoreCalcRequested message: jobId=%s, eventId=%s, gen=%d", jobID, eventID, generation))

	stx, err := std().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer stx.Rollback()
	qq := q().WithTx(stx)

	task, err := qq.GetTaskForClaim(ctx, jobID)
	if errors.Is(err, sql.ErrNoRows) {
		rlog.Warn(fmt.Sprintf("Job %s not found in database.", jobID))
		return nil
	}
	if err != nil {
		return err
	}
	jobEventID, jobGeneration, status, attempts := task.EventId, int(task.Generation), task.Status, task.Attempts
	if status == "COMPLETED" || status == "SUPERSEDED" {
		rlog.Info(fmt.Sprintf("Job %s is already %s. Skipping.", jobID, status))
		return nil
	}

	latestGeneration, err := qq.GetLatestGenerationState(ctx, jobEventID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && task.Generation < latestGeneration {
		rlog.Info(fmt.Sprintf("Job %s generation %d is superseded by latest generation %d.", jobID, jobGeneration, latestGeneration))
		if err := qq.MarkTaskSuperseded(ctx, jobID); err != nil {
			return err
		}
		return stx.Commit()
	}

	if status != "PENDING" && status != "FAILED" {
		rlog.Info(fmt.Sprintf("Job %s is in status %s. Skipping.", jobID, status))
		return nil
	}

	now := time.Now().UTC()
	if err := qq.ClaimTask(ctx, sqlc.ClaimTaskParams{
		Attempts:  attempts + 1,
		StartedAt: sql.NullTime{Time: now, Valid: true},
		ID:        jobID,
	}); err != nil {
		return err
	}
	if err := stx.Commit(); err != nil {
		return err
	}

	scoringType, err := q().GetEventScoringType(ctx, jobEventID)
	if errors.Is(err, sql.ErrNoRows) {
		rlog.Error(fmt.Sprintf("Event %s not found for Job %s. Marking failed.", jobEventID, jobID))
		if err := q().FailTask(ctx, sqlc.FailTaskParams{
			LastError:   sql.NullString{String: fmt.Sprintf("Event %s not found.", jobEventID), Valid: true},
			CompletedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
			ID:          jobID,
		}); err != nil {
			return err
		}
		_, err = scorecalc.ScoreCalcFailedTopic.Publish(ctx, scorecalc.ScoreCalcFailed{
			Version: 1, JobID: jobID, EventID: eventID, Generation: generation,
			FailedAt: time.Now().UTC().Format(time.RFC3339Nano),
			ErrorCode: "EVENT_NOT_FOUND",
			ErrorMessage: fmt.Sprintf("Event %s not found.", jobEventID),
			Retryable: false,
		})
		return err
	}
	if err != nil {
		return err
	}

	if scoringType != 1 {
		rlog.Info(fmt.Sprintf("Event %s is not points-based (scoringType=%d). Rejecting job %s.", jobEventID, scoringType, jobID))
		return q().SupersedeTaskWithError(ctx, sqlc.SupersedeTaskWithErrorParams{
			LastError:   sql.NullString{String: fmt.Sprintf("Rejected: event is not points-based (scoringType=%d)", scoringType), Valid: true},
			CompletedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
			ID:          jobID,
		})
	}

	members, err := q().ListEventMemberUserIDs(ctx, jobEventID)
	if err != nil {
		return err
	}

	resultRows, err := q().ListRaceResultInputs(ctx, jobEventID)
	if err != nil {
		return err
	}
	results := make([]scorecalc.RaceResultInput, 0, len(resultRows))
	for _, r := range resultRows {
		results = append(results, scorecalc.RaceResultInput{UserID: r.UserId, Points: int(r.Points)})
	}

	projection := scorecalc.CalculateEventProjection(scorecalc.EventScoreInput{
		EventID: jobEventID, Members: members, RaceResults: results,
	})
	checksum := scorecalc.ComputeChecksum(projection.Entries)

	rlog.Info(fmt.Sprintf("Calculation completed for Job %s, publishing results. Checksum: %s", jobID, checksum))
	_, err = scorecalc.ScoreCalcCompletedTopic.Publish(ctx, scorecalc.ScoreCalcCompleted{
		Version: 1, JobID: jobID, EventID: jobEventID, Generation: jobGeneration,
		ComputedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Result: projection, ResultChecksum: checksum,
	})
	return err
}
