# Northstar

Northstar is a full-stack AI generation SaaS with an OpenAI-compatible API,
multi-provider account pools, transactional credit billing, a user workspace,
and an operations console.

## Stack

- Vue 3 + TypeScript + Arco Design Vue
- Go + Gin + GORM
- PostgreSQL + Redis + RustFS/S3
- Docker Compose + Nginx

## Billing guarantees

Every paid generation owns one `credit_transactions` row:

1. The request reserves credits and debits the available balance in one DB transaction.
2. A successful provider + storage result changes `reserved` to `captured`.
3. Any provider, validation-after-reserve, storage, timeout, or abandoned-task failure changes `reserved` to `refunded` and restores the balance.
4. Status guards and unique event links make capture/refund idempotent.

User ledger: `GET /admin/api/billing/ledger`

Admin ledger: `GET /admin/api/billing/ledger/admin`

## Local frontend development

```powershell
cd frontend
npm install
npm run dev
```

Open `http://127.0.0.1:5174`. The frontend proxies API requests to the real
backend at `http://127.0.0.1:6666` by default.

## Full stack

宝塔面板 + Docker 的完整生产部署步骤见 [docs/DEPLOYMENT_BAOTA.md](docs/DEPLOYMENT_BAOTA.md)。

Copy `.env.example` to `.env`, replace every production secret and origin, then:

```bash
docker compose up -d --build
```

The web service binds to `127.0.0.1:2100` by default. Put your TLS reverse
proxy in front of it.

If Docker Hub is unavailable but the required runtime images already exist locally, use the host-build launcher:

```powershell
.\start-local.ps1
```

The launcher builds the frontend and Linux backend binary on the host, then packages them with the local Docker cache. The application remains available at `http://127.0.0.1:2100`.

## Admin access

The user workspace does not display a link to the operations console. Administrators enter through the protected path `http://127.0.0.1:2100/admin/overview`. Non-admin users are redirected back to the user workspace.

For a fresh production database, set `BOOTSTRAP_ADMIN_EMAIL` and
`BOOTSTRAP_ADMIN_PASSWORD` before the first start. The backend creates that
administrator once; public registration can never claim administrator access.

The configured bootstrap password must be 8-24 characters and contain upper
and lower case letters, a number, and a symbol. Remove it from the runtime
environment after the administrator has been created.

## OpenAI-compatible endpoints

- `GET /v1/models`
- `POST /v1/images/generations`
- `POST /v1/images/edits`
- `POST /v1/videos`
- `GET /v1/videos/:id`
- `GET /v1/videos/:id/content`
