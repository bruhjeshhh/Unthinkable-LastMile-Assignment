import { Routes, Route, Navigate } from "react-router-dom";
import { useState, useEffect } from "react";
import Login from "./pages/Login.jsx";
import Register from "./pages/Register.jsx";
import Shell from "./components/Shell.jsx";
import CustomerDashboard from "./pages/CustomerDashboard.jsx";
import AgentDashboard from "./pages/AgentDashboard.jsx";
import AdminDashboard from "./pages/AdminDashboard.jsx";
import { getSession, clearSession } from "./api/client.js";

export default function App() {
  const [session, setSession] = useState(getSession());

  useEffect(() => {
    setSession(getSession());
  }, []);

  const handleAuth = (token, user) => setSession({ token, user });
  const handleLogout = () => {
    clearSession();
    setSession(null);
  };

  if (!session) {
    return (
      <Routes>
        <Route path="/register" element={<Register onAuth={handleAuth} />} />
        <Route path="*" element={<Login onAuth={handleAuth} />} />
      </Routes>
    );
  }

  const { user } = session;

  return (
    <Shell user={user} onLogout={handleLogout}>
      <Routes>
        {user.role === "customer" && (
          <>
            <Route path="/" element={<CustomerDashboard />} />
            <Route path="*" element={<Navigate to="/" />} />
          </>
        )}
        {user.role === "agent" && (
          <>
            <Route path="/" element={<AgentDashboard user={user} />} />
            <Route path="*" element={<Navigate to="/" />} />
          </>
        )}
        {user.role === "admin" && (
          <>
            <Route path="/" element={<AdminDashboard />} />
            <Route path="*" element={<Navigate to="/" />} />
          </>
        )}
      </Routes>
    </Shell>
  );
}
