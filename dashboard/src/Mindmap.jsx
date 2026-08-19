import { useEffect, useRef, useState } from "react";

// Fixed canvas layout (viewBox units). Everything is positioned by hand to
// get the swoopy flowchart-style curves from the sketch — no layout lib.
// SLOT_COUNT is fixed at 7 for now while this view is being tuned against a
// wide showcase run; nothing here assumes it has to stay 7.
const SLOT_COUNT = 7;

const VB_W = 1280;
const VB_H = 640;

const SLOT_W = 168;
const SLOT_H = 62;
const SLOT_X = 250; // left edge
const SLOT_TOP = 40;
const SLOT_BOTTOM = VB_H - 40;
const SLOT_YS = Array.from({ length: SLOT_COUNT }, (_, i) =>
  SLOT_COUNT === 1 ? VB_H / 2 : SLOT_TOP + (i * (SLOT_BOTTOM - SLOT_TOP)) / (SLOT_COUNT - 1)
);

const HUB = { x: 70, y: VB_H / 2, r: 16 };
const FIX = { x: 660, y: VB_H / 2 - 35, w: 140, h: 70 };
// Sized generously (and left un-clamped in CSS) so the full report text
// shows — the "keep it short" instruction in the AI prompt is what actually
// keeps this from overflowing, not truncation here.
const SUMMARY = { x: 860, y: VB_H / 2 - 120, w: 300, h: 240 };

const STATUS_COLOR = {
  passed: "#22c55e",
  failed: "#ef4444",
  healed: "#3b82f6",
};

// Choreography timing: one run's worth of completed events (up to SLOT_COUNT,
// closed off by the backend's RunBoundary signal — see the runEnd effect
// below) is buffered, then replayed on a fixed timeline instead of popping in
// all at once. Stage 1: the slot column fills in, one cell per second. Stage
// 2: once the whole column has landed, every cell fires its own arrow into
// Fixes. Stage 3: once those arrows land, the AI Summary node waits for (or,
// if it's already arrived, immediately shows) this run's LLM-written report.
const REVEAL_GAP_MS = 1000; // gap between each slot appearing
const SETTLE_MS = 400; // pause after the last slot before the fixes arrows start
const FIX_EDGE_GAP_MS = 80; // stagger between each slot's arrow into fixes
const FIX_SETTLE_MS = 500; // pause after the last fixes arrow before summary can land
const MIN_SUMMARY_READ_MS = 6000; // once the AI report is actually showing, guarantee it stays up this long before the next run can clear it

// Cubic bezier with horizontal tangents at both ends — the "S-curve" look.
function swoopPath(x1, y1, x2, y2) {
  const dx = (x2 - x1) * 0.5;
  return `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`;
}

export default function Mindmap({ connected, current, trail, counts, report, runEnd }) {
  const [visibleSlots, setVisibleSlots] = useState(() => Array(SLOT_COUNT).fill(null));
  const [fixesArmed, setFixesArmed] = useState(false); // Fixes node has appeared
  const [fixEdgeCount, setFixEdgeCount] = useState(0); // how many slot->fixes arrows are lit, in order
  const [summaryStage, setSummaryStage] = useState("hidden"); // "hidden" | "waiting" | "shown"
  const [summaryEvent, setSummaryEvent] = useState(null);

  const bufferRef = useRef([]); // events collected for the run currently being buffered
  const timersRef = useRef([]);
  const seenIdRef = useRef(null); // last trail id already buffered
  const seenRunEndIdRef = useRef(null); // last run_end id already consumed
  const stageRef = useRef("hidden"); // mirrors summaryStage for synchronous reads in timers/effects
  const pendingReportRef = useRef(null); // latest report not yet consumed
  const consumedReportIdRef = useRef(null);
  const summaryShownAtRef = useRef(0); // when the current summary actually became visible
  const holdTimerRef = useRef(null); // pending "start the next batch" timer, while holding for read time

  const clearTimers = () => {
    timersRef.current.forEach(clearTimeout);
    timersRef.current = [];
  };

  const setStage = (s) => {
    stageRef.current = s;
    setSummaryStage(s);
  };

  // Shows the report if one's already arrived for this batch; otherwise
  // parks in "waiting" until the report effect below catches up.
  const revealSummaryIfReady = () => {
    const rpt = pendingReportRef.current;
    if (rpt && rpt.id !== consumedReportIdRef.current) {
      consumedReportIdRef.current = rpt.id;
      setSummaryEvent(rpt);
      setStage("shown");
      summaryShownAtRef.current = Date.now();
    } else if (stageRef.current !== "shown") {
      setStage("waiting");
    }
  };

  // Runs a batch now, unless a summary is currently on screen and hasn't
  // been up for MIN_SUMMARY_READ_MS yet — in that case, wait out the rest of
  // the read window first. A later call while already holding just replaces
  // what's queued, so only the freshest batch ever plays.
  const scheduleBatch = (batch) => {
    clearTimeout(holdTimerRef.current);
    const readyAt = stageRef.current === "shown" ? summaryShownAtRef.current + MIN_SUMMARY_READ_MS : 0;
    const wait = readyAt - Date.now();
    if (wait <= 0) {
      playBatch(batch);
    } else {
      holdTimerRef.current = setTimeout(() => playBatch(batch), wait);
    }
  };

  const resetVisuals = () => {
    setVisibleSlots(Array(SLOT_COUNT).fill(null));
    setFixesArmed(false);
    setFixEdgeCount(0);
    setSummaryEvent(null);
    setStage("hidden");
  };

  const playBatch = (batch) => {
    clearTimers();
    resetVisuals();

    batch.forEach((ev, i) => {
      timersRef.current.push(
        setTimeout(() => {
          setVisibleSlots((prev) => {
            const next = [...prev];
            next[i] = ev;
            return next;
          });
        }, i * REVEAL_GAP_MS)
      );
    });

    const slotsDoneAt = (batch.length - 1) * REVEAL_GAP_MS + SETTLE_MS;
    timersRef.current.push(setTimeout(() => setFixesArmed(true), slotsDoneAt));
    batch.forEach((_, i) => {
      timersRef.current.push(
        setTimeout(() => {
          setFixEdgeCount((c) => Math.max(c, i + 1));
        }, slotsDoneAt + i * FIX_EDGE_GAP_MS)
      );
    });

    const fixesDoneAt = slotsDoneAt + (batch.length - 1) * FIX_EDGE_GAP_MS + FIX_SETTLE_MS;
    timersRef.current.push(setTimeout(revealSummaryIfReady, fixesDoneAt));
  };

  // Buffer completed events as they stream in.
  useEffect(() => {
    if (trail.length === 0) return;
    const latest = trail[trail.length - 1];
    if (latest.id === seenIdRef.current) return;
    seenIdRef.current = latest.id;
    bufferRef.current = [...bufferRef.current, latest].slice(-SLOT_COUNT);
  }, [trail]);

  // The backend tells us explicitly when a run's events are all in (test
  // durations vary too much — headless Chrome cold starts, retries — to
  // infer "done" from a quiet gap in the stream). That's our cue to close the
  // buffer off and play it back as one batch.
  useEffect(() => {
    if (!runEnd || runEnd.id === seenRunEndIdRef.current) return;
    seenRunEndIdRef.current = runEnd.id;
    const batch = bufferRef.current;
    bufferRef.current = [];
    if (batch.length > 0) scheduleBatch(batch);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runEnd]);

  // A new report arrived: remember it, and if we're already sitting in
  // "waiting", reveal it immediately instead of waiting for the next batch.
  useEffect(() => {
    if (!report) return;
    pendingReportRef.current = report;
    if (stageRef.current === "waiting") revealSummaryIfReady();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [report]);

  // Fresh connection = fresh run: drop any mid-animation leftovers.
  useEffect(() => {
    if (!connected) {
      clearTimers();
      clearTimeout(holdTimerRef.current);
      bufferRef.current = [];
      seenIdRef.current = null;
      seenRunEndIdRef.current = null;
      pendingReportRef.current = null;
      consumedReportIdRef.current = null;
      summaryShownAtRef.current = 0;
      resetVisuals();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connected]);

  useEffect(() => {
    return () => {
      clearTimers();
      clearTimeout(holdTimerRef.current);
    };
  }, []);

  const isLive = current && (current.status === "running" || current.status === "retrying");

  return (
    <div className="mindmap">
      <svg viewBox={`0 0 ${VB_W} ${VB_H}`} className="mindmap-svg">
        <defs>
          <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="var(--muted)" />
          </marker>
          <marker id="arrow-hot" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#f59e0b" />
          </marker>
        </defs>

        {/* hub -> each slot: doesn't exist at all until that slot is revealed */}
        {SLOT_YS.map(
          (y, i) =>
            visibleSlots[i] && (
              <path key={"hub-" + i} d={swoopPath(HUB.x + HUB.r, HUB.y, SLOT_X, y)} className="edge edge-in" />
            )
        )}

        {/* slot -> fixes: every cell fires its own arrow, only once the whole
            column has landed. Color follows the cell's own outcome. */}
        {fixesArmed &&
          visibleSlots.map((ev, i) => {
            if (i >= fixEdgeCount) return null;
            const y = SLOT_YS[i];
            const color = (ev && STATUS_COLOR[ev.status]) || "var(--muted)";
            return (
              <path
                key={"fix-" + i}
                d={swoopPath(SLOT_X + SLOT_W, y, FIX.x, FIX.y + FIX.h / 2)}
                className="edge edge-in"
                style={{ stroke: color, opacity: 0.85 }}
                markerEnd="url(#arrow)"
              />
            );
          })}

        {/* fixes -> AI summary: dashed + pulsing while we wait on the model,
            solid once the report has actually landed */}
        {summaryStage !== "hidden" && (
          <path
            d={swoopPath(FIX.x + FIX.w, FIX.y + FIX.h / 2, SUMMARY.x, SUMMARY.y + SUMMARY.h / 2)}
            className={"edge edge-in" + (summaryStage === "shown" ? " edge-hot" : " edge-pending")}
            markerEnd={summaryStage === "shown" ? "url(#arrow-hot)" : "url(#arrow)"}
          />
        )}

        {/* hub */}
        <g className={"hub" + (connected ? " hub-live" : "")}>
          <circle cx={HUB.x} cy={HUB.y} r={HUB.r + 10} className="hub-ring" />
          {isLive && <circle key={current.id} cx={HUB.x} cy={HUB.y} r={HUB.r + 4} className="hub-pulse" />}
          <circle cx={HUB.x} cy={HUB.y} r={HUB.r} className="hub-dot" />
        </g>

        {/* slots: don't exist until revealed — no dashed placeholders sitting
            there in advance */}
        {visibleSlots.map(
          (ev, i) => ev && <SlotNode key={ev.id} ev={ev} x={SLOT_X} y={SLOT_YS[i] - SLOT_H / 2} w={SLOT_W} h={SLOT_H} />
        )}

        {/* fixes node: only appears once its arrows start arriving */}
        {fixesArmed && (
          <foreignObject x={FIX.x} y={FIX.y} width={FIX.w} height={FIX.h} overflow="visible">
            <div className="node fix-node pop-in">
              <div className="node-label">Fixes</div>
              <div className="node-detail">{batchTally(visibleSlots)}</div>
            </div>
          </foreignObject>
        )}

        {/* AI summary node: appears once we're waiting on (or have) a report */}
        {summaryStage !== "hidden" && (
          <foreignObject x={SUMMARY.x} y={SUMMARY.y} width={SUMMARY.w} height={SUMMARY.h} overflow="visible">
            <div className={"node summary-node pop-in" + (summaryStage === "shown" ? " summary-node-active" : "")}>
              <div className="node-label">AI Summary</div>
              <div className="node-detail" key={summaryEvent ? summaryEvent.id : "waiting"}>
                {summaryEvent ? summaryEvent.summary : "writing report…"}
              </div>
            </div>
          </foreignObject>
        )}
      </svg>

      <div className="mindmap-counts">
        <span className="dot-key" style={{ background: STATUS_COLOR.passed }} /> {counts.passed} passed
        <span className="dot-key" style={{ background: STATUS_COLOR.failed }} /> {counts.failed} failed
        <span className="dot-key" style={{ background: STATUS_COLOR.healed }} /> {counts.healed} healed
      </div>
    </div>
  );
}

// Per-batch tally shown inside the Fixes node — how this run's cells (so far
// revealed) actually resolved, distinct from the session-wide totals below.
function batchTally(slots) {
  const seen = slots.filter(Boolean);
  if (seen.length === 0) return "—";
  const healed = seen.filter((e) => e.status === "healed").length;
  const failed = seen.filter((e) => e.status === "failed").length;
  const passed = seen.filter((e) => e.status === "passed").length;
  const parts = [];
  if (healed) parts.push(`${healed} healed`);
  if (failed) parts.push(`${failed} flagged`);
  if (passed) parts.push(`${passed} clean`);
  return parts.join(" · ");
}

function SlotNode({ ev, x, y, w, h }) {
  const color = STATUS_COLOR[ev.status] || "var(--muted)";
  const suggestion = ev.status === "failed" && ev.metadata && ev.metadata.ai_suggestion;
  return (
    <foreignObject x={x} y={y} width={w} height={h} overflow="visible">
      <div className="node slot-node slot-in" style={{ borderColor: color }}>
        <div className="node-label" style={{ color }}>
          {ev.status}
        </div>
        <div className="node-detail">{ev.test_name}</div>
        {ev.status === "healed" && ev.metadata && (
          <div className="node-heal">
            <span className="old-sel">{ev.metadata.old_selector}</span> → <span className="new-sel">{ev.metadata.new_selector}</span>
          </div>
        )}
        {suggestion && <div className="node-heal node-suggestion">{suggestion}</div>}
      </div>
    </foreignObject>
  );
}
