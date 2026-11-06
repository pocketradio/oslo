import http from 'k6/http';
import { check, sleep } from 'k6';
import { baseURL, locationPayload, ridePayload } from './lib/api.js';

export const options = {
  scenarios: {
    matching_race: {
      executor: 'per-vu-iterations',
      vus: Number(__ENV.VUS || 200),
      iterations: 1,
      maxDuration: __ENV.DURATION || '5m',
    },
  },
  setupTimeout: '5m',
};

const password = __ENV.TEST_PASSWORD || 'oslo-load-test-password';

function params(token, index, key) {
  const headers = {
    'Content-Type': 'application/json',
    'X-Forwarded-For': `198.51.100.${(index % 240) + 1}`,
  };
  if (token) headers.Authorization = `Bearer ${token}`;
  if (key) headers['Idempotency-Key'] = key;
  return { headers };
}

function provision(role, label, index) {
  const email = `${label}-${Date.now()}-${Math.floor(Math.random() * 1000000)}@example.com`;
  const body = JSON.stringify({ email, password, role });
  const registration = http.post(`${baseURL}/auth/register`, body, params(null, index));
  if (registration.status !== 201 && registration.status !== 409) {
    throw new Error(`register ${role} failed with status ${registration.status}`);
  }
  const login = http.post(
    `${baseURL}/auth/login`,
    JSON.stringify({ email, password }),
    params(null, index + 1000),
  );
  if (login.status !== 200) throw new Error(`login ${role} failed with status ${login.status}`);
  return login.json('access_token');
}

export function setup() {
  const riders = [];
  for (let index = 0; index < Number(__ENV.VUS || 200); index += 1) {
    riders.push(provision('rider', `matching-rider-${index}`, index));
  }
  const drivers = [
    provision('driver', 'matching-driver-0', 500),
    provision('driver', 'matching-driver-1', 501),
  ];
  return { riders, drivers };
}

export default function (data) {
  const index = __VU - 1;
  const riderToken = data.riders[index];
  const driverToken = data.drivers[index % data.drivers.length];
  const location = http.put(
    `${baseURL}/drivers/location`,
    locationPayload((index % 3) * 0.0001),
    params(driverToken, index + 2000),
  );
  check(location, { 'race driver is available': (response) => response.status === 204 });

  const ride = http.post(
    `${baseURL}/rides`,
    ridePayload(),
    params(riderToken, index + 3000, `matching-${index}-${Date.now()}`),
  );
  check(ride, { 'race ride is created': (response) => response.status === 201 });
  if (ride.status === 201) {
    const rideID = ride.json('id');
    sleep(1);
    const status = http.get(`${baseURL}/rides/${rideID}`, params(riderToken, index + 4000));
    check(status, { 'race ride remains readable': (response) => response.status === 200 });
  }
}
