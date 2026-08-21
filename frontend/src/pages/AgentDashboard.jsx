import { useState, useEffect } from "react";
import { api } from "../api/client.js";
import StatusPill from "../components/StatusPill.jsx";
import Timeline from "../components/Timeline.jsx";

const NEXT_STATUS = {
  ASSIGNED: "PICKED_UP",
  PICKED_UP: "IN_TRANSIT",
  IN_TRANSIT: "OUT_FOR_DELIVERY",
};

const STATUS_LABELS = {
  ASSIGNED: "Assigned",
  PICKED_UP: "Picked Up",
  IN_TRANSIT: "In Transit",
  OUT_FOR_DELIVERY: "Out for Delivery",
  DELIVERED: "Delivered",
  FAILED: "Failed",
};

const FAILURE_REASONS = [
  "Recipient not available",
  "Wrong address",
  "Package refused",
  "Damaged package",
  "No access to building",
  "Other",
];

export default function AgentDashboard({ user }) {
  const [profile, setProfile] = useState(null);
  const [orders, setOrders] = useState([]);
  const [zones, setZones] = useState([]);
  const [selected, setSelected] = useState(null);
  const [history, setHistory] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [failModal, setFailModal] = useState(null);
  const [failReason, setFailReason] = useState("");
  const [failNote, setFailNote] = useState("");

  const loadAll = async () => {
    setLoading(true);
    setError("");
    try {
      const [profileData, ordersData, zonesData] = await Promise.all([
        api.getAgentProfile(),
        api.listOrders(),
        api.listZones(),
      ]);
      setProfile(profileData);
      setOrders(ordersData || []);
      setZones(zonesData || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadAll(); }, []);

  const zoneName = (id) => {
    const z = zones.find((z) => z.id === id);
    return z ? z.name : `Zone ${id}`;
  };

  const toggleAvailability = async () => {
    if (!profile) return;
    const next = profile.status === "available" ? "offline" : "available";
    try {
      await api.setAgentAvailability(user.id, { status: next });
      setProfile({ ...profile, status: next });
    } catch (err) {
      setError(err.message);
    }
  };

  const advance = async (order) => {
    setError("");
    const next = NEXT_STATUS[order.status];
    if (!next) return;
    try {
      await api.updateOrderStatus(order.id, { status: next });
      loadAll();
      if (selected?.id === order.id) viewOrder(order.id);
    } catch (err) {
      setError(err.message);
    }
  };

  const confirmDeliver = async (order) => {
    setError("");
    try {
      await api.updateOrderStatus(order.id, { status: "DELIVERED" });
      loadAll();
      if (selected?.id === order.id) viewOrder(order.id);
    } catch (err) {
      setError(err.message);
    }
  };

  const confirmFail = async () => {
    if (!failModal) return;
    setError("");
    const noteParts = [failReason];
    if (failNote.trim()) noteParts.push(failNote.trim());
    const note = noteParts.filter(Boolean).join(": ");
    try {
      await api.updateOrderStatus(failModal.id, { status: "FAILED", note });
      setFailModal(null);
      setFailReason("");
      setFailNote("");
      loadAll();
      if (selected?.id === failModal.id) viewOrder(failModal.id);
    } catch (err) {
      setError(err.message);
    }
  };

  const viewOrder = async (id) => {
    try {
      const data = await api.getOrder(id);
      setSelected(data.order);
      setHistory(data.history || []);
    } catch (err) {
      setError(err.message);
    }
  };

  const filtered = statusFilter
    ? orders.filter((o) => o.status === statusFilter)
    : orders;

  const stats = {
    active: orders.filter((o) => ["ASSIGNED", "PICKED_UP", "IN_TRANSIT", "OUT_FOR_DELIVERY"].includes(o.status)).length,
    delivered: orders.filter((o) => o.status === "DELIVERED").length,
    failed: orders.filter((o) => o.status === "FAILED").length,
    total: orders.length,
  };

  if (loading) {
    return (
      <div>
        <h1>My Deliveries</h1>
        <div className="loading-state">Loading your deliveries...</div>
      </div>
    );
  }

  return (
    <div>
      <h1>My Deliveries</h1>
      <p className="subtitle">Manage your assigned deliveries.</p>

      {error && <div className="error-banner">{error}</div>}

      <div className="stats-row">
        <div className="stat-card">
          <div className="stat-value">{stats.active}</div>
          <div className="stat-label">Active</div>
        </div>
        <div className="stat-card stat-delivered">
          <div className="stat-value">{stats.delivered}</div>
          <div className="stat-label">Delivered</div>
        </div>
        <div className="stat-card stat-failed">
          <div className="stat-value">{stats.failed}</div>
          <div className="stat-label">Failed</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{stats.total}</div>
          <div className="stat-label">Total</div>
        </div>
      </div>

      <div className="card" style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <span className="muted">Availability:</span>{" "}
          <strong className={profile?.status === "available" ? "text-green" : "text-muted"}>
            {profile?.status === "available" ? "Available" : profile?.status === "busy" ? "Busy" : "Offline"}
          </strong>
          {profile?.zone_name && (
            <span className="muted" style={{ marginLeft: 16 }}>Zone: <strong>{profile.zone_name}</strong></span>
          )}
        </div>
        <button className="secondary" onClick={toggleAvailability} disabled={profile?.status === "busy"}>
          {profile?.status === "available" ? "Go offline" : profile?.status === "busy" ? "Busy" : "Go available"}
        </button>
      </div>

      <div className="card">
        <div className="filter-row">
          <span className="muted" style={{ fontSize: 13 }}>Filter:</span>
          <button
            className={`filter-btn ${statusFilter === "" ? "active" : ""}`}
            onClick={() => setStatusFilter("")}
          >All</button>
          {Object.entries(STATUS_LABELS).map(([key, label]) => (
            <button
              key={key}
              className={`filter-btn ${statusFilter === key ? "active" : ""}`}
              onClick={() => setStatusFilter(key)}
            >{label}</button>
          ))}
        </div>

        {filtered.length === 0 ? (
          <div className="empty-state">
            {orders.length === 0 ? "No deliveries assigned right now." : "No orders match this filter."}
          </div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Route</th>
                <th>Zones</th>
                <th>Scheduled</th>
                <th>Type</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((o) => (
                <tr key={o.id}>
                  <td className="mono">#{o.id}</td>
                  <td>{o.pickup_pincode} &rarr; {o.drop_pincode}</td>
                  <td className="muted">{zoneName(o.from_zone_id)} &rarr; {zoneName(o.to_zone_id)}</td>
                  <td className="mono" style={{ fontSize: 12 }}>{o.scheduled_date || "—"}</td>
                  <td>{o.order_type} &middot; {o.payment_type}</td>
                  <td><StatusPill status={o.status} /></td>
                  <td style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                    <button className="secondary" onClick={() => viewOrder(o.id)}>Details</button>
                    {NEXT_STATUS[o.status] && (
                      <button onClick={() => advance(o)}>
                        Mark {NEXT_STATUS[o.status].replace(/_/g, " ")}
                      </button>
                    )}
                    {o.status === "OUT_FOR_DELIVERY" && (
                      <>
                        <button className="btn-success" onClick={() => confirmDeliver(o)}>Delivered</button>
                        <button className="btn-danger" onClick={() => { setFailModal(o); setFailReason(""); setFailNote(""); }}>Failed</button>
                      </>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {selected && (
        <div className="card">
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <h2>Order #{selected.id}</h2>
            <button className="secondary" onClick={() => { setSelected(null); setHistory([]); }}>Close</button>
          </div>
          <div className="detail-grid">
            <div>
              <label>Pickup</label>
              <p>{selected.pickup_address}</p>
              <p className="mono muted" style={{ fontSize: 12 }}>{selected.pickup_pincode} ({zoneName(selected.from_zone_id)})</p>
            </div>
            <div>
              <label>Drop</label>
              <p>{selected.drop_address}</p>
              <p className="mono muted" style={{ fontSize: 12 }}>{selected.drop_pincode} ({zoneName(selected.to_zone_id)})</p>
            </div>
            <div>
              <label>Scheduled</label>
              <p className="mono">{selected.scheduled_date || "Not scheduled"}</p>
            </div>
            <div>
              <label>Weight</label>
              <p>{selected.billable_weight_kg} kg (billable)</p>
            </div>
          </div>
          <h3 style={{ fontSize: 14, marginTop: 16 }}>Timeline</h3>
          <Timeline history={history} />
        </div>
      )}

      {failModal && (
        <div className="modal-overlay" onClick={() => setFailModal(null)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <h2>Report Failed Delivery</h2>
            <p className="muted" style={{ marginBottom: 16 }}>Order #{failModal.id} &mdash; {failModal.pickup_pincode} &rarr; {failModal.drop_pincode}</p>
            <label>Reason</label>
            <select value={failReason} onChange={(e) => setFailReason(e.target.value)} required>
              <option value="">Select a reason</option>
              {FAILURE_REASONS.map((r) => (
                <option key={r} value={r}>{r}</option>
              ))}
            </select>
            <label>Additional notes (optional)</label>
            <input
              value={failNote}
              onChange={(e) => setFailNote(e.target.value)}
              placeholder="e.g. left with neighbour at gate 3"
            />
            <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 16 }}>
              <button className="secondary" onClick={() => setFailModal(null)}>Cancel</button>
              <button className="btn-danger" onClick={confirmFail} disabled={!failReason}>Confirm Failed</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
