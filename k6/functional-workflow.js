import http from 'k6/http';
import { check, sleep } from 'k6';
import { baseURL, jsonParams, locationPayload, loginOrRegister, ridePayload } from './lib/api.js';

export const options = {
  scenarios: {
    workflow: { executor: 'per-vu-iterations', vus: Number(__ENV.VUS || 1), iterations: 1, maxDuration: __ENV.DURATION || '2m' },
  },
};

function expect(response, status, label) {
  return check(response, { [`${label} returns ${status}`]: (result) => result.status === status });
}

export default function () {
  const suffix = `${__VU}-${Date.now()}`;
  const rider = loginOrRegister('rider', `workflow-rider-${suffix}`);
  const driver = loginOrRegister('driver', `workflow-driver-${suffix}`);

  const fare = http.post(`${baseURL}/rides/fare-estimate`, ridePayload(), jsonParams(rider));
  expect(fare, 200, 'fare estimate');

  const location = http.put(`${baseURL}/drivers/location`, locationPayload(), jsonParams(driver));
  expect(location, 204, 'driver location');

  const key = `workflow-${suffix}`;
  const first = http.post(`${baseURL}/rides`, ridePayload(), jsonParams(rider, key));
  expect(first, 201, 'ride creation');
  if (first.status !== 201) return;

  const rideID = first.json('id');
  const duplicate = http.post(`${baseURL}/rides`, ridePayload(), jsonParams(rider, key));
  expect(duplicate, 201, 'idempotent duplicate');
  check(duplicate, { 'duplicate returns original ride': (response) => response.json('id') === rideID });

  let offerID = null;
  for (let attempt = 0; attempt < 20 && !offerID; attempt += 1) {
    const offers = http.get(`${baseURL}/drivers/offers`, jsonParams(driver));
    expect(offers, 200, 'pending offers');
    if (offers.status === 200 && offers.json().length > 0) offerID = offers.json()[0].id;
    if (!offerID) sleep(0.5);
  }
  check({ offerID }, { 'offer is eventually delivered': (value) => value.offerID !== null });
  if (!offerID) return;

  const accepted = http.post(`${baseURL}/drivers/offers/${offerID}/accept`, null, jsonParams(driver));
  expect(accepted, 204, 'offer acceptance');
  expect(http.post(`${baseURL}/drivers/rides/${rideID}/arrive`, null, jsonParams(driver)), 200, 'arrival');
  expect(http.post(`${baseURL}/drivers/rides/${rideID}/start`, null, jsonParams(driver)), 200, 'start');
  expect(http.post(`${baseURL}/drivers/rides/${rideID}/complete`, null, jsonParams(driver)), 200, 'completion');

  const finalRide = http.get(`${baseURL}/rides/${rideID}`, jsonParams(rider));
  expect(finalRide, 200, 'final ride read');
  check(finalRide, { 'ride is completed': (response) => response.json('status') === 'completed' });
}
