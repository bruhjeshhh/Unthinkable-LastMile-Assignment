const BASE_URL = import.meta.env.VITE_API_URL || "http://localhost:8080";

function getToken() {
  return localStorage.getItem("dt_token");
}

async function request(path, { method = "GET", body, auth = true } = {}) {
  const headers = { "Content-Type": "application/json" };
  if (auth) {
    const token = getToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
  }
  const res = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `Request failed (${res.status})`);
  }
  return data;
}

export const api = {
  register: (payload) => request("/api/auth/register", { method: "POST", body: payload, auth: false }),
  login: (payload) => request("/api/auth/login", { method: "POST", body: payload, auth: false }),

  previewCharge: (payload) => request("/api/orders/preview", { method: "POST", body: payload }),
  createOrder: (payload) => request("/api/orders", { method: "POST", body: payload }),
  listOrders: (params = {}) => {
    const qs = new URLSearchParams(params).toString();
    return request(`/api/orders${qs ? `?${qs}` : ""}`);
  },
  getOrder: (id) => request(`/api/orders/${id}`),
  updateOrderStatus: (id, payload) => request(`/api/orders/${id}/status`, { method: "PATCH", body: payload }),
  rescheduleOrder: (id, payload) => request(`/api/orders/${id}/reschedule`, { method: "POST", body: payload }),

  getAgentProfile: () => request("/api/agents/me"),
  listZones: () => request("/api/zones"),
  createZone: (payload) => request("/api/admin/zones", { method: "POST", body: payload }),
  mapPincode: (payload) => request("/api/admin/zones/map-pincode", { method: "POST", body: payload }),
  listRateCards: () => request("/api/admin/rate-cards"),
  upsertRateCard: (payload) => request("/api/admin/rate-cards", { method: "POST", body: payload }),
  listAgents: () => request("/api/admin/agents"),
  setAgentAvailability: (id, payload) => request(`/api/agents/${id}/availability`, { method: "PATCH", body: payload }),
  assignOrder: (id, payload) => request(`/api/admin/orders/${id}/assign`, { method: "POST", body: payload }),
  overrideOrderStatus: (id, payload) => request(`/api/admin/orders/${id}/override`, { method: "PATCH", body: payload }),
};

export function saveSession(token, user) {
  localStorage.setItem("dt_token", token);
  localStorage.setItem("dt_user", JSON.stringify(user));
}

export function getSession() {
  const token = getToken();
  const userRaw = localStorage.getItem("dt_user");
  if (!token || !userRaw) return null;
  try {
    return { token, user: JSON.parse(userRaw) };
  } catch {
    return null;
  }
}

export function clearSession() {
  localStorage.removeItem("dt_token");
  localStorage.removeItem("dt_user");
}
