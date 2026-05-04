import http from "k6/http";
import { check } from "k6";

export const options = {
  scenarios: {
    ramping_load: {
      executor: "ramping-vus",
      stages: [
        { duration: "10s", target: 100 },
        { duration: "20s", target: 300 },
        { duration: "10s", target: 500 },
        { duration: "10s", target: 0 },
      ],
    },
  },
};

export default function () {
  const res = http.get("http://localhost:8080/hello");

  check(res, {
    "status is 200": (r) => r.status === 200,
    "has gateway instance": (r) => r.headers["X-Gateway-Instance"] !== undefined,
    "has rate limit remaining": (r) => r.headers["X-Ratelimit-Remaining"] !== undefined,
  });
}