// testing full ride workflow 



import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  baseURL,
  checkStatus,
  jsonParams,
  loginOrRegister,
  ridePayload,
} from './lib/api.js';

export const options = {
  vus: 1, // virtual user count
  iterations: 1, // fn execution execution count
};

export default function () {

  // health chk
  const health = http.get(`${baseURL}/healthz`);

  check(health, { 'health check is ok': (response) => checkStatus(response, 200) });

  // create and authenticate a rider through the shared api helper.
  const riderToken = loginOrRegister('rider', 'smoke-rider');

  // estimate the fare without creating a ride.
  const fare = http.post(
    `${baseURL}/rides/fare-estimate`,
    ridePayload(),
    jsonParams(riderToken),
  );

  check(fare, { 'fare estimate succeeds': (response) => checkStatus(response, 200) });

  // create a ride with a unique idempotency key. date.now will be the key. 
  const ride = http.post(
    `${baseURL}/rides`,
    ridePayload(), // this will return the JSON str containing pickup and dest coords
    jsonParams(riderToken, `smoke-${Date.now()}`),// returns headers for json content type, auth, and idempotency.
  );

  check(ride, { 'ride creation succeeds': (response) => checkStatus(response, 201) });

  if (ride.status === 201) {
    const rideID = ride.json('id');
    const found = http.get(`${baseURL}/rides/${rideID}`, jsonParams(riderToken));

    check(found, { 'created ride is readable': (response) => checkStatus(response, 200) });

    // read and cancel the smoke-test ride so it does not remain active.
    const cancelled = http.post(
      `${baseURL}/rides/${rideID}/cancel`,
      null,
      jsonParams(riderToken),
    );

    check(cancelled, { 'ride cancellation succeeds': (response) => checkStatus(response, 200) });
  }
}
