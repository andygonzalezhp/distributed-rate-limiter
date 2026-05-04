import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  scenarios: {
    constant_load: {
      executor: "constant-vus",
      vus: 50,
      duration: "30s",
    },
  },
};

export default function () {
  const res = http.get("http://localhost:8080/hello");

  check(res, {
    "status is 200 or 429": (r) => r.status === 200 || r.status === 429,
    "gateway instance header exists": (r) => r.headers["X-Gateway-Instance"] !== undefined,
  });

  sleep(0.01);
}