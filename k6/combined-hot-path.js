import http from 'k6/http';
import { check, sleep } from 'k6';
import { baseURL, jsonParams, locationPayload, loginOrRegister, ridePayload } from './lib/api.js';

const rideRate = Number(__ENV.RIDE_RATE || 20);
const locationRate = Number(__ENV.LOCATION_RATE || 100);
const duration = __ENV.DURATION || '10m';

export const options = {
  setupTimeout: __ENV.SETUP_TIMEOUT || '5m',
  scenarios: {
    rides: {
      executor: 'constant-arrival-rate',
      rate: rideRate,
      timeUnit: '1s',
      duration,
      preAllocatedVUs: Number(__ENV.RIDE_VUS || 20),
      maxVUs: Number(__ENV.RIDE_MAX_VUS || 100),
      exec: 'createRide',
    },
    locations: {
      executor: 'constant-arrival-rate',
      rate: locationRate,
      timeUnit: '1s',
      duration,
      preAllocatedVUs: Number(__ENV.LOCATION_VUS || 20),
      maxVUs: Number(__ENV.LOCATION_MAX_VUS || 150),
      exec: 'updateLocation',
    },
    status: {
      executor: 'constant-vus',
      vus: Number(__ENV.STATUS_VUS || 5),
      duration,
      exec: 'readRideStatus',
    },
  },
};

function provision(role, label) {
  const token = loginOrRegister(role, label);
  sleep(Number(__ENV.SETUP_DELAY || 0.3));
  return token;
}

function clientParams(token, clientIndex, idempotencyKey) {
  const params = jsonParams(token, idempotencyKey);
  const sourceIndex = clientIndex % 63000;
  params.headers['X-Forwarded-For'] = `198.51.${Math.floor(sourceIndex / 250) + 1}.${(sourceIndex % 250) + 1}`;
  return params;
}

export function setup() {
  const riders = [];
  const statusRiders = [];
  const drivers = [];
  const riderCount = Number(__ENV.RIDER_TOKENS || 10);
  const driverCount = Number(__ENV.DRIVER_TOKENS || 10);

  for (let index = 0; index < riderCount; index += 1) {
    riders.push(provision('rider', `mixed-rider-${index}`));
  }
  for (let index = 0; index < Number(__ENV.STATUS_VUS || 5); index += 1) {
    statusRiders.push(provision('rider', `status-rider-${index}`));
  }
  for (let index = 0; index < driverCount; index += 1) {
    drivers.push(provision('driver', `mixed-driver-${index}`));
  }

  const statusRides = [];
  for (let index = 0; index < statusRiders.length; index += 1) {
    const response = http.post(
      `${baseURL}/rides`,
      ridePayload(),
      clientParams(statusRiders[index], index + 2000, `status-setup-${index}-${Date.now()}`),
    );
    if (response.status !== 201) throw new Error(`status setup ride failed with status ${response.status}`);
    statusRides.push(response.json('id'));
    sleep(Number(__ENV.SETUP_DELAY || 0.3));
  }

  return { riders, statusRiders, statusRides, drivers };
}

export function createRide(data) {
  const token = data.riders[(__VU * 1000 + __ITER) % data.riders.length];
  const response = http.post(
    `${baseURL}/rides`,
    ridePayload(),
    clientParams(token, __VU * 1000 + __ITER, `mixed-${__VU}-${__ITER}-${Date.now()}`),
  );
  check(response, {
    'mixed ride accepted': (result) => result.status === 201 || result.status === 409,
  });
}

export function updateLocation(data) {
  const token = data.drivers[(__VU - 1) % data.drivers.length];
  const response = http.put(
    `${baseURL}/drivers/location`,
    locationPayload((__VU % 20) * 0.00001),
    clientParams(token, __VU * 1000 + __ITER + 1000),
  );
  check(response, { 'mixed location accepted': (result) => result.status === 204 });
}

export function readRideStatus(data) {
  const rider = data.statusRiders[(__VU - 1) % data.statusRiders.length];
  const rideID = data.statusRides[(__VU - 1) % data.statusRides.length];
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const response = http.get(`${baseURL}/rides/${rideID}`, clientParams(rider, __VU * 1000 + __ITER + 2000));
    check(response, { 'ride status readable': (result) => result.status === 200 });
    sleep(1);
  }
}
