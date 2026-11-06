import http from 'k6/http';
import { check } from 'k6';
import { baseURL, jsonParams, loginOrRegister, ridePayload } from './lib/api.js';

export const options = { scenarios: { cancellation: { executor: 'per-vu-iterations', vus: Number(__ENV.VUS || 10), iterations: 1, maxDuration: '1m' } } };

export default function () {
  const rider = loginOrRegister('rider', `cancel-rider-${__VU}-${Date.now()}`);
  const key = `cancel-${__VU}-${Date.now()}`;
  const ride = http.post(`${baseURL}/rides`, ridePayload(), jsonParams(rider, key));
  check(ride, { 'ride created': (response) => response.status === 201 });
  if (ride.status !== 201) return;
  const rideID = ride.json('id');
  const first = http.post(`${baseURL}/rides/${rideID}/cancel`, null, jsonParams(rider));
  check(first, { 'cancel succeeds': (response) => response.status === 200 });
  const second = http.post(`${baseURL}/rides/${rideID}/cancel`, null, jsonParams(rider));
  check(second, { 'duplicate cancel is rejected safely': (response) => response.status === 409 || response.status === 200 });
}
