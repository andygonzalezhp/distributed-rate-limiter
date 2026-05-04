import http from "k6/http";
import { check } from "k6";

export const options = {
  vus: 1,
  iterations: 15,
};

export default function () {
  let path = "/hello";

  if (__ITER < 5) {
    path = "/admin";
  } else if (__ITER < 10) {
    path = "/hello";
  } else {
    path = "/search";
  }

  const res = http.get(`http://localhost:8080${path}`, {
    headers: {
      "X-API-Key": "route-test-user",
    },
  });

  check(res, {
    "status is 200 or 429": (r) => r.status === 200 || r.status === 429,
    "has route header": (r) => r.headers["X-Ratelimit-Route"] !== undefined,
    "has decision header": (r) => r.headers["X-Ratelimit-Decision"] !== undefined,
  });

  console.log(
    `${res.status} path=${path} route=${res.headers["X-Ratelimit-Route"]} remaining=${res.headers["X-Ratelimit-Remaining"]} decision=${res.headers["X-Ratelimit-Decision"]}`
  );
}