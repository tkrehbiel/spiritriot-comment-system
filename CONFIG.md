---
# SpiritRiot Comment System Configuration Specification
environment_variables:
  - name: DYNAMO_COMMENT_TABLE
    required: true
    services: [fetch-service, submit-service, page-service]
    description: "Name of the DynamoDB table storing comments data."
  - name: DYNAMO_USER_TABLE
    required: true
    services: [submit-service, page-service]
    description: "Name of the DynamoDB table storing user accounts."
  - name: DYNAMO_MASTODON_CLIENTS_TABLE
    required: true
    services: [submit-service]
    description: "Name of the DynamoDB table storing Mastodon API client registrations."
  - name: JWT_SECRET
    required: true
    services: [submit-service]
    description: "Cryptographic secret key used to sign and verify authenticated session tokens (Google, Mastodon, IndieAuth)."
  - name: GOOGLE_CLIENT_ID
    required: true
    services: [submit-service, dev-server]
    description: "Google OAuth Client ID for identity verification."
  - name: MASTODON_CLIENT_NAME
    required: true
    services: [submit-service]
    description: "The name displayed to users when authorizing Mastodon credentials."
  - name: WEBSITE_URL
    required: true
    services: [submit-service]
    description: "The main website domain for Mastodon client registrations (e.g., https://yourdomain.com)."
  - name: API_DOMAIN
    required: true
    services: [submit-service]
    description: "The root domain of your backend API endpoints (e.g., api.yourdomain.com)."
  - name: HTTP_ALLOWED_REFERRERS
    required: true
    services: [submit-service, page-service]
    description: "Comma-separated list of allowed client origins/referrers for CORS protection."
  - name: HTML_CSS
    required: true
    services: [page-service]
    description: "The URL to the main CSS stylesheet used by the no-JS comment entry page."
  - name: HTML_TITLE
    required: false
    services: [page-service]
    description: "The HTML page title for the no-JS comment entry page."

hugo_site_params:
  - key: spiritriot.commentPageURL
    required: true
    description: "The root URL of the fallback comment page (e.g., https://comments.yourdomain.com)."
  - key: spiritriot.apiURL
    required: true
    description: "The root API endpoint URL for comments submissions (e.g., https://api.yourdomain.com)."
---

# SpiritRiot Configuration Specification

This document details all environment variables and Hugo site configuration keys required to configure and run the SpiritRiot comment system.

## Environment Variables

These variables must be populated on the deployed AWS Lambda services (and in local development environments like `env.makefile` / `.env` files).

| Variable Name | Required | Services | Description |
| :--- | :--- | :--- | :--- |
| `DYNAMO_COMMENT_TABLE` | **Yes** | `fetch-service`, `submit-service`, `page-service` | The DynamoDB table containing live page comments. |
| `DYNAMO_USER_TABLE` | **Yes** | `submit-service`, `page-service` | The DynamoDB table storing users and verified identity links. |
| `DYNAMO_MASTODON_CLIENTS_TABLE` | **Yes** | `submit-service` | The DynamoDB table storing registered Mastodon client credentials. |
| `JWT_SECRET` | **Yes** | `submit-service` | The cryptographic secret used to sign session cookies/tokens for Google, Mastodon, and IndieAuth logins. |
| `GOOGLE_CLIENT_ID` | **Yes** | `submit-service` | The client OAuth ID registered with the Google Cloud Console credentials helper. |
| `MASTODON_CLIENT_NAME` | **Yes** | `submit-service` | The app name that will be registered dynamically when registering OAuth clients on user instances. |
| `WEBSITE_URL` | **Yes** | `submit-service` | The main website domain utilized in Mastodon dynamic app registrations. |
| `API_DOMAIN` | **Yes** | `submit-service` | Fallback API host domain. |
| `HTTP_ALLOWED_REFERRERS` | **Yes** | `submit-service`, `page-service` | Comma-separated list of allowed client origins. Requests with other referrers will be rejected with `403 Forbidden`. |
| `HTML_CSS` | **Yes** | `page-service` | Public URL link to the main comments CSS layout stylesheet. |
| `HTML_TITLE` | No | `page-service` | The header page title of the fallback comment entry form page. |

---

## Hugo Blog Site Parameters

These keys must be added to your Hugo blog repository's configuration (`hugo.toml`, `config.toml`, or `config.yaml`) under the `[params.spiritriot]` section:

### `spiritriot.commentPageURL`
- **Required**: **Yes**
- **Type**: String
- **Description**: The full URL pointing to the no-JS fallback comment page.
- **Example**: `https://comments.yourdomain.com`

### `spiritriot.apiURL`
- **Required**: **Yes**
- **Type**: String
- **Description**: The root URL of the SpiritRiot comment system API backend endpoints.
- **Example**: `https://api.yourdomain.com`
