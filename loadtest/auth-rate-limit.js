import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  scenarios: {
    authBurst: {
      executor: 'constant-arrival-rate',
      rate: 5,
      timeUnit: '1s',
      duration: '1m',
      preAllocatedVUs: 10,
      maxVUs: 30,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
  },
};

export default function () {
  const nonce = `${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
  const email = `k6-${nonce}@example.com`;
  const payload = JSON.stringify({ email });

  const response = http.post(`${BASE_URL}/api/v1/auth/magic-link/request`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(response, {
    'auth endpoint responds 200 or 429': (r) => r.status === 200 || r.status === 429,
  });

  sleep(0.2);
}
