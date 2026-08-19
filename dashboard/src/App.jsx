import { useEffect, useState } from "react";
import Mindmap from "./Mindmap.jsx";

const WS_URL = import.meta.env.VITE_WS_URL || "ws://localhost:18080/ws";
const TRAIL_MAX = 10;

// Status -> visual treatment for the 56px icon.
const STATUS_STYLE = {
  passed: { color: "#22c55e", Icon: CheckIcon, spin: false },
  failed: { color: "#ef4444", Icon: CrossIcon, spin: false },
  healed: { color: "#3b82f6", Icon: RefreshIcon, spin: false },
  running: { color: "#3b82f6", Icon: RefreshIcon, spin: true },
  retrying: { color: "#f59e0b", Icon: ClockIcon, spin: false },
};

const DOT_COLOR = {
  passed: "#22c55e",
  failed: "#ef4444",
  healed: "#3b82f6",
  running: "#3b82f6",
  retrying: "#f59e0b",
};

// Statuses that count as a finished test for the trail + tallies.
const COMPLETED = new Set(["passed", "failed", "healed"]);

export default function App() {
  const [current, setCurrent] = useState(null);
  const [connected, setConnected] = useState(false);
  const [trail, setTrail] = useState([]); // last ~10 completed events
  const [counts, setCounts] = useState({ passed: 0, failed: 0, healed: 0 });
  const [report, setReport] = useState(null); // latest whole-run AI report
  const [runEnd, setRunEnd] = useState(null); // latest "this run's events are all in" marker
  const [view, setView] = useState("mindmap"); // "mindmap" | "classic"

  useEffect(() => {
    let ws;
    let retry;
    const connect = () => {
      ws = new WebSocket(WS_URL);
      ws.onopen = () => {
        setConnected(true);
        // A fresh connection == a fresh run: reset trail + tallies.
        setTrail([]);
        setCounts({ passed: 0, failed: 0, healed: 0 });
        setReport(null);
        setRunEnd(null);
      };
      ws.onclose = () => {
        setConnected(false);
        retry = setTimeout(connect, 1500);
      };
      ws.onmessage = (msg) => {
        let ev;
        try {
          ev = JSON.parse(msg.data);
        } catch {
          return;
        }
        // Three message shapes share this socket: per-test TestEvents, one
        // RunBoundary marking "this run's events are all in" (kind ===
        // "run_end"), and one whole-run RunReport per suite run (kind ===
        // "report").
        if (ev.kind === "run_end") {
          setRunEnd(ev);
          return;
        }
        if (ev.kind === "report") {
          setReport(ev);
          return;
        }
        setCurrent(ev);
        if (COMPLETED.has(ev.status)) {
          setTrail((t) => [...t, ev].slice(-TRAIL_MAX));
          setCounts((c) => ({ ...c, [ev.status]: c[ev.status] + 1 }));
        }
      };
    };
    connect();
    return () => {
      clearTimeout(retry);
      ws && ws.close();
    };
  }, []);

  const style = current ? STATUS_STYLE[current.status] : null;
  const Icon = style ? style.Icon : null;

  return (
    <main className="stage">
      <div className="conn" data-on={connected}>
        {connected ? "live" : "connecting…"}
      </div>

      <div className="view-toggle">
        <button data-active={view === "mindmap"} onClick={() => setView("mindmap")}>
          mindmap
        </button>
        <button data-active={view === "classic"} onClick={() => setView("classic")}>
          classic
        </button>
      </div>

      {view === "mindmap" ? (
        <Mindmap connected={connected} current={current} trail={trail} counts={counts} report={report} runEnd={runEnd} />
      ) : current && style ? (
        <section className="card">
          <div
            className={"icon " + (style.spin ? "spin" : "")}
            style={{ color: style.color, borderColor: style.color }}
          >
            <Icon />
          </div>
          <div className="name">{current.test_name}</div>
          <Detail event={current} />
        </section>
      ) : (
        <section className="card idle">
          <div className="icon idle-icon">•</div>
          <div className="name">waiting for tests…</div>
          <div className="detail" />
        </section>
      )}

      {view === "classic" && (
        <>
          {/* History trail: small dots, newest on the right, last one ringed. */}
          <div className="trail">
            {trail.map((ev, i) => (
              <span
                key={ev.id}
                className={"dot" + (i === trail.length - 1 ? " ring" : "")}
                style={{ background: DOT_COLOR[ev.status] }}
              />
            ))}
          </div>

          {/* Running totals: three plain numbers, no cards, no borders. */}
          <div className="counts">
            <span>{counts.passed} passed</span>
            <span>{counts.failed} failed</span>
            <span>{counts.healed} healed</span>
          </div>
        </>
      )}
    </main>
  );
}

// The single detail line: healed shows old→new selector (old struck through,
// new in the accent color), failed shows the error, everything else stays blank.
function Detail({ event }) {
  if (event.status === "healed" && event.metadata) {
    return (
      <div className="detail">
        <span className="old-sel">{event.metadata.old_selector}</span>
        <span className="arrow"> → </span>
        <span className="new-sel">{event.metadata.new_selector}</span>
      </div>
    );
  }
  if (event.status === "failed") {
    const suggestion = event.metadata && event.metadata.ai_suggestion;
    if (suggestion) {
      return (
        <div className="detail">
          <span className="ai-sel">{suggestion}</span>
        </div>
      );
    }
    return <div className="detail">{event.error_message || ""}</div>;
  }
  return <div className="detail" />;
}

/* ---- inline icons (no extra deps) ---- */

function CheckIcon() {
  return (
    <svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}

function CrossIcon() {
  return (
    <svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  );
}

function RefreshIcon() {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 12a9 9 0 1 1-2.64-6.36" />
      <path d="M21 3v6h-6" />
    </svg>
  );
}

function ClockIcon() {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </svg>
  );
}
