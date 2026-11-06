import http from 'k6/http';
import { check, sleep } from 'k6';
import { baseURL, jsonParams, locationPayload, loginOrRegister, ridePayload } from './lib/api.js';

export const options = { scenarios: { retry: { executor: 'per-vu-iterations', vus: Number(__ENV.VUS || 1), iterations: 1, maxDuration: __ENV.DURATION || '2m' } } };

function pendingOffer(token) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    const response = http.get(`${baseURL}/drivers/offers`, jsonParams(token));
    if (response.status === 200 && response.json().length > 0) return response.json()[0];
    sleep(0.5);
  }
  return null;
}

export default function () {
  const suffix = `${__VU}-${Date.now()}`;
  const rider = loginOrRegister('rider', `retry-rider-${suffix}`);
  const driverA = loginOrRegister('driver', `retry-driver-a-${suffix}`);
  const driverB = loginOrRegister('driver', `retry-driver-b-${suffix}`);
  check(http.put(`${baseURL}/drivers/location`, locationPayload(0), jsonParams(driverA)), { 'driver A available': (r) => r.status === 204 });
  check(http.put(`${baseURL}/drivers/location`, locationPayload(0.0001), jsonParams(driverB)), { 'driver B available': (r) => r.status === 204 });

  const ride = http.post(`${baseURL}/rides`, ridePayload(), jsonParams(rider, `retry-${suffix}`));
  check(ride, { 'retry ride created': (r) => r.status === 201 });
  if (ride.status !== 201) return;

  const first = pendingOffer(driverA);
  check({ first }, { 'first driver receives offer': (v) => v.first !== null });
  if (!first) return;
  const rejected = http.post(`${baseURL}/drivers/offers/${first.id}/reject`, null, jsonParams(driverA));
  check(rejected, { 'first offer rejected': (r) => r.status === 204 });

  const second = pendingOffer(driverB);
  check({ second }, { 'next driver receives retry offer': (v) => v.second !== null });
  if (!second) return;
  const accepted = http.post(`${baseURL}/drivers/offers/${second.id}/accept`, null, jsonParams(driverB));
  check(accepted, { 'retry offer accepted': (r) => r.status === 204 });
}
