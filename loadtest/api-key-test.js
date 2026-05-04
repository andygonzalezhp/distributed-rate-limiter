import http from "k6/http";
import { check } from "k6";

export const options = {
  vus: 1,
  iterations: 24,
};

export default function () {
  const user = __ITER < 12 ? "user-123" : "user-456";

  const res = http.get("http://localhost:8080/hello", {
    headers: {
      "X-API-Key": user,
    },
  });

  check(res, {
    "status is 200 or 429": (r) => r.status === 200 || r.status === 429,
    "has client header": (r) => r.headers["X-Ratelimit-Client"] !== undefined,
  });

  console.log(
    `${res.status} client=${res.headers["X-Ratelimit-Client"]} remaining=${res.headers["X-Ratelimit-Remaining"]}`
  );
}