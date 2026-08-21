import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { api, saveSession } from "../api/client.js";

export default function Register({ onAuth }) {
  const [form, setForm] = useState({ name: "", email: "", phone: "", password: "", role: "customer", zone_id: "" });
  const [zones, setZones] = useState([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const update = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  useEffect(() => {
    if (form.role === "agent") {
      api.listZones().then(setZones).catch(() => {});
    }
  }, [form.role]);

  const submit = async (e) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const payload = { ...form };
      if (payload.role === "agent" && payload.zone_id) {
        payload.zone_id = parseInt(payload.zone_id, 10);
      } else {
        delete payload.zone_id;
      }
      const data = await api.register(payload);
      saveSession(data.token, data.user);
      onAuth(data.token, data.user);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={submit}>
        <div className="brand-mini">DELIVERY TRACKER</div>
        <h1>Create account</h1>
        {error && <div className="error-banner">{error}</div>}
        <label>Full name</label>
        <input value={form.name} onChange={update("name")} required />
        <label>Email</label>
        <input type="email" value={form.email} onChange={update("email")} required />
        <label>Phone</label>
        <input value={form.phone} onChange={update("phone")} placeholder="for SMS notifications" />
        <label>Password</label>
        <input type="password" value={form.password} onChange={update("password")} required />
        <label>Account type</label>
        <select value={form.role} onChange={update("role")}>
          <option value="customer">Customer</option>
          <option value="agent">Delivery agent</option>
        </select>
        {form.role === "agent" && zones.length > 0 && (
          <>
            <label>Zone</label>
            <select value={form.zone_id} onChange={update("zone_id")} required>
              <option value="">Select your zone</option>
              {zones.map((z) => (
                <option key={z.id} value={z.id}>{z.name}</option>
              ))}
            </select>
          </>
        )}
        <button type="submit" disabled={loading} style={{ width: "100%" }}>
          {loading ? "Creating..." : "Create account"}
        </button>
        <p style={{ textAlign: "center", fontSize: 13, marginTop: 16 }}>
          Already have an account? <Link to="/">Log in</Link>
        </p>
      </form>
    </div>
  );
}
