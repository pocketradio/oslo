import http from 'k6/http';
import { check } from 'k6';
import { baseURL, jsonParams, locationPayload, loginOrRegister } from './lib/api.js';

export const options = {
  scenarios: {
    location_updates: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RATE || 10),
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: Number(__ENV.PRE_ALLOCATED_VUS || 5),
      maxVUs: Number(__ENV.MAX_VUS || 50),
    },
  },
};

let token;

export default function () {
  token = token || loginOrRegister('driver', `location-driver-${__VU}`);

  const response = http.put(
    `${baseURL}/drivers/location`,
    locationPayload((__VU % 10) * 0.0001),
    jsonParams(token),
  );
  check(response, { 'location update succeeds': (result) => result.status === 204 });
}
