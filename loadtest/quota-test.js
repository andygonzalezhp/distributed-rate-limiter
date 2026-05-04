import http from "k6/http";
import { check } from "k6";

export const options = {
  vus: 1,
  iterations: 20,
};

export default function () {
  const res = http.get("http://localhost:8080/hello");

  check(res, {
    "status is 200 or 429": (r) => r.status === 200 || r.status === 429,
  });

  console.log(`${res.status} instance=${res.headers["X-Gateway-Instance"]} remaining=${res.headers["X-Ratelimit-Remaining"]}`);
}