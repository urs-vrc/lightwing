import { describe, expect, test } from "vitest";
import { inferFinishTimes, parseMarginToSeconds, type DerivedRow, type EditedResult, EMPTY_EDIT } from "./standings";

function makeMockRow(
  userId: string,
  position: string,
  finishTime: string,
  margin: string,
  rowState: "unchanged" | "new" | "modified" | "pending_delete" = "unchanged"
): DerivedRow {
  return {
    member: {
      userId,
      name: "User_" + userId,
      classTier: null as unknown as string,
    },
    savedResult: undefined,
    edit: {
      ...EMPTY_EDIT,
      position,
      finishTime,
      margin,
    },
    rowState,
  };
}

describe("inferFinishTimes", () => {
  test("standard contiguous positions with cumulative margins", () => {
    // Cumulative (netkeiba scale):
    // pos 1: 1:30.0
    // pos 2: 1:30.0 + 1 1/2 lengths (0.25s) = 1:30.25 -> formatted 1:30.3
    // pos 3: 1:30.25 + nose (0s) = 1:30.3
    const rows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "1 1/2"),
      makeMockRow("user3", "3", "", "nose"),
    ];

    const editedResults: Record<string, EditedResult> = {
      user1: rows[0].edit,
      user2: rows[1].edit,
      user3: rows[2].edit,
    };

    const result = inferFinishTimes(rows, editedResults);

    expect("error" in result).toBe(false);
    if (!("error" in result)) {
      expect(result.inferredCount).toBe(2);
      // user2: 1:30.0 + 0.25 = 1:30.25 -> formatted 1:30.3
      expect(result.edits.user2.finishTime).toBe("1:30.3");
      // user3: 1:30.25 + 0 = 1:30.25 -> formatted 1:30.3
      expect(result.edits.user3.finishTime).toBe("1:30.3");
    }
  });

  test("more distinct cumulative math", () => {
    // Cumulative (netkeiba scale):
    // pos 1: 1:20.0 (80.0s)
    // pos 2: + 2 lengths (0.3s) = 1:20.3
    // pos 3: + 3 lengths (0.5s) = 1:20.8
    const rows = [
      makeMockRow("user1", "1", "1:20.0", ""),
      makeMockRow("user2", "2", "", "2"),
      makeMockRow("user3", "3", "", "3"),
    ];

    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
      user3: rows[2].edit,
    });

    expect("error" in result).toBe(false);
    if (!("error" in result)) {
      expect(result.inferredCount).toBe(2);
      expect(result.edits.user2.finishTime).toBe("1:20.3");
      expect(result.edits.user3.finishTime).toBe("1:20.8");
    }
  });

  test("repeated inference: rerun after changing leader time recalculates downstream cumulatively", () => {
    const rows = [
      makeMockRow("user1", "1", "1:32.0", ""), // Updated leader time
      makeMockRow("user2", "2", "1:30.8", "1 1/2"),
      makeMockRow("user3", "3", "1:30.8", "nose"),
    ];

    const editedResults: Record<string, EditedResult> = {
      user1: rows[0].edit,
      user2: rows[1].edit,
      user3: rows[2].edit,
    };

    const result = inferFinishTimes(rows, editedResults);

    expect("error" in result).toBe(false);
    if (!("error" in result)) {
      expect(result.inferredCount).toBe(2);
      expect(result.edits.user2.finishTime).toBe("1:32.3");
      expect(result.edits.user3.finishTime).toBe("1:32.3");
    }
  });

  test("repeated inference: rerun after changing upstream margin recalculates downstream cumulatively", () => {
    const rows = [
      makeMockRow("user1", "1", "1:20.0", ""),
      makeMockRow("user2", "2", "1:21.0", "4"), // Changed margin from 2 to 4 lengths (0.7s)
      makeMockRow("user3", "3", "1:22.5", "3"),
    ];

    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
      user3: rows[2].edit,
    });

    expect("error" in result).toBe(false);
    if (!("error" in result)) {
      expect(result.inferredCount).toBe(2);
      expect(result.edits.user2.finishTime).toBe("1:20.7");
      expect(result.edits.user3.finishTime).toBe("1:21.2");
    }
  });

  test("error on missing position 1 row", () => {
    const rows = [
      makeMockRow("user2", "2", "", "1"),
      makeMockRow("user3", "3", "", "1"),
    ];
    const result = inferFinishTimes(rows, {
      user2: rows[0].edit,
      user3: rows[1].edit,
    });
    expect("error" in result).toBe(true);
    if ("error" in result) {
      expect(result.error).toContain("Missing position 1 in the standings sequence. Infer time requires contiguous official finishing positions starting from 1.");
    }
  });

  test("error on gap in sequence", () => {
    const rows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "1"),
      makeMockRow("user4", "4", "", "1"),
    ];
    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
      user4: rows[2].edit,
    });
    expect("error" in result).toBe(true);
    if ("error" in result) {
      expect(result.error).toContain("Missing position 3 in the standings sequence. Infer time requires contiguous official finishing positions starting from 1.");
    }
  });

  test("error on duplicate position", () => {
    const rows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "1"),
      makeMockRow("user3", "2", "", "1"),
    ];
    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
      user3: rows[2].edit,
    });
    expect("error" in result).toBe(true);
    if ("error" in result) {
      expect(result.error).toBe("Duplicate position 2 detected in standings.");
    }
  });

  test("error on non-numeric position", () => {
    const rows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2a", "", "1"),
    ];
    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
    });
    expect("error" in result).toBe(true);
    if ("error" in result) {
      expect(result.error).toBe("All rows participating in infer time must have numeric finishing positions.");
    }
  });

  test("error on blank position", () => {
    const rows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "", "", "1"),
    ];
    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
    });
    expect("error" in result).toBe(true);
    if ("error" in result) {
      expect(result.error).toBe("All rows participating in infer time must have numeric finishing positions.");
    }
  });

  test("error on position 1 missing leader finish time", () => {
    const rows = [
      makeMockRow("user1", "1", "", ""),
      makeMockRow("user2", "2", "", "1"),
    ];
    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
    });
    expect("error" in result).toBe(true);
    if ("error" in result) {
      expect(result.error).toBe("Unable to parse the leader finish time. Use m:ss.t format (e.g. 1:32.1).");
    }
  });

  test("error on blank or zero margin in downstream rows", () => {
    const blankRows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", ""),
    ];
    const resultBlank = inferFinishTimes(blankRows, {
      user1: blankRows[0].edit,
      user2: blankRows[1].edit,
    });
    expect("error" in resultBlank).toBe(true);
    if ("error" in resultBlank) {
      expect(resultBlank.error).toBe("Position 2 has an empty or zero margin, which blocks cumulative inference.");
    }

    const zeroRows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "0"),
    ];
    const resultZero = inferFinishTimes(zeroRows, {
      user1: zeroRows[0].edit,
      user2: zeroRows[1].edit,
    });
    expect("error" in resultZero).toBe(true);
    if ("error" in resultZero) {
      expect(resultZero.error).toBe("Position 2 has an empty or zero margin, which blocks cumulative inference.");
    }

    const dashRows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "—"),
    ];
    const resultDash = inferFinishTimes(dashRows, {
      user1: dashRows[0].edit,
      user2: dashRows[1].edit,
    });
    expect("error" in resultDash).toBe(true);
    if ("error" in resultDash) {
      expect(resultDash.error).toBe("Position 2 has an empty or zero margin, which blocks cumulative inference.");
    }
  });

  test("error on unrecognized margin text, but nose/head infer the same time", () => {
    const garbageRows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "foo"),
    ];
    const resultGarbage = inferFinishTimes(garbageRows, {
      user1: garbageRows[0].edit,
      user2: garbageRows[1].edit,
    });
    expect("error" in resultGarbage).toBe(true);
    if ("error" in resultGarbage) {
      expect(resultGarbage.error).toBe("Position 2 has an empty or zero margin, which blocks cumulative inference.");
    }

    const deadHeatRows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "nose"),
      makeMockRow("user3", "3", "", "head"),
    ];
    const resultDeadHeat = inferFinishTimes(deadHeatRows, {
      user1: deadHeatRows[0].edit,
      user2: deadHeatRows[1].edit,
      user3: deadHeatRows[2].edit,
    });
    expect("error" in resultDeadHeat).toBe(false);
    if (!("error" in resultDeadHeat)) {
      expect(resultDeadHeat.inferredCount).toBe(2);
      expect(resultDeadHeat.edits.user2.finishTime).toBe("1:30.0");
      expect(resultDeadHeat.edits.user3.finishTime).toBe("1:30.0");
    }
  });

  test("rows with pending_delete are completely ignored", () => {
    const rows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "1"),
      makeMockRow("user3", "3", "", "1", "pending_delete"), // ignored, sequence is still contiguous from 1 to 2!
    ];

    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
      user3: rows[2].edit,
    });

    expect("error" in result).toBe(false);
    if (!("error" in result)) {
      expect(result.inferredCount).toBe(1);
      expect(result.edits.user2.finishTime).toBe("1:30.2");
      expect(result.edits.user3.finishTime).toBe("");
    }
  });

  test("DSQ, DNF, and DNS rows are excluded from time inference", () => {
    const rows = [
      makeMockRow("user1", "1", "1:30.0", ""),
      makeMockRow("user2", "2", "", "1"),
      makeMockRow("user3", "3", "", "1"),
      makeMockRow("user5", "4", "", "1"),
      makeMockRow("user4", "2", "", "3"),
    ];
    // Override edits with resultStatus
    rows[1].edit = { ...rows[1].edit, resultStatus: "DSQ" };
    rows[2].edit = { ...rows[2].edit, resultStatus: "DNF" };
    rows[3].edit = { ...rows[3].edit, resultStatus: "DNS" };

    const result = inferFinishTimes(rows, {
      user1: rows[0].edit,
      user2: rows[1].edit,
      user3: rows[2].edit,
      user5: rows[3].edit,
      user4: rows[4].edit,
    });

    // DSQ, DNF, and DNS rows excluded — sequence is now position 1 (user1) and 2 (user4)
    expect("error" in result).toBe(false);
    if (!("error" in result)) {
      expect(result.inferredCount).toBe(1);
      // user4: 1:30.0 + 3 lengths (0.5s) = 1:30.5
      expect(result.edits.user4.finishTime).toBe("1:30.5");
      // DSQ/DNF/DNS rows were not modified
      expect(result.edits.user2.finishTime).toBe("");
      expect(result.edits.user3.finishTime).toBe("");
      expect(result.edits.user5.finishTime).toBe("");
    }
  });
});

describe("parseMarginToSeconds (netkeiba scale)", () => {
  test("zero-equivalent inputs", () => {
    expect(parseMarginToSeconds("")).toBe(0);
    expect(parseMarginToSeconds("—")).toBe(0);
    expect(parseMarginToSeconds("-")).toBe(0);
    expect(parseMarginToSeconds("0")).toBe(0);
  });

  test("nose and head carry no time difference", () => {
    expect(parseMarginToSeconds("nose")).toBe(0);
    expect(parseMarginToSeconds("Nose")).toBe(0);
    expect(parseMarginToSeconds("ハナ")).toBe(0);
    expect(parseMarginToSeconds("head")).toBe(0);
    expect(parseMarginToSeconds("アタマ")).toBe(0);
    expect(parseMarginToSeconds("dead heat")).toBe(0);
    expect(parseMarginToSeconds("同着")).toBe(0);
  });

  test("neck is distinct from a half length", () => {
    expect(parseMarginToSeconds("neck")).toBeCloseTo(0.05, 9);
    expect(parseMarginToSeconds("クビ")).toBeCloseTo(0.05, 9);
    expect(parseMarginToSeconds("1/2")).toBeCloseTo(0.1, 9);
  });

  test("official length anchors", () => {
    const cases: Array<[string, number]> = [
      ["3/4", 0.15],
      ["1", 0.2],
      ["1 length", 0.2],
      ["length", 0.2],
      ["2馬身", 0.3],
      ["1 1/4", 0.2],
      ["1 1/2", 0.25],
      ["1 3/4", 0.3],
      ["2", 0.3],
      ["2 1/2", 0.4],
      ["3", 0.5],
      ["3 1/2", 0.6],
      ["4", 0.7],
      ["5", 0.85],
      ["6", 1.0],
      ["7", 1.15],
      ["8", 1.3],
      ["9", 1.45],
      ["10", 1.6],
      ["大差", 1.7],
    ];
    for (const [input, want] of cases) {
      expect(parseMarginToSeconds(input), input).toBeCloseTo(want, 9);
    }
  });
});
