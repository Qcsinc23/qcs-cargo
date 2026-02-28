import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 25 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.02'],
    http_req_duration: ['p(95)<750'],
  },
};

export default function () {
  const health = http.get(`${BASE_URL}/api/v1/health`);
  check(health, {
    'health status is 200': (r) => r.status === 200,
  });

  const calculator = http.get(
    `${BASE_URL}/api/v1/calculator?destination=guyana&weight=5&l=12&w=8&h=6`
  );
  check(calculator, {
    'calculator status is 200': (r) => r.status === 200,
    'calculator has total': (r) => r.json('data.total') !== null,
  });

  const tracking = http.get(`${BASE_URL}/api/v1/track/LOADTEST-NOT-FOUND`);
  check(tracking, {
    'track endpoint responds expected miss': (r) => r.status === 404,
  });

  sleep(1);
}
