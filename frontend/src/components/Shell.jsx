export default function Shell({ user, onLogout, children }) {
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand"><span className="dot" /> Delivery Tracker</div>
        <div className="role-badge">{user.role}</div>
        <div className="nav-item active">
          {user.role === "customer" && "My Orders"}
          {user.role === "agent" && "My Deliveries"}
          {user.role === "admin" && "Operations"}
        </div>
        <div className="sidebar-footer">
          <div style={{ marginBottom: 8 }}>{user.name}<br /><span className="mono">{user.email}</span></div>
          <button className="secondary" style={{ width: "100%" }} onClick={onLogout}>Log out</button>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}
