import http from 'k6/http';
import { check, group } from 'k6';
import { Counter } from 'k6/metrics';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

export const options = {
  scenarios: {
    flash_sale_spike: {
      executor: 'shared-iterations',
      vus: 50,
      iterations: 100,
      maxDuration: '20s',
    },
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';
const ordersPlacedCounter = new Counter('orders_placed_successfully');
const oversellPreventedCounter = new Counter('oversell_prevented_conflict');

export default function () {
  const token = __ENV.JWT_TOKEN || 'test-jwt-token';
  const idempotencyKey = uuidv4();

  // Limited product: Cold Brew Nitro (Stock = 10 in seeds)
  const productID = 'b0000000-0000-0000-0000-000000000003';

  const payload = JSON.stringify({
    items: [
      {
        product_id: productID,
        quantity: 1,
      },
    ],
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
      'Idempotency-Key': idempotencyKey,
    },
  };

  const res = http.post(`${BASE_URL}/api/v1/orders`, payload, params);

  if (res.status === 201) {
    ordersPlacedCounter.add(1);
  } else if (res.status === 409) {
    oversellPreventedCounter.add(1);
  }

  check(res, {
    'status is 201 Created or 409 Conflict': (r) => r.status === 201 || r.status === 409,
  });
}

