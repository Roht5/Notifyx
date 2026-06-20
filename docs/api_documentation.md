# Notifyx API Specification & Documentation

Welcome to the **Notifyx API Documentation**. Notifyx is a high-throughput, multi-tenant notification engine written in Go. The platform exposes a RESTful JSON API for tenant administration, template management, notification dispatch (immediate, scheduled, and batch), and real-time delivery via WebSockets.

---

## 🔒 1. Authentication & Common Standards

### 1.1 Authentication Scheme
Notifyx distinguishes between **Tenant Admin routes** (unauthenticated for portfolio/demonstration purposes) and **Notification/Template ingestion routes** (strictly authenticated).

* **Header Authentication**: For ingestion APIs under `/api/v1/notifications` and `/api/v1/templates`, requests must include the `X-API-Key` header.
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  ```
* **Query Parameter Authentication**: Browsers' native WebSocket APIs cannot set custom HTTP headers on connection handshakes. Therefore, the WebSocket connection endpoint `/ws/connect` accepts credentials via query parameters (`apiKey`, `tenantId`, and `userId`).
* **API Key Format**: Newly generated API keys are prefixed with `nfx_` followed by a 64-character cryptographically random hex string (total length 68 characters).

### 1.2 Common Error Response Format
All endpoint validation, database, auth, and gateway errors return a consistent error body with appropriate HTTP status codes:

```json
{
  "error": "Detailed description of the error / validation failure"
}
```

* **Common HTTP Status Codes**:
  * `200 OK` – Successful retrieval or idempotent duplicate message match.
  * `201 Created` – Successful resource creation.
  * `204 No Content` – Successful deletion.
  * `400 Bad Request` – Malformed JSON body or invalid UUID query parameters.
  * `401 Unauthorized` – Missing `X-API-Key` header or invalid credentials.
  * `403 Forbidden` – Attempted action is not configured or forbidden (e.g., channel is disabled for the tenant).
  * `404 Not Found` – Requested resource (tenant, template, notification) does not exist.
  * `409 Conflict` – Resource name conflict (e.g. duplicate template name for the same tenant).
  * `422 Unprocessable Entity` – Semantic payload verification failed (e.g., missing required fields like `recipient_email` for the `email` channel).
  * `500 Internal Server Error` – Database failure, message broker publish failure, or internal crash.

---

## 🏢 2. Tenant Administration APIs (Public / Super-Admin)

These endpoints are open, enabling dashboard user interfaces or administrative services to manage tenants, configure their allowed channels, adjust rate limits, view tenant-specific analytical summaries, and generate API keys.

---

### 2.1 Create Tenant
Create a new tenant workspace in Notifyx. Doing so automatically allocates a default global rate cap and issues the tenant's initial API key.

* **Endpoint**: `POST /api/v1/tenants`
* **Request Header**: `Content-Type: application/json`
* **Request Body**:
  ```json
  {
    "name": "Acme Corporation"
  }
  ```
* **Validation Constraints**:
  * `name` (string, required): Must not be empty.
* **Response (201 Created)**:
  ```json
  {
    "id": "1e7be846-9d32-4752-965a-8b839bcaef16",
    "name": "Acme Corporation",
    "api_key": "nfx_b5569cc06a88b50f757f49352dc0029b35b62b1a99d98cf0f214dbdb87e74287"
  }
  ```
  > [!IMPORTANT]
  > The `api_key` string is only returned once during creation. Notifyx hashes the key using SHA-256 for future lookups and cannot retrieve or display this raw string again.

---

### 2.2 List Tenants
Fetch all registered tenants in the system.

* **Endpoint**: `GET /api/v1/tenants`
* **Response (200 OK)**:
  ```json
  [
    {
      "id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "name": "Acme Corporation",
      "global_rate_cap": 300,
      "created_at": "2026-06-20T07:15:00Z"
    }
  ]
  ```

---

### 2.3 Get Tenant Details
Retrieve a single tenant's metadata using their UUID.

* **Endpoint**: `GET /api/v1/tenants/:id`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Response (200 OK)**:
  ```json
  {
    "id": "1e7be846-9d32-4752-965a-8b839bcaef16",
    "name": "Acme Corporation",
    "global_rate_cap": 300,
    "created_at": "2026-06-20T07:15:00Z"
  }
  ```

---

### 2.4 Update Tenant Details
Rename a tenant workspace.

* **Endpoint**: `PUT /api/v1/tenants/:id`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Request Body**:
  ```json
  {
    "name": "Acme Corp International"
  }
  ```
* **Validation Constraints**:
  * `name` (string, required): Must not be empty.
* **Response (200 OK)**:
  ```json
  {
    "id": "1e7be846-9d32-4752-965a-8b839bcaef16",
    "name": "Acme Corp International",
    "global_rate_cap": 300,
    "created_at": "2026-06-20T07:15:00Z"
  }
  ```

---

### 2.5 Delete Tenant
Permanently remove a tenant. This is a cascading delete that purges all child records including API keys, template definitions, notification history logs, and analytics.

* **Endpoint**: `DELETE /api/v1/tenants/:id`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Response (204 No Content)**: Empty body.

---

### 2.6 Configure Channel Enablement
Toggle delivery channels (email, push, sms, inapp) for a tenant. A tenant is blocked from sending notifications through a channel unless it is explicitly enabled.

* **Endpoint**: `PUT /api/v1/tenants/:id/channels`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Request Body**:
  ```json
  {
    "channels": [
      { "channel": "email", "enabled": true },
      { "channel": "inapp", "enabled": true },
      { "channel": "sms", "enabled": false }
    ]
  }
  ```
* **Validation Constraints**:
  * `channel` (string, required): Must be one of `email`, `push`, `sms`, or `inapp`.
  * `enabled` (boolean, required): Sets status.
* **Response (200 OK)**:
  ```json
  [
    {
      "id": "2b31f7a2-11c5-4318-ae38-782a93b482bc",
      "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "channel": "email",
      "enabled": true,
      "created_at": "2026-06-20T07:16:00Z"
    },
    {
      "id": "cfa53b49-74d3-48ee-aef9-1a48c9df3c92",
      "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "channel": "inapp",
      "enabled": true,
      "created_at": "2026-06-20T07:16:00Z"
    },
    {
      "id": "b182d3cd-711e-450f-90db-33ee72bc1944",
      "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "channel": "sms",
      "enabled": false,
      "created_at": "2026-06-20T07:16:00Z"
    }
  ]
  ```

---

### 2.7 Retrieve Channel Configurations
Fetch the current opt-in configurations for all channels (email, push, sms, inapp) assigned to a tenant.

* **Endpoint**: `GET /api/v1/tenants/:id/channels`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Response (200 OK)**:
  ```json
  [
    {
      "id": "2b31f7a2-11c5-4318-ae38-782a93b482bc",
      "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "channel": "email",
      "enabled": true,
      "created_at": "2026-06-20T07:16:00Z"
    },
    {
      "id": "cfa53b49-74d3-48ee-aef9-1a48c9df3c92",
      "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "channel": "inapp",
      "enabled": true,
      "created_at": "2026-06-20T07:16:00Z"
    }
  ]
  ```

---

### 2.8 Configure Rate Limits
Modify the tenant-wide global sliding-window rate cap, and define channel-specific throughput thresholds.

* **Endpoint**: `PUT /api/v1/tenants/:id/rate-limits`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Request Body**:
  ```json
  {
    "global_cap": 500,
    "rate_limits": [
      { "channel": "email", "max_per_min": 50 },
      { "channel": "inapp", "max_per_min": 200 }
    ]
  }
  ```
* **Validation Constraints**:
  * `global_cap` (integer, optional): If supplied, must be greater than `0`. Represents aggregate sends permitted per minute across all channels.
  * `rate_limits` (array, required):
    * `channel` (string, required): Must be one of `email`, `push`, `sms`, or `inapp`.
    * `max_per_min` (integer, required): Must be greater than `0`. Represents maximum sends permitted per minute on that channel.
* **Response (200 OK)**:
  ```json
  {
    "global_cap": 500,
    "rate_limits": [
      {
        "id": "a67123d4-1a3b-4890-8dcf-44bfa201b1b3",
        "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
        "channel": "email",
        "max_per_min": 50,
        "created_at": "2026-06-20T07:18:00Z"
      },
      {
        "id": "e2ba7e1d-8cd4-4061-a083-d2d41b6973ba",
        "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
        "channel": "inapp",
        "max_per_min": 200,
        "created_at": "2026-06-20T07:18:00Z"
      }
    ]
  }
  ```

---

### 2.9 Retrieve Rate Limit Configurations
Fetch the current global and channel-specific rate limits configured for a tenant.

* **Endpoint**: `GET /api/v1/tenants/:id/rate-limits`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Response (200 OK)**:
  ```json
  {
    "global_cap": 500,
    "rate_limits": [
      {
        "id": "a67123d4-1a3b-4890-8dcf-44bfa201b1b3",
        "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
        "channel": "email",
        "max_per_min": 50,
        "created_at": "2026-06-20T07:18:00Z"
      },
      {
        "id": "e2ba7e1d-8cd4-4061-a083-d2d41b6973ba",
        "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
        "channel": "inapp",
        "max_per_min": 200,
        "created_at": "2026-06-20T07:18:00Z"
      }
    ]
  }
  ```

---

### 2.10 Retrieve Tenant Analytics
Fetch aggregated volumes, delivery success rates, and Dead Letter Queue (DLQ) event trends.

* **Endpoint**: `GET /api/v1/tenants/:id/analytics`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Query Parameters**:
  * `from` (string, optional): RFC3339 format (e.g. `2026-06-01T00:00:00Z`). Defaults to 30 days before `to`.
  * `to` (string, optional): RFC3339 format (e.g. `2026-06-20T23:59:59Z`). Defaults to current time.
* **Response (200 OK)**:
  ```json
  {
    "summary": {
      "total_sent": 14500,
      "total_delivered": 14350,
      "total_failed": 150
    },
    "channels": [
      {
        "channel": "email",
        "sent": 4500,
        "delivered": 4480,
        "failed": 20,
        "delivery_rate": 0.9955
      },
      {
        "channel": "inapp",
        "sent": 10000,
        "delivered": 9870,
        "failed": 130,
        "delivery_rate": 0.987
      }
    ],
    "dlq_trend": [
      {
        "date": "2026-06-18T00:00:00Z",
        "count": 4
      },
      {
        "date": "2026-06-19T00:00:00Z",
        "count": 12
      },
      {
        "date": "2026-06-20T00:00:00Z",
        "count": 9
      }
    ],
    "from": "2026-05-21T07:20:00Z",
    "to": "2026-06-20T07:20:00Z"
  }
  ```

---

### 2.11 Issue/Rotate API Key
Create a supplementary API key for rotation purposes. A tenant can have multiple active keys concurrently, facilitating zero-downtime key rotation.

* **Endpoint**: `POST /api/v1/tenants/:id/keys`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Response (201 Created)**:
  ```json
  {
    "id": "99ea4012-70b1-4f9e-a89e-2dc3fb81283e",
    "api_key": "nfx_7a12bcfcd729f3eb81a17dcde2b0e6bf39ab887ef1c0e32bdfce92cfa7129532"
  }
  ```

---

### 2.12 List Tenant API Keys (Metadata)
Lists metadata for keys assigned to a tenant. The raw secret string and hash are omitted.

* **Endpoint**: `GET /api/v1/tenants/:id/keys`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
* **Response (200 OK)**:
  ```json
  [
    {
      "id": "16ac9a42-7a2e-4b47-b8a9-0b1a0cb82ac7",
      "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "created_at": "2026-06-20T07:15:00Z"
    },
    {
      "id": "99ea4012-70b1-4f9e-a89e-2dc3fb81283e",
      "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "created_at": "2026-06-20T07:21:00Z"
    }
  ]
  ```

---

### 2.13 Revoke API Key
Instantly invalidate and delete a specific API key.

* **Endpoint**: `DELETE /api/v1/tenants/:id/keys/:key_id`
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the tenant.
  * `key_id` (string/UUID, required): The unique identifier of the key metadata row.
* **Response (204 No Content)**: Empty body.

---

## ✉️ 3. Notification Management APIs (Authenticated)

These ingestion endpoints process, queue, and index notifications. They require the `X-API-Key` header corresponding to a registered tenant.

---

### 3.1 Send Immediate Notification
Accepts a single notification dispatch instruction, enforces rate limits, validates recipient bindings, renders templates, deduplicates via idempotency key, records audit footprints, and publishes to Kafka.

* **Endpoint**: `POST /api/v1/notifications/send`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  Content-Type: application/json
  ```
* **Request Body (Direct Payload Example)**:
  ```json
  {
    "channel": "email",
    "priority": "high",
    "recipient_email": "user@example.com",
    "subject": "System Warning",
    "body": "Your account storage has exceeded 90%.",
    "idempotency_key": "user-signup-12345",
    "metadata": {
      "user_id": "usr_99",
      "tier": "enterprise"
    }
  }
  ```
* **Request Body (Template Reference Example)**:
  ```json
  {
    "channel": "sms",
    "recipient_phone": "+15550199",
    "template_id": "d3b07384-d113-4c9e-bf11-739fbef53de4",
    "variables": {
      "name": "Jane",
      "otp": "489210"
    }
  }
  ```

* **Validation Rules**:
  * `channel` (string, required): Must be one of `email`, `push`, `sms`, or `inapp`.
  * `priority` (string, optional): Must be one of `critical`, `high`, `normal`, or `low`. Defaults to `normal`.
  * `idempotency_key` (string, optional): Max 255 characters. Used to guarantee only one notification triggers for a duplicate request.
  * **Recipient Validation by Channel**:
    * If `channel = "email"`, then `recipient_email` is required. `subject` is also required (if template is omitted).
    * If `channel = "sms"`, then `recipient_phone` is required.
    * If `channel = "push"`, then `recipient_token` is required.
    * If `channel = "inapp"`, then `recipient_id` is required.
  * **Templates**:
    * If `template_id` is specified, direct `subject` and `body` fields are ignored. Instead, Notifyx fetches the template and replaces any placeholders `{{variable_name}}` with values supplied in `variables`. The template's configured channel must match the payload's `channel`.

* **Response (201 Created - Fresh Message)**:
  ```json
  {
    "notification_id": "893c5dcd-9a8c-4a31-b76b-9c7de9abcf12",
    "status": "queued"
  }
  ```
  * Note: The response field `status` will read `"queued"` (message successfully published to the Kafka topic for dispatch) or `"queued_rate_limited"` (held in the database and local Redis queue, bypassing Kafka until a slot clears).

* **Response (200 OK - Idempotent Duplicate Match)**:
  If a matching `idempotency_key` is submitted within the deduplication window (default 24 hours), Notifyx returns the existing notification details without recreating or republishing the event.
  ```json
  {
    "notification_id": "893c5dcd-9a8c-4a31-b76b-9c7de9abcf12",
    "status": "delivered"
  }
  ```

---

### 3.2 Schedule Notification
Saves a notification with a `pending` status, scheduling it to be dispatched at a designated time. A background cron worker checks for scheduled messages every 30 seconds and publishes them to Kafka when they become due.

* **Endpoint**: `POST /api/v1/notifications/schedule`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  Content-Type: application/json
  ```
* **Request Body**:
  All fields matching the standard `/send` request body, plus:
  ```json
  {
    "channel": "push",
    "recipient_token": "fcm_token_12345",
    "body": "Good morning! Check your daily stats.",
    "scheduled_at": "2026-06-25T08:00:00Z"
  }
  ```
* **Validation Constraints**:
  * `scheduled_at` (string, required): Must be a valid future timestamp formatted according to RFC3339.
* **Response (201 Created)**:
  ```json
  {
    "notification_id": "00ac82e2-9b2f-48d9-a9a7-0e6fb12cd712",
    "status": "pending",
    "scheduled_at": "2026-06-25T08:00:00Z"
  }
  ```

---

### 3.3 Batch Send Notifications
Allows submitting a single template pattern or message body to multiple recipients. Notifyx processes the recipients concurrently, respecting connection pool safety thresholds.

* **Endpoint**: `POST /api/v1/notifications/batch`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  Content-Type: application/json
  ```
* **Request Body**:
  ```json
  {
    "channel": "email",
    "subject": "Newsletter Issue #12",
    "body": "Hi there, read our latest issue online!",
    "idempotency_key": "newsletter-june",
    "recipients": [
      { "recipient_email": "user1@example.com" },
      { "recipient_email": "user2@example.com" },
      { "recipient_email": "user3@example.com" }
    ]
  }
  ```
* **Validation Constraints**:
  * `recipients` (array, required): Must contain at least one recipient block.
  * `idempotency_key` (string, optional): If provided, Notifyx appends the index of the recipient to the key (e.g., `newsletter-june:0`, `newsletter-june:1`) to ensure each recipient is treated as a unique, independent transaction.
* **Response (201 Created)**:
  ```json
  {
    "results": [
      {
        "index": 0,
        "notification_id": "787c88b2-b118-4903-b0cd-a99f3bd0e123"
      },
      {
        "index": 1,
        "notification_id": "c1f7b882-9cb4-4909-bfa2-9bb2fc23ef15"
      },
      {
        "index": 2,
        "error": "failed to verify channel"
      }
    ]
  }
  ```

---

### 3.4 Retrieve Notification History
Allows querying historical notifications for the tenant, supporting filters and pagination.

* **Endpoint**: `GET /api/v1/notifications/history`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  ```
* **Query Parameters**:
  * `page` (integer, optional): Page index. Defaults to `1`.
  * `limit` (integer, optional): Results per page. Defaults to `20`, maximum `100`.
  * `channel` (string, optional): Filter by channel (`email`, `push`, `sms`, `inapp`).
  * `status` (string, optional): Filter by status (`pending`, `queued`, `queued_rate_limited`, `delivered`, `failed`).
  * `from` (string, optional): RFC3339 timestamp.
  * `to` (string, optional): RFC3339 timestamp.
  * `recipient` (string, optional): Search keyword matching any of the recipient fields (`recipient_id`, `recipient_email`, `recipient_phone`, `recipient_token`) via case-insensitive wildcard pattern matching.
* **Response (200 OK)**:
  ```json
  {
    "notifications": [
      {
        "id": "893c5dcd-9a8c-4a31-b76b-9c7de9abcf12",
        "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
        "channel": "email",
        "priority": "high",
        "status": "delivered",
        "recipient_email": "user@example.com",
        "subject": "System Warning",
        "body": "Your account storage has exceeded 90%.",
        "metadata": {
          "user_id": "usr_99",
          "tier": "enterprise"
        },
        "idempotency_key": "user-signup-12345",
        "created_at": "2026-06-20T07:20:00Z",
        "expires_at": "2026-09-18T07:20:00Z"
      }
    ],
    "total": 128,
    "page": 1,
    "limit": 20
  }
  ```

---

### 3.5 Get Notification Details (with Delivery Attempts)
Returns detailed information for a single notification, along with an audit log of all delivery attempts.

* **Endpoint**: `GET /api/v1/notifications/:id`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  ```
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the notification.
* **Response (200 OK)**:
  ```json
  {
    "notification": {
      "id": "893c5dcd-9a8c-4a31-b76b-9c7de9abcf12",
      "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
      "channel": "email",
      "priority": "high",
      "status": "delivered",
      "recipient_email": "user@example.com",
      "subject": "System Warning",
      "body": "Your account storage has exceeded 90%.",
      "metadata": {
        "user_id": "usr_99",
        "tier": "enterprise"
      },
      "created_at": "2026-06-20T07:20:00Z",
      "expires_at": "2026-09-18T07:20:00Z"
    },
    "deliveries": [
      {
        "id": "e44f8ca2-5cb1-443e-b8d9-317bfbceab1b",
        "notification_id": "893c5dcd-9a8c-4a31-b76b-9c7de9abcf12",
        "channel": "email",
        "status": "failed",
        "attempts": 1,
        "error_message": "Resend API connection timed out",
        "delivered_at": null,
        "created_at": "2026-06-20T07:20:02Z"
      },
      {
        "id": "5cbef1aa-3901-447c-ae08-bc238ef12cda",
        "notification_id": "893c5dcd-9a8c-4a31-b76b-9c7de9abcf12",
        "channel": "email",
        "status": "delivered",
        "attempts": 2,
        "delivered_at": "2026-06-20T07:20:10Z",
        "created_at": "2026-06-20T07:20:08Z"
      }
    ]
  }
  ```

---

## 🗂️ 4. Template Management APIs (Authenticated)

These CRUD endpoints define dynamic templates with reusable structures and placeholder variables.

---

### 4.1 Create Template
Create a new template. Template names must be unique within a tenant workspace.

* **Endpoint**: `POST /api/v1/templates`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  Content-Type: application/json
  ```
* **Request Body**:
  ```json
  {
    "name": "welcome_email",
    "channel": "email",
    "subject": "Welcome to Notifyx, {{name}}!",
    "body": "Hello {{name}},\n\nWe are excited to have you on board!"
  }
  ```
* **Validation Constraints**:
  * `name` (string, required): Unique template name identifier.
  * `channel` (string, required): Must be one of `email`, `push`, `sms`, or `inapp`.
  * `body` (string, required): Supports placeholders in the `{{variable_name}}` format.
  * `subject` (string, required for `email` only): Template subject. Must be omitted or empty for other channels.
* **Response (201 Created)**:
  ```json
  {
    "id": "d3b07384-d113-4c9e-bf11-739fbef53de4",
    "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
    "name": "welcome_email",
    "channel": "email",
    "subject": "Welcome to Notifyx, {{name}}!",
    "body": "Hello {{name}},\n\nWe are excited to have you on board!",
    "created_at": "2026-06-20T07:30:00Z",
    "updated_at": "2026-06-20T07:30:00Z"
  }
  ```

---

### 4.2 List Templates
Lists all templates defined for the authenticated tenant.

* **Endpoint**: `GET /api/v1/templates`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  ```
* **Response (200 OK)**:
  ```json
  {
    "templates": [
      {
        "id": "d3b07384-d113-4c9e-bf11-739fbef53de4",
        "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
        "name": "welcome_email",
        "channel": "email",
        "subject": "Welcome to Notifyx, {{name}}!",
        "body": "Hello {{name}},\n\nWe are excited to have you on board!",
        "created_at": "2026-06-20T07:30:00Z",
        "updated_at": "2026-06-20T07:30:00Z"
      }
    ]
  }
  ```

---

### 4.3 Get Template Details
Fetches detailed info for a specific template.

* **Endpoint**: `GET /api/v1/templates/:id`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  ```
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the template.
* **Response (200 OK)**:
  ```json
  {
    "id": "d3b07384-d113-4c9e-bf11-739fbef53de4",
    "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
    "name": "welcome_email",
    "channel": "email",
    "subject": "Welcome to Notifyx, {{name}}!",
    "body": "Hello {{name}},\n\nWe are excited to have you on board!",
    "created_at": "2026-06-20T07:30:00Z",
    "updated_at": "2026-06-20T07:30:00Z"
  }
  ```

---

### 4.4 Update Template
Modify the definition of an existing template.

* **Endpoint**: `PUT /api/v1/templates/:id`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  Content-Type: application/json
  ```
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the template.
* **Request Body**:
  ```json
  {
    "name": "welcome_email_updated",
    "channel": "email",
    "subject": "Welcome to the family, {{name}}!",
    "body": "Hi {{name}},\n\nWe updated our workspace!"
  }
  ```
* **Validation Constraints**:
  * Same rules as the `POST /api/v1/templates` endpoint.
* **Response (200 OK)**:
  ```json
  {
    "id": "d3b07384-d113-4c9e-bf11-739fbef53de4",
    "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
    "name": "welcome_email_updated",
    "channel": "email",
    "subject": "Welcome to the family, {{name}}!",
    "body": "Hi {{name}},\n\nWe updated our workspace!",
    "created_at": "2026-06-20T07:30:00Z",
    "updated_at": "2026-06-20T07:35:00Z"
  }
  ```

---

### 4.5 Delete Template
Permanently remove a template.

* **Endpoint**: `DELETE /api/v1/templates/:id`
* **Request Header**:
  ```http
  X-API-Key: nfx_32byte_cryptographic_random_hex_string
  ```
* **Path Parameters**:
  * `id` (string/UUID, required): The unique identifier of the template.
* **Response (204 No Content)**: Empty body.

---

## ⚡ 5. Real-Time In-App Delivery (WebSockets)

Notifyx supports direct in-app message delivery. Clients establish a persistent connection with the server via WebSockets.

---

### 5.1 Connection Establishment
Clients connect via a standard WebSocket upgrade handshake. Custom headers cannot be sent during browser handshakes, so credentials must be provided as query parameters.

* **WebSocket URL**: `ws://<host>:<port>/ws/connect`
* **Query Parameters**:
  * `tenantId` (string/UUID, required): The tenant workspace identifier.
  * `userId` (string, required): The identifier of the recipient (binds to `recipient_id`).
  * `apiKey` (string, required): The tenant's API key.
* **Example Handshake URL**:
  ```http
  GET /ws/connect?tenantId=1e7be846-9d32-4752-965a-8b839bcaef16&userId=user_abc123&apiKey=nfx_b5569cc06a88b50f757f49352dc0029b35b62b1a99d98cf0f214dbdb87e74287 HTTP/1.1
  Host: localhost:8080
  Upgrade: websocket
  Connection: Upgrade
  Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
  Sec-WebSocket-Version: 13
  ```

* **Authentication Mechanics**:
  The handler hashes the `apiKey` parameter and retrieves the matching tenant from the database. It then verifies that the tenant's ID matches the `tenantId` parameter. If validation fails, the server rejects the upgrade request with a `401 Unauthorized` status.

---

### 5.2 Real-Time Message Frame Structure
Once connected, the server pushes JSON frames to the client when an in-app message is published.

* **Message Structure**:
  ```json
  {
    "id": "a9a7a6a5-b4b3-c2c1-d0d9-e8e7e6e5e4e3",
    "tenant_id": "1e7be846-9d32-4752-965a-8b839bcaef16",
    "channel": "inapp",
    "priority": "normal",
    "recipient_id": "user_abc123",
    "body": "Welcome to your real-time feed!",
    "metadata": {
      "toast_type": "info"
    },
    "created_at": "2026-06-20T07:40:00Z"
  }
  ```

---

### 5.3 Offline Queuing & Presence
* **Presence Tracking**: When a client connects, the server marks the user as **online** in Redis. When the client disconnects, the user's presence state is deleted.
* **Offline Queuing**: If an in-app notification is sent while a user is offline, the message is queued in Redis (using a sliding-window queue with a 7-day expiration).
* **Flush on Connect**: Upon establishing a connection, the server flushes any pending offline messages from Redis and delivers them sequentially to the client.

---

## 📈 6. System Metrics & Health

Monitoring endpoints are exposed to check system health and scrape metrics.

---

### 6.1 Liveness & Readiness Check
A basic endpoint for health check probes (e.g., Render or Kubernetes).

* **Endpoint**: `GET /health`
* **Response (200 OK)**:
  ```json
  {
    "status": "ok"
  }
  ```

---

### 6.2 Prometheus Metrics
Exposes Prometheus-compatible metric gauges and histograms.

* **Endpoint**: `GET /metrics`
* **Response (200 OK)**: Returns standard Prometheus metric expositions (e.g., `notifyx_notifications_total`, request latencies, system CPU/Memory stats, and Echo middleware metrics).
