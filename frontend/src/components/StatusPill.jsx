export default function StatusPill({ status }) {
  return <span className={`pill ${status.toLowerCase()}`}>{status.replace(/_/g, " ")}</span>;
}
