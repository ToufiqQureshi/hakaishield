import http from "k6/http";
import { check, sleep } from "k6";

const baseUrl = __ENV.BASE_URL || "https://www.theshalimarhotel.com/";

export const options = {
  vus: 1,
  iterations: 10,
  thresholds: {
    http_req_failed: ["rate==0"],
    http_req_duration: ["p(95)<2000"],
  },
};

export default function () {
  const response = http.get(baseUrl);
  check(response, {
    "returns a successful status": (res) => res.status >= 200 && res.status < 400,
  });
  sleep(1);
}