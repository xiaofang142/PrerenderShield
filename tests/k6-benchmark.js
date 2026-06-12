// Prerender Shield K6 压力测试脚本
// 使用方法: k6 run k6-benchmark.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// 自定义指标
const errorRate = new Rate('errors');
const latency = new Trend('latency');

// 测试配置
export const options = {
  // 场景配置
  scenarios: {
    // 场景1: 固定并发数
    constant_load: {
      executor: 'constant-vus',
      vus: 50,
      duration: '60s',
    },
    // 场景2: 逐步增加并发
    ramp_up: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: 20 },
        { duration: '20s', target: 50 },
        { duration: '20s', target: 100 },
        { duration: '20s', target: 50 },
        { duration: '20s', target: 0 },
      ],
    },
  },
  // 阈值配置
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
    errors: ['rate<0.01'],
  },
};

// 基础 URL
const BASE_URL = __ENV.BASE_URL || 'http://localhost:9598';

// 测试数据
let authToken = null;

// 初始化函数
export function setup() {
  // 登录获取 token
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: 'admin',
    password: '123456',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(loginRes, {
    'login successful': (r) => r.status === 200,
  });

  if (loginRes.status === 200) {
    const body = JSON.parse(loginRes.body);
    authToken = body.data?.token;
  }

  return { token: authToken };
}

// 主测试函数
export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
  };

  if (data.token) {
    headers['Authorization'] = `Bearer ${data.token}`;
  }

  // 测试1: 健康检查
  group('Health Check', () => {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/health`);
    const duration = Date.now() - start;
    
    latency.add(duration);
    
    check(res, {
      'health check status 200': (r) => r.status === 200,
      'health check response time < 100ms': (r) => r.timings.duration < 100,
    }) || errorRate.add(1);
  });

  // 测试2: 登录接口
  group('Login', () => {
    const start = Date.now();
    const res = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
      username: 'admin',
      password: '123456',
    }), { headers: { 'Content-Type': 'application/json' } });
    const duration = Date.now() - start;
    
    latency.add(duration);
    
    check(res, {
      'login status 200': (r) => r.status === 200,
      'login response time < 200ms': (r) => r.timings.duration < 200,
    }) || errorRate.add(1);
  });

  // 测试3: 获取站点列表
  if (data.token) {
    group('Get Sites', () => {
      const start = Date.now();
      const res = http.get(`${BASE_URL}/api/v1/sites`, { headers });
      const duration = Date.now() - start;
      
      latency.add(duration);
      
      check(res, {
        'get sites status 200': (r) => r.status === 200,
        'get sites response time < 100ms': (r) => r.timings.duration < 100,
      }) || errorRate.add(1);
    });

    // 测试4: 获取系统配置
    group('Get System Config', () => {
      const start = Date.now();
      const res = http.get(`${BASE_URL}/api/v1/system/config`, { headers });
      const duration = Date.now() - start;
      
      latency.add(duration);
      
      check(res, {
        'get config status 200': (r) => r.status === 200,
        'get config response time < 100ms': (r) => r.timings.duration < 100,
      }) || errorRate.add(1);
    });

    // 测试5: 获取监控数据
    group('Get Monitoring Stats', () => {
      const start = Date.now();
      const res = http.get(`${BASE_URL}/api/v1/monitoring/stats`, { headers });
      const duration = Date.now() - start;
      
      latency.add(duration);
      
      check(res, {
        'get monitoring status 200': (r) => r.status === 200,
        'get monitoring response time < 200ms': (r) => r.timings.duration < 200,
      }) || errorRate.add(1);
    });

    // 测试6: 获取预热统计
    group('Get Preheat Stats', () => {
      const start = Date.now();
      const res = http.get(`${BASE_URL}/api/v1/preheat/stats`, { headers });
      const duration = Date.now() - start;
      
      latency.add(duration);
      
      check(res, {
        'get preheat status 200': (r) => r.status === 200,
        'get preheat response time < 100ms': (r) => r.timings.duration < 100,
      }) || errorRate.add(1);
    });
  }

  // 测试7: 404 页面
  group('404 Page', () => {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/nonexistent`);
    const duration = Date.now() - start;
    
    latency.add(duration);
    
    check(res, {
      '404 page status 404': (r) => r.status === 404,
    }) || errorRate.add(1);
  });

  sleep(0.1);
}

// 清理函数
export function teardown(data) {
  // 可以在这里执行清理操作
  console.log('Test completed');
}

// 自定义摘要
export function handleSummary(data) {
  const summary = {
    timestamp: new Date().toISOString(),
    metrics: {
      http_reqs: data.metrics.http_reqs?.values?.count || 0,
      http_req_duration: {
        avg: data.metrics.http_req_duration?.values?.avg || 0,
        min: data.metrics.http_req_duration?.values?.min || 0,
        max: data.metrics.http_req_duration?.values?.max || 0,
        p90: data.metrics.http_req_duration?.values?.['p(90)'] || 0,
        p95: data.metrics.http_req_duration?.values?.['p(95)'] || 0,
        p99: data.metrics.http_req_duration?.values?.['p(99)'] || 0,
      },
      http_req_failed: data.metrics.http_req_failed?.values?.rate || 0,
      errors: data.metrics.errors?.values?.rate || 0,
    },
    thresholds: {},
  };

  // 检查阈值
  for (const [name, threshold] of Object.entries(data.thresholds || {})) {
    summary.thresholds[name] = threshold.ok ? 'PASSED' : 'FAILED';
  }

  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'summary.json': JSON.stringify(summary, null, 2),
  };
}

function textSummary(data, options) {
  let output = '\n';
  output += '========================================\n';
  output += '  Prerender Shield 压力测试结果\n';
  output += '========================================\n\n';
  
  output += `总请求数: ${data.metrics.http_reqs?.values?.count || 0}\n`;
  output += `平均响应时间: ${(data.metrics.http_req_duration?.values?.avg || 0).toFixed(2)}ms\n`;
  output += `最小响应时间: ${(data.metrics.http_req_duration?.values?.min || 0).toFixed(2)}ms\n`;
  output += `最大响应时间: ${(data.metrics.http_req_duration?.values?.max || 0).toFixed(2)}ms\n`;
  output += `P90 响应时间: ${(data.metrics.http_req_duration?.values?.['p(90)'] || 0).toFixed(2)}ms\n`;
  output += `P95 响应时间: ${(data.metrics.http_req_duration?.values?.['p(95)'] || 0).toFixed(2)}ms\n`;
  output += `P99 响应时间: ${(data.metrics.http_req_duration?.values?.['p(99)'] || 0).toFixed(2)}ms\n`;
  output += `失败率: ${((data.metrics.http_req_failed?.values?.rate || 0) * 100).toFixed(2)}%\n`;
  
  output += '\n========================================\n';
  output += '  阈值检查\n';
  output += '========================================\n\n';
  
  for (const [name, threshold] of Object.entries(data.thresholds || {})) {
    output += `${name}: ${threshold.ok ? '✅ PASSED' : '❌ FAILED'}\n`;
  }
  
  return output;
}
