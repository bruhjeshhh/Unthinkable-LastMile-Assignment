import { useState } from "react";
import { Link } from "react-router-dom";
import { api, saveSession } from "../api/client.js";

export default function Login({ onAuth }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const submit = async (e) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const data = await api.login({ email, password });
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
        <h1>Log in</h1>
        {error && <div className="error-banner">{error}</div>}
        <label>Email</label>
        <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        <label>Password</label>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        <button type="submit" disabled={loading} style={{ width: "100%" }}>
          {loading ? "Logging in..." : "Log in"}
        </button>
        <p style={{ textAlign: "center", fontSize: 13, marginTop: 16 }}>
          No account? <Link to="/register">Register</Link>
        </p>
      </form>
    </div>
  );
}
