import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

export const options = {
  stages: [
    { duration: '10s', target: 20 }, // Ramp up to 20 users
    { duration: '30s', target: 50 }, // Stay at 50 users
    { duration: '10s', target: 0 },  // Ramp down to 0
  ],
  thresholds: {
    http_req_duration: ['p(95)<200'], // 95% of requests should be below 200ms
    http_req_failed: ['rate<0.05'],    // Non-429 error rate below 5%
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

export default function () {
  // 1. Health check
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health returns 200': (r) => r.status === 200,
  });

  // 2. Fetch Catalog
  const productsRes = http.get(`${BASE_URL}/api/v1/products`);
  check(productsRes, {
    'products catalog returns 200': (r) => r.status === 200,
    'has products': (r) => JSON.parse(r.body).data.length > 0,
  });

  // 3. Place Order with Idempotency Key
  const idempotencyKey = uuidv4();
  const payload = JSON.stringify({
    items: [
      {
        product_id: 'b0000000-0000-0000-0000-000000000001',
        quantity: 1,
      },
    ],
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer ' + (__ENV.JWT_TOKEN || 'test-jwt-token'),
      'Idempotency-Key': idempotencyKey,
    },
  };

  const orderRes = http.post(`${BASE_URL}/api/v1/orders`, payload, params);
  check(orderRes, {
    'order responded with 201 or 409 or 429': (r) => [201, 409, 429].includes(r.status),
  });

  sleep(0.5);
}

