export default function Timeline({ history }) {
  if (!history || history.length === 0) return <p className="muted">No history yet.</p>;
  return (
    <ul className="timeline">
      {history.map((h) => (
        <li key={h.id}>
          <strong>{h.status.replace(/_/g, " ")}</strong> — {h.actor_role}
          {h.note ? <span className="muted"> · {h.note}</span> : null}
          <div className="ts">{new Date(h.created_at).toLocaleString()}</div>
        </li>
      ))}
    </ul>
  );
}
