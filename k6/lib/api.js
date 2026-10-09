import http from 'k6/http';

export const baseURL = __ENV.BASE_URL || 'http://localhost:8080';

const password = __ENV.TEST_PASSWORD || 'oslo-load-test-password';

export function jsonParams(token, idempotencyKey) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  if (idempotencyKey) {
    headers['Idempotency-Key'] = idempotencyKey;
  }
  return { headers };
}

export function loginOrRegister(role, label) {
  const email = `${label}-${Date.now()}-${Math.floor(Math.random() * 1000000)}@example.com`;
  const body = JSON.stringify({
    email,
    password,
    role,
  });

  const registration = http.post(`${baseURL}/auth/register`, body, jsonParams());
  if (registration.status !== 201 && registration.status !== 409) {
    throw new Error(`register ${role} failed with status ${registration.status}`);
  }

  const login = http.post(
    `${baseURL}/auth/login`,
    JSON.stringify({ email, password }),
    jsonParams(),
  );
  if (login.status !== 200) {
    throw new Error(`login ${role} failed with status ${login.status}`);
  }

  return login.json('access_token');
}

export function checkStatus(response, expected) {
  return response.status === expected;
}

export function ridePayload() {
  return JSON.stringify({
    pickup: { latitude: 19.076, longitude: 72.8777 },
    destination: { latitude: 19.0896, longitude: 72.8656 },
  });
}

export function locationPayload(offset = 0) {
  return JSON.stringify({
    latitude: 19.076 + offset,
    longitude: 72.8777 + offset,
    available: true,
  });
}
