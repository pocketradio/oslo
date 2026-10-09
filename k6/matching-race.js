import http from 'k6/http';
import { check, sleep } from 'k6';
import { baseURL, jsonParams, locationPayload, loginOrRegister, ridePayload } from './lib/api.js';

export const options = {
  scenarios: {
    matching_race: {
      executor: 'per-vu-iterations',
      vus: Number(__ENV.VUS || 10),
      iterations: 1,
      maxDuration: __ENV.DURATION || '30s',
    },
  },
};

let riderToken;

export function setup() {
  const drivers = [];
  const driverCount = Number(__ENV.DRIVERS || 2);

  for (let index = 0; index < driverCount; index += 1) {
    drivers.push(loginOrRegister('driver', `race-driver-${index}`));
  }

  return { drivers };
}

export default function (data) {
  riderToken = riderToken || loginOrRegister('rider', `race-rider-${__VU}`);
  const driverToken = data.drivers[(__VU - 1) % data.drivers.length];

  const location = http.put(
    `${baseURL}/drivers/location`,
    locationPayload((__VU % 3) * 0.0001),
    jsonParams(driverToken),
  );
  check(location, { 'race driver is available': (response) => response.status === 204 });

  const ride = http.post(
    `${baseURL}/rides`,
    ridePayload(),
    jsonParams(riderToken, `race-${__VU}-${__ITER}-${Date.now()}`),
  );
  check(ride, { 'race ride is created': (response) => response.status === 201 });

  if (ride.status === 201) {
    const rideID = ride.json('id');
    sleep(1);
    const status = http.get(`${baseURL}/rides/${rideID}`, jsonParams(riderToken));
    check(status, { 'race ride remains readable': (response) => response.status === 200 });
  }
}
