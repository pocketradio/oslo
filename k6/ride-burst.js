import http from 'k6/http';
import { check } from 'k6';
import { baseURL, jsonParams, loginOrRegister, ridePayload } from './lib/api.js';

export const options = {
  scenarios: {
    ride_burst: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RATE || 5),
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: Number(__ENV.PRE_ALLOCATED_VUS || 5),
      maxVUs: Number(__ENV.MAX_VUS || 50),
    },
  },
};

let token;

export default function () {
  token = token || loginOrRegister('rider', `burst-rider-${__VU}`);

  const response = http.post(
    `${baseURL}/rides`,
    ridePayload(),
    jsonParams(token, `burst-${__VU}-${__ITER}-${Date.now()}`),
  );
  check(response, {
    'ride burst request is accepted': (result) => result.status === 201 || result.status === 409,
  });
}
