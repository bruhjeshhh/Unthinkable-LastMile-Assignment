import { useState, useEffect } from "react";
import { api } from "../api/client.js";
import StatusPill from "../components/StatusPill.jsx";
import Timeline from "../components/Timeline.jsx";

const STATUSES = ["CREATED", "ASSIGNED", "PICKED_UP", "IN_TRANSIT", "OUT_FOR_DELIVERY", "DELIVERED", "FAILED", "RESCHEDULED", "CANCELLED"];

export default function AdminDashboard() {
  const [tab, setTab] = useState("orders");
  return (
    <div>
      <h1>Operations</h1>
      <p className="subtitle">Manage zones, rate cards, agents, and all orders.</p>
      <div style={{ display: "flex", gap: 8, marginBottom: 20 }}>
        {["orders", "zones", "rate-cards", "agents"].map((t) => (
          <button key={t} className={tab === t ? "" : "secondary"} onClick={() => setTab(t)}>
            {t.replace("-", " ")}
          </button>
        ))}
      </div>
      {tab === "orders" && <OrdersTab />}
      {tab === "zones" && <ZonesTab />}
      {tab === "rate-cards" && <RateCardsTab />}
      {tab === "agents" && <AgentsTab />}
    </div>
  );
}

function OrdersTab() {
  const [orders, setOrders] = useState([]);
  const [filters, setFilters] = useState({ status: "", zone_id: "", agent_id: "" });
  const [selected, setSelected] = useState(null);
  const [history, setHistory] = useState([]);
  const [error, setError] = useState("");
  const [overrideStatus, setOverrideStatus] = useState("");

  const load = async () => {
    const clean = Object.fromEntries(Object.entries(filters).filter(([, v]) => v));
    const data = await api.listOrders(clean);
    setOrders(data || []);
  };

  useEffect(() => { load(); }, [filters]);

  const viewOrder = async (id) => {
    const data = await api.getOrder(id);
    setSelected(data.order);
    setHistory(data.history || []);
  };

  const autoAssign = async (id) => {
    setError("");
    try {
      await api.assignOrder(id, {});
      load();
      if (selected?.id === id) viewOrder(id);
    } catch (err) { setError(err.message); }
  };

  const doOverride = async () => {
    if (!overrideStatus || !selected) return;
    setError("");
    try {
      await api.overrideOrderStatus(selected.id, { status: overrideStatus, note: "manual admin correction" });
      setOverrideStatus("");
      viewOrder(selected.id);
      load();
    } catch (err) { setError(err.message); }
  };

  return (
    <>
      <div className="card">
        <div className="grid-3">
          <div>
            <label>Filter by status</label>
            <select value={filters.status} onChange={(e) => setFilters({ ...filters, status: e.target.value })}>
              <option value="">All</option>
              {STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
            </select>
          </div>
          <div>
            <label>Zone ID</label>
            <input value={filters.zone_id} onChange={(e) => setFilters({ ...filters, zone_id: e.target.value })} placeholder="e.g. 1" />
          </div>
          <div>
            <label>Agent ID</label>
            <input value={filters.agent_id} onChange={(e) => setFilters({ ...filters, agent_id: e.target.value })} placeholder="e.g. 4" />
          </div>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="card">
        {orders.length === 0 ? (
          <div className="empty-state">No orders match these filters.</div>
        ) : (
          <table>
            <thead><tr><th>ID</th><th>Customer</th><th>Route (zones)</th><th>Charge</th><th>Agent</th><th>Status</th><th></th></tr></thead>
            <tbody>
              {orders.map((o) => (
                <tr key={o.id}>
                  <td className="mono">#{o.id}</td>
                  <td className="mono">u{o.customer_id}</td>
                  <td>z{o.from_zone_id} → z{o.to_zone_id}</td>
                  <td className="mono">₹{o.charge}</td>
                  <td className="mono">{o.agent_id ? `a${o.agent_id}` : "—"}</td>
                  <td><StatusPill status={o.status} /></td>
                  <td style={{ display: "flex", gap: 6 }}>
                    <button className="secondary" onClick={() => viewOrder(o.id)}>Details</button>
                    {!o.agent_id && <button onClick={() => autoAssign(o.id)}>Auto-assign</button>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {selected && (
        <div className="card">
          <h2>Order #{selected.id} — admin override</h2>
          <Timeline history={history} />
          <div style={{ display: "flex", gap: 8, marginTop: 16, alignItems: "flex-end" }}>
            <div style={{ flex: 1 }}>
              <label>Force status</label>
              <select value={overrideStatus} onChange={(e) => setOverrideStatus(e.target.value)}>
                <option value="">Select status</option>
                {STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
            <button onClick={doOverride}>Apply override</button>
          </div>
        </div>
      )}
    </>
  );
}

function ZonesTab() {
  const [zones, setZones] = useState([]);
  const [name, setName] = useState("");
  const [pincode, setPincode] = useState("");
  const [label, setLabel] = useState("");
  const [zoneId, setZoneId] = useState("");
  const [error, setError] = useState("");

  const load = async () => setZones((await api.listZones()) || []);
  useEffect(() => { load(); }, []);

  const createZone = async () => {
    if (!name) return;
    setError("");
    try { await api.createZone({ name }); setName(""); load(); } catch (e) { setError(e.message); }
  };

  const mapPincode = async () => {
    if (!zoneId || !pincode) return;
    setError("");
    try {
      await api.mapPincode({ zone_id: Number(zoneId), pincode, label });
      setPincode(""); setLabel("");
    } catch (e) { setError(e.message); }
  };

  return (
    <>
      {error && <div className="error-banner">{error}</div>}
      <div className="card">
        <h2>Create zone</h2>
        <div className="grid-2">
          <div><label>Zone name</label><input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. North Delhi" /></div>
          <div style={{ display: "flex", alignItems: "flex-end" }}><button onClick={createZone}>Add zone</button></div>
        </div>
      </div>
      <div className="card">
        <h2>Map pincode to zone</h2>
        <div className="grid-3">
          <div>
            <label>Zone</label>
            <select value={zoneId} onChange={(e) => setZoneId(e.target.value)}>
              <option value="">Select zone</option>
              {zones.map((z) => <option key={z.id} value={z.id}>{z.name} (#{z.id})</option>)}
            </select>
          </div>
          <div><label>Pincode</label><input value={pincode} onChange={(e) => setPincode(e.target.value)} /></div>
          <div><label>Area label (optional)</label><input value={label} onChange={(e) => setLabel(e.target.value)} /></div>
        </div>
        <button onClick={mapPincode}>Map pincode</button>
      </div>
      <div className="card">
        <h2>Zones</h2>
        {zones.length === 0 ? <div className="empty-state">No zones yet.</div> : (
          <table><thead><tr><th>ID</th><th>Name</th></tr></thead>
            <tbody>{zones.map((z) => <tr key={z.id}><td className="mono">{z.id}</td><td>{z.name}</td></tr>)}</tbody>
          </table>
        )}
      </div>
    </>
  );
}

function RateCardsTab() {
  const [zones, setZones] = useState([]);
  const [cards, setCards] = useState([]);
  const [form, setForm] = useState({ order_type: "B2C", from_zone_id: "", to_zone_id: "", base_fee: "", rate_per_kg: "", cod_surcharge: "" });
  const [error, setError] = useState("");

  const load = async () => {
    setZones((await api.listZones()) || []);
    setCards((await api.listRateCards()) || []);
  };
  useEffect(() => { load(); }, []);

  const update = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  const save = async () => {
    setError("");
    try {
      await api.upsertRateCard({
        order_type: form.order_type,
        from_zone_id: Number(form.from_zone_id),
        to_zone_id: Number(form.to_zone_id),
        base_fee: Number(form.base_fee || 0),
        rate_per_kg: Number(form.rate_per_kg),
        cod_surcharge: Number(form.cod_surcharge || 0),
      });
      load();
    } catch (e) { setError(e.message); }
  };

  const zoneName = (id) => zones.find((z) => z.id === id)?.name || `#${id}`;

  return (
    <>
      {error && <div className="error-banner">{error}</div>}
      <div className="card">
        <h2>Set rate card</h2>
        <p className="muted" style={{ marginTop: -8, marginBottom: 12 }}>Same from/to zone = intra-zone rate. Different = inter-zone rate.</p>
        <div className="grid-3">
          <div>
            <label>Order type</label>
            <select value={form.order_type} onChange={update("order_type")}>
              <option value="B2C">B2C</option>
              <option value="B2B">B2B</option>
            </select>
          </div>
          <div>
            <label>From zone</label>
            <select value={form.from_zone_id} onChange={update("from_zone_id")}>
              <option value="">Select</option>
              {zones.map((z) => <option key={z.id} value={z.id}>{z.name}</option>)}
            </select>
          </div>
          <div>
            <label>To zone</label>
            <select value={form.to_zone_id} onChange={update("to_zone_id")}>
              <option value="">Select</option>
              {zones.map((z) => <option key={z.id} value={z.id}>{z.name}</option>)}
            </select>
          </div>
        </div>
        <div className="grid-3">
          <div><label>Base fee (₹)</label><input type="number" value={form.base_fee} onChange={update("base_fee")} /></div>
          <div><label>Rate per kg (₹)</label><input type="number" value={form.rate_per_kg} onChange={update("rate_per_kg")} /></div>
          <div><label>COD surcharge (₹)</label><input type="number" value={form.cod_surcharge} onChange={update("cod_surcharge")} /></div>
        </div>
        <button onClick={save}>Save rate card</button>
      </div>
      <div className="card">
        <h2>Configured rate cards</h2>
        {cards.length === 0 ? <div className="empty-state">No rate cards configured yet.</div> : (
          <table>
            <thead><tr><th>Type</th><th>From</th><th>To</th><th>Base</th><th>₹/kg</th><th>COD</th></tr></thead>
            <tbody>
              {cards.map((c) => (
                <tr key={c.id}>
                  <td>{c.order_type}</td><td>{zoneName(c.from_zone_id)}</td><td>{zoneName(c.to_zone_id)}</td>
                  <td className="mono">₹{c.base_fee}</td><td className="mono">₹{c.rate_per_kg}</td><td className="mono">₹{c.cod_surcharge}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}

function AgentsTab() {
  const [agents, setAgents] = useState([]);
  const load = async () => setAgents((await api.listAgents()) || []);
  useEffect(() => { load(); }, []);
  return (
    <div className="card">
      <h2>Delivery agents</h2>
      {agents.length === 0 ? <div className="empty-state">No agents registered yet.</div> : (
        <table>
          <thead><tr><th>ID</th><th>Name</th><th>Zone</th><th>Status</th></tr></thead>
          <tbody>
            {agents.map((a) => (
              <tr key={a.user_id}>
                <td className="mono">a{a.user_id}</td><td>{a.name}</td>
                <td>{a.zone_id ? `z${a.zone_id}` : "—"}</td>
                <td><StatusPill status={a.status.toUpperCase()} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
