package eventmanager

import (
	"context"
	"testing"

	"encore.dev/beta/errs"
)

// Ports of the race/results/datasets DB-backed legacy tests:
//
//	ts-legacy/eventmanager/granular_results.test.ts
//	ts-legacy/eventmanager/auto_deferral.test.ts
//	ts-legacy/eventmanager/datasets.test.ts
//
// (ts-legacy/eventmanager/raceevents.test.ts and results.test.ts are static
// RBAC-matrix assertions already covered by auth/permissions_test.go.)

// --- granular_results.test.ts ---

func Test_GranularResultsGating(t *testing.T) {
	f := newFixtures(t)
	ctx := context.Background()

	admin := f.createUser("gradmin", "Granular Admin", nil, "SITE_ADMIN")
	authHeader := f.createSession(admin)
	p1 := f.createUser("grpart1", "Participant One", nil, "USER")
	p2 := f.createUser("grpart2", "Participant Two", nil, "USER")

	eventID := f.createEventDirect(admin, "Granular Event", "UNOFFICIAL", nil, true)

	race, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 1",
		DistanceMeters: 1000, TrackType: "Dirt", Location: "Tokyo", Grade: strptr("OP"),
	})
	if err != nil {
		t.Fatalf("CreateRaceEventCore: %v", err)
	}

	for _, uid := range []string{p1, p2} {
		if _, err := AddEventMemberCore(ctx, &AddEventMemberRequest{
			EventID: eventID, UserID: uid, Authorization: authHeader,
		}); err != nil {
			t.Fatalf("AddEventMemberCore(%s): %v", uid, err)
		}
	}
	if _, err := AddRaceEventMemberCore(ctx, &RaceMemberRequest{
		EventID: eventID, RaceID: race.ID, UserID: p1, Authorization: authHeader,
	}); err != nil {
		t.Fatalf("AddRaceEventMemberCore: %v", err)
	}

	// Race member result assignment succeeds; points resolve via the table.
	r1, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race.ID, UserID: p1, Authorization: authHeader,
		Position: intptr(2), Points: intptr(10),
	})
	if err != nil {
		t.Fatalf("AssignRaceResultCore(p1): %v", err)
	}
	if r1.UserID != p1 {
		t.Errorf("userId = %q, want %q", r1.UserID, p1)
	}
	if r1.Points != 10 {
		t.Errorf("points = %d, want 10 (OP table, position 2)", r1.Points)
	}

	// Event-only member without race registration is rejected.
	if _, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race.ID, UserID: p2, Authorization: authHeader,
		Points: intptr(5),
	}); err == nil {
		t.Error("assign to non-race member should fail, got nil")
	}

	// Bulk replace including the unregistered user fails.
	if _, err := ReplaceRaceResultsCore(ctx, &BulkResultsRequest{
		EventID: eventID, RaceID: race.ID, Authorization: authHeader,
		Results: []*RaceResultInput{{UserID: p1, Points: intptr(12)}, {UserID: p2, Points: intptr(8)}},
	}); err == nil {
		t.Error("replace including non-race member should fail, got nil")
	}

	// Merge including the unregistered user fails.
	if _, err := MergeRaceResultsCore(ctx, &BulkResultsRequest{
		EventID: eventID, RaceID: race.ID, Authorization: authHeader,
		Results: []*RaceResultInput{{UserID: p2, Points: intptr(8)}},
	}); err == nil {
		t.Error("merge including non-race member should fail, got nil")
	}
}

func Test_NonGranularFallsBackToEventMembership(t *testing.T) {
	f := newFixtures(t)
	ctx := context.Background()

	admin := f.createUser("ngadmin", "Non-Granular Admin", nil, "SITE_ADMIN")
	authHeader := f.createSession(admin)
	p1 := f.createUser("ngpart1", "Participant One", nil, "USER")
	p2 := f.createUser("ngpart2", "Participant Two", nil, "USER")

	eventID := f.createEventDirect(admin, "Non-Granular Event", "UNOFFICIAL", nil, false)

	race, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 1",
		DistanceMeters: 1000, TrackType: "Dirt", Location: "Tokyo", Grade: strptr("OP"),
	})
	if err != nil {
		t.Fatalf("CreateRaceEventCore: %v", err)
	}
	if _, err := AddEventMemberCore(ctx, &AddEventMemberRequest{
		EventID: eventID, UserID: p1, Authorization: authHeader,
	}); err != nil {
		t.Fatalf("AddEventMemberCore: %v", err)
	}

	r1, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race.ID, UserID: p1, Authorization: authHeader,
		Position: intptr(2), Points: intptr(10),
	})
	if err != nil {
		t.Fatalf("AssignRaceResultCore(p1): %v", err)
	}
	if r1.UserID != p1 {
		t.Errorf("userId = %q, want %q", r1.UserID, p1)
	}

	if _, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race.ID, UserID: p2, Authorization: authHeader,
		Points: intptr(5),
	}); err == nil {
		t.Error("assign to non-member should fail, got nil")
	}
}

// --- auto_deferral.test.ts ---

func Test_AutoDeferralOnOPWin(t *testing.T) {
	f := newFixtures(t)
	ctx := context.Background()

	admin := f.createUser("adadmin", "Defer Admin", nil, "SITE_ADMIN")
	authHeader := f.createSession(admin)
	u1 := f.createUser("adungraded", "Ungraded Competitor", nil, "USER")
	u2 := f.createUser("adgraded", "Graded Competitor", strptr("G1"), "USER")

	eventID := f.createEventDirect(admin, "Auto Defer Test Event", "UNOFFICIAL", nil, false)
	f.addEventMemberDirect(eventID, u1)
	f.addEventMemberDirect(eventID, u2)

	race1, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 1 OP",
		DistanceMeters: 1200, TrackType: "Turf", Location: "Kyoto", Grade: strptr("OP"),
	})
	if err != nil {
		t.Fatalf("create race1: %v", err)
	}
	race2, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 2 GIII",
		DistanceMeters: 1600, TrackType: "Dirt", Location: "Tokyo", Grade: strptr("GIII"),
	})
	if err != nil {
		t.Fatalf("create race2: %v", err)
	}

	// Ungraded user wins the OP race.
	if _, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race1.ID, UserID: u1, Authorization: authHeader,
		Position: intptr(1),
	}); err != nil {
		t.Fatalf("assign win: %v", err)
	}

	r2, err := ListRaceResultsCore(ctx, &RaceResultsQuery{EventID: eventID, RaceID: race2.ID})
	if err != nil {
		t.Fatalf("list race2: %v", err)
	}
	u1r2 := findResult(r2.Results, u1)
	if u1r2 == nil {
		t.Fatalf("expected auto-created DEFERRED row for u1 in race2")
	}
	if u1r2.ResultStatus == nil || *u1r2.ResultStatus != "DEFERRED" {
		t.Errorf("race2 status = %v, want DEFERRED", u1r2.ResultStatus)
	}
	if u1r2.Points != 0 {
		t.Errorf("race2 points = %d, want 0", u1r2.Points)
	}

	// Graded user winning the same race does not disturb u1's deferral.
	if _, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race1.ID, UserID: u2, Authorization: authHeader,
		Position: intptr(1),
	}); err != nil {
		t.Fatalf("assign u2 win: %v", err)
	}

	// Removing u1's win reverts the deferral to null.
	if _, err := DeleteRaceResultCore(ctx, &DeleteRaceResultRequest{
		EventID: eventID, RaceID: race1.ID, UserID: u1, Authorization: authHeader,
	}); err != nil {
		t.Fatalf("delete win: %v", err)
	}
	r2after, err := ListRaceResultsCore(ctx, &RaceResultsQuery{EventID: eventID, RaceID: race2.ID})
	if err != nil {
		t.Fatalf("list race2 after: %v", err)
	}
	u1after := findResult(r2after.Results, u1)
	if u1after == nil {
		t.Fatalf("expected u1 row to persist in race2 after revert")
	}
	if u1after.ResultStatus != nil {
		t.Errorf("race2 status after revert = %q, want null", *u1after.ResultStatus)
	}
}

func Test_AutoDeferralCustomTables(t *testing.T) {
	f := newFixtures(t)
	ctx := context.Background()

	admin := f.createUser("cdadmin", "Custom Defer Admin", nil, "SITE_ADMIN")
	authHeader := f.createSession(admin)
	u1 := f.createUser("cdungraded", "Ungraded Custom", nil, "USER")

	eventID := f.createEventDirect(admin, "Custom Auto Defer Event", "UNOFFICIAL", nil, false)
	customTables := `{"autoDeferEnabled":true,` +
		`"OP":{"1":12,"2":10,"3":8,"4":7,"5":6,"6":5,"7":4,"8":3,"9":2,"10":1,"autoDefer":false},` +
		`"GIII":{"1":15,"2":12,"3":10,"4":8,"5":6,"6":5,"7":4,"8":3,"9":2,"10":1,"autoDefer":true},` +
		`"GII":{"1":19,"2":15,"3":12,"4":9,"5":8,"6":6,"7":5,"8":3,"9":2,"10":1,"autoDefer":false},` +
		`"GI":{"1":25,"2":18,"3":15,"4":12,"5":10,"6":8,"7":6,"8":4,"9":2,"10":1,"autoDefer":false}}`
	if _, err := db.Exec(ctx,
		`UPDATE "event" SET "scoringRulesMode"='CUSTOM', "customScoringTables"=$1 WHERE id=$2`,
		[]byte(customTables), eventID); err != nil {
		t.Fatalf("set custom tables: %v", err)
	}
	f.addEventMemberDirect(eventID, u1)

	race1, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 1 GIII",
		DistanceMeters: 1200, TrackType: "Turf", Location: "Kyoto", Grade: strptr("GIII"),
	})
	if err != nil {
		t.Fatalf("create race1: %v", err)
	}
	race2, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 2 OP",
		DistanceMeters: 1600, TrackType: "Dirt", Location: "Tokyo", Grade: strptr("OP"),
	})
	if err != nil {
		t.Fatalf("create race2: %v", err)
	}

	// OP win on Race 2 must NOT defer (custom OP autoDefer=false).
	if _, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race2.ID, UserID: u1, Authorization: authHeader,
		Position: intptr(1),
	}); err != nil {
		t.Fatalf("assign OP win: %v", err)
	}
	r1, err := ListRaceResultsCore(ctx, &RaceResultsQuery{EventID: eventID, RaceID: race1.ID})
	if err != nil {
		t.Fatalf("list race1: %v", err)
	}
	if findResult(r1.Results, u1) != nil {
		t.Error("OP win should not create a race1 row under custom tables")
	}

	// GIII win on Race 1 MUST defer race2 (custom GIII autoDefer=true, seq 1 < seq 2).
	if _, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race1.ID, UserID: u1, Authorization: authHeader,
		Position: intptr(1),
	}); err != nil {
		t.Fatalf("assign GIII win: %v", err)
	}
	r2, err := ListRaceResultsCore(ctx, &RaceResultsQuery{EventID: eventID, RaceID: race2.ID})
	if err != nil {
		t.Fatalf("list race2: %v", err)
	}
	u1r2 := findResult(r2.Results, u1)
	if u1r2 == nil || u1r2.ResultStatus == nil || *u1r2.ResultStatus != "DEFERRED" {
		t.Errorf("race2 status = %v, want DEFERRED", u1r2)
	}
}

func Test_AutoDeferralRespectsSequenceOrdering(t *testing.T) {
	f := newFixtures(t)
	ctx := context.Background()

	admin := f.createUser("seqadmin", "Seq Admin", nil, "SITE_ADMIN")
	authHeader := f.createSession(admin)
	u1 := f.createUser("sequngraded", "Ungraded Competitor", nil, "USER")

	eventID := f.createEventDirect(admin, "Sequence Auto Defer Event", "UNOFFICIAL", nil, false)
	f.addEventMemberDirect(eventID, u1)

	race1, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 1 OP", Sequence: intptr(1),
		DistanceMeters: 1200, TrackType: "Turf", Location: "Kyoto", Grade: strptr("OP"),
	})
	if err != nil {
		t.Fatalf("create race1: %v", err)
	}
	race2, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 2 OP", Sequence: intptr(2),
		DistanceMeters: 1400, TrackType: "Turf", Location: "Hanshin", Grade: strptr("OP"),
	})
	if err != nil {
		t.Fatalf("create race2: %v", err)
	}
	race3, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race 3 GIII", Sequence: intptr(3),
		DistanceMeters: 1600, TrackType: "Dirt", Location: "Tokyo", Grade: strptr("GIII"),
	})
	if err != nil {
		t.Fatalf("create race3: %v", err)
	}

	// User finished 2nd in Race 1
	if _, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race1.ID, UserID: u1, Authorization: authHeader,
		Position: intptr(2),
	}); err != nil {
		t.Fatalf("assign race1 pos 2: %v", err)
	}

	// User wins Race 2 (OP grade win)
	if _, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race2.ID, UserID: u1, Authorization: authHeader,
		Position: intptr(1),
	}); err != nil {
		t.Fatalf("assign race2 win: %v", err)
	}

	// Race 1 (sequence 1) result must remain intact (Position 2, points 10) and NOT be converted to DEFERRED!
	r1, err := ListRaceResultsCore(ctx, &RaceResultsQuery{EventID: eventID, RaceID: race1.ID})
	if err != nil {
		t.Fatalf("list race1: %v", err)
	}
	u1r1 := findResult(r1.Results, u1)
	if u1r1 == nil {
		t.Fatalf("expected result in race1")
	}
	if u1r1.ResultStatus != nil && *u1r1.ResultStatus == "DEFERRED" {
		t.Errorf("race1 (earlier sequence) status = DEFERRED, expected prior result intact")
	}
	if u1r1.Position == nil || *u1r1.Position != 2 {
		t.Errorf("race1 position = %v, want 2", u1r1.Position)
	}
	if u1r1.Points != 10 {
		t.Errorf("race1 points = %d, want 10", u1r1.Points)
	}

	// Race 3 (sequence 3, strictly after Race 2 win) MUST be auto-deferred!
	r3, err := ListRaceResultsCore(ctx, &RaceResultsQuery{EventID: eventID, RaceID: race3.ID})
	if err != nil {
		t.Fatalf("list race3: %v", err)
	}
	u1r3 := findResult(r3.Results, u1)
	if u1r3 == nil || u1r3.ResultStatus == nil || *u1r3.ResultStatus != "DEFERRED" {
		t.Errorf("race3 status = %v, want DEFERRED", u1r3)
	}
}

func Test_ManualDeferredStatus(t *testing.T) {
	f := newFixtures(t)
	ctx := context.Background()

	admin := f.createUser("mdadmin", "Manual Defer Admin", nil, "SITE_ADMIN")
	authHeader := f.createSession(admin)
	u1 := f.createUser("mdcomp", "Manual Defer Competitor", nil, "USER")

	eventID := f.createEventDirect(admin, "Manual Defer Event", "UNOFFICIAL", nil, false)
	f.addEventMemberDirect(eventID, u1)

	race, err := CreateRaceEventCore(ctx, &CreateRaceEventRequest{
		EventID: eventID, Authorization: authHeader, Name: "Race GIII",
		DistanceMeters: 1600, TrackType: "Turf", Location: "Kyoto", Grade: strptr("GIII"),
	})
	if err != nil {
		t.Fatalf("create race: %v", err)
	}
	res, err := AssignRaceResultCore(ctx, &AssignRaceResultRequest{
		EventID: eventID, RaceID: race.ID, UserID: u1, Authorization: authHeader,
		Position: intptr(5), ResultStatus: strptr("DEFERRED"),
	})
	if err != nil {
		t.Fatalf("assign DEFERRED: %v", err)
	}
	if res.ResultStatus == nil || *res.ResultStatus != "DEFERRED" {
		t.Errorf("status = %v, want DEFERRED", res.ResultStatus)
	}
	if res.Points != 0 {
		t.Errorf("points = %d, want 0", res.Points)
	}
	fetched, err := ListRaceResultsCore(ctx, &RaceResultsQuery{EventID: eventID, RaceID: race.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	u1res := findResult(fetched.Results, u1)
	if u1res == nil || u1res.ResultStatus == nil || *u1res.ResultStatus != "DEFERRED" || u1res.Points != 0 {
		t.Errorf("persisted row = %+v, want DEFERRED/0", u1res)
	}
}

// --- datasets.test.ts ---

func Test_DatasetCreateListUpdate(t *testing.T) {
	f := newFixtures(t)
	ctx := context.Background()

	owner := f.createUser("dsowner", "Dataset Owner", nil, "SITE_ADMIN")
	authHeader := f.createSession(owner)
	eventID := f.createEventDirect(owner, "Main Championship", "UNOFFICIAL", nil, false)

	created, err := CreateDatasetCore(ctx, &CreateDatasetRequest{
		EventID: eventID, Authorization: authHeader,
		Source: "results_test.csv", Rows: 150, Status: strptr("PENDING"),
	})
	if err != nil {
		t.Fatalf("CreateDatasetCore: %v", err)
	}
	if created.EventID != eventID || created.Source != "results_test.csv" ||
		created.Rows != 150 || created.Status != "PENDING" {
		t.Errorf("dataset = %+v, want matching fields", created)
	}

	listed, err := ListDatasetsCore(ctx, &DatasetsQuery{EventID: eventID})
	if err != nil {
		t.Fatalf("ListDatasetsCore: %v", err)
	}
	if len(listed.Datasets) != 1 || listed.Datasets[0].ID != created.ID {
		t.Errorf("list = %+v, want the created dataset", listed.Datasets)
	}

	updated, err := UpdateDatasetStatusCore(ctx, &UpdateDatasetStatusRequest{
		EventID: eventID, DatasetID: created.ID, Authorization: authHeader, Status: "DONE",
	})
	if err != nil {
		t.Fatalf("UpdateDatasetStatusCore: %v", err)
	}
	if updated.Status != "DONE" || updated.ImportedAt == nil {
		t.Errorf("updated = %+v, want DONE with importedAt", updated)
	}
	listed2, err := ListDatasetsCore(ctx, &DatasetsQuery{EventID: eventID})
	if err != nil {
		t.Fatalf("re-list: %v", err)
	}
	if listed2.Datasets[0].Status != "DONE" {
		t.Errorf("re-list status = %q, want DONE", listed2.Datasets[0].Status)
	}
}

func Test_DatasetCreateRejectsMissingEvent(t *testing.T) {
	f := newFixtures(t)
	ctx := context.Background()

	user := f.createUser("dsuser", "Regular User", nil, "USER")
	authHeader := f.createSession(user)

	_, err := CreateDatasetCore(ctx, &CreateDatasetRequest{
		EventID: "non-existent-event", Authorization: authHeader,
		Source: "test.csv", Rows: 10,
	})
	if errs.Code(err) != errs.NotFound {
		t.Errorf("code = %v, want not_found (err: %v)", errs.Code(err), err)
	}
}

// addEventMemberDirect inserts an event membership row without the API.
func (f *fixtures) addEventMemberDirect(eventID, userID string) {
	f.t.Helper()
	if _, err := db.Exec(f.ctx,
		`INSERT INTO "event_member" (id, "eventId", "userId") VALUES ($1,$2,$3)`,
		"eventmember-"+newID()[:8], eventID, userID); err != nil {
		f.t.Fatalf("insert event member: %v", err)
	}
}

func findResult(results []*RaceResultView, userID string) *RaceResultView {
	for _, r := range results {
		if r.UserID == userID {
			return r
		}
	}
	return nil
}
