# WAF Module Test Plan

## 1. API Testing

### 1.1 Authentication
- **Endpoint**: `POST /api/v1/auth/login`
- **Input**: `{"username": "admin", "password": "..."}`
- **Expected**: 200 OK, returns JWT token.

### 1.2 WAF Status
- **Endpoint**: `GET /api/v1/firewall/status`
- **Headers**: `Authorization: Bearer <token>`
- **Expected**: 200 OK, returns global WAF status.

### 1.3 Site WAF Configuration
- **Endpoint**: `GET /api/v1/firewall/config/:site_id`
- **Expected**: 200 OK, returns specific site WAF configuration.

- **Endpoint**: `POST /api/v1/firewall/config/:site_id`
- **Input**: JSON body with WAF settings (enabled, rules, etc.)
- **Expected**: 200 OK, configuration updated.

### 1.4 WAF Logs
- **Endpoint**: `GET /api/v1/firewall/logs`
- **Params**: `site_id`, `page`, `page_size`
- **Expected**: 200 OK, returns list of blocked requests.

## 2. UI Testing

### 2.1 Dashboard
- Verify WAF statistics (blocked requests count).
- Check for console errors.

### 2.2 Sites Management
- Verify "WAF Config" button navigates to the correct settings page.
- Verify site list loads correctly.

### 2.3 WAF Settings Page
- Verify form loads with current configuration.
- Verify "Save" button updates the configuration.
- Verify "Return" button navigates back to sites list.

## 3. Log Collection
- Collect backend logs (`backend.log` or stdout).
- Collect frontend console logs.
