import { useState, useEffect } from "react";
import { api } from "../api/client.js";
import StatusPill from "../components/StatusPill.jsx";
import Timeline from "../components/Timeline.jsx";

const emptyForm = {
  pickup_address: "", pickup_pincode: "",
  drop_address: "", drop_pincode: "",
  length_cm: "", breadth_cm: "", height_cm: "", actual_weight_kg: "",
  order_type: "B2C", payment_type: "PREPAID",
};

export default function CustomerDashboard() {
  const [orders, setOrders] = useState([]);
  const [form, setForm] = useState(emptyForm);
  const [preview, setPreview] = useState(null);
  const [error, setError] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [selected, setSelected] = useState(null);
  const [history, setHistory] = useState([]);
  const [rescheduleDate, setRescheduleDate] = useState("");

  const loadOrders = async () => {
    const data = await api.listOrders();
    setOrders(data || []);
  };

  useEffect(() => { loadOrders(); }, []);

  const update = (k) => (e) => {
    setForm({ ...form, [k]: e.target.value });
    setPreview(null);
  };

  const numeric = (f) => ({
    ...f,
    length_cm: Number(f.length_cm), breadth_cm: Number(f.breadth_cm),
    height_cm: Number(f.height_cm), actual_weight_kg: Number(f.actual_weight_kg),
  });

  const doPreview = async () => {
    setError(""); setPreviewLoading(true);
    try {
      const data = await api.previewCharge(numeric(form));
      setPreview(data);
    } catch (err) {
      setError(err.message);
      setPreview(null);
    } finally {
      setPreviewLoading(false);
    }
  };

  const confirmOrder = async () => {
    setError("");
    try {
      await api.createOrder(numeric(form));
      setForm(emptyForm);
      setPreview(null);
      loadOrders();
    } catch (err) {
      setError(err.message);
    }
  };

  const viewOrder = async (id) => {
    const data = await api.getOrder(id);
    setSelected(data.order);
    setHistory(data.history || []);
  };

  const submitReschedule = async () => {
    if (!rescheduleDate) return;
    await api.rescheduleOrder(selected.id, { new_date: rescheduleDate });
    setRescheduleDate("");
    viewOrder(selected.id);
    loadOrders();
  };

  return (
    <div>
      <h1>My Orders</h1>
      <p className="subtitle">Create a new delivery order and track existing ones.</p>

      <div className="card">
        <h2>New order</h2>
        {error && <div className="error-banner">{error}</div>}
        <div className="grid-2">
          <div>
            <label>Pickup address</label>
            <input value={form.pickup_address} onChange={update("pickup_address")} />
          </div>
          <div>
            <label>Pickup pincode</label>
            <input value={form.pickup_pincode} onChange={update("pickup_pincode")} />
          </div>
        </div>
        <div className="grid-2">
          <div>
            <label>Drop address</label>
            <input value={form.drop_address} onChange={update("drop_address")} />
          </div>
          <div>
            <label>Drop pincode</label>
            <input value={form.drop_pincode} onChange={update("drop_pincode")} />
          </div>
        </div>
        <div className="grid-3">
          <div><label>Length (cm)</label><input type="number" value={form.length_cm} onChange={update("length_cm")} /></div>
          <div><label>Breadth (cm)</label><input type="number" value={form.breadth_cm} onChange={update("breadth_cm")} /></div>
          <div><label>Height (cm)</label><input type="number" value={form.height_cm} onChange={update("height_cm")} /></div>
        </div>
        <div className="grid-3">
          <div><label>Actual weight (kg)</label><input type="number" value={form.actual_weight_kg} onChange={update("actual_weight_kg")} /></div>
          <div>
            <label>Order type</label>
            <select value={form.order_type} onChange={update("order_type")}>
              <option value="B2C">B2C</option>
              <option value="B2B">B2B</option>
            </select>
          </div>
          <div>
            <label>Payment type</label>
            <select value={form.payment_type} onChange={update("payment_type")}>
              <option value="PREPAID">Prepaid</option>
              <option value="COD">Cash on delivery</option>
            </select>
          </div>
        </div>

        {preview && (
          <div className="charge-preview">
            <div>Billable weight: <strong>{preview.billable_weight_kg} kg</strong> (volumetric: {preview.volumetric_weight_kg} kg)</div>
            <div className="amount">₹{preview.charge}</div>
          </div>
        )}

        <div style={{ display: "flex", gap: 10 }}>
          <button className="secondary" onClick={doPreview} disabled={previewLoading}>
            {previewLoading ? "Calculating..." : "Calculate charge"}
          </button>
          <button onClick={confirmOrder} disabled={!preview}>Confirm order</button>
        </div>
      </div>

      <div className="card">
        <h2>Order history</h2>
        {orders.length === 0 ? (
          <div className="empty-state">No orders yet — create one above.</div>
        ) : (
          <table>
            <thead><tr><th>ID</th><th>Route</th><th>Charge</th><th>Status</th><th></th></tr></thead>
            <tbody>
              {orders.map((o) => (
                <tr key={o.id}>
                  <td className="mono">#{o.id}</td>
                  <td>{o.pickup_pincode} → {o.drop_pincode}</td>
                  <td className="mono">₹{o.charge}</td>
                  <td><StatusPill status={o.status} /></td>
                  <td><button className="secondary" onClick={() => viewOrder(o.id)}>Track</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {selected && (
        <div className="card">
          <h2>Order #{selected.id} timeline</h2>
          <Timeline history={history} />
          {selected.status === "FAILED" && (
            <div style={{ marginTop: 16 }}>
              <label>Reschedule delivery date</label>
              <input type="date" value={rescheduleDate} onChange={(e) => setRescheduleDate(e.target.value)} style={{ maxWidth: 220 }} />
              <button onClick={submitReschedule}>Reschedule</button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
